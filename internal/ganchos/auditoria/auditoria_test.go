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
			Tipo: ganchos.Ajustar, Origem: ganchos.Origem{Gancho: "escopo_da_conversa", Fixo: true},
		}},
		{Evento: ganchos.EventoMensagemRecebida, Decisao: ganchos.Decisao{
			Tipo: ganchos.Ajustar, Motivo: "dados mascarados", Quantidade: 2,
			Origem: ganchos.Origem{Gancho: "pii", Fixo: true},
		}},
		{Evento: ganchos.EventoAntesDaFerramenta, Falhou: true, Causa: ganchos.CausaPrazo, Decisao: ganchos.Decisao{
			Tipo: ganchos.Bloquear, Motivo: ganchos.MotivoDeFalha,
			Origem: ganchos.Origem{Gancho: "regra", RegraID: "r-1", RegraVersao: 3},
		}},
	}
	base := auditoria.Linha{Origem: ganchos.OrigemConcierge, AdministradoraID: "adm-a", SessaoID: "s-1"}

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
		CriadoEm: quando, Origem: ganchos.OrigemConcierge, Evento: "mensagem_recebida", Gancho: gancho,
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
			if origem == ganchos.OrigemConcierge {
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

func TestNoLotePulaLinhaSemDataELogaErro(t *testing.T) {
	b := bancotest.Novo(t)
	var saida bytes.Buffer
	antes := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&saida, nil)))
	t.Cleanup(func() { slog.SetDefault(antes) })

	lote := banco.NovoLote()
	auditoria.NoLote(lote, []auditoria.Linha{linha("boa", time.Now()), linha("sem-data", time.Time{})})
	if err := lote.Enviar(t.Context(), b.Pool); err != nil {
		t.Fatalf("enviar: %v", err)
	}

	if n := contar(t, b); n != 1 {
		t.Errorf("gravadas = %d, quer 1", n)
	}
	if log := saida.String(); !strings.Contains(log, "level=ERROR") || !strings.Contains(log, "gancho=sem-data") {
		t.Errorf("log sem ERROR da linha sem data: %s", log)
	}
}

func TestGravarRecusaLinhaSemDataEGravaNada(t *testing.T) {
	b := bancotest.Novo(t)

	err := auditoria.Gravar(t.Context(), b, []auditoria.Linha{linha("boa", time.Now()), linha("sem-data", time.Time{})}, nil)

	if err == nil {
		t.Error("Gravar aceitou linha sem criado_em")
	}
	if n := contar(t, b); n != 0 {
		t.Errorf("gravadas = %d, quer 0", n)
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

func TestApagarEmVariosLotesRemoveTudoQueVenceu(t *testing.T) {
	b := bancotest.Novo(t)
	agora := time.Now()
	velha := agora.Add(-auditoria.Retencao - time.Hour)
	linhas := []auditoria.Linha{linha("1", velha), linha("2", velha), linha("3", velha), linha("4", velha),
		linha("5", velha), linha("nova", agora)}
	if err := auditoria.Gravar(t.Context(), b, linhas, nil); err != nil {
		t.Fatalf("gravar: %v", err)
	}

	n, err := auditoria.ApagarEmLotes(t.Context(), b.Ent, agora, 2)

	if err != nil || n != 5 {
		t.Fatalf("apagadas = %d, err %v; quer 5", n, err)
	}
	if restam := contar(t, b); restam != 1 {
		t.Errorf("restam %d, quer 1", restam)
	}
}

func TestLinhasDeUsaOMotivoDeAuditoria(t *testing.T) {
	regs := []ganchos.Registro{{Decisao: ganchos.Decisao{
		Tipo: ganchos.Bloquear, Motivo: `a mensagem atual ("meu cpf 123.456.789-09") não é um sim`,
		MotivoAuditoria: "escrita sem confirmação explícita do morador",
	}}}
	linhas := auditoria.LinhasDe(regs, auditoria.Linha{CriadoEm: time.Now()})
	if len(linhas) != 1 || linhas[0].Motivo != "escrita sem confirmação explícita do morador" {
		t.Fatalf("linhas = %+v, want o motivo de auditoria, sem o texto do morador", linhas)
	}
}
