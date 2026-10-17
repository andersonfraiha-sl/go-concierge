package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var recursosDeTeste = []Recurso{
	{Name: "unidades.listar", Kind: KindAPI, Visibility: "private", Description: "unidades da pessoa"},
	{Name: "faq.buscar", Kind: KindKnowledge, Visibility: "public", Description: "FAQ", Params: []ParamMeta{{Name: "query", Type: "string", Required: true}}},
	{Name: "visitantes.liberar", Kind: KindAPI, Visibility: "private", Write: true, Description: "libera visitante"},
}

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
	escreve("agentes/pai/agent.yaml", "nome: pai\ndescricao: orquestra o condomínio\nrecursos: [unidades.listar, visitantes.liberar]\n")
	escreve("agentes/pai/persona.md", "# Persona\n\n- cordial\n")
	escreve("agentes/pai/routers/rotas.md", "# Roteamento\n\n- Dúvida sobre o regimento → rota `regimento`.\n- Qualquer outra coisa → rota `self`.\n")
	escreve("agentes/pai/knowledge/glossary.yaml", "terms:\n  - id: unidade\n    display_name: Unidade\n    synonyms: [apartamento, apê]\n    definition: a unidade do morador\n    labels: [cadastro]\n")
	escreve("agentes/pai/assets/rodape.md", "— administração\n")
	escreve("agentes/pai/agentes/regimento/agent.yaml", "nome: regimento\ndescricao: responde pelo regimento\nrecursos: [faq.buscar, inexistente.recurso]\n")
	escreve("agentes/pai/agentes/regimento/workflows/principal.md", "# Workflow\n\n1. Leia `knowledge/regimento.md` (ler_dado).\n2. Use `assets/rodape.md` no fim.\n3. Consulte `knowledge/glossary.yaml`.\n4. Nunca leia `knowledge/inexistente.md`.\n")
	escreve("agentes/pai/agentes/regimento/rules/output.md", "# Contrato\n\n## Resposta\n")
	escreve("agentes/pai/agentes/regimento/knowledge/regimento.md", "# Regimento\n\nArt. 1\n")
	escreve("agentes/pai/agentes/regimento/knowledge/questions/silencio.md", "---\ntype: Question\ntitle: Pergunta — silêncio\nanswers: [PERMITIDO, PROIBIDO]\n---\n## Answers\n")
	escreve("agentes/pai/agentes/faq/agent.yaml", "nome: faq\ndescricao: dúvidas gerais\n")
	escreve("agentes/pai/agentes/faq/workflows/principal.md", "# Workflow\n\n1. Leia `knowledge/regimento.md`.\n")
	escreve("agentes/pai/agentes/visita/agent.yaml", "nome: visita\ndescricao: libera visitante\n")
	escreve("agentes/pai/agentes/visita/caso.yaml", "id: liberacao\nname: Liberação\nsteps:\n  - resource: unidades.listar\n    save_as: unidades\n  - collect: [nome]\n  - confirm: \"ok?\"\n  - resource: visitantes.liberar\n    with: {unidade_id: \"{{.unidade}}\"}\noutput: \"Protocolo {{.r.protocolo}}\"\n")
	c, err := NewCatalogo(root)
	if err != nil {
		t.Fatal(err)
	}
	c.Recursos = func() []Recurso { return recursosDeTeste }
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
		t.Fatalf("pai deveria ser orquestrador inferido, veio %s", pai.Tipo)
	}
	if len(pai.Subagentes) != 3 {
		t.Fatalf("esperava 3 subagentes, veio %d", len(pai.Subagentes))
	}
	reg := Find(tree, "agentes/pai/agentes/regimento")
	if reg == nil || reg.Tipo != Operador {
		t.Fatalf("regimento deveria ser operador: %+v", reg)
	}
	// herança: knowledge e assets do pai aparecem como herdados; o próprio não repete
	if len(reg.Herdados["assets"]) != 1 || reg.Herdados["assets"][0].HerdadoDe != "pai" {
		t.Fatalf("assets herdados: %+v", reg.Herdados["assets"])
	}
	if len(reg.Herdados["knowledge"]) != 1 || reg.Herdados["knowledge"][0].Caminho != "knowledge/glossary.yaml" {
		t.Fatalf("knowledge herdado: %+v", reg.Herdados["knowledge"])
	}
	if len(reg.Herdados["rules"]) != 0 {
		t.Fatal("rules não deveria herdar")
	}
	// persona herdada
	if reg.PersonaDe != "pai" || !strings.Contains(reg.Persona, "cordial") {
		t.Fatalf("persona herdada: de=%q %q", reg.PersonaDe, reg.Persona)
	}
	if pai.PersonaDe != "" || pai.Persona == "" {
		t.Fatal("pai deveria ter persona própria")
	}
}

