package harness

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/okf"
)

func semEnv(string) string { return "" }

// catalogoExemplo abre okf/ do repositório e devolve o concierge.
func catalogoExemplo(t *testing.T) (*okf.Catalogo, *okf.Agent) {
	t.Helper()
	cat, err := okf.NewCatalogo(filepath.Join("..", "..", "okf"))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := cat.Tree()
	if err != nil {
		t.Fatal(err)
	}
	a := okf.Find(tree, "agentes/concierge")
	if a == nil {
		t.Fatal("agentes/concierge não encontrado em okf/")
	}
	return cat, a
}

func escreve(t *testing.T, root, rel, conteudo string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chama(t *testing.T, ag *Agente, nome string, args any) map[string]any {
	t.Helper()
	tl, ok := ag.Tool(nome)
	if !ok {
		t.Fatalf("tool %s não registrada; tools: %v", nome, ag.NomesDeTools())
	}
	raw, _ := json.Marshal(args)
	out, err := tl.InvokableRun(context.Background(), string(raw))
	if err != nil {
		t.Fatalf("%s: %v", nome, err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("%s devolveu algo que não é JSON: %s", nome, out)
	}
	return m
}

func TestInspecaoDoConcierge(t *testing.T) {
	cat, a := catalogoExemplo(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv})
	if err != nil {
		t.Fatal(err)
	}
	if ag.Instrucao != cat.Projecao(a) {
		t.Error("instrução deve ser a projeção do catálogo")
	}
	if !ag.Mock {
		t.Error("sem chave, o modelo deve ser o mock")
	}
	nomes := strings.Join(ag.NomesDeTools(), " ")
	for _, q := range []string{"ler_dado", "chamar_api"} {
		if !strings.Contains(nomes, q) {
			t.Errorf("concierge deveria ter a tool %s; tem: %s", q, nomes)
		}
	}
	if strings.Contains(nomes, "invocar_agente") {
		t.Error("orquestrador delega pela passada de rota, não por invocar_agente")
	}
	if ag.Roteador == "" {
		t.Error("orquestrador precisa do roteador")
	}
	reg, ok := ag.Subagentes["regimento"]
	if !ok {
		t.Fatal("subagente regimento não construído")
	}
	if _, ok := reg.Tool("ler_dado"); !ok {
		t.Errorf("regimento deveria ter ler_dado; tem %v", reg.NomesDeTools())
	}
}

func TestRoteiaParaRegimentoECitaArtigo(t *testing.T) {
	cat, a := catalogoExemplo(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), ag, "posso fazer barulho depois das 22h?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rota) == 0 || res.Rota[0].Tipo != "rota" || res.Rota[0].Detalhe != "regimento" {
		t.Fatalf("esperava a rota regimento primeiro; trace: %+v", res.Rota)
	}
	if res.Agente != "agentes/concierge/agentes/regimento" {
		t.Errorf("quem respondeu: %s", res.Agente)
	}
	if !strings.Contains(res.Resposta, "Art. 10") {
		t.Errorf("resposta deveria citar Art. 10; veio: %s", res.Resposta)
	}
	var viuLerDado bool
	for _, p := range res.Rota {
		if p.Tipo == "tool" && strings.HasPrefix(p.Detalhe, "ler_dado") {
			viuLerDado = true
		}
	}
	if !viuLerDado {
		t.Errorf("o subagente deveria ter lido o regimento com ler_dado; trace: %+v", res.Rota)
	}
	if res.Uso.Chamadas < 3 || res.Uso.Total == 0 {
		t.Errorf("uso deveria somar roteador + 2 passos do loop; veio %+v", res.Uso)
	}
	if len(res.Mensagens) < 3 {
		t.Errorf("histórico da execução curto demais: %d", len(res.Mensagens))
	}
}

func TestRotaSelfQuandoNadaCasa(t *testing.T) {
	cat, a := catalogoExemplo(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), ag, "xyzzy", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rota[0].Detalhe != "self" || res.Agente != "agentes/concierge" {
		t.Errorf("sem palavra-chave a rota é self e o concierge responde; veio %+v / %s", res.Rota[0], res.Agente)
	}
}

