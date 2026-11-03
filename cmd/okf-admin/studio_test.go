package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go-concierge/internal/atende"
	"go-concierge/internal/okf"
)

const skillMudanca = "/api/skills/agendar-mudanca"

func TestEuDev(t *testing.T) {
	admExemplo := "adm-exemplo"
	gestor := []string{
		"skills.ler", "conversas.ver", "conversas.atender", "pedidos.ver", "memoria.ver",
		"condominios.habilitar", "skills.editar_textos", "pedidos.editar", "memoria.esquecer",
	}
	superlogica := eu{
		ID: "dev-superlogica", Nome: "Dev Superlógica", Papel: "admin_superlogica",
		Permissoes: []string{
			"skills.ler", "skills.editar_textos", "skills.editar_fluxo", "skills.criar", "skills.excluir",
			"ferramentas.editar", "condominios.habilitar", "condominios.editar", "conversas.ver",
			"conversas.atender", "pedidos.ver", "pedidos.editar", "memoria.ver", "memoria.esquecer",
			"administradoras.editar", "usuarios.editar", "studio.avancado",
		},
		Dev: true,
	}
	casos := []struct {
		nome   string
		papel  string
		quer   eu
		querOK bool
	}{
		{"vazio vira construtor", "", superlogica, true},
		{"construtor", "construtor", superlogica, true},
		{"gestor", "gestor", eu{
			ID: "dev-gestor", Nome: "Dev Gestor", Papel: "gestor_administradora",
			Permissoes: gestor, AdministradoraID: &admExemplo, Dev: true,
		}, true},
		{"gestor avancado", "gestor_avancado", eu{
			ID: "dev-gestor", Nome: "Dev Gestor", Papel: "gestor_administradora",
			Permissoes: append(slices.Clone(gestor), "studio.avancado"), AdministradoraID: &admExemplo, Dev: true,
		}, true},
		{"desconhecido", "xyz", eu{}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, ok := euDev(c.papel)
			if ok != c.querOK || !reflect.DeepEqual(got, c.quer) {
				t.Fatalf("euDev(%q) = %+v, %v; quer %+v, %v", c.papel, got, ok, c.quer, c.querOK)
			}
		})
	}
}

// pedido é uma chamada à API; corpo recebe o servidor para montar o que depende do estado (a versão atual).
type pedido struct {
	metodo, caminho string
	corpo           func(t *testing.T, h http.Handler) string
	cabecalhos      map[string]string
}

