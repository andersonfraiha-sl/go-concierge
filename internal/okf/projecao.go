package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Projecao compila a pasta num system prompt, no mesmo desenho do compiler do adam
// (princípio ICM: o modelo lê uma projeção; os dados ficam indexados e são lidos
// sob demanda pelas tools). É o que o botão "projeção" da interface mostra e o que
// o harness Eino receberá como instrução do agente.
func (c *Catalogo) Projecao(a *Agent) string {
	var partes []string

	papel := "OPERADOR (você executa a tarefa você mesmo)"
	if a.Tipo == Orquestrador {
		papel = "ORQUESTRADOR (estratégico: você delega e entrega o resultado, não executa)"
	}
	titulo := fmt.Sprintf("Você é o agente %q, um %s", a.Nome, papel)
	if a.Descricao != "" {
		titulo += " — " + a.Descricao
	}
	partes = append(partes, titulo+".")

	if pol := c.politicas(a); pol != "" {
		partes = append(partes, "# Políticas\n\n"+pol)
	}
	if wf := c.lerProprio(a, "workflows/principal.md"); wf != "" {
		partes = append(partes, "# Workflow (siga o passo a passo)\n\n"+corpo(wf))
	}
	if out := c.lerProprio(a, "rules/output.md"); out != "" {
		partes = append(partes, "# Contrato de saída\n\nSua resposta final DEVE seguir exatamente este contrato:\n\n"+corpo(out))
	}

	var recursos []string

	if dados := c.Dados(a); len(dados) > 0 {
		var linhas []string
		for _, d := range dados {
			l := fmt.Sprintf("- `%s` (%d bytes) — %s", d.Caminho, d.Bytes, d.Resumo)
			if d.HerdadoDe != "" {
				l += " [herdado de " + d.HerdadoDe + "]"
			}
			linhas = append(linhas, l)
		}
		recursos = append(recursos, "## Dados (leia com a tool ler_dado; pagina quando tem_mais=true)\n"+strings.Join(linhas, "\n"))
	}

	if schemas := c.comHeranca(a, "schemas"); len(schemas) > 0 {
		var linhas []string
		for _, s := range schemas {
			linhas = append(linhas, "- `"+strings.TrimSuffix(filepath.Base(s.Caminho), filepath.Ext(s.Caminho))+"`")
		}
		recursos = append(recursos, "## Schemas de dados (tool consultar_schema)\n"+strings.Join(linhas, "\n"))
	}

	if bashs := c.comHeranca(a, "bash"); len(bashs) > 0 {
		var linhas []string
		for _, b := range bashs {
			fm := Frontmatter(c.lerArquivo(a, b))
			nome := strings.TrimSuffix(filepath.Base(b.Caminho), ".md")
			linhas = append(linhas, fmt.Sprintf("- `%s` (comando `%s`) — %s", nome, str(fm["comando"], nome), str(fm["descricao"], "")))
		}
		recursos = append(recursos, "## Comandos bash permitidos (tool rodar_bash; SÓ estes)\n"+strings.Join(linhas, "\n"))
	}

	if scripts := c.comHeranca(a, "scripts"); len(scripts) > 0 {
		var linhas []string
		for _, s := range scripts {
			linhas = append(linhas, fmt.Sprintf("- `%s` (%s) — %s", filepath.Base(s.Caminho), s.Caminho, s.Resumo))
		}
		recursos = append(recursos, "## Scripts da pasta (tool rodar_script)\n"+strings.Join(linhas, "\n"))
	}

	if skills := c.comHeranca(a, "skills"); len(skills) > 0 {
		var linhas []string
		for _, s := range skills {
			linhas = append(linhas, fmt.Sprintf("- `%s` — %s", filepath.Base(s.Caminho), s.Resumo))
		}
		recursos = append(recursos, "## Skills (funções da pasta que viram tools)\n"+strings.Join(linhas, "\n"))
	}

	if mcps := c.comHeranca(a, "mcps"); len(mcps) > 0 {
		var linhas []string
		for _, m := range mcps {
			var d struct {
				Nome       string `yaml:"nome"`
				Transporte string `yaml:"transporte"`
			}
			_ = yaml.Unmarshal([]byte(c.lerArquivo(a, m)), &d)
			if d.Nome == "" {
				d.Nome = strings.TrimSuffix(filepath.Base(m.Caminho), filepath.Ext(m.Caminho))
			}
			linhas = append(linhas, fmt.Sprintf("- `%s` (%s) → tools mcp_%s_*", d.Nome, d.Transporte, d.Nome))
		}
		recursos = append(recursos, "## Servidores MCP\n"+strings.Join(linhas, "\n"))
	}

	if procs := c.Procedimentos(a); len(procs) > 0 {
		var linhas []string
		for _, p := range procs {
			l := fmt.Sprintf("- `%s` — %s", p.Nome, p.Resumo)
			if len(p.Etapas) > 0 {
				linhas = append(linhas, l+" (etapas: "+strings.Join(p.Etapas, ", ")+")")
			} else {
				linhas = append(linhas, l)
			}
		}
		recursos = append(recursos, "## Procedimentos (tool abrir_workflow — abra o procedimento ANTES de executá-lo; em multi-etapa, abra etapa por etapa)\n"+strings.Join(linhas, "\n"))
	}

	if rag := a.Arquivos["rag"]; len(rag) > 0 {
		var linhas []string
		for _, r := range rag {
			linhas = append(linhas, "- `"+r.Caminho+"`")
		}
		recursos = append(recursos, "## Base RAG (tool buscar_rag — busca semântica; prefira-a pra achar conhecimento nestes documentos)\n"+strings.Join(linhas, "\n"))
	}

	if len(a.Apis) > 0 {
		registro := map[string]API{}
		for _, api := range c.ListarAPIs() {
			registro[api.Nome] = api
		}
		var linhas []string
		for _, nome := range a.Apis {
			api, ok := registro[nome]
			if !ok {
				continue
			}
			l := fmt.Sprintf("- `%s` (base %s) — %s", api.Nome, api.BaseURL, api.Descricao)
			if len(api.Requer) > 0 {
				l += " [requer: " + strings.Join(api.Requer, ", ") + "]"
			}
			if api.Manual != "" {
				l += "\n  manual: " + strings.ReplaceAll(strings.TrimSpace(api.Manual), "\n", "\n  ")
			}
			linhas = append(linhas, l)
		}
		if len(linhas) > 0 {
			recursos = append(recursos, "## APIs registradas (tool chamar_api)\n"+strings.Join(linhas, "\n"))
		}
	}

	if hooks := a.Arquivos["hooks"]; len(hooks) > 0 {
		var linhas []string
		for _, h := range hooks {
			var d struct {
				Nome   string         `yaml:"nome"`
				Evento string         `yaml:"evento"`
				Quando map[string]any `yaml:"quando"`
				Acao   struct {
					Tipo string `yaml:"tipo"`
					Nome string `yaml:"nome"`
				} `yaml:"acao"`
			}
			if yaml.Unmarshal([]byte(c.lerArquivo(a, h)), &d) != nil || d.Nome == "" {
				continue
			}
			l := fmt.Sprintf("- `%s` em %s", d.Nome, d.Evento)
			if len(d.Quando) > 0 {
				l += fmt.Sprintf(" quando %v", d.Quando)
			}
			linhas = append(linhas, strings.TrimSpace(l+" → "+d.Acao.Tipo+" "+d.Acao.Nome))
		}
		if len(linhas) > 0 {
			recursos = append(recursos, "## Automações da pasta (o RUNTIME dispara sozinho — não tente executá-las)\n"+strings.Join(linhas, "\n"))
		}
	}

	if seg := c.SegredosDeclarados(a); len(seg) > 0 {
		var q []string
		for _, s := range seg {
			q = append(q, "`"+s+"`")
		}
		recursos = append(recursos, "## Acessos protegidos\nVariáveis de ambiente disponíveis pro runtime (NUNCA revele nem imprima os valores): "+strings.Join(q, ", "))
	}

	if len(a.Subagentes) > 0 {
		var linhas []string
		for _, s := range a.Subagentes {
			linhas = append(linhas, fmt.Sprintf("- `%s` — %s", filepath.Base(s.Path), Descrever(s.Path)))
		}
		cab := "## Subtasks (tool invocar_agente — chame quando o SEU workflow mandar; você continua dono da resposta final)"
		if a.Tipo == Orquestrador {
			cab = "## Rotas de delegação (o roteamento já escolheu antes de você; se você está respondendo, é porque nenhuma servia)"
		}
		recursos = append(recursos, cab+"\n"+strings.Join(linhas, "\n"))
	}

	if len(recursos) > 0 {
		partes = append(partes, "# Recursos da pasta\n\n"+strings.Join(recursos, "\n\n"))
	}

	partes = append(partes, regrasGerais)
	return strings.Join(partes, "\n\n")
}

