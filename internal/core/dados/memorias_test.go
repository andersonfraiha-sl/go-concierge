package dados

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/core/banco/bancotest"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/memoria"
	"go-concierge/internal/harness/pedidos"
	"go-concierge/internal/identidade"
)

func TestMemoriasHTTPRespeitamCanalERecortes(t *testing.T) {
	b := bancotest.Novo(t)
	r, contato, _ := registroValido(t)
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, r.ChannelID, r.CompanyID, r.CondoID, r.ChannelID.String()); err != nil {
		t.Fatal(err)
	}
	servidor, cliente, _, err := IniciarComDados(NovoGravadorPostgres(b), canaispg.NovoPostgres(b.Pool), sessoespg.NovoSessaoPostgres(b), NovasMemoriasPostgres(b))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	store := NovasMemorias(cliente)
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	m, err := store.Ler(ctx, contato)
	if err != nil || m.Fatos == nil || len(m.Fatos) != 0 || m.CompanyID != contato.CompanyID {
		t.Fatalf("memória vazia: %+v erro=%v", m, err)
	}
	m.Fatos = []memoria.Fato{{Chave: "preferencia", Texto: "primeiro", CondoID: contato.CondoID, Quando: time.Now(), Confianca: 1}}
	if err := store.Gravar(ctx, contato, m); err != nil {
		t.Fatal(err)
	}
	gravada, err := store.Ler(ctx, contato)
	if err != nil || len(gravada.Fatos) != 1 || gravada.Fatos[0].Texto != "primeiro" {
		t.Fatalf("memória gravada: %+v erro=%v", gravada, err)
	}
	outro := contato
	beta := 99
	outro.CondoID = &beta
	if _, err := store.Ler(ctx, outro); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("condomínio cruzado: %v", err)
	}
	if err := store.Gravar(ctx, outro, m); !errors.Is(err, atende.ErrGravacaoRecusada) || errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("gravação fora do canal: %v", err)
	}
	invalida := m
	invalida.Fatos = []memoria.Fato{{Chave: "", Texto: "inválida", CondoID: contato.CondoID}}
	if err := store.Gravar(ctx, contato, invalida); !errors.Is(err, atende.ErrGravacaoNaoAplicada) || errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("gravação inválida: %v", err)
	}
	gravada, err = store.Ler(ctx, contato)
	if err != nil || len(gravada.Fatos) != 1 || gravada.Fatos[0].Texto != "primeiro" {
		t.Fatalf("rollback: %+v erro=%v", gravada, err)
	}
	if _, err := store.Listar(ctx, r.CompanyID); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("listagem runtime: %v", err)
	}
	ctxAdmin := identidade.ComUsuario(t.Context(), "gestor")
	if _, err := store.Listar(ctxAdmin, r.CompanyID); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("principal administrativo sem prova: %v", err)
	}
}

func TestCanalFicaEstavelDuranteTransacaoDeMemoria(t *testing.T) {
	b := bancotest.Novo(t)
	r, contato, _ := registroValido(t)
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, r.ChannelID, r.CompanyID, r.CondoID, r.ChannelID.String()); err != nil {
		t.Fatal(err)
	}
	tx, err := b.Pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err := validarContatoNoCanal(t.Context(), tx, r.CompanyID, r.ChannelID, contato); err != nil {
		t.Fatal(err)
	}
	iniciou := make(chan struct{})
	fim := make(chan error, 1)
	go func() {
		close(iniciou)
		ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelar()
		_, err := b.Pool.Exec(ctx, `UPDATE channels SET condo_id=99 WHERE id=$1`, r.ChannelID)
		fim <- err
	}()
	<-iniciou
	select {
	case err := <-fim:
		t.Fatalf("canal alterado antes do fim da transação: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Rollback(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := <-fim; err != nil {
		t.Fatal(err)
	}
}

type memoriaIndisponivel struct{}

func (memoriaIndisponivel) Ler(context.Context, string, uuid.UUID, escopo.Contato) (memoria.Memoria, error) {
	return memoria.Memoria{}, errors.New("falha de infraestrutura")
}

func (memoriaIndisponivel) Gravar(context.Context, string, uuid.UUID, escopo.Contato, memoria.Memoria) error {
	return errors.New("resultado desconhecido")
}

func TestMemoriaHTTPNaoConfundeFalhaComVazioOuRollback(t *testing.T) {
	r, contato, _ := registroValido(t)
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: r.ChannelID, CompanyID: r.CompanyID}}
	sessoes := atende.NovaSessaoEmMemoria()
	gravador := gravadorFunc(func(context.Context, atende.RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error {
		return nil
	})
	servidor, cliente, _, err := IniciarComDados(gravador, repo, sessoes, memoriaIndisponivel{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	if _, err := cliente.lerMemoria(ctx, contato); !errors.Is(err, ErrDadosIndisponiveis) {
		t.Fatalf("leitura = %v", err)
	}
	err = cliente.gravarMemoria(ctx, contato, memoria.Memoria{CompanyID: contato.CompanyID, Telefone: contato.Telefone})
	if !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) {
		t.Fatalf("gravação = %v", err)
	}
}
