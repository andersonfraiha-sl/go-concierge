package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"go-concierge/internal/acesso"
	"go-concierge/internal/core/administracao/regras"
	"go-concierge/internal/core/banco/bancotest"
	auditoriapg "go-concierge/internal/core/dados/postgres/auditoria"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/ganchos"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/identidade"
)

type acessoRegrasTeste struct{ acessoAdministracaoTeste }

func (a *acessoRegrasTeste) Carregar(ctx context.Context, usuario string, imp escopo.Implantacao) (acesso.Acesso, error) {
	if usuario != "global" || a.revogado.Load() {
		return a.acessoAdministracaoTeste.Carregar(ctx, usuario, imp)
	}
	return acesso.Acesso{UsuarioID: usuario, Implantacao: imp, Papeis: map[string]acesso.Papel{"editor": {ID: "editor", Permissoes: []acesso.Permissao{acesso.GanchosEditar, acesso.GanchosHistorico}}}, Atribuicoes: []acesso.Atribuicao{{PapelID: "editor", Escopo: acesso.Escopo{Tipo: acesso.TipoSuperlogica}}}}, nil
}

func TestRegrasEConsultasAdminHTTPPermissoesVersaoRevogacaoEIncerteza(t *testing.T) {
	b := bancotest.Novo(t)
	autoridade, err := identidade.NovaAutoridade()
	if err != nil {
		t.Fatal(err)
	}
	acessos := &acessoRegrasTeste{}
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Acessos: NovosAcessosPrivados(autoridade.Verificador(), acessos, escopo.Implantacao{}), Regras: regras.NovoPostgres(b.Ent), Consultas: NovasConsultasPostgres(b, nil)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	ctxDe := func(usuario string) context.Context {
		p, e := autoridade.Emitir(usuario)
		if e != nil {
			t.Fatal(e)
		}
		return identidade.ComProva(t.Context(), p)
	}
	global, auditor := ctxDe("global"), ctxDe("auditor")
	api := NovasRegrasAdministrativas(cliente)
	r := regras.RegraGravada{ID: uuid.NewString(), Nome: "teste HTTP", Evento: ganchos.EventoMensagemRecebida, AdministradoraID: "empresa", Condominios: []string{"42"}, Ativa: true, AlteradaPor: "não confiar"}
	r.Definicao, err = json.Marshal(map[string]any{"nome": r.Nome, "evento": r.Evento, "alvo": regras.Alvo{Administradora: r.AdministradoraID, Condominios: r.Condominios}, "ativa": true})
	if err != nil {
		t.Fatal(err)
	}
	salva, err := api.Salvar(global, r, 0)
	if err != nil || salva.Versao != 1 || salva.AlteradaPor != "global" {
		t.Fatalf("criação: %+v %v", salva, err)
	}
	if _, err := api.Salvar(ctxDe("gestor"), r, 1); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("gestor ganhou edição de regras: %v", err)
	}
	if _, err := api.Ler(auditor, r.ID); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("histórico ganhou leitura administrativa: %v", err)
	}
	if v, err := api.LerVersao(auditor, r.ID, 1); err != nil || v.Versao != 1 {
		t.Fatalf("versão autorizada: %+v %v", v, err)
	}
	if _, err := api.Salvar(global, r, 0); !errors.Is(err, regras.ErrVersaoVelha) {
		t.Fatalf("versão velha: %v", err)
	}
	perdida := &perderRespostaAuditoria{destino: cliente.http.Transport}
	cliente.http.Transport = perdida
	_, err = api.Salvar(global, r, 1)
	cliente.http.Transport = perdida.destino
	if !errors.Is(err, atende.ErrGravacaoIncerta) || perdida.chamadas != 1 {
		t.Fatalf("resposta perdida: %v chamadas=%d", err, perdida.chamadas)
	}
	if v, err := api.Ler(global, r.ID); err != nil || v.Versao != 2 {
		t.Fatalf("commit perdido: %+v %v", v, err)
	}
	agora := time.Now().UTC().Add(-time.Minute)
	linhas := []auditoria.Linha{}
	for _, condo := range []string{"42", "43"} {
		linhas = append(linhas, auditoria.Linha{CriadoEm: agora, Origem: ganchos.OrigemConcierge, Evento: string(ganchos.EventoAntesDeResponder), Gancho: "teste", Decisao: "bloquear", Motivo: "teste", AdministradoraID: "empresa", CondominioID: condo, Protocolo: "não divulgar"})
	}
	if err := auditoriapg.Gravar(t.Context(), b, linhas, nil); err != nil {
		t.Fatal(err)
	}
	consultas := NovasConsultas(cliente)
	f := auditoriapg.Filtro{Desde: agora.Add(-time.Hour), Ate: agora.Add(time.Hour)}
	lidas, _, err := consultas.Historico(auditor, f, nil, 50)
	if err != nil || len(lidas) != 1 || lidas[0].CondominioID != "42" || lidas[0].Protocolo != "" {
		t.Fatalf("histórico fora do escopo ou protocolo exposto: %+v %v", lidas, err)
	}
	if _, err := consultas.Disparos(auditor, agora.Add(-time.Hour), []string{r.ID}); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("auditor ganhou contagem global: %v", err)
	}
	acessos.revogado.Store(true)
	if _, err := api.LerVersao(auditor, r.ID, 1); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("versão após revogação: %v", err)
	}
	if _, _, err := consultas.Historico(auditor, f, nil, 50); !errors.Is(err, atende.ErrEscopoSessao) {
		t.Fatalf("histórico após revogação: %v", err)
	}
}

func TestTransporteDadosRejeitaDestinoExternoAntesDeEnviar(t *testing.T) {
	for _, destino := range []string{"http://externo.invalid:8090/dados", "https://127.0.0.1:8090/dados", "http://usuario:segredo@127.0.0.1:8090/dados", "http://127.0.0.1/dados"} {
		_, enviada, err := requisitarJSON(t.Context(), &http.Client{}, destino, "credencial", struct{}{}, &struct{}{}, 1024, 1024, time.Second)
		if err == nil || enviada {
			t.Fatalf("destino indevido chegou ao transporte: %s %v", destino, err)
		}
	}
}