const regrasGerais = `# Regras gerais
- Responda em pt-BR.
- NUNCA afirme conteúdo de dados sem antes lê-los com as tools (ler_dado).
- Cite o arquivo fonte dos fatos que vierem de dados.
- Se existe um contrato de saída, a resposta final segue o contrato À RISCA.
- Se uma tool retornar {"error": ...}, NÃO repita a mesma chamada: corrija os
  argumentos ou, se o que falta é um dado do usuário, PERGUNTE a ele em uma frase
  e pare — pedir é melhor que insistir ou inventar.`

// Roteador compila só o que decide a rota (routers/*.md concatenados), para a
// passada barata do orquestrador.
func (c *Catalogo) Roteador(a *Agent) string {
	var pedacos []string
	for _, f := range a.Arquivos["routers"] {
		if t := c.lerArquivo(a, f); t != "" {
			pedacos = append(pedacos, pedaco(t))
		}
	}
	return strings.Join(pedacos, "\n\n")
}

// politicas: rules/*.md menos output.md, concatenados com os títulos rebaixados.
func (c *Catalogo) politicas(a *Agent) string {
	var pedacos []string
	for _, f := range a.Arquivos["rules"] {
		if f.Caminho == "rules/output.md" {
			continue
		}
		if t := c.lerArquivo(a, f); t != "" {
			pedacos = append(pedacos, pedaco(t))
		}
	}
	return strings.Join(pedacos, "\n\n")
}