func TestFronteira(t *testing.T) {
	_, root := montaFamilia(t)
	reg := filepath.Join(root, "agentes", "pai", "agentes", "regimento")
	casos := map[string]bool{
		filepath.Join(reg, "knowledge", "x.md"):                                          true,
		filepath.Join(root, "agentes", "pai", "knowledge"):                               true,
		filepath.Join(root, "agentes", "pai", "agentes", "faq", "knowledge"):             false,
		filepath.Join(root, "nexus"):                                                     false,
		filepath.Join(root, "agentes", "pai", "agentes"):                                 false,
		filepath.Join(root, "agentes", "pai", "agentes", "regimento", "agentes", "neto"): true,
	}
	for alvo, esperado := range casos {
		if got := Permitido(reg, alvo); got != esperado {
			t.Errorf("Permitido(%s) = %v, esperava %v", alvo, got, esperado)
		}
	}
}

func TestVinculosRecursosEProblemas(t *testing.T) {
	c, _ := montaFamilia(t)
	tree, _ := c.Tree()
	reg := Find(tree, "agentes/pai/agentes/regimento")
	estado := map[string]Vinculo{}
	for _, v := range c.Vinculos(reg) {
		estado[v.Tipo+":"+v.Alvo] = v
	}
	if v := estado["conhecimento:knowledge/regimento.md"]; !v.OK || v.Resolvido != "próprio" {
		t.Errorf("knowledge/regimento.md deveria resolver como próprio: %+v", v)
	}
	if v := estado["asset:assets/rodape.md"]; !v.OK || v.Resolvido != "herdado de pai" {
		t.Errorf("assets/rodape.md deveria vir do pai: %+v", v)
	}
	if v := estado["conhecimento:knowledge/glossary.yaml"]; !v.OK {
		t.Errorf("glossário herdado deveria resolver: %+v", v)
	}
	if v := estado["conhecimento:knowledge/inexistente.md"]; v.OK {
		t.Errorf("knowledge/inexistente.md deveria quebrar: %+v", v)
	}
	if v := estado["recurso:faq.buscar"]; !v.OK || !strings.Contains(v.Resolvido, "knowledge") {
		t.Errorf("faq.buscar deveria resolver no Nexus: %+v", v)
	}
	if v := estado["recurso:inexistente.recurso"]; v.OK {
		t.Errorf("recurso inexistente deveria quebrar: %+v", v)
	}
	if v := estado["persona:herdada"]; !v.OK {
		t.Errorf("persona herdada deveria aparecer: %+v", v)
	}
	// o faq cita knowledge/regimento.md, que existe só no IRMÃO
	faq := Find(tree, "agentes/pai/agentes/faq")
	achou := false
	for _, v := range c.Vinculos(faq) {
		if v.Alvo == "knowledge/regimento.md" {
			achou = true
			if v.OK || !strings.Contains(v.Problema, "fronteira") {
				t.Errorf("deveria apontar a fronteira: %+v", v)
			}
		}
	}
	if !achou {
		t.Fatal("vínculo do faq não encontrado")
	}
	// pai declara escrita fora do caso: erro com solução
	pai := tree[0]
	temErroEscrita := false
	for _, p := range pai.Problemas {
		if strings.Contains(p.Texto, "visitantes.liberar") && strings.Contains(p.Texto, "ESCRITA") {
			temErroEscrita = true
		}
	}
	if !temErroEscrita {
		t.Errorf("pai deveria acusar escrita em agent.yaml: %+v", pai.Problemas)
	}
	// o caso do visita: escrita dentro do caso, com confirm: ok
	visita := Find(tree, "agentes/pai/agentes/visita")
	if visita.Caso == nil || visita.Caso.ID != "liberacao" || !visita.Caso.Confirma || len(visita.Caso.Recursos) != 2 {
		t.Fatalf("caso: %+v", visita.Caso)
	}
	for _, v := range c.Vinculos(visita) {
		if v.Tipo == "recurso" && !v.OK {
			t.Errorf("recurso do caso deveria resolver: %+v", v)
		}
	}
	for _, p := range visita.Problemas {
		if p.Nivel == "erro" {
			t.Errorf("visita não deveria ter erro: %+v", p)
		}
	}
}

