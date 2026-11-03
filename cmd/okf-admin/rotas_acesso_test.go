package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
	"go-concierge/internal/atende"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
	"go-concierge/internal/okf"
)

// acessosFake monta o Acesso como o Postgres: papéis iniciais, as atribuições do usuário e o
// condomínio → administradora atual lido do mapeamento no momento da requisição.
type acessosFake struct {
	atribuicoes map[string][]acesso.Atribuicao
	mapeamento  admin.Repositorio
}

func (f acessosFake) Carregar(ctx context.Context, usuarioID string, imp escopo.Implantacao) (acesso.Acesso, error) {
	ats, ok := f.atribuicoes[usuarioID]
	if !ok {
		return acesso.Acesso{}, acesso.ErrUsuarioDesconhecido
	}
	m, err := f.mapeamento.Ler(ctx)
	if err != nil {
		return acesso.Acesso{}, err
	}
	donas := map[string]string{}
	for _, c := range m.Condominios {
		if c.Administradora != "" {
			donas[c.ID] = c.Administradora
		}
	}
	papeis := map[string]acesso.Papel{}
	for _, p := range acesso.PapeisIniciais() {
		papeis[p.ID] = p
	}
	return acesso.Acesso{UsuarioID: usuarioID, Papeis: papeis, Atribuicoes: ats, AdministradoraDe: donas, Implantacao: imp}, nil
}

func naAdministradora(papel, adm string) []acesso.Atribuicao {
	return []acesso.Atribuicao{{PapelID: papel, Escopo: acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: adm}}}
}

// usuariosTeste: admin na Superlógica; gestor e operador de adm-a; gestor de adm-b; um
// construtor só em adm-a (tem as permissões, não o escopo da Superlógica); e um sem papel.
func usuariosTeste() map[string][]acesso.Atribuicao {
	return map[string][]acesso.Atribuicao{
		"admin":           {{PapelID: "admin_superlogica", Escopo: acesso.Escopo{Tipo: acesso.TipoSuperlogica}}},
		"gestor-a":        naAdministradora("gestor_administradora", "adm-a"),
		"gestor-b":        naAdministradora("gestor_administradora", "adm-b"),
		"operador-a":      naAdministradora("operador_administradora", "adm-a"),
		"construtor-a":    naAdministradora("construtor_superlogica", "adm-a"),
		"sem-papel":       {},
		"gestor-ab":       append(naAdministradora("gestor_administradora", "adm-a"), naAdministradora("gestor_administradora", "adm-b")...),
		usuarioSemeadoDev: {{PapelID: "admin_superlogica", Escopo: acesso.Escopo{Tipo: acesso.TipoSuperlogica}}},
	}
}

type ambienteRotas struct {
	s        *servidor
	handler  http.Handler
	repo     *admin.EmMemoria
	sessaoA  string // conversa resolvida em adm-a
	sessaoA2 string // outra conversa de adm-a (a que a rota de reiniciar apaga)
}