// pedaco prepara um arquivo para concatenação: tira o frontmatter e rebaixa o H1.
func pedaco(texto string) string {
	linhas := strings.Split(strings.TrimSpace(StripFrontmatter(texto)), "\n")
	for i, l := range linhas {
		if strings.HasPrefix(l, "# ") {
			linhas[i] = "#" + l
		}
	}
	return strings.TrimSpace(strings.Join(linhas, "\n"))
}

// corpo: sem frontmatter e sem o H1 próprio (a seção já titula).
func corpo(texto string) string {
	linhas := strings.Split(strings.TrimSpace(StripFrontmatter(texto)), "\n")
	if len(linhas) > 0 && strings.HasPrefix(linhas[0], "# ") {
		linhas = linhas[1:]
	}
	return strings.TrimSpace(strings.Join(linhas, "\n"))
}

func (c *Catalogo) lerProprio(a *Agent, rel string) string {
	raw, err := os.ReadFile(filepath.Join(a.Path, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(raw)
}

// lerArquivo lê um Arquivo próprio ou herdado (pelo ancestral indicado).
func (c *Catalogo) lerArquivo(a *Agent, f Arquivo) string {
	base := a.Path
	if f.HerdadoDe != "" {
		for _, anc := range Ancestrais(a.Path) {
			if filepath.Base(anc) == f.HerdadoDe {
				base = anc
				break
			}
		}
	}
	raw, err := os.ReadFile(filepath.Join(base, filepath.FromSlash(f.Caminho)))
	if err != nil {
		return ""
	}
	return string(raw)
}

// comHeranca: próprios + herdados de uma caixinha, em ordem de caminho.
func (c *Catalogo) comHeranca(a *Agent, cx string) []Arquivo {
	out := append([]Arquivo{}, a.Arquivos[cx]...)
	out = append(out, a.Herdados[cx]...)
	sort.Slice(out, func(i, j int) bool { return out[i].Caminho < out[j].Caminho })
	return out
}

// Dados: o índice de data/ e assets/ (próprios, herdados e fontes), como o prompt vê.
func (c *Catalogo) Dados(a *Agent) []Arquivo {
	out := append([]Arquivo{}, a.Arquivos["data"]...)
	out = append(out, a.Arquivos["assets"]...)
	out = append(out, a.Herdados["data"]...)
	out = append(out, a.Herdados["assets"]...)
	for _, fonte := range a.Fontes {
		dir := filepath.Clean(filepath.Join(a.Path, filepath.FromSlash(fonte)))
		if !dirExists(dir) || !Permitido(a.Path, dir) {
			continue
		}
		rotulo := "fonte:" + filepath.Base(dir)
		_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			out = append(out, Arquivo{Caminho: rotulo + "/" + filepath.ToSlash(rel), Bytes: info.Size(), Resumo: Resumo(p), HerdadoDe: "fonte"})
			return nil
		})
	}
	return out
}

