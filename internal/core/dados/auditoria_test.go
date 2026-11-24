package dados

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go-concierge/internal/core/banco/bancotest"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/ganchos"
	"go-concierge/internal/harness/ganchos/auditoria"
)

func TestAuditoriaHTTPRespeitaEscopoFalhaParcialERespostaPerdida(t *testing.T) {
	b := bancotest.Novo(t)
	r, _, _ := registroValido(t)
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, r.ChannelID, r.CompanyID, r.CondoID, r.ChannelID.String()); err != nil {
		t.Fatal(err)
	}
	pg := NovaAuditoriaPostgres(b)
	var falhas atomic.Int32
	pg.AoFalhar(func(string) { falhas.Add(1) })
	servidor, cliente, _, err := IniciarCompleto(NovoGravadorPostgres(b), canaispg.NovoPostgres(b.Pool), sessoespg.NovoSessaoPostgres(b), NovasMemoriasPostgres(b), NovosPedidosPostgres(b), pg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	linha := auditoria.Linha{CriadoEm: time.Now(), Origem: ganchos.OrigemConcierge, Evento: "mensagem_recebida", Gancho: "fixo", Decisao: "bloquear", Motivo: "motivo", AdministradoraID: r.CompanyID, CondominioID: "42", ChannelID: r.ChannelID.String()}
	contar := func(esperado int) {
		t.Helper()
		var n int
		if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM hook_audit_logs`).Scan(&n); err != nil || n != esperado {
			t.Fatalf("auditoria: %d, esperado %d, erro=%v", n, esperado, err)
		}
	}
	if err := cliente.GravarAuditoria(ctx, []auditoria.Linha{linha}); err != nil {
		t.Fatal(err)
	}
	contar(1)
	semEscopo := linha
	semEscopo.AdministradoraID, semEscopo.ChannelID, semEscopo.Gancho = "", "", "sem-escopo"
	if err := cliente.GravarAuditoria(ctx, []auditoria.Linha{semEscopo}); err != nil {
		t.Fatal(err)
	}
	var company, channel string
	if err := b.Pool.QueryRow(t.Context(), `SELECT company_id, channel_id::text FROM hook_audit_logs WHERE hook='sem-escopo'`).Scan(&company, &channel); err != nil || company != r.CompanyID || channel != r.ChannelID.String() {
		t.Fatalf("escopo derivado: empresa=%q canal=%q erro=%v", company, channel, err)
	}
	fora := linha
	fora.CondominioID = "99"
	if err := cliente.GravarAuditoria(ctx, []auditoria.Linha{linha, fora}); !errors.Is(err, atende.ErrGravacaoRecusada) || errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("condomínio cruzado: %v", err)
	}
	outroCtx := atende.ComEscopoSessao(t.Context(), "outra-empresa", r.ChannelID)
	if err := cliente.GravarAuditoria(outroCtx, []auditoria.Linha{linha}); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("empresa cruzada: %v", err)
	}
	contar(2)
	ruim := linha
	ruim.Motivo = "inválido\x00"
	if err := cliente.GravarAuditoria(ctx, []auditoria.Linha{linha, ruim}); !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) {
		t.Fatalf("falha parcial: %v", err)
	}
	contar(3)
	if falhas.Load() != 1 {
		t.Fatalf("falhas contabilizadas: %d", falhas.Load())
	}
	destino := cliente.base
	var chamadas atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, destino+r.URL.Path, bytes.NewReader(corpo))
		if err != nil {
			t.Error(err)
			return
		}
		req.Header = r.Header.Clone()
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		_, _ = io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("servidor de dados: HTTP %d", res.StatusCode)
			return
		}
		chamadas.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	t.Cleanup(proxy.Close)
	cliente.base = proxy.URL
	err = cliente.GravarAuditoria(ctx, []auditoria.Linha{linha})
	cliente.base = destino
	if !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) || chamadas.Load() != 1 {
		t.Fatalf("resposta perdida: erro=%v chamadas=%d", err, chamadas.Load())
	}
	contar(4)
}
