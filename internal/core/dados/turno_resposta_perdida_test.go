package dados

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/core/banco/bancotest"
	okf "go-concierge/internal/core/catalogo/arquivos"
	adminpg "go-concierge/internal/core/dados/postgres/admin"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	pedidospg "go-concierge/internal/core/dados/postgres/pedidos"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	harness "go-concierge/internal/harness/agentes"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/pedidos"
)

type pedidosContados struct {
	pedidos.Store
	gravacoes atomic.Int32
}

func (p *pedidosContados) Gravar(ctx context.Context, c escopo.Contato, cart pedidos.Carteira) error {
	p.gravacoes.Add(1)
	return p.Store.Gravar(ctx, c, cart)
}

func TestTurnoHTTPRespostaPerdidaAposCommitNaoRepete(t *testing.T) {
	b := bancotest.Novo(t)
	channel := uuid.New()
	condo := 101
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,name,channel_type,provider,endpoint_identifier,external_channel_id,feature_mode,enabled,root_agent,config_jsonb,updated_at) VALUES($1,'empresa',$2,'Alpha','WHATSAPP','ATENDE','55119000','tenant','INHERIT',true,'agentes/concierge','{"nexus_account_id":"cond-001","gruvi_company_id":"gruvi"}',now())`, channel, condo); err != nil {
		t.Fatal(err)
	}
	cat, err := okf.NewCatalogo(filepath.Join("..", "..", "..", "okf"))
	if err != nil {
		t.Fatal(err)
	}
	servidor, cliente, _, err := IniciarComCanais(NovoGravadorPostgres(b), canaispg.NovoPostgres(b.Pool))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	destino := cliente.url
	var aplicadas atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, destino, bytes.NewReader(corpo))
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
		aplicadas.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	t.Cleanup(proxy.Close)
	cliente.url = proxy.URL
	store := &pedidosContados{Store: pedidospg.NovoCache(pedidospg.NovoPostgres(b))}
	out := &atende.Outbox{}
	svc := &atende.Servico{
		Canais: cliente, Cat: cat, Agente: "agentes/concierge", Mapeamento: adminpg.NovoPostgres(b.Ent),
		Sessoes: atende.NewSessoes(sessoespg.NovoSessaoPostgres(b)), Pedidos: store, Sender: out,
		Config: harness.Config{Mock: true, Env: func(string) string { return "" }}, GravacaoTurno: cliente,
	}
	msg := entrada.Mensagem{ID: "evento", ConversaID: "protocolo", ChannelID: channel, CompanyID: "empresa", Provider: "ATENDE", ChannelType: "WHATSAPP", Tenant: "tenant", Endpoint: "55119000", Contato: entrada.Contato{Phone: "5511999999999"}, Origem: entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp"}, Destino: entrada.Destino{Conector: "atende-emulado"}, Texto: "bom dia"}
	sess, err := svc.Processar(t.Context(), msg)
	if sess == nil || !errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("turno: sessão=%v erro=%v", sess != nil, err)
	}
	ctx := atende.ComEscopoSessao(t.Context(), "empresa", channel)
	if _, ok, err := sessoespg.NovoSessaoPostgres(b).Carregar(ctx, sess.ID); err != nil || !ok {
		t.Fatalf("commit: sessão=%v erro=%v", ok, err)
	}
	entregues := len(out.Itens(sess.ID))
	msg.ID = "outro-evento"
	if _, err := svc.Processar(t.Context(), msg); !errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("turno posterior = %v", err)
	}
	if err := svc.Sessoes.Persistir(ctx, sess); !errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("persistência posterior = %v", err)
	}
	if aplicadas.Load() != 1 || store.gravacoes.Load() != 0 || len(out.Itens(sess.ID)) != entregues {
		t.Fatalf("aplicações=%d fallback=%d entregas=%d", aplicadas.Load(), store.gravacoes.Load(), len(out.Itens(sess.ID)))
	}
}