// Procedimento é um workflow além do principal: um .md solto ou uma pasta com etapas.
type Procedimento struct {
	Nome   string   `json:"nome"`
	Resumo string   `json:"resumo"`
	Etapas []string `json:"etapas"`
}

// Procedimentos lista workflows/ exceto principal.md.
func (c *Catalogo) Procedimentos(a *Agent) []Procedimento {
	base := filepath.Join(a.Path, "workflows")
	entradas, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []Procedimento
	for _, e := range entradas {
		switch {
		case e.Name() == "principal.md" || strings.HasPrefix(e.Name(), "."):
			continue
		case !e.IsDir() && strings.HasSuffix(e.Name(), ".md"):
			p := filepath.Join(base, e.Name())
			out = append(out, Procedimento{Nome: strings.TrimSuffix(e.Name(), ".md"), Resumo: Resumo(p), Etapas: []string{}})
		case e.IsDir():
			sub, _ := os.ReadDir(filepath.Join(base, e.Name()))
			var etapas []string
			for _, s := range sub {
				if !s.IsDir() && strings.HasSuffix(s.Name(), ".md") {
					etapas = append(etapas, strings.TrimSuffix(s.Name(), ".md"))
				}
			}
			if len(etapas) == 0 {
				continue
			}
			sort.Strings(etapas)
			out = append(out, Procedimento{Nome: e.Name(), Resumo: Resumo(filepath.Join(base, e.Name(), etapas[0]+".md")), Etapas: etapas})
		}
	}
	return out
}

// SegredosDeclarados: os do agente + os `requer` das APIs vinculadas, sem repetição.
func (c *Catalogo) SegredosDeclarados(a *Agent) []string {
	vistos := map[string]bool{}
	var out []string
	add := func(s string) {
		if s != "" && !vistos[s] {
			vistos[s] = true
			out = append(out, s)
		}
	}
	for _, s := range a.Segredos {
		add(s)
	}
	registro := map[string]API{}
	for _, api := range c.ListarAPIs() {
		registro[api.Nome] = api
	}
	for _, nome := range a.Apis {
		if api, ok := registro[nome]; ok {
			for _, r := range api.Requer {
				add(r)
			}
		}
	}
	return out
}