func TestConhecimentoEIndice(t *testing.T) {
	c, _ := montaFamilia(t)
	tree, _ := c.Tree()
	reg := Find(tree, "agentes/pai/agentes/regimento")
	k := c.Conhecimento(reg)
	if len(k.Docs) != 2 {
		t.Fatalf("docs: %+v", k.Docs)
	}
	if len(k.Termos) != 1 || k.Termos[0].ID != "unidade" || k.Termos[0].HerdadoDe != "pai" {
		t.Fatalf("termos: %+v", k.Termos)
	}
	idx := c.Indice(reg)
	if len(idx) != 4 {
		t.Fatalf("índice deveria ter 2 próprios + glossário + rodapé: %+v", idx)
	}
	raw, err := c.LerDado(reg, "assets/rodape.md")
	if err != nil || !strings.Contains(string(raw), "administração") {
		t.Fatalf("LerDado herdado: %v %q", err, raw)
	}
	if _, err := c.LerDado(reg, "knowledge/nada.md"); err == nil {
		t.Fatal("fora do índice deveria falhar")
	}
}

func TestProjecao(t *testing.T) {
	c, _ := montaFamilia(t)
	tree, _ := c.Tree()
	reg := Find(tree, "agentes/pai/agentes/regimento")
	p := c.Projecao(reg)
	for _, trecho := range []string{"OPERADOR", "# Persona", "cordial", "# Workflow", "# Contrato de saída", "`knowledge/regimento.md`", "[herdado de pai]", "chamar_recurso", "`faq.buscar` (knowledge, public)", "# Regras gerais"} {
		if !strings.Contains(p, trecho) {
			t.Errorf("projeção sem %q:\n%s", trecho, p)
		}
	}
	if strings.Contains(p, "inexistente.recurso") {
		t.Error("recurso quebrado não deveria ir ao prompt")
	}
	visita := c.Projecao(Find(tree, "agentes/pai/agentes/visita"))
	if !strings.Contains(visita, "# Caso de uso `liberacao`") || !strings.Contains(visita, "Recursos de escrita") || !strings.Contains(visita, "confirmar (sim/não)") {
		t.Errorf("projeção do caso:\n%s", visita)
	}
	pai := c.Projecao(tree[0])
	if !strings.Contains(pai, "ORQUESTRADOR") || !strings.Contains(pai, "Rotas de delegação") {
		t.Errorf("projeção do pai:\n%s", pai)
	}
}

func TestCriarMoverClonarExcluir(t *testing.T) {
	c, root := montaFamilia(t)
	rel, mud, err := c.CriarAgente(NovoAgente{Pai: "agentes/pai", Nome: "observador", Tipo: Operador, Descricao: "observa a sessão", Caixinhas: []string{"rules", "workflows", "knowledge"}, Caso: true, Recursos: []string{"faq.buscar"}})
	if err != nil {
		t.Fatal(err)
	}
	if rel != "agentes/pai/agentes/observador" {
		t.Fatalf("rel: %s", rel)
	}
	rotas, _ := os.ReadFile(filepath.Join(root, "agentes", "pai", "routers", "rotas.md"))
	if !strings.Contains(string(rotas), "rota `observador`") || strings.Index(string(rotas), "rota `observador`") > strings.Index(string(rotas), "rota `self`") {
		t.Fatalf("rota não registrada antes do self:\n%s\n%v", rotas, mud)
	}
	dir := filepath.Join(root, "agentes", "pai", "agentes", "observador")
	for _, f := range []string{"knowledge/glossary.yaml", "caso.yaml", "rules/output.md"} {
		if !fileExists(filepath.Join(dir, filepath.FromSlash(f))) {
			t.Fatalf("%s não criado", f)
		}
	}
	if fileExists(filepath.Join(dir, PersonaMD)) {
		t.Fatal("subagente operador não deveria ganhar persona por padrão")
	}
	cfg, _ := os.ReadFile(filepath.Join(dir, AgentYAML))
	if !strings.Contains(string(cfg), "recursos:\n    - faq.buscar") {
		t.Fatalf("agent.yaml sem recursos:\n%s", cfg)
	}

	novoRel, _, err := c.Mover(rel, "", false)
	if err != nil || novoRel != "agentes/observador" {
		t.Fatal(err, novoRel)
	}
	rotas, _ = os.ReadFile(filepath.Join(root, "agentes", "pai", "routers", "rotas.md"))
	if strings.Contains(string(rotas), "observador") {
		t.Fatalf("rota deveria ter sido removida:\n%s", rotas)
	}
	cloneRel, _, err := c.Mover(novoRel, "agentes/pai", true)
	if err != nil || cloneRel != "agentes/pai/agentes/observador" || !dirExists(filepath.Join(root, "agentes", "observador")) {
		t.Fatal(err, cloneRel)
	}
	if _, _, err := c.Mover("agentes/pai", "agentes/pai/agentes/regimento", false); err == nil {
		t.Fatal("deveria recusar mover o pai para dentro do filho")
	}
	if _, err := c.ExcluirAgente(cloneRel); err != nil {
		t.Fatal(err)
	}
	rotas, _ = os.ReadFile(filepath.Join(root, "agentes", "pai", "routers", "rotas.md"))
	if strings.Contains(string(rotas), "observador") {
		t.Fatal("rota não removida na exclusão")
	}
}

