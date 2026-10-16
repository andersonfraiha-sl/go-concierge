package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// montaFamilia cria um catálogo temporário com um pai orquestrador e dois filhos.
func montaFamilia(t *testing.T) (*Catalogo, string) {
	t.Helper()
	root := t.TempDir()
	escreve := func(rel, conteudo string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escreve("agentes/pai/agent.yaml", "nome: pai\ndescricao: orquestra o condomínio\n")
	escreve("agentes/pai/routers/rotas.md", "# Roteamento\n\n- Dúvida sobre o regimento → rota `regimento`.\n- Qualquer outra coisa → rota `self`.\n")
	escreve("agentes/pai/data/consultas.md", "---\ndescription: consultas prontas\n---\n# Consultas\n")
	escreve("agentes/pai/data/okf/glossary.yaml", "terms:\n  - id: unidade\n    display_name: Unidade\n    synonyms: [apartamento, apê]\n    definition: a unidade do morador\n    labels: [cadastro]\n")
	escreve("agentes/pai/schemas/dw.yaml", "tabelas: {}\n")
	escreve("agentes/pai/agentes/regimento/agent.yaml", "nome: regimento\ndescricao: responde pelo regimento\nfontes: [../../data]\n")
	escreve("agentes/pai/agentes/regimento/workflows/principal.md", "# Workflow\n\n1. Leia `data/regimento.md` (ler_dado).\n2. Use `assets/rodape.md` no fim.\n3. Consulte `fonte:data/consultas.md`.\n4. Nunca leia `data/inexistente.md`.\n")
	escreve("agentes/pai/agentes/regimento/rules/output.md", "# Contrato\n\n## Resposta\n")
	escreve("agentes/pai/agentes/regimento/data/regimento.md", "# Regimento\n\nArt. 1\n")
	escreve("agentes/pai/agentes/regimento/data/okf/questions/silencio.md", "---\ntype: Question\ntitle: Pergunta — silêncio\nanswers: [PERMITIDO, PROIBIDO]\n---\n## Answers\n")
	escreve("agentes/pai/agentes/regimento/assets/rodape.md", "— administração\n")
	escreve("agentes/pai/agentes/faq/agent.yaml", "nome: faq\ndescricao: dúvidas gerais\n")
	escreve("agentes/pai/agentes/faq/workflows/principal.md", "# Workflow\n\n1. Leia `data/regimento.md`.\n")
	escreve("apis/nexus.yaml", "nome: nexus\ndescricao: recursos do condomínio\nbase_url: http://nexus\nrequer: [NEXUS_TOKEN]\nmanual: |\n  POST /v1/resources/{name}/invoke\n")
	escreve("bases/portaria/base.yaml", "nome: portaria\ntipo: rest\ndescricao: acesso à portaria\nsegredos: [PORTARIA_TOKEN]\napis: [nexus]\n")
	escreve("bases/portaria/schemas/portaria.yaml", "tabelas: {}\n")
	escreve("tools/assets/rodape-oficial.md", "— Administração do condomínio\n")
	c, err := NewCatalogo(root)
	if err != nil {
		t.Fatal(err)
	}
	return c, root
}

func TestArvoreHerancaETipo(t *testing.T) {
	c, _ := montaFamilia(t)
	tree, err := c.Tree()
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Nome != "pai" {
		t.Fatalf("esperava só o pai na raiz, veio %+v", tree)
	}
	pai := tree[0]
	if pai.Tipo != Orquestrador || pai.TipoDeclarado {
		t.Fatalf("pai deveria ser orquestrador inferido (routers + subagentes), veio %s", pai.Tipo)
	}
	if len(pai.Subagentes) != 2 {
		t.Fatalf("esperava 2 subagentes, veio %d", len(pai.Subagentes))
	}
	reg := Find(tree, "agentes/pai/agentes/regimento")
	if reg == nil || reg.Tipo != Operador {
		t.Fatalf("regimento deveria ser operador: %+v", reg)
	}
	// herança: schemas e data do pai aparecem como herdados; o próprio data não é repetido
	if len(reg.Herdados["schemas"]) != 1 || reg.Herdados["schemas"][0].HerdadoDe != "pai" {
		t.Fatalf("schemas herdados: %+v", reg.Herdados["schemas"])
	}
	for _, f := range reg.Herdados["data"] {
		if f.Caminho == "data/regimento.md" {
			t.Fatal("o próprio data/regimento.md não pode vir como herdado")
		}
	}
	// rules não herda
	if len(reg.Herdados["rules"]) != 0 {
		t.Fatal("rules não deveria herdar")
	}
}

func TestFronteira(t *testing.T) {
	_, root := montaFamilia(t)
	reg := filepath.Join(root, "agentes", "pai", "agentes", "regimento")
	casos := map[string]bool{
		filepath.Join(reg, "data", "x.md"):                                               true,
		filepath.Join(root, "agentes", "pai", "data"):                                    true,
		filepath.Join(root, "agentes", "pai", "agentes", "faq", "data"):                  false, // irmão
		filepath.Join(root, "bases"):                                                     false, // fora da árvore
		filepath.Join(root, "agentes", "pai", "agentes"):                                 false, // a área dos filhos do pai
		filepath.Join(root, "agentes", "pai", "agentes", "regimento", "agentes", "neto"): true,
	}
	for alvo, esperado := range casos {
		if got := Permitido(reg, alvo); got != esperado {
			t.Errorf("Permitido(%s) = %v, esperava %v", alvo, got, esperado)
		}
	}
	if anc := Ancestrais(reg); len(anc) != 1 || filepath.Base(anc[0]) != "pai" {
		t.Fatalf("ancestrais: %v", anc)
	}
}

func TestVinculosEProblemas(t *testing.T) {
	c, _ := montaFamilia(t)
	tree, _ := c.Tree()
	reg := Find(tree, "agentes/pai/agentes/regimento")
	vs := c.Vinculos(reg)
	estado := map[string]Vinculo{}
	for _, v := range vs {
		estado[v.Tipo+":"+v.Alvo] = v
	}
	if v := estado["dado:data/regimento.md"]; !v.OK || v.Resolvido != "próprio" {
		t.Errorf("data/regimento.md deveria resolver como próprio: %+v", v)
	}
	if v := estado["asset:assets/rodape.md"]; !v.OK {
		t.Errorf("assets/rodape.md deveria resolver: %+v", v)
	}
	if v := estado["fonte:fonte:data/consultas.md"]; !v.OK {
		t.Errorf("fonte:data/consultas.md deveria resolver pela fonte: %+v", v)
	}
	if v := estado["dado:data/inexistente.md"]; v.OK || v.Problema == "" {
		t.Errorf("data/inexistente.md deveria quebrar: %+v", v)
	}
	if v := estado["fonte:../../data"]; !v.OK {
		t.Errorf("fontes ../../data deveria ser permitida: %+v", v)
	}
	// o faq cita data/regimento.md, que existe só no IRMÃO: quebra com dica de fronteira
	faq := Find(tree, "agentes/pai/agentes/faq")
	achou := false
	for _, v := range c.Vinculos(faq) {
		if v.Alvo == "data/regimento.md" {
			achou = true
			if v.OK || !strings.Contains(v.Problema, "fronteira") {
				t.Errorf("deveria apontar a fronteira: %+v", v)
			}
		}
	}
	if !achou {
		t.Fatal("vínculo do faq não encontrado")
	}
	// rotas do pai
	pai := tree[0]
	for _, v := range c.Vinculos(pai) {
		if v.Tipo == "rota" && v.Alvo == "regimento" && !v.OK {
			t.Errorf("rota regimento deveria resolver: %+v", v)
		}
	}
	// problemas: o regimento tem um vínculo quebrado (erro) e falta politicas (nenhum aviso obrigatório sobre isso)
	erros := 0
	for _, p := range reg.Problemas {
		if p.Nivel == "erro" {
			erros++
		}
	}
	if erros != 1 {
		t.Errorf("esperava 1 erro no regimento, veio %d: %+v", erros, reg.Problemas)
	}
}

func TestConhecimentoOKF(t *testing.T) {
	c, _ := montaFamilia(t)
	tree, _ := c.Tree()
	reg := Find(tree, "agentes/pai/agentes/regimento")
	k := c.Conhecimento(reg)
	if len(k.Docs) != 1 || k.Docs[0].Tipo != "Question" || len(k.Docs[0].Respostas) != 2 {
		t.Fatalf("docs OKF: %+v", k.Docs)
	}
	if len(k.Termos) != 1 || k.Termos[0].ID != "unidade" || k.Termos[0].HerdadoDe != "pai" {
		t.Fatalf("termos: %+v", k.Termos)
	}
}

func TestProjecao(t *testing.T) {
	c, _ := montaFamilia(t)
	tree, _ := c.Tree()
	reg := Find(tree, "agentes/pai/agentes/regimento")
	p := c.Projecao(reg)
	for _, trecho := range []string{"OPERADOR", "# Workflow", "# Contrato de saída", "`data/regimento.md`", "[herdado de pai]", "`fonte:data/consultas.md`", "consultar_schema", "# Regras gerais"} {
		if !strings.Contains(p, trecho) {
			t.Errorf("projeção sem %q:\n%s", trecho, p)
		}
	}
	pai := c.Projecao(tree[0])
	if !strings.Contains(pai, "ORQUESTRADOR") || !strings.Contains(pai, "Rotas de delegação") {
		t.Errorf("projeção do pai:\n%s", pai)
	}
}

func TestCriarMoverClonarExcluir(t *testing.T) {
	c, root := montaFamilia(t)
	rel, mud, err := c.CriarAgente(NovoAgente{Pai: "agentes/pai", Nome: "observador", Tipo: Operador, Descricao: "observa a sessão e segmenta assuntos", Caixinhas: []string{"rules", "workflows", "data"}})
	if err != nil {
		t.Fatal(err)
	}
	if rel != "agentes/pai/agentes/observador" {
		t.Fatalf("rel: %s", rel)
	}
	rotas, _ := os.ReadFile(filepath.Join(root, "agentes", "pai", "routers", "rotas.md"))
	if !strings.Contains(string(rotas), "rota `observador`") {
		t.Fatalf("rota não registrada no pai:\n%s\nmudanças: %v", rotas, mud)
	}
	if idx := strings.Index(string(rotas), "rota `observador`"); idx > strings.Index(string(rotas), "rota `self`") {
		t.Fatal("a rota nova entrou depois do self")
	}
	if !fileExists(filepath.Join(root, "agentes", "pai", "agentes", "observador", "data", "okf", "glossary.yaml")) {
		t.Fatal("glossário OKF não criado")
	}

	// mover para a raiz: some do pai
	novoRel, _, err := c.Mover(rel, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if novoRel != "agentes/observador" {
		t.Fatalf("novo rel: %s", novoRel)
	}
	rotas, _ = os.ReadFile(filepath.Join(root, "agentes", "pai", "routers", "rotas.md"))
	if strings.Contains(string(rotas), "observador") {
		t.Fatalf("rota deveria ter sido removida:\n%s", rotas)
	}

	// clonar de volta para o pai: volta a rota, original fica
	cloneRel, _, err := c.Mover(novoRel, "agentes/pai", true)
	if err != nil {
		t.Fatal(err)
	}
	if cloneRel != "agentes/pai/agentes/observador" || !dirExists(filepath.Join(root, "agentes", "observador")) {
		t.Fatalf("clone: %s", cloneRel)
	}
	rotas, _ = os.ReadFile(filepath.Join(root, "agentes", "pai", "routers", "rotas.md"))
	if !strings.Contains(string(rotas), "rota `observador`") {
		t.Fatal("rota do clone não registrada")
	}

	// mover para dentro de si mesmo é recusado
	if _, _, err := c.Mover("agentes/pai", "agentes/pai/agentes/regimento", false); err == nil {
		t.Fatal("deveria recusar mover o pai para dentro do filho")
	}

	// excluir tira a rota
	if _, err := c.ExcluirAgente(cloneRel); err != nil {
		t.Fatal(err)
	}
	rotas, _ = os.ReadFile(filepath.Join(root, "agentes", "pai", "routers", "rotas.md"))
	if strings.Contains(string(rotas), "observador") {
		t.Fatal("rota não removida na exclusão")
	}
}

func TestArquivosEConfig(t *testing.T) {
	c, root := montaFamilia(t)
	if err := c.SalvarArquivo("agentes/pai/agentes/faq", "rules/politicas.md", []byte("# Políticas\n\n- x\n")); err != nil {
		t.Fatal(err)
	}
	if err := c.SalvarArquivo("agentes/pai/agentes/faq", "../pai/agent.yaml", []byte("x")); err == nil {
		t.Fatal("deveria recusar fuga de caminho")
	}
	if err := c.SalvarArquivo("agentes/pai/agentes/faq", "agentes/x/agent.yaml", []byte("x")); err == nil {
		t.Fatal("deveria recusar escrever em agentes/")
	}
	if err := c.SalvarArquivo("agentes/pai/agentes/faq", "rules/x.exe", []byte("x")); err == nil {
		t.Fatal("deveria recusar extensão")
	}
	if err := c.SalvarArquivo("agentes/pai/agentes/faq", "agent.yaml", []byte("nome: faq\nfontes: [../regimento/data]\n")); err == nil {
		t.Fatal("deveria recusar fonte de irmão no agent.yaml")
	}
	if err := c.SalvarConfig("agentes/pai/agentes/faq", map[string]any{"descricao": "dúvidas gerais do condomínio", "tipo": "operador", "apis": []any{"nexus"}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "agentes", "pai", "agentes", "faq", "agent.yaml"))
	s := string(raw)
	if !strings.HasPrefix(s, "nome: faq\ntipo: operador\ndescricao: dúvidas gerais do condomínio\n") || !strings.Contains(s, "apis:\n    - nexus") {
		t.Fatalf("agent.yaml gravado fora da ordem esperada:\n%s", s)
	}
	if err := c.ExcluirArquivo("agentes/pai/agentes/faq", "rules/politicas.md"); err != nil {
		t.Fatal(err)
	}
	if err := c.ExcluirArquivo("agentes/pai/agentes/faq", "agent.yaml"); err == nil {
		t.Fatal("não pode excluir o agent.yaml")
	}
}

func TestAplicarCatalogo(t *testing.T) {
	c, root := montaFamilia(t)
	mud, err := c.Aplicar("agentes/pai/agentes/faq", "base", "portaria")
	if err != nil {
		t.Fatal(err)
	}
	if len(mud) < 3 {
		t.Fatalf("esperava schema + segredos + apis, veio %v", mud)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "agentes", "pai", "agentes", "faq", "agent.yaml"))
	if !strings.Contains(string(raw), "PORTARIA_TOKEN") || !strings.Contains(string(raw), "nexus") {
		t.Fatalf("agent.yaml sem a fusão:\n%s", raw)
	}
	mud, err = c.Aplicar("agentes/pai/agentes/faq", "base", "portaria")
	if err != nil || len(mud) != 1 || !strings.Contains(mud[0], "nada a fazer") {
		t.Fatalf("aplicar de novo deveria ser idempotente: %v %v", mud, err)
	}
	mud, err = c.Aplicar("agentes/pai/agentes/faq", "asset", "rodape-oficial.md")
	if err != nil || mud[0] != "assets/rodape-oficial.md" {
		t.Fatalf("asset: %v %v", mud, err)
	}
	if _, err := c.Aplicar("agentes/pai/agentes/faq", "api", "inexistente"); err == nil {
		t.Fatal("api desconhecida deveria falhar")
	}
	tree, _ := c.Tree()
	faq := Find(tree, "agentes/pai/agentes/faq")
	if seg := c.SegredosDeclarados(faq); len(seg) != 2 {
		t.Fatalf("segredos declarados (próprio + requer da api): %v", seg)
	}
}

func TestZipIdaEVolta(t *testing.T) {
	c, _ := montaFamilia(t)
	dados, err := c.ExportarZip("agentes/pai/agentes/regimento")
	if err != nil {
		t.Fatal(err)
	}
	arqs, err := c.ImportarZip("agentes/regimento-copia", dados)
	if err != nil {
		t.Fatal(err)
	}
	if len(arqs) < 4 {
		t.Fatalf("arquivos importados: %v", arqs)
	}
	tree, _ := c.Tree()
	if Find(tree, "agentes/regimento-copia") == nil {
		t.Fatal("agente importado não aparece na árvore")
	}
	if _, err := c.ImportarZip("agentes/lixo", []byte("nao e zip")); err == nil {
		t.Fatal("zip inválido deveria falhar")
	}
}

func TestRotasComSecoes(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "routers"), 0o755)
	os.WriteFile(filepath.Join(dir, AgentYAML), []byte("nome: x\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "routers", "rotas.md"), []byte("# Roteamento\n\n## Triagem → rota `triagem`\n\n- o que ataco primeiro\n\n## Nenhuma serve\n\n- Se nenhuma linha descreve a pergunta → rota `self`: diga o que\n  você consegue responder.\n"), 0o644)
	m, err := RegistrarRota(dir, "novo", "boletim do dia.")
	if err != nil || m == "" {
		t.Fatal(err, m)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "routers", "rotas.md"))
	s := string(raw)
	i := strings.Index(s, "rota `novo`")
	j := strings.Index(s, "## Nenhuma serve")
	if i == -1 || j == -1 || i > j {
		t.Fatalf("a rota nova deveria entrar antes da seção do self:\n%s", s)
	}
	if !strings.Contains(s, "- Boletim do dia → rota `novo`.") {
		t.Fatalf("critério mal formatado:\n%s", s)
	}
	if m, _ := RegistrarRota(dir, "novo", "de novo"); m != "" {
		t.Fatal("registrar duas vezes não pode duplicar")
	}
	m, _ = RemoverRota(dir, "self")
	raw, _ = os.ReadFile(filepath.Join(dir, "routers", "rotas.md"))
	if m == "" || strings.Contains(string(raw), "rota `self`") || strings.Contains(string(raw), "você consegue responder") {
		t.Fatalf("remover deveria tirar o bloco inteiro (com a continuação):\n%s", raw)
	}
}
