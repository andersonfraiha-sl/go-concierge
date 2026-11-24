package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/core/banco/bancotest"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/pedidos"
	"go-concierge/internal/identidade"
)

type sessaoComFalha struct {
	atende.SessaoStore
	err    error
	aplica bool
}

func (s sessaoComFalha) Salvar(ctx context.Context, r atende.RegistroSessao) error {
	if s.aplica {
		if err := s.SessaoStore.Salvar(ctx, r); err != nil {
			return err
		}
	}
	return s.err
}

func TestSessoesHTTPComEscopoEEstadoVazio(t *testing.T) {
	b := bancotest.Novo(t)
	r, _, _ := registroValido(t)
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, r.ChannelID, r.CompanyID, r.CondoID, r.ChannelID.String()); err != nil {
		t.Fatal(err)
	}
	store := sessoespg.NovoSessaoPostgres(b)
	servidor, cliente, _, err := IniciarComCanais(NovoGravadorPostgres(b), canaispg.NovoPostgres(b.Pool), store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	sessoes := NovasSessoes(cliente)
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	if _, achou, err := sessoes.Carregar(ctx, r.ID); err != nil || achou {
		t.Fatalf("ausente: achou=%v erro=%v", achou, err)
	}
	if err := sessoes.Salvar(ctx, r); err != nil {
		t.Fatal(err)
	}
	obtida, achou, err := sessoes.Carregar(ctx, r.ID)
	if err != nil || !achou || obtida.ID != r.ID || obtida.ChannelID != r.ChannelID {
		t.Fatalf("carregar: achou=%v erro=%v", achou, err)
	}
	negado := atende.ComEscopoSessao(t.Context(), "outra", r.ChannelID)
	if _, _, err := sessoes.Carregar(negado, r.ID); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("empresa cruzada: %v", err)
	}
	if err := sessoes.Salvar(negado, r); !errors.Is(err, atende.ErrGravacaoRecusada) || errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("gravação cruzada: %v", err)
	}
	if _, _, err := sessoes.Carregar(t.Context(), r.ID); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("sem canal: %v", err)
	}
}

func TestSessoesRoteadasReservamOperacoesAdministrativas(t *testing.T) {
	r, _, _ := registroValido(t)
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: r.ChannelID, CompanyID: r.CompanyID}}
	store := atende.NovaSessaoEmMemoria()
	gravador := gravadorFunc(func(context.Context, atende.RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		return nil
	})
	servidor, cliente, _, err := IniciarComCanais(gravador, repo, store)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	sessoes := NovasSessoes(cliente)
	if _, err := sessoes.Listar(t.Context(), r.CompanyID, 10); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("listagem sem principal: %v", err)
	}
	ctxAdmin := atende.ComEscopoSessao(identidade.ComUsuario(t.Context(), "usuario"), r.CompanyID, uuid.Nil)
	if err := sessoes.Salvar(ctxAdmin, r); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("principal sem prova: %v", err)
	}
	if err := store.Salvar(atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID), r); err != nil {
		t.Fatal(err)
	}
	ctxRuntime := atende.ComEscopoSessao(identidade.ComoRuntime(ctxAdmin), r.CompanyID, r.ChannelID)
	if _, achou, err := sessoes.Carregar(ctxRuntime, r.ID); err != nil || !achou || repo.chamadas.Load() != 1 {
		t.Fatalf("runtime: achou=%v chamadas=%d erro=%v", achou, repo.chamadas.Load(), err)
	}
}

func TestSessaoHTTPRespostaPerdidaEhIncerta(t *testing.T) {
	r, _, _ := registroValido(t)
	store := atende.NovaSessaoEmMemoria()
	var aplicadas atomic.Int32
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var p pedidoSessao
		if err := json.NewDecoder(req.Body).Decode(&p); err != nil {
			t.Error(err)
			return
		}
		ctx := atende.ComEscopoSessao(req.Context(), p.Empresa, p.Canal)
		if err := store.Salvar(ctx, *p.Registro); err != nil {
			t.Error(err)
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
	defer servidor.Close()
	cliente := &Cliente{base: servidor.URL, token: "teste", http: servidor.Client()}
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	err := cliente.salvarSessao(ctx, r)
	if !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) || aplicadas.Load() != 1 {
		t.Fatalf("resultado=%v aplicações=%d", err, aplicadas.Load())
	}
}

func TestSessaoHTTPExigeProvaDeNaoAplicacao(t *testing.T) {
	r, _, _ := registroValido(t)
	for _, tc := range []struct {
		nome    string
		err     error
		aplica  bool
		incerta bool
	}{
		{"erro após aplicação", errors.New("falha desconhecida"), true, true},
		{"não aplicada comprovada", errors.Join(atende.ErrGravacaoNaoAplicada, errors.New("rollback")), false, false},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			repo := &repositorioCanalTeste{canal: canais.Canal{ID: r.ChannelID, CompanyID: r.CompanyID}}
			base := atende.NovaSessaoEmMemoria()
			store := sessaoComFalha{SessaoStore: base, err: tc.err, aplica: tc.aplica}
			gravador := gravadorFunc(func(context.Context, atende.RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
				return nil
			})
			servidor, cliente, _, err := IniciarComCanais(gravador, repo, store)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
			ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
			err = cliente.salvarSessao(ctx, r)
			if errors.Is(err, atende.ErrGravacaoIncerta) != tc.incerta || errors.Is(err, atende.ErrGravacaoNaoAplicada) == tc.incerta {
				t.Fatalf("resultado=%v incerta=%v", err, tc.incerta)
			}
			_, achou, err := base.Carregar(ctx, r.ID)
			if err != nil || achou != tc.aplica {
				t.Fatalf("achou=%v erro=%v", achou, err)
			}
		})
	}
}