func TestArquivosConfigEVinculo(t *testing.T) {
	c, root := montaFamilia(t)
	faq := "agentes/pai/agentes/faq"
	if err := c.SalvarArquivo(faq, "rules/politicas.md", []byte("# Políticas\n\n- x\n")); err != nil {
		t.Fatal(err)
	}
	if err := c.SalvarArquivo(faq, PersonaMD, []byte("# Persona\n")); err != nil {
		t.Fatal(err)
	}
	for _, ruim := range []string{"../pai/agent.yaml", "agentes/x/agent.yaml", "rules/x.exe", "data/x.md", "mcps/x.yaml"} {
		if err := c.SalvarArquivo(faq, ruim, []byte("x")); err == nil {
			t.Fatalf("deveria recusar %s", ruim)
		}
	}
	if err := c.SalvarArquivo(faq, CasoYAML, []byte("name: sem id\n")); err == nil {
		t.Fatal("caso.yaml sem id deveria ser recusado")
	}
	if err := c.SalvarConfig(faq, map[string]any{"descricao": "dúvidas gerais do condomínio", "tipo": "operador", "recursos": []any{"faq.buscar"}}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "agentes", "pai", "agentes", "faq", "agent.yaml"))
	if !strings.HasPrefix(string(raw), "nome: faq\ntipo: operador\ndescricao: dúvidas gerais do condomínio\n") || !strings.Contains(string(raw), "recursos:\n    - faq.buscar") {
		t.Fatalf("agent.yaml fora da ordem:\n%s", raw)
	}
	mud, err := c.VincularRecurso(faq, "unidades.listar", true)
	if err != nil || len(mud) != 1 {
		t.Fatal(err, mud)
	}
	if mud, _ := c.VincularRecurso(faq, "unidades.listar", true); mud != nil {
		t.Fatal("ligar de novo deveria ser idempotente")
	}
	if _, err := c.VincularRecurso(faq, "nao.existe", true); err == nil {
		t.Fatal("recurso fora da account deveria falhar")
	}
	if _, err := c.VincularRecurso(faq, "visitantes.liberar", true); err == nil || !strings.Contains(err.Error(), "ESCRITA") {
		t.Fatalf("escrita em agent.yaml deveria ser recusada: %v", err)
	}
	if _, err := c.VincularRecurso(faq, "unidades.listar", false); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(filepath.Join(root, "agentes", "pai", "agentes", "faq", "agent.yaml"))
	if strings.Contains(string(raw), "unidades.listar") {
		t.Fatalf("desligar não tirou:\n%s", raw)
	}
	if err := c.ExcluirArquivo(faq, "agent.yaml"); err == nil {
		t.Fatal("não pode excluir o agent.yaml")
	}
	if err := c.ExcluirArquivo(faq, PersonaMD); err != nil {
		t.Fatal(err)
	}
}

func TestSemNexusNaoConfere(t *testing.T) {
	c, _ := montaFamilia(t)
	c.Recursos = nil
	tree, _ := c.Tree()
	reg := Find(tree, "agentes/pai/agentes/regimento")
	for _, v := range c.Vinculos(reg) {
		if v.Tipo == "recurso" && (!v.OK || !strings.Contains(v.Resolvido, "não conferido")) {
			t.Errorf("sem Nexus deveria ficar 'não conferido': %+v", v)
		}
	}
}

func TestFixtureDeRecursos(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.yaml")
	os.WriteFile(p, []byte("recursos:\n  - name: b.x\n    kind: api\n    write: true\n  - name: a.x\n    kind: knowledge\n    visibility: public\n"), 0o644)
	l, err := LerRecursosFixture(p)
	if err != nil || len(l) != 2 || l[0].Name != "a.x" || l[1].Visibility != "private" || l[1].Classe() != "escrita" || l[0].Classe() != "conhecimento" {
		t.Fatalf("%v %+v", err, l)
	}
}

func TestZipIdaEVolta(t *testing.T) {
	c, _ := montaFamilia(t)
	dados, err := c.ExportarZip("agentes/pai/agentes/regimento")
	if err != nil {
		t.Fatal(err)
	}
	arqs, err := c.ImportarZip("agentes/regimento-copia", dados)
	if err != nil || len(arqs) < 4 {
		t.Fatal(err, arqs)
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
	if i, j := strings.Index(s, "rota `novo`"), strings.Index(s, "## Nenhuma serve"); i == -1 || j == -1 || i > j {
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
		t.Fatalf("remover deveria tirar o bloco inteiro:\n%s", raw)
	}
}
