package arquivos

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing/fstest"
)

// Caixinha é uma das subpastas padronizadas de um agente.
type Caixinha struct {
	Nome      string   `json:"nome"`
	Titulo    string   `json:"titulo"`
	Descricao string   `json:"descricao"`
	Herda     bool     `json:"herda"`  // desce do ancestral para os filhos
	Nucleo    bool     `json:"nucleo"` // identidade do agente
	Ext       []string `json:"ext"`
	Recursivo bool     `json:"recursivo"`
	Tool      string   `json:"tool,omitempty"`
}

// Caixinhas é o inventário, na ordem em que a interface as mostra.
var Caixinhas = []Caixinha{
	{Nome: "rules", Titulo: "Regras", Descricao: "output.md é o contrato de saída; qualquer outro .md é política (concatenados, títulos rebaixados a H2).", Nucleo: true, Ext: []string{".md"}},
	{Nome: "workflows", Titulo: "Workflows", Descricao: "principal.md é o que o agente faz dentro das próprias skills; outros .md ou subpastas com etapas 00_/10_ são procedimentos (tool abrir_workflow).", Nucleo: true, Ext: []string{".md"}, Recursivo: true, Tool: "abrir_workflow"},
	{Nome: "routers", Titulo: "Roteamento", Descricao: "rotas.md (critério → rota `nome`) e desambiguacao.md; com subagentes, torna o agente orquestrador.", Nucleo: true, Ext: []string{".md"}},
	{Nome: "knowledge", Titulo: "Conhecimento", Descricao: "Conhecimento OKF do agente: glossary.yaml (termos, sinônimos) e docs com frontmatter type: Domain | Question | Condition | Policy | Vocabulary. Indexado no prompt e lido com ler_dado; herda para os filhos.", Herda: true, Ext: []string{".md", ".yaml", ".yml", ".json", ".txt", ".csv"}, Recursivo: true, Tool: "ler_dado"},
	{Nome: "assets", Titulo: "Assets", Descricao: "Templates, rodapés e imagens, citados como `assets/<arquivo>`; indexados como dados e herdados pelos filhos.", Herda: true, Ext: []string{""}, Recursivo: true, Tool: "ler_dado"},
	{Nome: "evals", Titulo: "Evals", Descricao: "Casos que provam o comportamento (pergunta, rota, contem, nao_contem).", Ext: []string{".yaml", ".yml"}},
}

// CaixinhaPorNome devolve a definição de uma caixinha.
func CaixinhaPorNome(nome string) (Caixinha, bool) {
	for _, c := range Caixinhas {
		if c.Nome == nome {
			return c, true
		}
	}
	return Caixinha{}, false
}

// Catalogo é a raiz do formato (agentes/) mais a fonte de recursos do Nexus.
type Catalogo struct {
	Root     string
	arquivos fstest.MapFS
	// Recursos devolve os recursos do Nexus da account (tools, conteúdos, memórias).
	// nil = Nexus indisponível: as declarações não são conferidas.
	Recursos func() []Recurso
	// Persistir roda depois de uma escrita validada, sob a trava do catálogo.
	Persistir         func() error
	PrepararEscrita   func() (func(), error)
	PrepararRecursos  func() func() []Recurso
	remocoesPendentes []string
	// trava do processo: as escritas e o gancho andam uma de cada vez
	mu sync.Mutex
}

// NewCatalogo abre (sem criar) a raiz. A pasta agentes/ é criada se faltar.
func NewCatalogo(root string) (*Catalogo, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("raiz do catálogo não existe: %s", abs)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("raiz do catálogo não é pasta: %s", abs)
	}
	if err := os.MkdirAll(filepath.Join(abs, DirAgentes), 0o755); err != nil {
		return nil, err
	}
	return &Catalogo{Root: abs}, nil
}

