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

	"github.com/google/uuid"

	"go-concierge/internal/core/banco/bancotest"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/harness/pedidos"
)

func TestPedidosHTTPTransacaoEscopoEColisao(t *testing.T) {
	b := bancotest.Novo(t)
	r, contato, cart := registroValido(t)
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, r.ChannelID, r.CompanyID, r.CondoID, r.ChannelID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO catalog_files(path,content,sha256,updated_at) VALUES('agentes/test/caso.yaml','','test',now()); INSERT INTO use_cases(id,file_path,agent_path,name,summary,visibility,source_sha256,updated_at) VALUES('reserva','agentes/test/caso.yaml','agentes/test','Reserva','','PUBLIC','test',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO company_use_cases(id,company_id,use_case_id,enabled,default_available,updated_at) VALUES($1,$2,'reserva',true,true,now())`, uuid.New(), r.CompanyID); err != nil {
		t.Fatal(err)
	}
	servidor, cliente, _, err := IniciarComDados(NovoGravadorPostgres(b), canaispg.NovoPostgres(b.Pool), sessoespg.NovoSessaoPostgres(b), NovasMemoriasPostgres(b), NovosPedidosPostgres(b))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	store := NovosPedidos(cliente)
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	vazia, err := store.Ler(ctx, contato)
	if err != nil || len(vazia.Pedidos) != 0 || vazia.CompanyID != contato.CompanyID || vazia.Telefone != contato.Telefone {
		t.Fatalf("carteira vazia: %+v erro=%v", vazia, err)
	}
	agora := time.Now()
	cart.Pedidos = []*pedidos.Pedido{{Protocolo: "P-COLISAO", Caso: "reserva", Estado: pedidos.Coletando, CondoID: contato.CondoID, CriadoEm: agora, AtualizadoEm: agora}}
	if err := store.Gravar(ctx, contato, cart); err != nil {
		t.Fatal(err)
	}
	outro := contato
	outro.Telefone = "5511888888888"
	segunda := pedidos.Carteira{CompanyID: outro.CompanyID, Telefone: outro.Telefone, Pedidos: []*pedidos.Pedido{{Protocolo: "P-COLISAO", Caso: "reserva", Estado: pedidos.Coletando, CondoID: outro.CondoID, CriadoEm: agora, AtualizadoEm: agora}}}
	if err := store.Gravar(ctx, outro, segunda); err != nil {
		t.Fatal(err)
	}
	if segunda.Pedidos[0].Protocolo == "P-COLISAO" || segunda.Pedidos[0].Protocolo == "" {
		t.Fatalf("colisão não resolvida: %+v", segunda.Pedidos[0])
	}
	lida, err := store.Ler(ctx, outro)
	if err != nil || len(lida.Pedidos) != 1 || lida.Pedidos[0].Protocolo != segunda.Pedidos[0].Protocolo {
		t.Fatalf("protocolo gravado: %+v erro=%v", lida, err)
	}
	invalida := segunda
	p := *segunda.Pedidos[0]
	p.Caso = "nao-ativado"
	invalida.Pedidos = []*pedidos.Pedido{&p}
	if err := store.Gravar(ctx, outro, invalida); !errors.Is(err, atende.ErrGravacaoNaoAplicada) || errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("gravação inválida: %v", err)
	}
	lida, err = store.Ler(ctx, outro)
	if err != nil || len(lida.Pedidos) != 1 || lida.Pedidos[0].Caso != "reserva" {
		t.Fatalf("rollback: %+v erro=%v", lida, err)
	}
	fora := outro
	condo := 99
	fora.CondoID = &condo
	if _, err := store.Ler(ctx, fora); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("leitura fora do escopo: %v", err)
	}
	if err := store.Gravar(ctx, fora, pedidos.Carteira{}); !errors.Is(err, atende.ErrGravacaoRecusada) || errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("gravação fora do escopo: %v", err)
	}
	destino := cliente.base
	var respostasPerdidas atomic.Int32
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
		respostasPerdidas.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	t.Cleanup(proxy.Close)
	cliente.base = proxy.URL
	err = store.Gravar(ctx, outro, pedidos.Carteira{})
	cliente.base = destino
	if !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) || respostasPerdidas.Load() != 1 {
		t.Fatalf("resposta perdida: erro=%v chamadas=%d", err, respostasPerdidas.Load())
	}
	lida, err = store.Ler(ctx, outro)
	if err != nil || len(lida.Pedidos) != 0 {
		t.Fatalf("substituição vazia: %+v erro=%v", lida, err)
	}
	lida, err = store.Ler(ctx, contato)
	if err != nil || len(lida.Pedidos) != 1 || lida.Pedidos[0].Protocolo != "P-COLISAO" {
		t.Fatalf("outro contato alterado: %+v erro=%v", lida, err)
	}
}

type pedidosIndisponiveis struct{}

func (pedidosIndisponiveis) Ler(context.Context, string, uuid.UUID, escopo.Contato) (pedidos.Carteira, error) {
	return pedidos.Carteira{}, errors.New("falha de infraestrutura")
}

func (pedidosIndisponiveis) Gravar(context.Context, string, uuid.UUID, escopo.Contato, pedidos.Carteira) ([]TrocaProtocolo, error) {
	return nil, errors.New("resultado desconhecido")
}

func TestPedidosHTTPNaoConfundemFalhaComVazioOuRollback(t *testing.T) {
	r, contato, _ := registroValido(t)
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: r.ChannelID, CompanyID: r.CompanyID}}
	servidor, cliente, _, err := IniciarComDados(gravadorFunc(nil), repo, atende.NovaSessaoEmMemoria(), memoriaIndisponivel{}, pedidosIndisponiveis{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	if _, err := cliente.lerPedidos(ctx, contato); !errors.Is(err, ErrDadosIndisponiveis) {
		t.Fatalf("leitura = %v", err)
	}
	if err := cliente.gravarPedidos(ctx, contato, pedidos.Carteira{}); !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) {
		t.Fatalf("gravação = %v", err)
	}
}

type pedidosAplicados struct{ chamadas atomic.Int32 }

func (p *pedidosAplicados) Ler(context.Context, string, uuid.UUID, escopo.Contato) (pedidos.Carteira, error) {
	return pedidos.Carteira{}, nil
}

func (p *pedidosAplicados) Gravar(context.Context, string, uuid.UUID, escopo.Contato, pedidos.Carteira) ([]TrocaProtocolo, error) {
	p.chamadas.Add(1)
	return []TrocaProtocolo{}, nil
}

func TestPedidosHTTPRespostaPerdidaNaoRepete(t *testing.T) {
	r, contato, _ := registroValido(t)
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: r.ChannelID, CompanyID: r.CompanyID}}
	store := &pedidosAplicados{}
	servidor, cliente, _, err := IniciarComDados(gravadorFunc(nil), repo, atende.NovaSessaoEmMemoria(), memoriaIndisponivel{}, store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	destino := cliente.base
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
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = conn.Close()
	}))
	t.Cleanup(proxy.Close)
	cliente.base = proxy.URL
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	err = cliente.gravarPedidos(ctx, contato, pedidos.Carteira{})
	if !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) || store.chamadas.Load() != 1 {
		t.Fatalf("erro=%v chamadas=%d", err, store.chamadas.Load())
	}
}
