// Package okf lê, valida, projeta e escreve agentes no formato de PASTA
// (AgentAsFolder, o "formato pastinha" do harness Python `adam`) e os documentos
// de conhecimento OKF que vivem dentro dessas pastas.
//
// Um agente não é código: é uma pasta. A raiz tem só o agent.yaml; o resto são
// caixinhas (rules/, workflows/, routers/, hooks/, skills/, schemas/, mcps/, bash/,
// scripts/, rag/, data/, assets/, evals/) e agentes/<sub>/ para subagentes, com a
// mesma anatomia, recursivamente.
//
// Duas regras duras vêm do adam e valem aqui:
//
//   - herança: skills, schemas, mcps, bash, scripts, data e assets do ancestral
//     valem para os filhos (o mais próximo vence); rules, workflows, routers,
//     hooks, evals e rag não descem;
//   - fronteira: um agente só referencia a própria pasta ou os recursos PRÓPRIOS
//     de um ancestral; irmão, sobrinho e qualquer coisa fora da árvore são proibidos.
package okf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	Orquestrador = "orquestrador"
	Operador     = "operador"

	DirAgentes = "agentes"
	AgentYAML  = "agent.yaml"
)

// Caixinha é uma das subpastas padronizadas de um agente.
type Caixinha struct {
	Nome      string   `json:"nome"`
	Titulo    string   `json:"titulo"`
	Descricao string   `json:"descricao"`
	Herda     bool     `json:"herda"`     // desce do ancestral para os filhos
	Nucleo    bool     `json:"nucleo"`    // identidade do agente (rules, workflows, routers)
	Ext       []string `json:"ext"`       // extensões aceitas ("" = qualquer)
	Recursivo bool     `json:"recursivo"` // lista subpastas (data, assets, rag, workflows)
	Tool      string   `json:"tool,omitempty"`
}

// Caixinhas é o inventário, na ordem em que a interface as mostra.
var Caixinhas = []Caixinha{
	{Nome: "rules", Titulo: "Regras", Descricao: "output.md é o contrato de saída; qualquer outro .md é política (concatenados, títulos rebaixados a H2).", Nucleo: true, Ext: []string{".md"}},
	{Nome: "workflows", Titulo: "Workflows", Descricao: "principal.md é o passo a passo projetado; outros .md ou subpastas com etapas 00_/10_ são procedimentos (tool abrir_workflow).", Nucleo: true, Ext: []string{".md"}, Recursivo: true, Tool: "abrir_workflow"},
	{Nome: "routers", Titulo: "Roteamento", Descricao: "rotas.md (critério → rota `nome`) e desambiguacao.md; com subagentes, torna o agente orquestrador.", Nucleo: true, Ext: []string{".md"}},
	{Nome: "evals", Titulo: "Evals", Descricao: "Casos que provam o comportamento (pergunta, rota, contem, nao_contem).", Ext: []string{".yaml", ".yml"}},
	{Nome: "hooks", Titulo: "Hooks", Descricao: "Automações determinísticas que o runtime dispara após eventos (apos_rota, apos_tool, apos_resposta, ao_falhar).", Ext: []string{".yaml", ".yml"}},
	{Nome: "skills", Titulo: "Skills", Descricao: "Funções que viram tools em processo (assinatura + docstring).", Herda: true, Ext: []string{".py", ".go", ".md"}},
	{Nome: "schemas", Titulo: "Schemas", Descricao: "Estrutura dos dados (tabelas, colunas, domínios) → tool consultar_schema.", Herda: true, Ext: []string{".yaml", ".yml", ".json"}, Tool: "consultar_schema"},
	{Nome: "mcps", Titulo: "MCPs", Descricao: "Descritores de servidores MCP (stdio/http) → tools mcp_<servidor>_<tool>.", Herda: true, Ext: []string{".yaml", ".yml"}, Tool: "mcp_*"},
	{Nome: "bash", Titulo: "Bash", Descricao: "Allowlist de binários (frontmatter comando:) → tool rodar_bash.", Herda: true, Ext: []string{".md"}, Tool: "rodar_bash"},
	{Nome: "scripts", Titulo: "Scripts", Descricao: "Programas da pasta, rodados em subprocesso → tool rodar_script.", Herda: true, Ext: []string{".py", ".sh", ".go"}, Tool: "rodar_script"},
	{Nome: "rag", Titulo: "RAG", Descricao: "Documentos fragmentados e indexados por semelhança → tool buscar_rag. Não herda (o índice é da pasta).", Ext: []string{""}, Recursivo: true, Tool: "buscar_rag"},
	{Nome: "data", Titulo: "Dados", Descricao: "Arquivos indexados no prompt e lidos sob demanda → tool ler_dado. data/okf/ guarda o conhecimento OKF tipado.", Herda: true, Ext: []string{""}, Recursivo: true, Tool: "ler_dado"},
	{Nome: "assets", Titulo: "Assets", Descricao: "Estáticos (templates, rodapés, imagens), indexados como dados; referenciados por `assets/<arquivo>`.", Herda: true, Ext: []string{""}, Recursivo: true, Tool: "ler_dado"},
}