func TestStudioAPI(t *testing.T) {
	versaoAntes := "" // lida no corpo do PUT de fluxo, conferida na resposta
	casos := []struct {
		nome       string
		antes      []pedido // precisam dar 2xx
		pedido     pedido
		querStatus int
		conferir   func(t *testing.T, corpo map[string]any)
	}{
		{
			nome:       "lista as skills do assistente",
			pedido:     pedido{metodo: "GET", caminho: "/api/skills"},
			querStatus: 200,
			conferir: func(t *testing.T, corpo map[string]any) {
				exigirCampos(t, corpo, "assistente", "skills")
				if corpo["assistente"] != "agentes/concierge" {
					t.Fatalf("assistente = %v", corpo["assistente"])
				}
				nomes := []string{}
				for _, item := range corpo["skills"].([]any) {
					skill := item.(map[string]any)
					exigirCampos(t, skill, "nome", "id", "titulo", "descricao", "escrita", "tipo", "recursos", "caminho")
					nomes = append(nomes, skill["nome"].(string))
					if skill["nome"] == "agendar-mudanca" && skill["caminho"] != "agentes/concierge/agentes/agendar-mudanca" {
						t.Fatalf("caminho = %v", skill["caminho"])
					}
				}
				if !slices.Contains(nomes, "agendar-mudanca") || slices.Contains(nomes, "observador-sessao") {
					t.Fatalf("skills = %v", nomes)
				}
			},
		},
		{
			nome:       "le os textos da skill",
			pedido:     pedido{metodo: "GET", caminho: skillMudanca},
			querStatus: 200,
			conferir: func(t *testing.T, corpo map[string]any) {
				exigirCampos(t, corpo, "caminho", "textos", "versao")
				exigirCampos(t, corpo["textos"].(map[string]any), "titulo", "quandoUsar", "frasesDeExemplo", "palavrasChave", "temFluxo")
			},
		},
		{
			nome:       "le o fluxo da skill",
			pedido:     pedido{metodo: "GET", caminho: skillMudanca + "/fluxo"},
			querStatus: 200,
			conferir: func(t *testing.T, corpo map[string]any) {
				exigirCampos(t, corpo, "fluxo", "versao", "problemas")
				fluxo := corpo["fluxo"].(map[string]any)
				exigirCampos(t, fluxo, "blocos", "informacoes")
				blocos := fluxo["blocos"].([]any)
				ultimo := blocos[len(blocos)-1].(map[string]any)
				exigirCampos(t, ultimo, "id", "tipo")
				if len(blocos) != 6 || ultimo["tipo"] != "responder" {
					t.Fatalf("blocos = %d, último %v", len(blocos), ultimo["tipo"])
				}
			},
		},
		{
			nome: "grava o fluxo na versao atual",
			pedido: pedido{metodo: "PUT", caminho: skillMudanca + "/fluxo", corpo: func(t *testing.T, h http.Handler) string {
				lido := lerFluxo(t, h)
				ultimo := &lido.Fluxo.Blocos[len(lido.Fluxo.Blocos)-1]
				ultimo.Texto += " Até logo!"
				versaoAntes = lido.Versao
				return corpoJSON(t, map[string]any{"versao": lido.Versao, "fluxo": lido.Fluxo})
			}},
			querStatus: 200,
			conferir: func(t *testing.T, corpo map[string]any) {
				exigirCampos(t, corpo, "versao", "problemas")
				if corpo["versao"] == versaoAntes {
					t.Fatalf("versão não mudou: %v", corpo["versao"])
				}
			},
		},
		{
			nome: "recusa gravar sem confirmar",
			pedido: pedido{metodo: "PUT", caminho: skillMudanca + "/fluxo", corpo: func(t *testing.T, h http.Handler) string {
				lido := lerFluxo(t, h)
				lido.Fluxo.Blocos = slices.DeleteFunc(lido.Fluxo.Blocos, func(b okf.Bloco) bool { return b.Tipo == okf.BlocoConfirmar })
				return corpoJSON(t, map[string]any{"versao": lido.Versao, "fluxo": lido.Fluxo})
			}},
			querStatus: 422,
			conferir: func(t *testing.T, corpo map[string]any) {
				exigirErro(t, corpo, "FLUXO_INVALIDO")
				problema := corpo["problemas"].([]any)[0].(map[string]any)
				exigirCampos(t, problema, "blocoId", "codigo", "nivel", "mensagem")
				if problema["codigo"] != "GRAVAR_SEM_CONFIRMAR" {
					t.Fatalf("problema = %v", problema)
				}
			},
		},
		{
			nome: "recusa fluxo de versao velha",
			pedido: pedido{metodo: "PUT", caminho: skillMudanca + "/fluxo", corpo: func(t *testing.T, h http.Handler) string {
				lido := lerFluxo(t, h)
				return corpoJSON(t, map[string]any{"versao": "0000000000000000", "fluxo": lido.Fluxo})
			}},
			querStatus: 409,
			conferir:   exigirErroCom("VERSAO_DESATUALIZADA", "Esta skill mudou desde que você abriu. Recarregue para continuar."),
		},
		{
			nome: "grava os textos na versao atual",
			pedido: pedido{metodo: "PUT", caminho: skillMudanca + "/textos", corpo: func(t *testing.T, h http.Handler) string {
				lidos := lerTextos(t, h)
				lidos.Textos.FrasesDeExemplo = append(lidos.Textos.FrasesDeExemplo, "quero marcar a mudança")
				return corpoJSON(t, lidos)
			}},
			querStatus: 200,
			conferir: func(t *testing.T, corpo map[string]any) {
				exigirCampos(t, corpo, "versao")
			},
		},
		{
			nome: "recusa textos sem titulo",
			pedido: pedido{metodo: "PUT", caminho: skillMudanca + "/textos", corpo: func(t *testing.T, h http.Handler) string {
				lidos := lerTextos(t, h)
				lidos.Textos.Titulo = " "
				return corpoJSON(t, lidos)
			}},
			querStatus: 422,
			conferir:   exigirErroCom("TITULO_VAZIO", "Dê um título para a skill."),
		},
		{
			nome: "recusa textos de versao velha",
			pedido: pedido{metodo: "PUT", caminho: skillMudanca + "/textos", corpo: func(t *testing.T, h http.Handler) string {
				lidos := lerTextos(t, h)
				return corpoJSON(t, textosParaGravar{Versao: "0000000000000000", Textos: lidos.Textos})
			}},
			querStatus: 409,
			conferir:   exigirErroCom("VERSAO_DESATUALIZADA", "Esta skill mudou desde que você abriu. Recarregue para continuar."),
		},
		{
			nome:       "fluxo de skill sem fluxo",
			pedido:     pedido{metodo: "GET", caminho: "/api/skills/faq/fluxo"},
			querStatus: 404,
			conferir:   exigirErroCom("SEM_FLUXO", "Esta skill não tem fluxo: ela responde pela base de conhecimento."),
		},
		{
			nome:       "nome fora do padrao",
			pedido:     pedido{metodo: "GET", caminho: "/api/skills/Nao%20Existe"},
			querStatus: 404,
			conferir:   exigirErroCom("SKILL_NAO_ENCONTRADA", "Esta skill não existe."),
		},
		{
			nome:       "skill que nao existe",
			pedido:     pedido{metodo: "GET", caminho: "/api/skills/nao-existe/fluxo"},
			querStatus: 404,
			conferir:   exigirErroCom("SKILL_NAO_ENCONTRADA", "Esta skill não existe."),
		},
		{
			nome:       "corpo que nao e json",
			pedido:     pedido{metodo: "POST", caminho: "/api/skills", corpo: texto("{")},
			querStatus: 400,
			conferir:   exigirErroCom("CORPO_INVALIDO", "Não entendi o pedido."),
		},
		{
			nome:       "cria com nome invalido",
			pedido:     pedido{metodo: "POST", caminho: "/api/skills", corpo: texto(`{"nome":"Reserva Salão","titulo":"Reserva"}`)},
			querStatus: 400,
			conferir:   exigirErroCom("NOME_INVALIDO", "Use letras minúsculas, números e hífen."),
		},
		{
			nome:       "cria sem titulo",
			pedido:     pedido{metodo: "POST", caminho: "/api/skills", corpo: texto(`{"nome":"nova-skill","titulo":""}`)},
			querStatus: 422,
			conferir:   exigirErroCom("TITULO_VAZIO", "Dê um título para a skill."),
		},
		{
			nome:       "cria skill",
			pedido:     pedido{metodo: "POST", caminho: "/api/skills", corpo: texto(`{"nome":"nova-skill","titulo":"Nova skill"}`)},
			querStatus: 201,
			conferir:   exigirNome("nova-skill"),
		},
		{
			nome:       "cria skill que ja existe",
			pedido:     pedido{metodo: "POST", caminho: "/api/skills", corpo: texto(`{"nome":"agendar-mudanca","titulo":"Outra"}`)},
			querStatus: 409,
			conferir:   exigirErroCom("SKILL_JA_EXISTE", "Já existe uma skill com esse nome."),
		},
		{
			nome:       "duplica skill",
			antes:      []pedido{{metodo: "POST", caminho: "/api/skills", corpo: texto(`{"nome":"nova-skill","titulo":"Nova skill"}`)}},
			pedido:     pedido{metodo: "POST", caminho: "/api/skills/nova-skill/duplicar", corpo: texto(`{"nome":"nova-skill-2","titulo":"Cópia"}`)},
			querStatus: 201,
			conferir:   exigirNome("nova-skill-2"),
		},
		{
			nome:       "duplica skill que nao existe",
			pedido:     pedido{metodo: "POST", caminho: "/api/skills/nao-existe/duplicar", corpo: texto(`{"nome":"copia","titulo":"Cópia"}`)},
			querStatus: 404,
			conferir:   exigirErroCom("SKILL_NAO_ENCONTRADA", "Esta skill não existe."),
		},
		{
			nome:       "exclui skill",
			antes:      []pedido{{metodo: "POST", caminho: "/api/skills", corpo: texto(`{"nome":"nova-skill","titulo":"Nova skill"}`)}},
			pedido:     pedido{metodo: "DELETE", caminho: "/api/skills/nova-skill"},
			querStatus: 204,
		},
		{
			nome:       "exclui skill que nao existe",
			pedido:     pedido{metodo: "DELETE", caminho: "/api/skills/nao-existe"},
			querStatus: 404,
			conferir:   exigirErroCom("SKILL_NAO_ENCONTRADA", "Esta skill não existe."),
		},
		{
			nome:       "eu sem papel",
			pedido:     pedido{metodo: "GET", caminho: "/api/eu"},
			querStatus: 200,
			conferir: func(t *testing.T, corpo map[string]any) {
				exigirCampos(t, corpo, "id", "nome", "papel", "permissoes", "administradoraId", "dev")
				if corpo["administradoraId"] != nil || corpo["dev"] != true {
					t.Fatalf("eu = %v", corpo)
				}
			},
		},
		{
			nome:       "eu como gestor",
			pedido:     pedido{metodo: "GET", caminho: "/api/eu", cabecalhos: map[string]string{"X-Studio-Papel": "gestor"}},
			querStatus: 200,
			conferir: func(t *testing.T, corpo map[string]any) {
				if corpo["administradoraId"] != "adm-exemplo" || slices.Contains(corpo["permissoes"].([]any), any("skills.editar_fluxo")) {
					t.Fatalf("eu = %v", corpo)
				}
			},
		},
		{
			nome:       "eu com papel desconhecido",
			pedido:     pedido{metodo: "GET", caminho: "/api/eu", cabecalhos: map[string]string{"X-Studio-Papel": "xyz"}},
			querStatus: 400,
			conferir: func(t *testing.T, corpo map[string]any) {
				exigirErro(t, corpo, "PAPEL_DESCONHECIDO")
			},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h := handlerDeTeste(t)
			for _, p := range c.antes {
				if w := chamar(t, h, p); w.Code/100 != 2 {
					t.Fatalf("preparo %s %s: %d %s", p.metodo, p.caminho, w.Code, w.Body.String())
				}
			}
			w := chamar(t, h, c.pedido)
			if w.Code != c.querStatus {
				t.Fatalf("status %d, quer %d: %s", w.Code, c.querStatus, w.Body.String())
			}
			if c.conferir == nil {
				if w.Body.Len() != 0 {
					t.Fatalf("corpo inesperado: %s", w.Body.String())
				}
				return
			}
			var corpo map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
				t.Fatalf("corpo %q: %v", w.Body.String(), err)
			}
			c.conferir(t, corpo)
		})
	}
}