func TestLerDadoPagina(t *testing.T) {
	root := t.TempDir()
	escreve(t, root, "agentes/leitor/agent.yaml", "nome: leitor\ndescricao: lê\n")
	escreve(t, root, "agentes/leitor/workflows/principal.md", "# Workflow\n\n1. Leia `data/grande.md`.\n")
	grande := strings.Repeat("0123456789", 1500) // 15000 caracteres → 3 páginas
	escreve(t, root, "agentes/leitor/data/grande.md", grande)
	cat, err := okf.NewCatalogo(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := cat.Load(filepath.Join(root, "agentes", "leitor"))
	if err != nil {
		t.Fatal(err)
	}
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv})
	if err != nil {
		t.Fatal(err)
	}

	p1 := chama(t, ag, "ler_dado", map[string]any{"caminho": "data/grande.md"})
	if p1["paginas"] != float64(3) || p1["tem_mais"] != true || len(p1["conteudo"].(string)) != PaginaChars {
		t.Errorf("página 1: %v paginas, tem_mais=%v, %d chars", p1["paginas"], p1["tem_mais"], len(p1["conteudo"].(string)))
	}
	p3 := chama(t, ag, "ler_dado", map[string]any{"caminho": "data/grande.md", "pagina": 3})
	if p3["tem_mais"] != false || len(p3["conteudo"].(string)) != 3000 {
		t.Errorf("página 3: tem_mais=%v, %d chars", p3["tem_mais"], len(p3["conteudo"].(string)))
	}
	p2 := chama(t, ag, "ler_dado", map[string]any{"caminho": "data/grande.md", "pagina": 2})
	if p1["conteudo"].(string)+p2["conteudo"].(string)+p3["conteudo"].(string) != grande {
		t.Error("as páginas concatenadas devem reconstruir o arquivo")
	}
	if e := chama(t, ag, "ler_dado", map[string]any{"caminho": "data/grande.md", "pagina": 4}); e["error"] == nil {
		t.Error("página 4 deveria dar erro")
	}
	if e := chama(t, ag, "ler_dado", map[string]any{"caminho": "../../apis/nexus.yaml"}); e["error"] == nil {
		t.Error("caminho fora do índice deveria dar erro")
	}
}

// transporteProibido falha o teste se alguém tentar sair para a rede.
type transporteProibido struct{ t *testing.T }

func (tp transporteProibido) RoundTrip(r *http.Request) (*http.Response, error) {
	tp.t.Errorf("não deveria chamar a rede: %s %s", r.Method, r.URL)
	return nil, errors.New("rede proibida no teste")
}