// CaixinhaPorNome devolve a definição de uma caixinha (ok=false se não existe).
func CaixinhaPorNome(nome string) (Caixinha, bool) {
	for _, c := range Caixinhas {
		if c.Nome == nome {
			return c, true
		}
	}
	return Caixinha{}, false
}

// Arquivo é um item de uma caixinha, próprio ou herdado.
type Arquivo struct {
	Caminho   string `json:"caminho"` // relativo à pasta do agente (ou do ancestral)
	Bytes     int64  `json:"bytes"`
	Resumo    string `json:"resumo,omitempty"`
	HerdadoDe string `json:"herdado_de,omitempty"` // nome do ancestral dono do arquivo
}

// Agent é a pasta carregada.
type Agent struct {
	Path string `json:"-"`
	// Rel é o caminho relativo à raiz do catálogo, com barras: agentes/concierge/agentes/regimento.
	Rel           string               `json:"caminho"`
	Nome          string               `json:"nome"`
	Tipo          string               `json:"tipo"`
	TipoDeclarado bool                 `json:"tipo_declarado"`
	Descricao     string               `json:"descricao"`
	Modelo        string               `json:"modelo,omitempty"`
	Fontes        []string             `json:"fontes"`
	Apis          []string             `json:"apis"`
	Segredos      []string             `json:"segredos"`
	Config        map[string]any       `json:"config"`
	Arquivos      map[string][]Arquivo `json:"arquivos"` // por caixinha: só os próprios
	Herdados      map[string][]Arquivo `json:"herdados"` // por caixinha: dos ancestrais, não sobrepostos
	Subagentes    []*Agent             `json:"subagentes"`
	Problemas     []Problema           `json:"problemas"`
}

// Problema é algo errado ou suspeito encontrado ao carregar ou validar.
type Problema struct {
	Nivel   string `json:"nivel"` // erro | aviso
	Onde    string `json:"onde"`  // arquivo ou campo
	Texto   string `json:"texto"`
	Solucao string `json:"solucao,omitempty"`
}

