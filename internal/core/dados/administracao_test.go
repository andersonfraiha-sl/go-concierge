package dados

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"go-concierge/internal/acesso"
	"go-concierge/internal/core/banco/bancotest"
	adminpg "go-concierge/internal/core/dados/postgres/admin"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	admin "go-concierge/internal/harness/configuracao"
	"go-concierge/internal/identidade"
)

type acessoAdministracaoTeste struct{ revogado atomic.Bool }

func (a *acessoAdministracaoTeste) Carregar(_ context.Context, usuario string, imp escopo.Implantacao) (acesso.Acesso, error) {
	permissoes := []acesso.Permissao{acesso.SkillsLer, acesso.CondominiosHabilitar}
	if usuario == "sem-leitura" {
		permissoes = []acesso.Permissao{acesso.ConversasVer}
	}
	if usuario == "auditor" {
		permissoes = []acesso.Permissao{acesso.GanchosHistorico}
	}
	if a.revogado.Load() {
		permissoes = nil
	}
	return acesso.Acesso{UsuarioID: usuario, Implantacao: imp, Papeis: map[string]acesso.Papel{"teste": {ID: "teste", Permissoes: permissoes}}, Atribuicoes: []acesso.Atribuicao{{PapelID: "teste", Escopo: acesso.EscopoCondominio("empresa", "42"), AdministradoraID: "empresa"}}}, nil
}

func TestAdministracaoHTTPReaplicaPermissaoEspecificaEscopoERevogacao(t *testing.T) {
	b := bancotest.Novo(t)
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES('10000000-0000-0000-0000-000000000001','empresa',42,'WEB','test','a','INHERIT',true,'root',now()),('10000000-0000-0000-0000-000000000002','empresa',43,'WEB','test','b','INHERIT',true,'root',now()),('10000000-0000-0000-0000-000000000003','outra',42,'WEB','test','c','INHERIT',true,'root',now())`); err != nil {
		t.Fatal(err)
	}
	autoridade, err := identidade.NovaAutoridade()
	if err != nil {
		t.Fatal(err)
	}
	acessos := &acessoAdministracaoTeste{}
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Acessos: NovosAcessosPrivados(autoridade.Verificador(), acessos, escopo.Implantacao{}), Administracao: adminpg.NovoPostgres(b.Ent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctxDe := func(usuario string) context.Context {
		prova, e := autoridade.Emitir(usuario)
		if e != nil {
			t.Fatal(e)
		}
		return identidade.ComProva(t.Context(), prova)
	}
	ctx := ctxDe("gestor")
	api := NovaAdministracao(cliente)
	m, err := api.Ler(ctx)
	if err != nil || len(m.Administradoras) != 1 || m.Administradoras[0].ID != "empresa" || len(m.Condominios) != 1 || m.Condominios[0].ID != "42" || len(m.Payloads) != 0 || len(m.Orientacoes) != 0 {
		t.Fatalf("metadados fora do escopo: %+v erro=%v", m, err)
	}
	for _, usuario := range []string{"sem-leitura", "auditor"} {
		if _, err := api.Ler(ctxDe(usuario)); !errors.Is(err, atende.ErrEscopoSessao) {
			t.Fatalf("%s ganhou SkillsLer: %v", usuario, err)
		}
	}
	if m, err := api.Ler(identidade.ComPermissao(ctxDe("auditor"), string(acesso.GanchosHistorico))); err != nil || len(m.Condominios) != 1 {
		t.Fatalf("metadados do histórico: %+v %v", m, err)
	}
	alvo := admin.Alvo{Dona: "empresa", Condominio: "42"}
	if _, err := api.SalvarOrientacao(ctx, alvo, admin.Orientacao{Geral: "permitida"}); err != nil {
		t.Fatal(err)
	}
	condo := 42
	if m, err := api.ConteudoConfigurado(ctx, "empresa", &condo); err != nil || m.Orientacoes["42"].Geral != "permitida" {
		t.Fatalf("conteúdo: %+v %v", m, err)
	}
	for _, fora := range []admin.Alvo{{Dona: "empresa", Condominio: "43"}, {Dona: "outra", Condominio: "42"}, {Condominio: admin.Padrao}} {
		if _, err := api.SalvarOrientacao(ctx, fora, admin.Orientacao{Geral: "indevida"}); !errors.Is(err, atende.ErrEscopoSessao) || errors.Is(err, atende.ErrGravacaoIncerta) {
			t.Fatalf("escrita fora do escopo: %+v %v", fora, err)
		}
	}
	if _, err := api.SalvarPayload(ctx, alvo, "recurso", map[string]any{"segredo": "indevido"}); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("CondominiosHabilitar ganhou FerramentasEditar: %v", err)
	}
	acessos.revogado.Store(true)
	if _, err := api.Ler(ctx); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("leitura revogada: %v", err)
	}
	if _, err := api.SalvarOrientacao(ctx, alvo, admin.Orientacao{Geral: "revogada"}); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("escrita revogada: %v", err)
	}
	acessos.revogado.Store(false)
	perdida := &perderRespostaAuditoria{destino: cliente.http.Transport}
	cliente.http.Transport = perdida
	_, err = api.SalvarOrientacao(ctx, alvo, admin.Orientacao{Geral: "commit confirmado no banco"})
	cliente.http.Transport = perdida.destino
	if !errors.Is(err, atende.ErrGravacaoIncerta) || perdida.chamadas != 1 {
		t.Fatalf("perda de resposta: %v chamadas=%d", err, perdida.chamadas)
	}
	if m, err := api.ConteudoConfigurado(ctx, "empresa", &condo); err != nil || m.Orientacoes["42"].Geral != "commit confirmado no banco" {
		t.Fatalf("commit perdido: %+v %v", m, err)
	}
}