func TestStudioAPINaoVazaErroInterno(t *testing.T) {
	w := httptest.NewRecorder()
	s := &servidor{log: log.New(io.Discard, "", 0)}
	s.responderErro(w, os.ErrPermission)
	if w.Code != 500 || strings.Contains(w.Body.String(), "permission") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var corpo map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	exigirErroCom("ERRO_INTERNO", "Não deu para concluir. Tente de novo.")(t, corpo)
}

func TestStudioErroDoCliente(t *testing.T) {
	casos := []struct {
		nome      string
		err       error
		querNoLog bool
	}{
		{"cliente desistiu nao vai ao log", fmt.Errorf("prazo do catálogo: %w", context.Canceled), false},
		{"prazo estourado vai ao log", fmt.Errorf("prazo do catálogo: %w", context.DeadlineExceeded), true},
		{"arquivo sumido vai ao log", fmt.Errorf("ler fluxo: %w", fs.ErrNotExist), true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var registro strings.Builder
			s := &servidor{log: log.New(&registro, "", 0)}
			w := httptest.NewRecorder()
			s.responderErro(w, c.err)
			if w.Code != 500 || (registro.Len() > 0) != c.querNoLog {
				t.Fatalf("status %d, log %q", w.Code, registro.String())
			}
		})
	}
}

