package dados

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/acesso"
	"go-concierge/internal/core/banco/bancotest"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/identidade"
)

type acessoSessoesAdminTeste struct{ revogado atomic.Bool }

func (a *acessoSessoesAdminTeste) Carregar(_ context.Context, usuario string, imp escopo.Implantacao) (acesso.Acesso, error) {
	r := acesso.Acesso{UsuarioID: usuario, Implantacao: imp, Papeis: map[string]acesso.Papel{
		"operador": {ID: "operador", Permissoes: []acesso.Permissao{acesso.ConversasVer, acesso.ConversasAtender, acesso.MemoriaVer, acesso.MemoriaEsquecer, acesso.PedidosVer, acesso.PedidosEditar}},
	}}
	if !a.revogado.Load() {
		r.Atribuicoes = []acesso.Atribuicao{{PapelID: "operador", Escopo: acesso.EscopoCondominio("empresa", "42"), AdministradoraID: "empresa"}}
	}
	return r, nil
}

func TestSessoesAdminHTTPReaplicamPermissoesNoEstadoAtualENovo(t *testing.T) {
	b := bancotest.Novo(t)
	r, _, _ := registroValido(t)
	outra := r
	outra.ID, outra.ChannelID = "outra-sessao", uuid.New()
	condo := 43
	outra.CondoID = &condo
	var snapshot map[string]any
	if err := json.Unmarshal(r.Dados, &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot["id"], snapshot["channel_id"], snapshot["condo_id"] = outra.ID, outra.ChannelID, condo
	snapshot["escopo"] = escopo.Contato{CompanyID: outra.CompanyID, CondoID: outra.CondoID, Telefone: outra.UserPhone}
	var err error
	outra.Dados, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	store := sessoespg.NovoSessaoPostgres(b)
	for _, reg := range []atende.RegistroSessao{r, outra} {
		if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, reg.ChannelID, reg.CompanyID, reg.CondoID, reg.ChannelID.String()); err != nil {
			t.Fatal(err)
		}
		if err := store.Salvar(atende.ComEscopoSessao(t.Context(), reg.CompanyID, reg.ChannelID), reg); err != nil {
			t.Fatal(err)
		}
	}
	autoridade, err := identidade.NovaAutoridade()
	if err != nil {
		t.Fatal(err)
	}
	acessos := &acessoSessoesAdminTeste{}
	servidor, cliente, _, err := IniciarCompleto(NovoGravadorPostgres(b), canaispg.NovoPostgres(b.Pool), store, memoriaIndisponivel{}, pedidosIndisponiveis{}, auditoriaTeste{}, NovosAcessosPrivados(autoridade.Verificador(), acessos, escopo.Implantacao{}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	prova, err := autoridade.Emitir("operador")
	if err != nil {
		t.Fatal(err)
	}
	ctx := atende.ComEscopoSessao(identidade.ComProva(identidade.ComUsuario(t.Context(), "operador"), prova), r.CompanyID, uuid.Nil)
	leitura := identidade.ComPermissao(ctx, string(acesso.ConversasVer))
	escrita := identidade.ComPermissao(ctx, string(acesso.ConversasAtender))
	sessoes := NovasSessoes(cliente)
	if reg, achou, err := sessoes.Carregar(leitura, r.ID); err != nil || !achou || reg.ID != r.ID {
		t.Fatalf("leitura autorizada: achou=%v erro=%v", achou, err)
	}
	if _, achou, err := sessoes.Carregar(leitura, outra.ID); err != nil || achou {
		t.Fatalf("outro condomínio exposto: achou=%v erro=%v", achou, err)
	}
	if _, err := sessoes.Listar(leitura, r.CompanyID, 10); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("listagem ampliou atribuição de condomínio: %v", err)
	}
	if err := sessoes.Salvar(escrita, r); err != nil {
		t.Fatalf("gravação autorizada: %v", err)
	}
	if err := sessoes.Salvar(escrita, outra); !errors.Is(err, atende.ErrGravacaoRecusada) || errors.Is(err, atende.ErrGravacaoIncerta) {
		t.Fatalf("gravação de outro condomínio: %v", err)
	}
	invadida := r
	invadida.ID = outra.ID
	if err := json.Unmarshal(r.Dados, &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot["id"] = outra.ID
	invadida.Dados, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := sessoes.Salvar(escrita, invadida); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("registro atual de outro condomínio sobrescrito: %v", err)
	}
	if err := sessoes.Remover(escrita, outra.ID); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("registro de outro condomínio removido: %v", err)
	}
	acessos.revogado.Store(true)
	if _, achou, err := sessoes.Carregar(leitura, r.ID); err != nil || achou {
		t.Fatalf("revogação ignorada na leitura: achou=%v erro=%v", achou, err)
	}
	if err := sessoes.Salvar(escrita, r); !errors.Is(err, atende.ErrGravacaoRecusada) {
		t.Fatalf("revogação ignorada na escrita: %v", err)
	}
	acessos.revogado.Store(false)
	if err := sessoes.Remover(escrita, r.ID); err != nil {
		t.Fatalf("remoção autorizada: %v", err)
	}
	if _, achou, err := store.Carregar(atende.ComEscopoSessao(t.Context(), r.CompanyID, outra.ChannelID), outra.ID); err != nil || !achou {
		t.Fatalf("registro protegido alterado: achou=%v erro=%v", achou, err)
	}
}