// Catalogo é a raiz do formato: agentes/, bases/, tools/, apis/.
type Catalogo struct {
	Root string
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
	return fileExists(filepath.Join(dir, AgentYAML)) ||
		fileExists(filepath.Join(dir, "workflows", "principal.md")) ||
		fileExists(filepath.Join(dir, "workflows.md"))
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
// níveis agentes/. Para quando o nível acima não é agente (a raiz do catálogo).
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

// ---- carregamento -------------------------------------------------------

// Tree carrega todos os agentes de primeiro nível (agentes/<nome>) com seus filhos.
func (c *Catalogo) Tree() ([]*Agent, error) {
	base := filepath.Join(c.Root, DirAgentes)
	entradas, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var out []*Agent
	for _, e := range entradas {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(base, e.Name())
		if !IsAgent(dir) {
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
	if !dirExists(dir) {
		return nil, fmt.Errorf("pasta de agente não existe: %s", dir)
	}
	if !IsAgent(dir) {
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
		Fontes:     []string{},
		Apis:       []string{},
		Segredos:   []string{},
		Subagentes: []*Agent{},
		Problemas:  []Problema{},
	}

	if cfgPath := filepath.Join(dir, AgentYAML); fileExists(cfgPath) {
		raw, err := os.ReadFile(cfgPath)
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
		a.Fontes = stringList(a.Config["fontes"])
		a.Apis = stringList(a.Config["apis"])
		a.Segredos = stringList(a.Config["segredos"])
		if v, ok := a.Config["tipo"].(string); ok {
			t := strings.ToLower(strings.TrimSpace(v))
			if t == Orquestrador || t == Operador {
				a.Tipo = t
				a.TipoDeclarado = true
			} else if t != "" {
				a.Problemas = append(a.Problemas, Problema{Nivel: "aviso", Onde: AgentYAML, Texto: fmt.Sprintf("tipo %q desconhecido; vale orquestrador ou operador", v)})
			}
		}
	}

	// caixinhas próprias
	for _, cx := range Caixinhas {
		a.Arquivos[cx.Nome] = listar(dir, cx, "")
	}
	// legado na raiz
	for _, nome := range []string{"politicas.md", "output.md", "workflows.md", "router.md"} {
		if fileExists(filepath.Join(dir, nome)) {
			a.Problemas = append(a.Problemas, Problema{Nivel: "aviso", Onde: nome, Texto: "forma legada na raiz; mova para a caixinha correspondente", Solucao: "rules/ ou workflows/ ou routers/"})
		}
	}
	// herdados: do avô ao pai, o mais próximo vence; o próprio sobrepõe por caminho
	ancs := Ancestrais(dir)
	for _, cx := range Caixinhas {
		if !cx.Herda {
			continue
		}
		vistos := map[string]bool{}
		for _, f := range a.Arquivos[cx.Nome] {
			vistos[f.Caminho] = true
		}
		var herdados []Arquivo
		for _, anc := range ancs { // do pai ao avô: o mais próximo entra primeiro
			for _, f := range listar(anc, cx, filepath.Base(anc)) {
				if vistos[f.Caminho] {
					continue
				}
				vistos[f.Caminho] = true
				herdados = append(herdados, f)
			}
		}
		a.Herdados[cx.Nome] = herdados
	}

	// subagentes
	ninho := filepath.Join(dir, DirAgentes)
	if dirExists(ninho) {
		entradas, _ := os.ReadDir(ninho)
		for _, e := range entradas {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			sub := filepath.Join(ninho, e.Name())
			if !IsAgent(sub) {
				continue
			}
			s, err := c.Load(sub)
			if err != nil {
				return nil, err
			}
			a.Subagentes = append(a.Subagentes, s)
		}
	}

	// tipo inferido: routers/ + subagentes = orquestrador
	if a.Tipo == "" {
		if len(a.Arquivos["routers"]) > 0 && len(a.Subagentes) > 0 {
			a.Tipo = Orquestrador
		} else {
			a.Tipo = Operador
		}
	}

	a.Problemas = append(a.Problemas, c.validar(a)...)
	for _, cx := range Caixinhas {
		if a.Arquivos[cx.Nome] == nil {
			a.Arquivos[cx.Nome] = []Arquivo{}
		}
		if a.Herdados[cx.Nome] == nil {
			a.Herdados[cx.Nome] = []Arquivo{}
		}
	}
	return a, nil
}

// listar enumera os arquivos de uma caixinha numa pasta de agente.
func listar(dir string, cx Caixinha, herdadoDe string) []Arquivo {
	base := filepath.Join(dir, cx.Nome)
	if !dirExists(base) {
		return nil
	}
	var out []Arquivo
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != base && (strings.HasPrefix(d.Name(), ".") || d.Name() == "__pycache__") {
				return filepath.SkipDir
			}
			if p != base && !cx.Recursivo {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if !extOK(cx, d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, Arquivo{
			Caminho:   filepath.ToSlash(rel),
			Bytes:     info.Size(),
			Resumo:    Resumo(p),
			HerdadoDe: herdadoDe,
		})
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

func stringList(v any) []string {
	out := []string{}
	switch t := v.(type) {
	case []any:
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
	case []string:
		out = append(out, t...)
	case string:
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// ---- texto: frontmatter e resumo ----------------------------------------

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

// StripFrontmatter devolve o corpo sem o bloco inicial.
func StripFrontmatter(texto string) string {
	_, corpo := splitFrontmatter(texto)
	return corpo
}

func splitFrontmatter(texto string) (fm, corpo string) {
	if !strings.HasPrefix(texto, "---") {
		return "", texto
	}
	fim := strings.Index(texto[3:], "\n---")
	if fim == -1 {
		return "", texto
	}
	fm = texto[3 : 3+fim]
	corpo = texto[3+fim+4:]
	return fm, strings.TrimLeft(corpo, "\n")
}

// Resumo: uma linha curta sobre o arquivo, para o índice do prompt e a interface.
func Resumo(p string) string {
	raw, err := os.ReadFile(p)
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
		if d, ok := fm["description"].(string); ok && d != "" {
			return corta(d, 160)
		}
		if d, ok := fm["descricao"].(string); ok && d != "" {
			return corta(d, 160)
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
			for _, k := range []string{"descricao", "description", "title", "titulo", "nome"} {
				if d, ok := m[k].(string); ok && d != "" {
					return corta(d, 160)
				}
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

// Descrever: a descrição curta de uma pasta (para o índice de subagentes e rotas).
func Descrever(dir string) string {
	if raw, err := os.ReadFile(filepath.Join(dir, AgentYAML)); err == nil {
		var m map[string]any
		if yaml.Unmarshal(raw, &m) == nil {
			if d, ok := m["descricao"].(string); ok && strings.TrimSpace(d) != "" {
				return strings.TrimSpace(d)
			}
		}
	}
	for _, cand := range []string{filepath.Join(dir, "workflows", "principal.md"), filepath.Join(dir, "workflows.md")} {
		if fileExists(cand) {
			return Resumo(cand)
		}
	}
	return ""
}

// Pai devolve a pasta do agente pai, ou "" na raiz do catálogo.
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