func TestComPrazoTransformaPanicoEmErro(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/skills", nil)
	_, err := comPrazo(r, func() (string, error) { panic("catálogo quebrado") })
	if err == nil || !strings.Contains(err.Error(), "catálogo quebrado") {
		t.Fatalf("err = %v", err)
	}
}

// handlerDeTeste monta as rotas do Studio sobre uma cópia do catálogo real.
func handlerDeTeste(t *testing.T) http.Handler {
	t.Helper()
	raiz := t.TempDir()
	for _, pasta := range []string{"agentes", "nexus"} {
		if err := os.CopyFS(filepath.Join(raiz, pasta), os.DirFS(filepath.Join("..", "..", "okf", pasta))); err != nil {
			t.Fatal(err)
		}
	}
	cat, err := okf.NewCatalogo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	recursos, err := okf.LerRecursosFixture(filepath.Join(raiz, "nexus", "cond-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cat.Recursos = func() []okf.Recurso { return recursos }
	s := &servidor{cat: cat, atende: &atende.Servico{Agente: "agentes/concierge"}, log: log.New(io.Discard, "", 0)}
	mux := http.NewServeMux()
	s.rotasStudio(mux)
	return mux
}

func chamar(t *testing.T, h http.Handler, p pedido) *httptest.ResponseRecorder {
	t.Helper()
	corpo := ""
	if p.corpo != nil {
		corpo = p.corpo(t, h)
	}
	r := httptest.NewRequest(p.metodo, p.caminho, strings.NewReader(corpo))
	for k, v := range p.cabecalhos {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func texto(s string) func(*testing.T, http.Handler) string {
	return func(*testing.T, http.Handler) string { return s }
}

func lerFluxo(t *testing.T, h http.Handler) okf.FluxoLido {
	t.Helper()
	var lido okf.FluxoLido
	lerJSON(t, h, skillMudanca+"/fluxo", &lido)
	return lido
}

func lerTextos(t *testing.T, h http.Handler) textosLidos {
	t.Helper()
	var lidos textosLidos
	lerJSON(t, h, skillMudanca, &lidos)
	return lidos
}

func lerJSON(t *testing.T, h http.Handler, caminho string, destino any) {
	t.Helper()
	w := chamar(t, h, pedido{metodo: "GET", caminho: caminho})
	if w.Code != 200 {
		t.Fatalf("GET %s: %d %s", caminho, w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), destino); err != nil {
		t.Fatal(err)
	}
}

func corpoJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// exigirCampos confere os nomes de campo que os schemas Zod do Studio leem.
func exigirCampos(t *testing.T, obj map[string]any, campos ...string) {
	t.Helper()
	for _, campo := range campos {
		if _, ok := obj[campo]; !ok {
			t.Fatalf("falta %q em %v", campo, obj)
		}
	}
}

func exigirErro(t *testing.T, corpo map[string]any, codigo string) {
	t.Helper()
	exigirCampos(t, corpo, "codigo", "mensagem")
	if corpo["codigo"] != codigo {
		t.Fatalf("codigo = %v, quer %s", corpo["codigo"], codigo)
	}
}

func exigirErroCom(codigo, mensagem string) func(*testing.T, map[string]any) {
	return func(t *testing.T, corpo map[string]any) {
		t.Helper()
		exigirErro(t, corpo, codigo)
		if corpo["mensagem"] != mensagem {
			t.Fatalf("mensagem = %v", corpo["mensagem"])
		}
		if _, temProblemas := corpo["problemas"]; temProblemas {
			t.Fatalf("problemas fora do 422: %v", corpo)
		}
	}
}

func exigirNome(nome string) func(*testing.T, map[string]any) {
	return func(t *testing.T, corpo map[string]any) {
		t.Helper()
		if corpo["nome"] != nome {
			t.Fatalf("nome = %v, quer %s", corpo["nome"], nome)
		}
	}
}