// IsAgent diz se a pasta tem a forma mínima de agente.
func IsAgent(dir string) bool {
	return fileExists(filepath.Join(dir, AgentYAML)) || fileExists(filepath.Join(dir, "workflows", "principal.md"))
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// Ancestrais lista as pastas de AGENTE acima desta (pai, avô…), subindo pelos
// níveis agentes/. Para quando o nível acima não é agente (a raiz).
func Ancestrais(dir string) []string {
	var out []string
	atual := filepath.Clean(dir)
	for filepath.Base(filepath.Dir(atual)) == DirAgentes {
		pai := filepath.Dir(filepath.Dir(atual))
		if !IsAgent(pai) {
			break
		}
		out = append(out, pai)
		atual = pai
	}
	return out
}

// Permitido aplica a fronteira: dentro da própria pasta, ou recurso PRÓPRIO de um
// ancestral (nunca a área agentes/ dele, que é dos irmãos).
func Permitido(agente, alvo string) bool {
	agente = filepath.Clean(agente)
	alvo = filepath.Clean(alvo)
	if dentro(agente, alvo) {
		return true
	}
	for _, anc := range Ancestrais(agente) {
		if dentro(anc, alvo) && !dentro(filepath.Join(anc, DirAgentes), alvo) {
			return true
		}
	}
	return false
}

func dentro(base, alvo string) bool {
	rel, err := filepath.Rel(base, alvo)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func (c *Catalogo) IsAgent(dir string) bool {
	return c.fileExists(filepath.Join(dir, AgentYAML)) || c.fileExists(filepath.Join(dir, "workflows", "principal.md"))
}

func (c *Catalogo) fileExists(p string) bool {
	st, err := c.stat(p)
	return err == nil && !st.IsDir()
}

func (c *Catalogo) dirExists(p string) bool {
	st, err := c.stat(p)
	return err == nil && st.IsDir()
}

// Ancestrais lista as pastas de AGENTE acima desta (pai, avô…), subindo pelos
// níveis agentes/. Para quando o nível acima não é agente (a raiz).
func (c *Catalogo) ancestrais(dir string) []string {
	var out []string
	atual := filepath.Clean(dir)
	for filepath.Base(filepath.Dir(atual)) == DirAgentes {
		pai := filepath.Dir(filepath.Dir(atual))
		if !c.IsAgent(pai) {
			break
		}
		out = append(out, pai)
		atual = pai
	}
	return out
}

// Permitido aplica a fronteira: dentro da própria pasta, ou recurso PRÓPRIO de um
// ancestral (nunca a área agentes/ dele, que é dos irmãos).
func (c *Catalogo) Permitido(agente, alvo string) bool {
	agente = filepath.Clean(agente)
	alvo = filepath.Clean(alvo)
	if dentro(agente, alvo) {
		return true
	}
	for _, anc := range c.ancestrais(agente) {
		if dentro(anc, alvo) && !dentro(filepath.Join(anc, DirAgentes), alvo) {
			return true
		}
	}
	return false
}

// Tree carrega todos os agentes de primeiro nível com seus filhos.
func (c *Catalogo) Tree() ([]*Agent, error) {
	base := filepath.Join(c.Root, DirAgentes)
	entradas, err := c.readDir(base)
	if err != nil {
		return nil, err
	}
	var out []*Agent
	for _, e := range entradas {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(base, e.Name())
		if !c.IsAgent(dir) {
			continue
		}
		a, err := c.Load(dir)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nome < out[j].Nome })
	return out, nil
}

// ResolveAgente transforma um caminho relativo (agentes/x/agentes/y) numa pasta
// absoluta dentro da raiz, recusando qualquer fuga.
func (c *Catalogo) ResolveAgente(rel string) (string, error) {
	rel = filepath.ToSlash(filepath.Clean("/" + rel))[1:]
	partes := strings.Split(rel, "/")
	if len(partes) < 2 || partes[0] != DirAgentes {
		return "", errors.New("caminho inválido (use agentes/<nome>)")
	}
	for i, p := range partes {
		if i%2 == 0 {
			if p != DirAgentes {
				return "", errors.New("caminho inválido: níveis devem alternar agentes/<nome>")
			}
			continue
		}
		if !SlugOK(p) {
			return "", fmt.Errorf("nome inválido: %q (use letras minúsculas, dígitos e hífen)", p)
		}
	}
	abs := filepath.Join(c.Root, filepath.FromSlash(rel))
	if !dentro(c.Root, abs) {
		return "", errors.New("caminho fora da raiz")
	}
	return abs, nil
}

// SlugOK: nome de pasta de agente.
func SlugOK(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-' || r == '_':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// Load carrega uma pasta de agente (e, recursivamente, os subagentes).
func (c *Catalogo) Load(dir string) (*Agent, error) {
	dir = filepath.Clean(dir)
	if !c.dirExists(dir) {
		return nil, fmt.Errorf("pasta de agente não existe: %s", dir)
	}
	if !c.IsAgent(dir) {
		return nil, fmt.Errorf("%s não é uma pasta de agente: precisa de agent.yaml ou workflows/principal.md", dir)
	}
	rel, _ := filepath.Rel(c.Root, dir)
	a := &Agent{
		Path:       dir,
		Rel:        filepath.ToSlash(rel),
		Nome:       filepath.Base(dir),
		Config:     map[string]any{},
		Arquivos:   map[string][]Arquivo{},
		Herdados:   map[string][]Arquivo{},
		Recursos:   []string{},
		Subagentes: []*Agent{},
		Problemas:  []Problema{},
	}

	if cfgPath := filepath.Join(dir, AgentYAML); c.fileExists(cfgPath) {
		raw, err := c.readFile(cfgPath)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(raw, &a.Config); err != nil {
			a.Problemas = append(a.Problemas, Problema{Nivel: "erro", Onde: AgentYAML, Texto: "YAML inválido: " + err.Error()})
		}
		if a.Config == nil {
			a.Config = map[string]any{}
		}
		if v, ok := a.Config["nome"].(string); ok && v != "" {
			a.Nome = v
		}
		if v, ok := a.Config["descricao"].(string); ok {
			a.Descricao = strings.TrimSpace(v)
		}
		if v, ok := a.Config["modelo"].(string); ok {
			a.Modelo = v
		}
		a.Recursos = stringList(a.Config["recursos"])
		if v, ok := a.Config["tipo"].(string); ok {
			t := strings.ToLower(strings.TrimSpace(v))
			if t == Orquestrador || t == Operador {
				a.Tipo = t
				a.TipoDeclarado = true
			} else if t != "" {
				a.Problemas = append(a.Problemas, Problema{Nivel: "aviso", Onde: AgentYAML, Texto: fmt.Sprintf("tipo %q desconhecido; vale orquestrador ou operador", v)})
			}
		}
		for _, k := range []string{"fontes", "apis", "segredos"} {
			if _, ok := a.Config[k]; ok {
				a.Problemas = append(a.Problemas, Problema{Nivel: "aviso", Onde: AgentYAML + " → " + k, Texto: "campo do formato antigo, ignorado", Solucao: "recursos do Nexus vão em `recursos:`; conhecimento e assets herdam do pai automaticamente"})
			}
		}
	}

	// caixinhas próprias e herdadas (do pai ao avô: o mais próximo entra primeiro)
	ancs := c.ancestrais(dir)
	for _, cx := range Caixinhas {
		a.Arquivos[cx.Nome] = c.listar(dir, cx, "")
		if a.Arquivos[cx.Nome] == nil {
			a.Arquivos[cx.Nome] = []Arquivo{}
		}
		a.Herdados[cx.Nome] = []Arquivo{}
		if !cx.Herda {
			continue
		}
		vistos := map[string]bool{}
		for _, f := range a.Arquivos[cx.Nome] {
			vistos[f.Caminho] = true
		}
		for _, anc := range ancs {
			for _, f := range c.listar(anc, cx, filepath.Base(anc)) {
				if vistos[f.Caminho] {
					continue
				}
				vistos[f.Caminho] = true
				a.Herdados[cx.Nome] = append(a.Herdados[cx.Nome], f)
			}
		}
	}
	// caixinhas do formato antigo
	for _, velha := range []string{"data", "skills", "schemas", "mcps", "bash", "scripts", "rag", "hooks"} {
		if c.dirExists(filepath.Join(dir, velha)) {
			a.Problemas = append(a.Problemas, Problema{Nivel: "aviso", Onde: velha + "/", Texto: "caixinha do formato antigo; não entra na projeção", Solucao: "conhecimento vai em knowledge/, templates em assets/, tools vêm do Nexus"})
		}
	}

	// persona: só o agente de PRIMEIRO NÍVEL (o orquestrador que fala com o morador)
	// tem persona.md. Subagentes produzem resultados, não falam com o morador: não têm
	// persona nem a herdam; a voz é aplicada pelo orquestrador na entrega. Um
	// persona.md num subagente é ignorado.
	if len(ancs) == 0 {
		if raw, err := c.readFile(filepath.Join(dir, PersonaMD)); err == nil {
			a.Persona = string(raw)
		}
	} else if c.fileExists(filepath.Join(dir, PersonaMD)) {
		a.Problemas = append(a.Problemas, Problema{Nivel: "aviso", Onde: PersonaMD, Texto: "persona só existe no agente de primeiro nível; este arquivo é ignorado", Solucao: "apague-o: a voz é a de " + filepath.Base(ancs[len(ancs)-1]) + ", aplicada na entrega"})
	}

	// caso de uso
	if p := filepath.Join(dir, CasoYAML); c.fileExists(p) {
		caso, err := c.loadCaso(p)
		if err != nil {
			a.Problemas = append(a.Problemas, Problema{Nivel: "erro", Onde: CasoYAML, Texto: err.Error()})
		} else {
			a.Caso = caso
		}
	}

	// subagentes
	ninho := filepath.Join(dir, DirAgentes)
	if c.dirExists(ninho) {
		entradas, _ := c.readDir(ninho)
		for _, e := range entradas {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			sub := filepath.Join(ninho, e.Name())
			if !c.IsAgent(sub) {
				continue
			}
			s, err := c.Load(sub)
			if err != nil {
				return nil, err
			}
			a.Subagentes = append(a.Subagentes, s)
		}
	}

	if a.Tipo == "" {
		if len(a.Arquivos["routers"]) > 0 && len(a.Subagentes) > 0 {
			a.Tipo = Orquestrador
		} else {
			a.Tipo = Operador
		}
	}

	a.Problemas = append(a.Problemas, c.validar(a)...)
	return a, nil
}

// listar enumera os arquivos de uma caixinha numa pasta de agente.
func (c *Catalogo) listar(dir string, cx Caixinha, herdadoDe string) []Arquivo {
	base := filepath.Join(dir, cx.Nome)
	if !c.dirExists(base) {
		return nil
	}
	var out []Arquivo
	_ = c.walkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != base && (strings.HasPrefix(d.Name(), ".") || !cx.Recursivo) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || !extOK(cx, d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, Arquivo{Caminho: filepath.ToSlash(rel), Bytes: info.Size(), Resumo: c.Resumo(p), HerdadoDe: herdadoDe})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Caminho < out[j].Caminho })
	return out
}

