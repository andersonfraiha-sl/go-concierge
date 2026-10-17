package harness

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/nexus"
	"go-concierge/internal/okf"
)

func semEnv(string) string { return "" }

// catalogoExemplo abre okf/ do repositório com o instantâneo do Nexus da cond-001
// e devolve o concierge.
func catalogoExemplo(t *testing.T) (*okf.Catalogo, *okf.Agent) {
	t.Helper()
	cat, err := okf.NewCatalogo(filepath.Join("..", "..", "okf"))
	if err != nil {
		t.Fatal(err)
	}
	cat.Recursos = nexus.Fonte(nil, "cond-001", filepath.Join(cat.Root, "nexus"))
	if cat.Recursos == nil {
		t.Fatal("instantâneo okf/nexus/cond-001.yaml não encontrado")
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

func erroDe(out map[string]any) string {
	s, _ := out["error"].(string)
	return s
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
	for _, q := range []string{"ler_dado", "chamar_recurso"} {
		if !strings.Contains(nomes, q) {
			t.Errorf("concierge deveria ter a tool %s; tem: %s", q, nomes)
		}
	}
	for _, q := range []string{"invocar_agente", "chamar_api", "consultar_schema", "rodar_"} {
		if strings.Contains(nomes, q) {
			t.Errorf("concierge não deveria ter %s; tem: %s", q, nomes)
		}
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
	if _, ok := reg.Tool("chamar_recurso"); ok {
		t.Error("regimento não declara recursos; não deveria ter chamar_recurso")
	}
	lib, ok := ag.Subagentes["liberacao-visitante"]
	if !ok {
		t.Fatal("subagente liberacao-visitante não construído")
	}
	if _, ok := lib.Tool("chamar_recurso"); !ok {
		t.Errorf("liberacao-visitante declara recursos; deveria ter chamar_recurso; tem %v", lib.NomesDeTools())
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
	var leuRegimento bool
	for _, p := range res.Rota {
		if p.Tipo == "tool" && strings.HasPrefix(p.Detalhe, "ler_dado") && strings.Contains(p.Detalhe, "knowledge/regimento.md") {
			leuRegimento = true
		}
	}
	if !leuRegimento {
		t.Errorf("o subagente deveria ter lido knowledge/regimento.md com ler_dado; trace: %+v", res.Rota)
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
	escreve(t, root, "agentes/leitor/workflows/principal.md", "# Workflow\n\n1. Leia `knowledge/grande.md`.\n")
	grande := strings.Repeat("0123456789", 1500) // 15000 caracteres → 3 páginas
	escreve(t, root, "agentes/leitor/knowledge/grande.md", grande)
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

	p1 := chama(t, ag, "ler_dado", map[string]any{"caminho": "knowledge/grande.md"})
	if p1["paginas"] != float64(3) || p1["tem_mais"] != true || len(p1["conteudo"].(string)) != PaginaChars {
		t.Errorf("página 1: %v paginas, tem_mais=%v, %d chars", p1["paginas"], p1["tem_mais"], len(p1["conteudo"].(string)))
	}
	p3 := chama(t, ag, "ler_dado", map[string]any{"caminho": "knowledge/grande.md", "pagina": 3})
	if p3["tem_mais"] != false || len(p3["conteudo"].(string)) != 3000 {
		t.Errorf("página 3: tem_mais=%v, %d chars", p3["tem_mais"], len(p3["conteudo"].(string)))
	}
	p2 := chama(t, ag, "ler_dado", map[string]any{"caminho": "knowledge/grande.md", "pagina": 2})
	if p1["conteudo"].(string)+p2["conteudo"].(string)+p3["conteudo"].(string) != grande {
		t.Error("as páginas concatenadas devem reconstruir o arquivo")
	}
	if e := chama(t, ag, "ler_dado", map[string]any{"caminho": "knowledge/grande.md", "pagina": 4}); erroDe(e) == "" {
		t.Error("página 4 deveria dar erro")
	}
	if e := chama(t, ag, "ler_dado", map[string]any{"caminho": "../../nexus/cond-001.yaml"}); erroDe(e) == "" {
		t.Error("caminho fora do índice deveria dar erro")
	}
}

// nexusFalso sobe um Nexus de mentira que registra o último invoke.
func nexusFalso(t *testing.T) (*nexus.Client, *map[string]any) {
	t.Helper()
	var ultimo map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/invoke") || r.Method != http.MethodPost {
			http.Error(w, "rota inesperada "+r.Method+" "+r.URL.Path, 404)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		ultimo = map[string]any{"path": r.URL.Path, "auth": r.Header.Get("Authorization")}
		var corpo map[string]any
		_ = json.Unmarshal(raw, &corpo)
		for k, v := range corpo {
			ultimo[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"u1","label":"Bloco B - Apto 305"}]`))
	}))
	t.Cleanup(srv.Close)
	return nexus.New(srv.URL, "tok-teste"), &ultimo
}

func TestChamarRecursoRecusaEscritaENaoDeclarado(t *testing.T) {
	cat, a := catalogoExemplo(t)
	cli, _ := nexusFalso(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, Nexus: cli, Identidade: Identidade{Authenticated: true, AccountID: "cond-001", Role: "morador"}})
	if err != nil {
		t.Fatal(err)
	}
	lib := ag.Subagentes["liberacao-visitante"]
	out := chama(t, lib, "chamar_recurso", map[string]any{"nome": "visitantes.liberar", "params": map[string]any{"unidade_id": "u1", "nome": "Ana", "telefone": "5511999990001"}})
	if !strings.Contains(erroDe(out), "escrita só pelo caso de uso") {
		t.Errorf("escrita deveria ser recusada; veio %v", out)
	}
	out = chama(t, lib, "chamar_recurso", map[string]any{"nome": "areas.listar"})
	if !strings.Contains(erroDe(out), "não está declarado") {
		t.Errorf("recurso não declarado deveria ser recusado; veio %v", out)
	}
	out = chama(t, lib, "chamar_recurso", map[string]any{"nome": "inexistente.recurso"})
	if erroDe(out) == "" {
		t.Errorf("recurso inexistente deveria ser recusado; veio %v", out)
	}
}

func TestChamarRecursoPrivadoParaContatoPublico(t *testing.T) {
	cat, a := catalogoExemplo(t)
	cli, ultimo := nexusFalso(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, Nexus: cli}) // identidade zero = público
	if err != nil {
		t.Fatal(err)
	}
	out := chama(t, ag, "chamar_recurso", map[string]any{"nome": "unidades.listar"})
	if !strings.Contains(erroDe(out), "privado") {
		t.Errorf("recurso private deveria ser recusado para contato público; veio %v", out)
	}
	if *ultimo != nil {
		t.Error("não deveria ter chamado o Nexus")
	}
	// público continua permitido
	out = chama(t, ag, "chamar_recurso", map[string]any{"nome": "faq.buscar", "params": map[string]any{"query": "piscina"}})
	if erroDe(out) != "" || out["recurso"] != "faq.buscar" {
		t.Errorf("faq.buscar é public e deveria passar; veio %v", out)
	}
	if (*ultimo)["account_id"] != "cond-001" || (*ultimo)["auth"] != "Bearer tok-teste" {
		t.Errorf("invoke montado errado: %v", *ultimo)
	}
	// params obrigatórios
	out = chama(t, ag, "chamar_recurso", map[string]any{"nome": "faq.buscar"})
	if !strings.Contains(erroDe(out), "query") {
		t.Errorf("faltando query deveria recusar antes de chamar; veio %v", out)
	}
}

func TestChamarRecursoAutenticadoEIdempotencia(t *testing.T) {
	cat, a := catalogoExemplo(t)
	cli, ultimo := nexusFalso(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, Nexus: cli, Sessao: "thr-1", Identidade: Identidade{Authenticated: true, AccountID: "cond-001", Role: "morador"}})
	if err != nil {
		t.Fatal(err)
	}
	out := chama(t, ag, "chamar_recurso", map[string]any{"nome": "unidades.listar"})
	if erroDe(out) != "" {
		t.Fatalf("morador identificado deveria listar unidades; veio %v", out)
	}
	lista, _ := out["resultado"].([]any)
	if len(lista) != 1 {
		t.Errorf("resultado deveria ser a lista do Nexus; veio %v", out["resultado"])
	}
	idem, _ := (*ultimo)["idempotency_key"].(string)
	if idem != "thr-1:concierge:unidades.listar:1" {
		t.Errorf("chave de idempotência: %q", idem)
	}
	if (*ultimo)["path"] != "/v1/resources/unidades.listar/invoke" {
		t.Errorf("rota do invoke: %v", (*ultimo)["path"])
	}
}

func TestChamarRecursoSemNexus(t *testing.T) {
	cat, a := catalogoExemplo(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv}) // Nexus nil
	if err != nil {
		t.Fatal(err)
	}
	out := chama(t, ag, "chamar_recurso", map[string]any{"nome": "faq.buscar", "params": map[string]any{"query": "x"}})
	if !strings.Contains(erroDe(out), "Nexus indisponível") {
		t.Errorf("sem cliente deveria dizer Nexus indisponível; veio %v", out)
	}
}

func TestInvocarAgente(t *testing.T) {
	root := t.TempDir()
	escreve(t, root, "agentes/chefe/agent.yaml", "nome: chefe\ntipo: operador\ndescricao: operador com ajudante\n")
	escreve(t, root, "agentes/chefe/workflows/principal.md", "# Workflow\n\n1. Delegue ao `ajudante`.\n")
	escreve(t, root, "agentes/chefe/agentes/ajudante/agent.yaml", "nome: ajudante\ndescricao: ajuda\n")
	escreve(t, root, "agentes/chefe/agentes/ajudante/workflows/principal.md", "# Workflow\n\n1. Leia `knowledge/notas.md`.\n")
	escreve(t, root, "agentes/chefe/agentes/ajudante/rules/output.md", "# Contrato\n\n## Resposta\numa linha\n")
	escreve(t, root, "agentes/chefe/agentes/ajudante/knowledge/notas.md", "# Notas\n\n## Piscina\n\nA piscina abre às 8h.\n\n## Salão\n\nO salão fecha às 23h.\n")
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
	if nomes := strings.Join(ag.NomesDeTools(), " "); !strings.Contains(nomes, "invocar_agente") || strings.Contains(nomes, "chamar_recurso") {
		t.Errorf("tools do chefe: %s", nomes)
	}
	out := chama(t, ag, "invocar_agente", map[string]any{"nome": "ajudante", "mensagem": "a que horas o salão fecha?"})
	if out["agente"] != "ajudante" || !strings.Contains(out["resposta"].(string), "23h") {
		t.Errorf("subagente deveria responder pela seção do salão; veio %v", out)
	}
	if e := chama(t, ag, "invocar_agente", map[string]any{"nome": "ninguem", "mensagem": "oi"}); erroDe(e) == "" {
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

func TestSemRotasTiraOItemInteiro(t *testing.T) {
	texto := "# Roteamento\n\n- Regras do regimento (barulho, obras),\n  \"o que diz o regimento\" → rota `regimento`.\n- Dúvida geral → rota `faq`.\n- Liberar visitante, amigo ou\n  prestador → rota `liberacao-visitante`.\n- Qualquer outra coisa → rota `self`.\n"
	r := semRotas(texto, []string{"liberacao-visitante", "regimento"})
	if strings.Contains(r, "liberacao-visitante") || strings.Contains(r, "regimento") || !strings.Contains(r, "rota `faq`") || !strings.Contains(r, "rota `self`") || !strings.Contains(r, "# Roteamento") {
		t.Fatalf("semRotas:\n%s", r)
	}
	if semRotas("", []string{"x"}) != "" || semRotas(texto, nil) != texto {
		t.Fatal("casos vazios")
	}
}

func TestCasosHabilitadosFiltramSubagentes(t *testing.T) {
	cat, a := catalogoExemplo(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, Casos: func(nome, casoID string) bool { return nome != "liberacao-visitante" }})
	if err != nil {
		t.Fatal(err)
	}
	if _, tem := ag.Subagentes["liberacao-visitante"]; tem {
		t.Fatal("caso desabilitado não deveria ser montado")
	}
	if strings.Contains(ag.Roteador, "liberacao-visitante") || strings.Contains(ag.Instrucao, "`liberacao-visitante`") {
		t.Fatalf("roteador/instrução ainda citam o caso desabilitado:\n%s", ag.Roteador)
	}
	if _, tem := ag.Subagentes["regimento"]; !tem {
		t.Fatal("os demais continuam")
	}
}

func TestRecursoLocalNaoDependeDoNexus(t *testing.T) {
	cat, a := catalogoExemplo(t)
	cfg := Config{Mock: true, Env: semEnv, Identidade: Identidade{Authenticated: true, AccountID: "cond-001", Role: "morador"},
		Locais: map[string]func(context.Context, map[string]any) (any, error){
			"unidades.listar": func(context.Context, map[string]any) (any, error) {
				return []map[string]any{{"id": "109", "label": "Jataí - Unidade Operacao Predial"}}, nil
			},
		}}
	ag, err := Build(context.Background(), cat, a, cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ag.chamarRecurso(context.Background(), &argsRecurso{Nome: "unidades.listar"})
	if err != nil {
		t.Fatal(err)
	}
	if out["error"] != nil || out["origem"] != "identidade" {
		t.Fatalf("o recurso local deveria responder sem Nexus: %+v", out)
	}
	lista, _ := out["resultado"].([]map[string]any)
	if len(lista) != 1 || lista[0]["id"] != "109" {
		t.Fatalf("resultado: %+v", out["resultado"])
	}
	// sem o local, o mesmo pedido cai no "Nexus indisponível"
	cfg.Locais = nil
	ag2, _ := Build(context.Background(), cat, a, cfg)
	if out, _ := ag2.chamarRecurso(context.Background(), &argsRecurso{Nome: "unidades.listar"}); out["error"] == nil {
		t.Fatalf("sem Nexus e sem local deveria dar erro: %+v", out)
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
	if r := m.escolherRota("vou fazer uma reforma com pintor na sala"); r != "comunicar-obra" {
		t.Errorf("reforma → comunicar-obra (caso de uso trazido do catálogo); veio %s", r)
	}
	if r := m.escolherRota("quero liberar a entrada de um visitante"); r != "liberacao-visitante" {
		t.Errorf("visitante → liberacao-visitante; veio %s", r)
	}
	if r := m.escolherRota("obrigado, era só isso, pode encerrar a conversa"); r != "self" {
		t.Errorf("despedida → self (o observador nunca é rota); veio %s", r)
	}
}
