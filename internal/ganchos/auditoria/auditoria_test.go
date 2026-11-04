package auditoria_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/ent/ganchosauditoria"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/auditoria"
)

func TestLinhasDeUsaOMotivoDoGanchoESoRegistraOQueMexeu(t *testing.T) {
	regs := []ganchos.Registro{
		{Evento: ganchos.EventoAntesDeResponder, Decisao: ganchos.Decisao{Tipo: ganchos.Deixar}},
		{Evento: ganchos.EventoMensagemRecebida, Decisao: ganchos.Decisao{
			Tipo: ganchos.Ajustar, Motivo: "dados mascarados", Quantidade: 2,
			Origem: ganchos.Origem{Gancho: "pii", Fixo: true},
		}},
		{Evento: ganchos.EventoAntesDaFerramenta, Falhou: true, Causa: ganchos.CausaPrazo, Decisao: ganchos.Decisao{
			Tipo: ganchos.Bloquear, Motivo: ganchos.MotivoDeFalha,
			Origem: ganchos.Origem{Gancho: "regra", RegraID: "r-1", RegraVersao: 3},
		}},
	}
	base := auditoria.Linha{Origem: auditoria.OrigemConcierge, AdministradoraID: "adm-a", SessaoID: "s-1"}

	got := auditoria.LinhasDe(regs, base)

	if len(got) != 2 {
		t.Fatalf("linhas = %d, quer 2 (Deixar sem falha não grava)", len(got))
	}
	if g := got[0]; g.Gancho != "pii" || g.Decisao != "ajustar" || g.Motivo != "dados mascarados" ||
		g.Quantidade != 2 || g.Evento != "mensagem_recebida" || g.AdministradoraID != "adm-a" || g.SessaoID != "s-1" {
		t.Errorf("linha 0 = %+v", g)
	}
	if g := got[1]; g.Decisao != "falhou" || g.RegraID != "r-1" || g.RegraVersao != 3 || g.Motivo != ganchos.MotivoDeFalha {
		t.Errorf("linha 1 = %+v", g)
	}
}

func linha(gancho string, quando time.Time) auditoria.Linha {
	return auditoria.Linha{
		CriadoEm: quando, Origem: auditoria.OrigemConcierge, Evento: "mensagem_recebida", Gancho: gancho,
		Decisao: "bloquear", Motivo: "motivo", AdministradoraID: "adm-a", Quantidade: 1,
	}
}

func contar(t *testing.T, b banco.Banco) int {
	t.Helper()
	n, err := b.Ent.GanchosAuditoria.Query().Count(t.Context())
	if err != nil {
		t.Fatalf("contar: %v", err)
	}
	return n
}

func TestNoLoteGravaNLinhasNumaIda(t *testing.T) {
	b := bancotest.Novo(t)
	agora := time.Now()
	linhas := []auditoria.Linha{linha("a", agora), linha("b", agora), linha("c", agora)}

	lote := banco.NovoLote()
	auditoria.NoLote(lote, linhas)
	if err := lote.Enviar(t.Context(), b.Pool); err != nil {
		t.Fatalf("enviar: %v", err)
	}

	if n := contar(t, b); n != 3 {
		t.Fatalf("gravadas = %d, quer 3", n)
	}
	gravada, err := b.Ent.GanchosAuditoria.Query().Where(ganchosauditoria.Gancho("b")).Only(t.Context())
	if err != nil || gravada.Decisao != "bloquear" || gravada.Quantidade != 1 || gravada.AdministradoraID != "adm-a" {
		t.Errorf("linha b = %+v, err %v", gravada, err)
	}
}

func TestGravarComLinhaInvalidaGravaAsOutrasELogaErro(t *testing.T) {
	b := bancotest.Novo(t)
	var saida bytes.Buffer
	antes := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&saida, nil)))
	t.Cleanup(func() { slog.SetDefault(antes) })
	agora := time.Now()
	ruim := linha("ruim", agora)
	ruim.Motivo = "nul\x00no texto" // o Postgres recusa NUL em text
	var falhas atomic.Int32

	err := auditoria.Gravar(context.Background(), b, []auditoria.Linha{linha("a", agora), ruim, linha("c", agora)},
		func(origem string) {
			if origem == auditoria.OrigemConcierge {
				falhas.Add(1)
			}
		})

	if err == nil {
		t.Error("Gravar não devolveu o erro da linha perdida")
	}
	if n := contar(t, b); n != 2 {
		t.Errorf("gravadas = %d, quer 2", n)
	}
	if falhas.Load() != 1 {
		t.Errorf("aoFalhar chamado %d vezes, quer 1", falhas.Load())
	}
	if log := saida.String(); !strings.Contains(log, "level=ERROR") || !strings.Contains(log, "gancho=ruim") {
		t.Errorf("log sem ERROR da linha ruim: %s", log)
	}
}

func TestApagarRemoveSoOQuePassouDe12Meses(t *testing.T) {
	b := bancotest.Novo(t)
	agora := time.Now()
	velha := linha("velha", agora.Add(-auditoria.Retencao-time.Hour))
	nova := linha("nova", agora.Add(-auditoria.Retencao+time.Hour))
	if err := auditoria.Gravar(t.Context(), b, []auditoria.Linha{velha, nova}, nil); err != nil {
		t.Fatalf("gravar: %v", err)
	}

	n, err := auditoria.Apagar(t.Context(), b.Ent, agora)

	if err != nil || n != 1 {
		t.Fatalf("apagadas = %d, err %v; quer 1", n, err)
	}
	restou, err := b.Ent.GanchosAuditoria.Query().Only(t.Context())
	if err != nil || restou.Gancho != "nova" {
		t.Errorf("restou %+v, err %v", restou, err)
	}
}

func TestReterParaComOContexto(t *testing.T) {
	b := bancotest.Novo(t)
	ctx, cancel := context.WithCancel(t.Context())
	feito := make(chan struct{})
	go func() { auditoria.Reter(ctx, b.Ent, time.Now); close(feito) }()

	cancel()

	select {
	case <-feito:
	case <-time.After(5 * time.Second):
		t.Fatal("Reter não parou com o ctx cancelado")
	}
}