func extOK(cx Caixinha, nome string) bool {
	if len(cx.Ext) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(nome))
	for _, e := range cx.Ext {
		if e == "" || e == ext {
			return true
		}
	}
	return false
}

// Frontmatter devolve o bloco YAML inicial (--- … ---) como mapa, ou vazio.
func Frontmatter(texto string) map[string]any {
	fm, _ := splitFrontmatter(texto)
	out := map[string]any{}
	if fm == "" {
		return out
	}
	_ = yaml.Unmarshal([]byte(fm), &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

// Resumo: uma linha curta sobre o arquivo, para o índice do prompt e a interface.
func (c *Catalogo) Resumo(p string) string {
	raw, err := c.readFile(p)
	if err != nil {
		return ""
	}
	if len(raw) > 64*1024 {
		raw = raw[:64*1024]
	}
	texto := string(raw)
	ext := strings.ToLower(filepath.Ext(p))
	switch ext {
	case ".md":
		fm := Frontmatter(texto)
		for _, k := range []string{"description", "descricao", "title", "titulo"} {
			if d, ok := fm[k].(string); ok && d != "" {
				return corta(d, 160)
			}
		}
		texto = StripFrontmatter(texto)
	case ".json":
		var v any
		if json.Unmarshal(raw, &v) == nil {
			switch t := v.(type) {
			case map[string]any:
				keys := make([]string, 0, len(t))
				for k := range t {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				if len(keys) > 8 {
					keys = keys[:8]
				}
				return "json com chaves: " + strings.Join(keys, ", ")
			case []any:
				return fmt.Sprintf("json com lista de %d itens", len(t))
			}
		}
	case ".yaml", ".yml":
		var m map[string]any
		if yaml.Unmarshal(raw, &m) == nil {
			for _, k := range []string{"descricao", "description", "title", "titulo", "nome", "name"} {
				if d, ok := m[k].(string); ok && d != "" {
					return corta(d, 160)
				}
			}
			if t, ok := m["terms"].([]any); ok {
				return fmt.Sprintf("glossário com %d termos", len(t))
			}
		}
	case ".png", ".jpg", ".jpeg", ".gif", ".pdf", ".webp":
		return "binário " + strings.TrimPrefix(ext, ".")
	}
	for _, linha := range strings.Split(texto, "\n") {
		linha = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(linha), "#"))
		if linha != "" {
			return corta(linha, 160)
		}
	}
	return ""
}

func corta(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Descrever: a descrição curta de uma pasta (índice de subagentes e rotas).
func (c *Catalogo) Descrever(dir string) string {
	if raw, err := c.readFile(filepath.Join(dir, AgentYAML)); err == nil {
		var m map[string]any
		if yaml.Unmarshal(raw, &m) == nil {
			if d, ok := m["descricao"].(string); ok && strings.TrimSpace(d) != "" {
				return strings.TrimSpace(d)
			}
		}
	}
	if p := filepath.Join(dir, "workflows", "principal.md"); c.fileExists(p) {
		return c.Resumo(p)
	}
	return ""
}

func Resumo(p string) string { return (&Catalogo{}).Resumo(p) }

func Descrever(dir string) string { return (&Catalogo{}).Descrever(dir) }

// Pai devolve a pasta do agente pai, ou "" na raiz.
func Pai(dir string) string {
	if a := Ancestrais(dir); len(a) > 0 {
		return a[0]
	}
	return ""
}

// Find procura um agente carregado pelo caminho relativo.
func Find(tree []*Agent, rel string) *Agent {
	for _, a := range tree {
		if a.Rel == rel {
			return a
		}
		if f := Find(a.Subagentes, rel); f != nil {
			return f
		}
	}
	return nil
}

func (c *Catalogo) Ancestrais(dir string) []string { return c.ancestrais(dir) }

func (c *Catalogo) pai(dir string) string {
	if ancestrais := c.ancestrais(dir); len(ancestrais) > 0 {
		return ancestrais[0]
	}
	return ""
}