// novoAmbienteRotas monta o servidor inteiro sobre uma cópia do catálogo: as escritas dos
// casos "com permissão" não tocam no okf do repositório.
func novoAmbienteRotas(t *testing.T) ambienteRotas {
	t.Helper()
	raiz := t.TempDir()
	if err := os.CopyFS(raiz, os.DirFS(filepath.Join("..", "..", "okf"))); err != nil {
		t.Fatal(err)
	}
	cat, err := okf.NewCatalogo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	fix, err := okf.LerRecursosFixture(filepath.Join(raiz, "nexus", "cond-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cat.Recursos = func() []okf.Recurso { return fix }
	repo := admin.NovaEmMemoria()
	for _, a := range []admin.Administradora{{ID: "adm-a", Nome: "A"}, {ID: "adm-b", Nome: "B"}} {
		if _, err := repo.SalvarAdministradora(t.Context(), a); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []admin.Condominio{{ID: "cond-a", Nome: "Cond A", Administradora: "adm-a"}, {ID: "cond-b", Nome: "Cond B", Administradora: "adm-b"}} {
		if _, err := repo.SalvarCondominio(t.Context(), c); err != nil {
			t.Fatal(err)
		}
	}
	memorias := memoria.NewArquivos(t.TempDir())
	outbox := &atende.Outbox{}
	sessoes := atende.NewSessoes(atende.NovaSessaoEmMemoria())
	s := &servidor{
		cat: cat, web: os.DirFS(filepath.Join("..", "..", "web", "okf")), account: "cond-001", log: log.New(io.Discard, "", 0),
		atende: &atende.Servico{
			Cat: cat, Agente: "agentes/concierge", Mapeamento: repo, Sessoes: sessoes, Sender: outbox, Memorias: memorias,
			Config: harness.Config{Mock: true, Env: func(string) string { return "" }},
		},
		outbox: outbox, emulador: atende.NewEmulador(), memorias: memorias, admin: repo,
		acessos:      acessosFake{atribuicoes: usuariosTeste(), mapeamento: repo},
		autenticador: autenticadorDev{}, authMode: authModeDev,
		registro: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	mcp := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return ambienteRotas{
		s: s, handler: s.rotear(mcp), repo: repo,
		sessaoA: sessaoEm(t, sessoes, "prot-a", "adm-a"), sessaoA2: sessaoEm(t, sessoes, "prot-a2", "adm-a"),
	}
}

// sessaoEm abre uma conversa já resolvida para a administradora (o Processar faria isso).
func sessaoEm(t *testing.T, sessoes *atende.Sessoes, protocolo, adm string) string {
	t.Helper()
	sess, err := sessoes.Abrir(t.Context(), entrada.Mensagem{ConversaID: protocolo, Origem: entrada.Origem{Conector: "atende-emulado", ConexaoID: "console"}})
	if err != nil {
		t.Fatal(err)
	}
	sess.AdministradoraID = adm
	if err := sessoes.Persistir(t.Context(), sess); err != nil {
		t.Fatal(err)
	}
	return sess.ID
}

func (a ambienteRotas) pedir(t *testing.T, usuario, metodo, url, corpo string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(metodo, url, strings.NewReader(corpo))
	if usuario != "" {
		r.Header.Set("X-Dev-Usuario", usuario)
	}
	w := httptest.NewRecorder()
	a.handler.ServeHTTP(w, r)
	return w
}

func TestRotasExigemPermissao(t *testing.T) {
	amb := novoAmbienteRotas(t)
	sess := func(id string) string { return strings.ReplaceAll(id, ":", "%3A") }
	const tel = "5519999992710"
	casos := []struct {
		nome, metodo, url, corpo string
		sem, com                 string
		quer                     int // status do usuário com permissão
	}{
		// skills.ler em qualquer escopo
		{nome: "agentes", metodo: "GET", url: "/api/agentes", sem: "sem-papel", com: "operador-a", quer: 200},
		{nome: "agente", metodo: "GET", url: "/api/agente?agente=agentes/concierge", sem: "sem-papel", com: "operador-a", quer: 200},
		{nome: "arquivo", metodo: "GET", url: "/api/arquivo?agente=agentes/concierge&caminho=persona.md", sem: "sem-papel", com: "operador-a", quer: 200},
		{nome: "arquivo modelo", metodo: "GET", url: "/api/arquivo/modelo?caminho=persona.md", sem: "sem-papel", com: "operador-a", quer: 200},
		{nome: "exportar", metodo: "GET", url: "/api/agentes/exportar?agente=agentes/concierge", sem: "sem-papel", com: "operador-a", quer: 200},
		{nome: "anatomia", metodo: "GET", url: "/api/anatomia", sem: "sem-papel", com: "operador-a", quer: 200},
		{nome: "projecao", metodo: "GET", url: "/api/projecao?agente=agentes/concierge", sem: "sem-papel", com: "operador-a", quer: 200},
		{nome: "recursos", metodo: "GET", url: "/api/nexus/recursos", sem: "sem-papel", com: "operador-a", quer: 200},
		// skills.editar_fluxo na Superlógica: o construtor de adm-a tem a permissão, não o escopo
		{nome: "criar agente", metodo: "POST", url: "/api/agentes", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
		{nome: "config do agente", metodo: "PUT", url: "/api/agente/config", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
		{nome: "excluir agente", metodo: "DELETE", url: "/api/agente?agente=agentes/nao-existe", sem: "construtor-a", com: "admin", quer: 400},
		{nome: "mover agente", metodo: "POST", url: "/api/agentes/mover", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
		{nome: "importar agente", metodo: "POST", url: "/api/agentes/importar?caminho=x", sem: "construtor-a", com: "admin", quer: 400},
		{nome: "salvar arquivo", metodo: "PUT", url: "/api/arquivo", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
		{nome: "excluir arquivo", metodo: "DELETE", url: "/api/arquivo?agente=agentes/nao-existe&caminho=x.md", sem: "construtor-a", com: "admin", quer: 400},
		// ferramentas.editar na Superlógica
		{nome: "salvar recurso", metodo: "PUT", url: "/api/nexus/recurso", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
		{nome: "remover recurso", metodo: "DELETE", url: "/api/nexus/recurso?nome=nao-existe", sem: "construtor-a", com: "admin", quer: 400},
		{nome: "vincular recurso", metodo: "POST", url: "/api/agente/recursos", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
		// studio.avancado na Superlógica
		{nome: "ler orquestracao", metodo: "GET", url: "/api/admin/orquestracao", sem: "construtor-a", com: "admin", quer: 200},
		{nome: "salvar orquestracao", metodo: "PUT", url: "/api/admin/orquestracao", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
		{nome: "webhook emulado", metodo: "POST", url: "/emulador/atende/webhook", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
		{nome: "caixa do emulador", metodo: "POST", url: "/emulador/atende/messages", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 400},
		{nome: "proxy mcp", metodo: "POST", url: "/mcp/gruvi/bff", corpo: `{}`, sem: "construtor-a", com: "admin", quer: 200},
		// estado: skills.ler em qualquer escopo
		{nome: "estado", metodo: "GET", url: "/api/admin/estado?administradora=adm-a", sem: "sem-papel", com: "operador-a", quer: 200},
		// administradoras.editar na Superlógica
		{nome: "criar administradora", metodo: "POST", url: "/api/admin/administradora", corpo: `{"id":"adm-c","nome":"C"}`, sem: "construtor-a", com: "admin", quer: 201},
		{nome: "editar administradora", metodo: "PUT", url: "/api/admin/administradora", corpo: `{"id":"adm-a","nome":"A"}`, sem: "construtor-a", com: "admin", quer: 200},
		{nome: "remover administradora", metodo: "DELETE", url: "/api/admin/administradora?id=adm-zz", sem: "construtor-a", com: "admin", quer: 200},
		// condominios.editar na administradora do corpo (e na dona atual)
		{nome: "criar condominio", metodo: "POST", url: "/api/admin/condominio", corpo: `{"id":"cond-n","nome":"N","administradora_id":"adm-a"}`, sem: "gestor-a", com: "construtor-a", quer: 201},
		{nome: "criar condominio em outra administradora", metodo: "POST", url: "/api/admin/condominio", corpo: `{"id":"cond-m","nome":"M","administradora_id":"adm-b"}`, sem: "construtor-a", com: "admin", quer: 201},
		{nome: "criar condominio sem dona", metodo: "POST", url: "/api/admin/condominio", corpo: `{"id":"cond-s","nome":"S"}`, sem: "construtor-a", com: "admin", quer: 201},
		{nome: "editar condominio", metodo: "PUT", url: "/api/admin/condominio", corpo: `{"id":"cond-a","nome":"Cond A","administradora_id":"adm-a"}`, sem: "gestor-a", com: "construtor-a", quer: 200},
		{nome: "tomar condominio de outra", metodo: "PUT", url: "/api/admin/condominio", corpo: `{"id":"cond-b","nome":"Cond B","administradora_id":"adm-a"}`, sem: "construtor-a", com: "admin", quer: 200},
		{nome: "remover condominio", metodo: "DELETE", url: "/api/admin/condominio?id=cond-zz", sem: "construtor-a", com: "admin", quer: 200},
		// condominios.habilitar no condomínio (padrão: Superlógica)
		{nome: "habilitacao", metodo: "PUT", url: "/api/admin/habilitacao", corpo: `{"condominio":"cond-a","caso":"faq","habilitado":true}`, sem: "gestor-b", com: "gestor-a", quer: 200},
		{nome: "habilitacao padrao", metodo: "PUT", url: "/api/admin/habilitacao", corpo: `{"condominio":"*","caso":"faq","habilitado":true}`, sem: "gestor-a", com: "admin", quer: 200},
		{nome: "habilitacao sem condominio vale o padrao", metodo: "PUT", url: "/api/admin/habilitacao", corpo: `{"caso":"faq","habilitado":true}`, sem: "gestor-a", com: "admin", quer: 200},
		{nome: "orientacao", metodo: "PUT", url: "/api/admin/orientacao", corpo: `{"condominio":"cond-a","geral":"x"}`, sem: "gestor-b", com: "gestor-a", quer: 200},
		{nome: "orientacao padrao", metodo: "PUT", url: "/api/admin/orientacao", corpo: `{"condominio":"*","geral":"x"}`, sem: "gestor-a", com: "admin", quer: 200},
		{nome: "ajustes", metodo: "PUT", url: "/api/admin/ajustes", corpo: `{"condominio":"cond-a","ajustes":{}}`, sem: "gestor-b", com: "gestor-a", quer: 200},
		{nome: "ajustes padrao", metodo: "PUT", url: "/api/admin/ajustes", corpo: `{"condominio":"*","ajustes":{}}`, sem: "gestor-a", com: "admin", quer: 200},
		{nome: "sugerir payload", metodo: "POST", url: "/api/admin/sugerir-payload", corpo: `{"condominio":"cond-a","recurso":"nao-existe"}`, sem: "gestor-b", com: "gestor-a", quer: 404},
		// ferramentas.editar no condomínio
		{nome: "payload", metodo: "PUT", url: "/api/admin/payload", corpo: `{"condominio":"cond-a","recurso":"x","payload":{}}`, sem: "gestor-a", com: "construtor-a", quer: 200},
		{nome: "payload padrao", metodo: "PUT", url: "/api/admin/payload", corpo: `{"condominio":"*","recurso":"x","payload":{}}`, sem: "construtor-a", com: "admin", quer: 200},
		// conversas.ver na administradora pedida (a da sessão, quando há sessão)
		{nome: "sessoes", metodo: "GET", url: "/api/atende/sessoes?administradora=adm-a", sem: "gestor-b", com: "operador-a", quer: 200},
		{nome: "sessao", metodo: "GET", url: "/api/atende/sessao?id=" + sess(amb.sessaoA), sem: "gestor-b", com: "operador-a", quer: 200},
		{nome: "saidas da sessao", metodo: "GET", url: "/api/atende/saidas?id=" + sess(amb.sessaoA), sem: "gestor-b", com: "operador-a", quer: 200},
		// studio.avancado na Superlógica: identidade (Gruvi com a company global), configuração
		// da implantação e a caixa de saída inteira não são de uma administradora
		{nome: "identidade", metodo: "GET", url: "/api/atende/identidade?phone=" + tel + "&administradora=adm-a", sem: "operador-a", com: "admin", quer: 200},
		{nome: "identidade de quem tem studio só em A", metodo: "GET", url: "/api/atende/identidade?phone=" + tel + "&administradora=adm-a", sem: "construtor-a", com: "admin", quer: 200},
		{nome: "config do atende", metodo: "GET", url: "/api/atende/config?administradora=adm-a", sem: "construtor-a", com: "admin", quer: 200},
		{nome: "caixa de saida inteira", metodo: "GET", url: "/api/atende/saidas?administradora=adm-a", sem: "construtor-a", com: "admin", quer: 200},
		// conversas.atender na administradora da sessão
		{nome: "transbordo", metodo: "POST", url: "/api/atende/transbordo", corpo: `{"sessao_id":"` + amb.sessaoA + `","acao":"x"}`, sem: "gestor-b", com: "operador-a", quer: 400},
		{nome: "reiniciar sessao", metodo: "DELETE", url: "/api/atende/sessao?id=" + sess(amb.sessaoA2), sem: "gestor-b", com: "operador-a", quer: 200},
		// pedidos
		{nome: "pedidos", metodo: "GET", url: "/api/pedidos?phone=" + tel + "&administradora=adm-a", sem: "gestor-b", com: "operador-a", quer: 200},
		{nome: "exportar pedidos", metodo: "GET", url: "/api/pedidos/exportar?phone=" + tel + "&administradora=adm-a", sem: "gestor-b", com: "operador-a", quer: 400},
		{nome: "acao de pedido", metodo: "POST", url: "/api/pedidos/acao?administradora=adm-a", corpo: `{"acao":"x"}`, sem: "operador-a", com: "gestor-a", quer: 400},
		{nome: "preencher pedido de outra sessao", metodo: "POST", url: "/api/pedidos/acao?administradora=adm-b", corpo: `{"acao":"preencher","sessao_id":"` + amb.sessaoA + `"}`, sem: "gestor-b", com: "gestor-a", quer: 404},
		// memória
		{nome: "memoria", metodo: "GET", url: "/api/memoria?phone=" + tel + "&administradora=adm-a", sem: "gestor-b", com: "operador-a", quer: 200},
		{nome: "memorias", metodo: "GET", url: "/api/memorias?administradora=adm-a", sem: "gestor-b", com: "operador-a", quer: 200},
		{nome: "esquecer memoria", metodo: "DELETE", url: "/api/memoria?phone=" + tel + "&administradora=adm-a&chave=x", sem: "operador-a", com: "gestor-a", quer: 404},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if w := amb.pedir(t, c.sem, c.metodo, c.url, c.corpo); w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "SEM_PERMISSAO") {
				t.Fatalf("%s sem permissão: status %d: %s", c.sem, w.Code, w.Body.String())
			}
			if w := amb.pedir(t, c.com, c.metodo, c.url, c.corpo); w.Code != c.quer {
				t.Fatalf("%s com permissão: status %d, quero %d: %s", c.com, w.Code, c.quer, w.Body.String())
			}
		})
	}
}

func TestRotasAutenticacao(t *testing.T) {
	amb := novoAmbienteRotas(t)
	casos := []struct {
		nome, usuario, url string
		status             int
		trecho             string
	}{
		{nome: "sem cabeçalho e sem usuário padrão", url: "/api/agentes", status: http.StatusUnauthorized, trecho: "NAO_AUTENTICADO"},
		{nome: "usuário desconhecido", usuario: "fantasma", url: "/api/agentes", status: http.StatusUnauthorized, trecho: "NAO_AUTENTICADO"},
		{nome: "gestor de A pede memória de B", usuario: "gestor-a", url: "/api/memoria?phone=5519999992710&administradora=adm-b", status: http.StatusForbidden, trecho: "SEM_PERMISSAO"},
		{nome: "página do Admin é pública", url: "/", status: http.StatusOK},
		{nome: "Console é público", url: "/console", status: http.StatusOK},
		{nome: "Backdoor é público", url: "/backdoor", status: http.StatusOK},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := amb.pedir(t, c.usuario, "GET", c.url, "")
			if w.Code != c.status || !strings.Contains(w.Body.String(), c.trecho) {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
		})
	}
	t.Run("usuário padrão do modo dev", func(t *testing.T) {
		amb.s.autenticador = autenticadorDev{padrao: "operador-a"}
		defer func() { amb.s.autenticador = autenticadorDev{} }()
		if w := amb.pedir(t, "", "GET", "/api/agentes", ""); w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestSessaoFicaComAAdministradoraGravada: a conversa gravada em A continua de A depois que o
// condomínio passa para B; o gestor de B não a vê.
func TestSessaoFicaComAAdministradoraGravada(t *testing.T) {
	amb := novoAmbienteRotas(t)
	if _, err := amb.repo.SalvarCondominio(t.Context(), admin.Condominio{ID: "cond-a", Nome: "Cond A", Administradora: "adm-b"}); err != nil {
		t.Fatal(err)
	}
	url := "/api/atende/sessao?id=" + strings.ReplaceAll(amb.sessaoA, ":", "%3A") + "&administradora=adm-b"
	if w := amb.pedir(t, "gestor-b", "GET", url, ""); w.Code != http.StatusForbidden {
		t.Fatalf("gestor de B: status %d: %s", w.Code, w.Body.String())
	}
	urlDeA := "/api/atende/sessao?id=" + strings.ReplaceAll(amb.sessaoA, ":", "%3A")
	if w := amb.pedir(t, "gestor-a", "GET", urlDeA, ""); w.Code != http.StatusOK {
		t.Fatalf("gestor de A: status %d: %s", w.Code, w.Body.String())
	}
	w := amb.pedir(t, "gestor-b", "GET", "/api/atende/sessoes?administradora=adm-b", "")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "assuma o atendimento") {
		t.Fatalf("lista de B traz a conversa de A: %d %s", w.Code, w.Body.String())
	}
}

func TestEscritasDoMapeamento(t *testing.T) {
	casos := []struct {
		nome, usuario, metodo, url, corpo string
		status                            int
		codigo                            string
	}{
		{nome: "criar condomínio repetido", usuario: "admin", metodo: "POST", url: "/api/admin/condominio", corpo: `{"id":"cond-a","nome":"Intruso","administradora_id":"adm-b"}`, status: http.StatusConflict, codigo: "JA_EXISTE"},
		{nome: "criar administradora repetida", usuario: "admin", metodo: "POST", url: "/api/admin/administradora", corpo: `{"id":"adm-a","nome":"Outra"}`, status: http.StatusConflict, codigo: "JA_EXISTE"},
		{nome: "editar condomínio inexistente", usuario: "admin", metodo: "PUT", url: "/api/admin/condominio", corpo: `{"id":"cond-zz","nome":"Z"}`, status: http.StatusNotFound, codigo: "NAO_ENCONTRADO"},
		{nome: "editar administradora inexistente", usuario: "admin", metodo: "PUT", url: "/api/admin/administradora", corpo: `{"id":"adm-zz","nome":"Z"}`, status: http.StatusNotFound, codigo: "NAO_ENCONTRADO"},
		{nome: "criar condomínio novo", usuario: "construtor-a", metodo: "POST", url: "/api/admin/condominio", corpo: `{"id":"cond-n","nome":"N","administradora_id":"adm-a"}`, status: http.StatusCreated},
		{nome: "gestor de A escreve o padrão", usuario: "gestor-a", metodo: "PUT", url: "/api/admin/orientacao", corpo: `{"condominio":"*","geral":"x"}`, status: http.StatusForbidden, codigo: "SEM_PERMISSAO"},
		{nome: "gestor de A escreve condomínio de B", usuario: "gestor-a", metodo: "PUT", url: "/api/admin/habilitacao", corpo: `{"condominio":"cond-b","caso":"faq"}`, status: http.StatusForbidden, codigo: "SEM_PERMISSAO"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			amb := novoAmbienteRotas(t)
			w := amb.pedir(t, c.usuario, c.metodo, c.url, c.corpo)
			if w.Code != c.status || !strings.Contains(w.Body.String(), c.codigo) {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
		})
	}
	t.Run("criar repetido não sobrescreve", func(t *testing.T) {
		amb := novoAmbienteRotas(t)
		amb.pedir(t, "admin", "POST", "/api/admin/condominio", `{"id":"cond-a","nome":"Intruso","administradora_id":"adm-b"}`)
		m, err := amb.repo.Ler(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if c, _ := m.Condominio("cond-a"); c.Nome != "Cond A" || c.Administradora != "adm-a" {
			t.Fatalf("cond-a sobrescrito: %+v", c)
		}
	})
}

func TestEstadoMostraSoAsAdministradorasVisiveis(t *testing.T) {
	amb := novoAmbienteRotas(t)
	var corpo struct {
		Mapeamento      admin.Mapeamento       `json:"mapeamento"`
		Administradoras []admin.Administradora `json:"administradoras"`
	}
	w := amb.pedir(t, "gestor-a", "GET", "/api/admin/estado", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	if len(corpo.Administradoras) != 1 || corpo.Administradoras[0].ID != "adm-a" || len(corpo.Mapeamento.Condominios) != 1 || corpo.Mapeamento.Condominios[0].ID != "cond-a" {
		t.Fatalf("gestor de A vê além de A: %+v", corpo)
	}
	if w := amb.pedir(t, "gestor-a", "GET", "/api/admin/estado?administradora=adm-b", ""); w.Code != http.StatusForbidden {
		t.Fatalf("gestor de A pedindo B: status %d", w.Code)
	}
	w = amb.pedir(t, "admin", "GET", "/api/admin/estado", "")
	if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	if len(corpo.Administradoras) != 2 || len(corpo.Mapeamento.Condominios) != 2 {
		t.Fatalf("admin da Superlógica vê tudo: %+v", corpo)
	}
}

func TestLogDoMiddleware(t *testing.T) {
	amb := novoAmbienteRotas(t)
	var buf bytes.Buffer
	amb.s.registro = slog.New(slog.NewJSONHandler(&buf, nil))
	amb.handler = amb.s.rotear(nil)
	if w := amb.pedir(t, "operador-a", "GET", "/api/memoria?phone=5519999992710&administradora=adm-a", ""); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	linhas := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(linhas) != 1 {
		t.Fatalf("quero uma linha, veio:\n%s", buf.String())
	}
	var linha map[string]any
	if err := json.Unmarshal([]byte(linhas[0]), &linha); err != nil {
		t.Fatalf("linha não é JSON: %v: %s", err, linhas[0])
	}
	for _, campo := range []string{"request_id", "usuario_id", "operacao", "duracao_ms"} {
		if _, ok := linha[campo]; !ok {
			t.Fatalf("falta %s: %s", campo, linhas[0])
		}
	}
	if linha["usuario_id"] != "operador-a" || linha["operacao"] != "GET /api/memoria" {
		t.Fatalf("linha: %s", linhas[0])
	}
	if !strings.Contains(linhas[0], "5519****2710") || strings.Contains(linhas[0], "5519999992710") {
		t.Fatalf("telefone sem máscara: %s", linhas[0])
	}
}

// autenticadorFixo devolve sempre o mesmo usuário, como faria um login futuro.
type autenticadorFixo string

func (a autenticadorFixo) Autenticar(*http.Request) (string, error) { return string(a), nil }

func TestUsuarioSemeadoSoValeEmModoDev(t *testing.T) {
	casos := []struct {
		nome     string
		authMode string
		quer     int
	}{
		{nome: "modo dev aceita", authMode: authModeDev, quer: http.StatusOK},
		{nome: "fora de dev recusa", authMode: "", quer: http.StatusUnauthorized},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			amb := novoAmbienteRotas(t)
			amb.s.autenticador, amb.s.authMode = autenticadorFixo(usuarioSemeadoDev), c.authMode
			if w := amb.pedir(t, "", "GET", "/api/agentes", ""); w.Code != c.quer {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestEscritaRecusadaAntesDeGravar: o recorte da resposta é decidido antes da escrita; quem
// tem duas administradoras e não diz qual recebe 400 sem que nada seja gravado.
func TestEscritaRecusadaAntesDeGravar(t *testing.T) {
	amb := novoAmbienteRotas(t)
	corpo := `{"condominio":"cond-a","caso":"faq","habilitado":false}`
	w := amb.pedir(t, "gestor-ab", "PUT", "/api/admin/habilitacao", corpo)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "ADMINISTRADORA_OBRIGATORIA") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	m, err := amb.repo.Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, gravou := m.Habilitacoes["cond-a"]; gravou {
		t.Fatalf("a escrita foi gravada antes do 400: %+v", m.Habilitacoes)
	}
	if w := amb.pedir(t, "gestor-ab", "PUT", "/api/admin/habilitacao?administradora=adm-a", corpo); w.Code != http.StatusOK {
		t.Fatalf("com a administradora pedida: status %d: %s", w.Code, w.Body.String())
	}
}

func TestChavesDeRoteamentoSoComASuperlogica(t *testing.T) {
	casos := []struct {
		nome, usuario, metodo, corpo string
		quer                         int
	}{
		{nome: "criar com company", usuario: "construtor-a", metodo: "POST", corpo: `{"id":"cond-n","nome":"N","administradora_id":"adm-a","company_id":"5982"}`, quer: http.StatusForbidden},
		{nome: "criar com tenant", usuario: "construtor-a", metodo: "POST", corpo: `{"id":"cond-n","nome":"N","administradora_id":"adm-a","tenant":"t"}`, quer: http.StatusForbidden},
		{nome: "criar sem chaves", usuario: "construtor-a", metodo: "POST", corpo: `{"id":"cond-n","nome":"N","administradora_id":"adm-a"}`, quer: http.StatusCreated},
		{nome: "editar mudando tenant", usuario: "construtor-a", metodo: "PUT", corpo: `{"id":"cond-a","nome":"Cond A","administradora_id":"adm-a","tenant":"outro"}`, quer: http.StatusForbidden},
		{nome: "editar mantendo as chaves", usuario: "construtor-a", metodo: "PUT", corpo: `{"id":"cond-a","nome":"Novo nome","administradora_id":"adm-a"}`, quer: http.StatusOK},
		{nome: "superlógica muda tenant", usuario: "admin", metodo: "PUT", corpo: `{"id":"cond-a","nome":"Cond A","administradora_id":"adm-a","tenant":"outro"}`, quer: http.StatusOK},
		{nome: "superlógica cria com company", usuario: "admin", metodo: "POST", corpo: `{"id":"cond-n","nome":"N","administradora_id":"adm-a","company_id":"5982"}`, quer: http.StatusCreated},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			amb := novoAmbienteRotas(t)
			if w := amb.pedir(t, c.usuario, c.metodo, "/api/admin/condominio", c.corpo); w.Code != c.quer {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// acessosQueMovem carrega o Acesso e, logo depois, passa o condomínio para outra dona: a
// escrita já autorizada tem de ser recusada pelo repositório.
type acessosQueMovem struct {
	acessosFake
	condominio admin.Condominio
}

func (a acessosQueMovem) Carregar(ctx context.Context, usuarioID string, imp escopo.Implantacao) (acesso.Acesso, error) {
	acc, err := a.acessosFake.Carregar(ctx, usuarioID, imp)
	if err != nil {
		return acc, err
	}
	_, err = a.mapeamento.SalvarCondominio(ctx, a.condominio)
	return acc, err
}

func TestCondominioMovidoNoMeioDaRequisicao(t *testing.T) {
	escritas := []struct{ nome, metodo, url, corpo string }{
		{nome: "habilitação", metodo: "PUT", url: "/api/admin/habilitacao", corpo: `{"condominio":"cond-a","caso":"faq"}`},
		{nome: "orientação", metodo: "PUT", url: "/api/admin/orientacao", corpo: `{"condominio":"cond-a","geral":"x"}`},
		{nome: "ajustes", metodo: "PUT", url: "/api/admin/ajustes", corpo: `{"condominio":"cond-a","ajustes":{"r":{"x":1}}}`},
		{nome: "payload", metodo: "PUT", url: "/api/admin/payload", corpo: `{"condominio":"cond-a","recurso":"r","payload":{"x":1}}`},
		{nome: "remover", metodo: "DELETE", url: "/api/admin/condominio?id=cond-a"},
		{nome: "editar", metodo: "PUT", url: "/api/admin/condominio", corpo: `{"id":"cond-a","nome":"Cond A","administradora_id":"adm-a"}`},
	}
	for _, e := range escritas {
		t.Run(e.nome, func(t *testing.T) {
			amb := novoAmbienteRotas(t)
			amb.s.acessos = acessosQueMovem{
				acessosFake: acessosFake{atribuicoes: usuariosTeste(), mapeamento: amb.repo},
				condominio:  admin.Condominio{ID: "cond-a", Nome: "Cond A", Administradora: "adm-b"},
			}
			w := amb.pedir(t, "construtor-a", e.metodo, e.url, e.corpo)
			if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "CONFLITO_CONCORRENTE") {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			m, err := amb.repo.Ler(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if c, ok := m.Condominio("cond-a"); !ok || c.Administradora != "adm-b" || len(m.Habilitacoes["cond-a"])+len(m.Payloads["cond-a"]) > 0 {
				t.Fatalf("a escrita passou depois da mudança de dona: %+v", m)
			}
		})
	}
}

func TestTransbordoNaoEcoaErroInterno(t *testing.T) {
	amb := novoAmbienteRotas(t)
	// responder sem transbordo ativo é recusado pelo serviço; o erro dele não vai na resposta
	w := amb.pedir(t, "admin", "POST", "/api/atende/transbordo", `{"sessao_id":"`+amb.sessaoA+`","acao":"responder","texto":"oi"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "TRANSBORDO_RECUSADO") || strings.Contains(w.Body.String(), "assuma o atendimento") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}

func TestLogDeRotaInexistenteDaAPI(t *testing.T) {
	amb := novoAmbienteRotas(t)
	var buf bytes.Buffer
	amb.s.registro = slog.New(slog.NewJSONHandler(&buf, nil))
	amb.handler = amb.s.rotear(nil)
	amb.pedir(t, "admin", "GET", "/api/nao-existe", "")
	amb.pedir(t, "", "GET", "/styles.css", "")
	if n := strings.Count(buf.String(), "\n"); n != 1 || !strings.Contains(buf.String(), `"caminho":"/api/nao-existe"`) {
		t.Fatalf("quero só a linha de /api/nao-existe:\n%s", buf.String())
	}
}