func TestChamarAPIRecusaSemSegredo(t *testing.T) {
	cat, a := catalogoExemplo(t)
	ag, err := Build(context.Background(), cat, a, Config{
		Mock:       true,
		Env:        semEnv,
		HTTPClient: &http.Client{Transport: transporteProibido{t}},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := chama(t, ag, "chamar_api", map[string]any{"nome": "nexus", "metodo": "GET", "rota": "/v1/identity?phone=5511999990001"})
	msg, _ := out["error"].(string)
	if !strings.Contains(msg, "NEXUS_TOKEN") {
		t.Errorf("deveria recusar por falta de NEXUS_TOKEN; veio %v", out)
	}
	out = chama(t, ag, "chamar_api", map[string]any{"nome": "atende", "metodo": "GET", "rota": "/health"})
	if msg, _ := out["error"].(string); !strings.Contains(msg, "não está vinculada") {
		t.Errorf("api fora do agent.yaml deveria ser recusada; veio %v", out)
	}
}

type funcRoundTripper func(*http.Request) (*http.Response, error)

func (f funcRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestChamarAPIComSegredoResolveHeader(t *testing.T) {
	cat, a := catalogoExemplo(t)
	var visto *http.Request
	rt := func(r *http.Request) (*http.Response, error) {
		visto = r
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: http.NoBody}, nil
	}
	ag, err := Build(context.Background(), cat, a, Config{
		Mock: true,
		Env: func(k string) string {
			if k == "NEXUS_TOKEN" {
				return "segredo-de-teste"
			}
			return ""
		},
		HTTPClient: &http.Client{Transport: funcRoundTripper(rt)},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := chama(t, ag, "chamar_api", map[string]any{"nome": "nexus", "metodo": "get", "rota": "/v1/identity"})
	if out["error"] != nil || out["status"] != float64(200) {
		t.Fatalf("esperava 200 sem erro; veio %v", out)
	}
	if visto == nil || visto.Header.Get("Authorization") != "Bearer segredo-de-teste" || visto.URL.String() != "http://127.0.0.1:8081/v1/identity" {
		t.Errorf("requisição montada errada: %+v", visto)
	}
}

func TestInvocarAgenteEStubs(t *testing.T) {
	root := t.TempDir()
	escreve(t, root, "agentes/chefe/agent.yaml", "nome: chefe\ntipo: operador\ndescricao: operador com ajudante\n")
	escreve(t, root, "agentes/chefe/workflows/principal.md", "# Workflow\n\n1. Delegue ao `ajudante`.\n")
	escreve(t, root, "agentes/chefe/scripts/listar.sh", "#!/bin/sh\nls\n")
	escreve(t, root, "agentes/chefe/mcps/nexus.yaml", "nome: nexus\ntransporte: http\n")
	escreve(t, root, "agentes/chefe/agentes/ajudante/agent.yaml", "nome: ajudante\ndescricao: ajuda\n")
	escreve(t, root, "agentes/chefe/agentes/ajudante/workflows/principal.md", "# Workflow\n\n1. Leia `data/notas.md`.\n")
	escreve(t, root, "agentes/chefe/agentes/ajudante/rules/output.md", "# Contrato\n\n## Resposta\numa linha\n")
	escreve(t, root, "agentes/chefe/agentes/ajudante/data/notas.md", "# Notas\n\n## Piscina\n\nA piscina abre às 8h.\n\n## Salão\n\nO salão fecha às 23h.\n")
	cat, err := okf.NewCatalogo(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := cat.Load(filepath.Join(root, "agentes", "chefe"))
	if err != nil {
		t.Fatal(err)
	}
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv})
	if err != nil {
		t.Fatal(err)
	}
	nomes := strings.Join(ag.NomesDeTools(), " ")
	for _, q := range []string{"invocar_agente", "rodar_script", "mcp_nexus"} {
		if !strings.Contains(nomes, q) {
			t.Errorf("faltou a tool %s em %s", q, nomes)
		}
	}
	out := chama(t, ag, "invocar_agente", map[string]any{"nome": "ajudante", "mensagem": "a que horas o salão fecha?"})
	if out["agente"] != "ajudante" || !strings.Contains(out["resposta"].(string), "23h") {
		t.Errorf("subagente deveria responder pela seção do salão; veio %v", out)
	}
	if e := chama(t, ag, "rodar_script", map[string]any{"nome": "listar.sh"}); !strings.Contains(e["error"].(string), "ainda não suportado") {
		t.Errorf("stub deveria dizer que não suporta; veio %v", e)
	}
	if e := chama(t, ag, "invocar_agente", map[string]any{"nome": "ninguem", "mensagem": "oi"}); e["error"] == nil {
		t.Error("subagente inexistente deveria dar erro")
	}
}

func TestProfundidadeMaxima(t *testing.T) {
	// cadeia n1 → n2 → n3 → n4 → n5 → n6: todos orquestradores com a mesma rota
	// "fundo"; no nível 4 o flip para e o agente responde como self.
	root := t.TempDir()
	rel := "agentes/n1"
	for i := 1; i <= 6; i++ {
		escreve(t, root, rel+"/agent.yaml", "nome: n"+string(rune('0'+i))+"\ndescricao: nível\n")
		escreve(t, root, rel+"/workflows/principal.md", "# Workflow\n\n1. Responda.\n")
		if i < 6 {
			escreve(t, root, rel+"/routers/rotas.md", "# Roteamento\n\n- Pergunta sobre o fundo do poço → rota `n"+string(rune('1'+i))+"`.\n- Qualquer outra coisa → rota `self`.\n")
			rel += "/agentes/n" + string(rune('1'+i))
		}
	}
	cat, err := okf.NewCatalogo(root)
	if err != nil {
		t.Fatal(err)
	}
	a, err := cat.Load(filepath.Join(root, "agentes", "n1"))
	if err != nil {
		t.Fatal(err)
	}
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(context.Background(), ag, "qual é o fundo do poço?", nil)
	if err != nil {
		t.Fatal(err)
	}
	flips := 0
	for _, p := range res.Rota {
		if p.Tipo == "rota" {
			flips++
		}
	}
	if flips != ProfundidadeMax || !strings.HasSuffix(res.Agente, "/n5") {
		t.Errorf("esperava %d flips parando em n5; flips=%d agente=%s trace=%+v", ProfundidadeMax, flips, res.Agente, res.Rota)
	}
}

func TestMockEscolheRotaPorGlossario(t *testing.T) {
	cat, a := catalogoExemplo(t)
	m := NewMock(cat, a)
	if r := m.escolherRota("posso fazer barulho depois das 22h?"); r != "regimento" {
		t.Errorf("barulho → regimento; veio %s", r)
	}
	if r := m.escolherRota("até que horas a piscina abre? qual o horário da portaria?"); r != "faq" {
		t.Errorf("horários → faq; veio %s", r)
	}
	if r := m.escolherRota("vou fazer uma reforma com pintor na sala"); r != "regimento" {
		t.Errorf("reforma (glossário: obra → regimento); veio %s", r)
	}
	if r := m.escolherRota("obrigado, era só isso, pode encerrar a conversa"); r != "observador-sessao" {
		t.Errorf("despedida → observador-sessao; veio %s", r)
	}
}
