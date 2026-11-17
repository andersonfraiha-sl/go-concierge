package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/atende"
	"go-concierge/internal/okf"
)

const skillMudanca = "/api/skills/agendar-mudanca"

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
			nome: "recusa visibilidade invalida",
			pedido: pedido{metodo: "PUT", caminho: skillMudanca + "/textos", corpo: func(t *testing.T, h http.Handler) string {
				lidos := lerTextos(t, h)
				lidos.Textos.Visibilidade = "todos"
				return corpoJSON(t, lidos)
			}},
			querStatus: 422,
			conferir:   exigirErroCom("VISIBILIDADE_INVALIDA", "Escolha quem pode usar a skill."),
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

func TestSkillsReaisTemResumo(t *testing.T) {
	h := handlerDeTeste(t)
	var corpo skillsDoAssistente
	lerJSON(t, h, "/api/skills", &corpo)
	for _, skill := range corpo.Skills {
		runas := len([]rune(skill.Resumo))
		if skill.Resumo == "" || runas > okf.LimiteDoResumo {
			t.Errorf("%s: resumo = %q (%d runas)", skill.Nome, skill.Resumo, runas)
		}
		if skill.Resumo == skill.Descricao {
			t.Errorf("%s: resumo igual à descrição", skill.Nome)
		}
	}
}

func TestSkillSemCasoSemResumoFicaVazio(t *testing.T) {
	h := handlerDeTesteComAjuste(t, func(raiz string) {
		apagarLinha(t, filepath.Join(raiz, "agentes/concierge/agentes/regimento/agent.yaml"), "resumo:")
	})
	var corpo skillsDoAssistente
	lerJSON(t, h, "/api/skills", &corpo)
	skill, ok := skillPorNome(corpo, "regimento")
	if !ok {
		t.Fatal("regimento não apareceu em /api/skills")
	}
	if skill.Resumo != "" {
		t.Errorf("resumo = %q, quer vazio", skill.Resumo)
	}
}

func TestCasoSemSummaryDerivaResumo(t *testing.T) {
	h := handlerDeTesteComAjuste(t, func(raiz string) {
		apagarLinha(t, filepath.Join(raiz, "agentes/concierge/agentes/listar-unidades/caso.yaml"), "summary:")
	})
	var corpo skillsDoAssistente
	lerJSON(t, h, "/api/skills", &corpo)
	skill, ok := skillPorNome(corpo, "listar-unidades")
	if !ok {
		t.Fatal("listar-unidades não apareceu em /api/skills")
	}
	quer := "Mostra as unidades (apartamentos/casas) vinculadas ao cadastro de quem pergunta."
	if skill.Resumo != quer {
		t.Errorf("resumo = %q, quer %q", skill.Resumo, quer)
	}
}

func TestSkillsTrazemIcone(t *testing.T) {
	h := handlerDeTeste(t)
	var corpo skillsDoAssistente
	lerJSON(t, h, "/api/skills", &corpo)
	mudanca, ok := skillPorNome(corpo, "agendar-mudanca")
	if !ok || mudanca.Icone != "truck" {
		t.Errorf("agendar-mudanca.icone = %q", mudanca.Icone)
	}
	faq, ok := skillPorNome(corpo, "faq")
	if !ok || faq.Icone != "" {
		t.Errorf("faq.icone = %q", faq.Icone)
	}
}

func skillPorNome(corpo skillsDoAssistente, nome string) (skillResumo, bool) {
	i := slices.IndexFunc(corpo.Skills, func(s skillResumo) bool { return s.Nome == nome })
	if i < 0 {
		return skillResumo{}, false
	}
	return corpo.Skills[i], true
}

// apagarLinha remove toda linha do arquivo que começa (sem os espaços) com o prefixo dado.
func apagarLinha(t *testing.T, caminho, prefixo string) {
	t.Helper()
	raw, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	linhas := strings.Split(string(raw), "\n")
	restantes := make([]string, 0, len(linhas))
	for _, l := range linhas {
		if !strings.HasPrefix(strings.TrimSpace(l), prefixo) {
			restantes = append(restantes, l)
		}
	}
	if err := os.WriteFile(caminho, []byte(strings.Join(restantes, "\n")), 0o644); err != nil {
		t.Fatal(err)
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
			registro := capturarSlog(t)
			s := &servidor{log: log.New(io.Discard, "", 0)}
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
	_, err := comPrazo(r, prazoCatalogo, func() (string, error) { panic("catálogo quebrado") })
	if err == nil || !strings.Contains(err.Error(), "catálogo quebrado") {
		t.Fatalf("err = %v", err)
	}
}

// O teto de parede vale só com prazo: sem ele (handler de teste) o trabalho lento termina e
// responde, em vez de virar "prazo do catálogo" numa máquina carregada.
func TestComPrazoSoCortaComPrazo(t *testing.T) {
	tests := []struct {
		nome    string
		prazo   time.Duration
		querErr error
	}{
		{"com prazo, trabalho preso estoura", time.Millisecond, context.DeadlineExceeded},
		{"sem prazo, trabalho lento termina", 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			solta := make(chan struct{})
			if tt.prazo == 0 {
				time.AfterFunc(20*time.Millisecond, func() { close(solta) })
			} else {
				defer close(solta)
			}
			r := httptest.NewRequest("GET", "/api/skills", nil)
			v, err := comPrazo(r, tt.prazo, func() (string, error) { <-solta; return "lido", nil })
			if !errors.Is(err, tt.querErr) || (tt.querErr == nil && v != "lido") {
				t.Fatalf("v, err = %q, %v; quer err %v", v, err, tt.querErr)
			}
		})
	}
}

// handlerDeTeste monta as rotas do Studio sobre uma cópia do catálogo real.
func handlerDeTeste(t *testing.T) http.Handler {
	t.Helper()
	return handlerDeTesteComAjuste(t, func(string) {})
}

// handlerDeTesteComAjuste é handlerDeTeste, mas deixa mexer num arquivo da cópia (regressão
// de resumo/ícone) antes de montar as rotas.
func handlerDeTesteComAjuste(t *testing.T, ajustar func(raiz string)) http.Handler {
	t.Helper()
	s := servidorDeTeste(t, ajustar)
	mux := http.NewServeMux()
	s.rotasStudio(mux)
	return mux
}

// servidorDeTeste monta um servidor sobre uma cópia do catálogo real, sem montar rotas:
// quem chama decide quais rotasX registrar (Studio, Admin, ...).
func servidorDeTeste(t *testing.T, ajustar func(raiz string)) *servidor {
	t.Helper()
	raiz := t.TempDir()
	for _, pasta := range []string{"agentes", "nexus"} {
		if err := os.CopyFS(filepath.Join(raiz, pasta), os.DirFS(filepath.Join("..", "..", "okf", pasta))); err != nil {
			t.Fatal(err)
		}
	}
	ajustar(raiz)
	cat, err := okf.NewCatalogo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	recursos, err := okf.LerRecursosFixture(filepath.Join(raiz, "nexus", "cond-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cat.Recursos = func() []okf.Recurso { return recursos }
	return &servidor{cat: cat, atende: &atende.Servico{Agente: "agentes/concierge"}, log: log.New(io.Discard, "", 0),
		acessos: acessosFake{atribuicoes: usuariosTeste(), mapeamento: mapeamentoTeste(t)}, autenticador: autenticadorDev{padrao: "admin"}}
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
