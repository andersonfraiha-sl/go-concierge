package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Projecao compila a pasta num system prompt (princípio ICM: o modelo lê uma
// projeção; os dados ficam indexados e são lidos sob demanda pelas tools). É o que
// a interface mostra em "projeção" e o que o harness Eino recebe como instrução.
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

	if p := strings.TrimSpace(a.Persona); p != "" {
		partes = append(partes, "# Persona\n\n"+corpo(p))
	}
	if pol := c.politicas(a); pol != "" {
		partes = append(partes, "# Políticas\n\n"+pol)
	}
	if wf := c.lerProprio(a, "workflows/principal.md"); wf != "" {
		partes = append(partes, "# Workflow (siga o passo a passo)\n\n"+corpo(wf))
	}
	if a.Caso != nil {
		var linhas []string
		for i, p := range a.Caso.Passos() {
			linhas = append(linhas, fmt.Sprintf("%d. %s", i+1, p))
		}
		cab := fmt.Sprintf("# Caso de uso `%s` — %s\n\n%s\n\nO harness executa estes passos de forma determinística; você só decide e redige quando um passo pedir:\n\n", a.Caso.ID, a.Caso.Name, strings.TrimSpace(a.Caso.Description))
		partes = append(partes, cab+strings.Join(linhas, "\n"))
	}
	if out := c.lerProprio(a, "rules/output.md"); out != "" {
		partes = append(partes, "# Contrato de saída\n\nSua resposta final DEVE seguir exatamente este contrato:\n\n"+corpo(out))
	}

	var recursos []string

	if idx := c.Indice(a); len(idx) > 0 {
		var linhas []string
		for _, d := range idx {
			l := fmt.Sprintf("- `%s` (%d bytes) — %s", d.Caminho, d.Bytes, d.Resumo)
			if d.HerdadoDe != "" {
				l += " [herdado de " + d.HerdadoDe + "]"
			}
			linhas = append(linhas, l)
		}
		recursos = append(recursos, "## Conhecimento e assets (leia com a tool ler_dado; pagina quando tem_mais=true; um index.md lista o que há na pasta dele e os links `[x](knowledge/...)` são caminhos que ler_dado aceita)\n"+strings.Join(linhas, "\n"))
	}

	if procs := c.Procedimentos(a); len(procs) > 0 {
		var linhas []string
		for _, p := range procs {
			l := fmt.Sprintf("- `%s` — %s", p.Nome, p.Resumo)
			if len(p.Etapas) > 0 {
				l += " (etapas: " + strings.Join(p.Etapas, ", ") + ")"
			}
			linhas = append(linhas, l)
		}
		recursos = append(recursos, "## Procedimentos (tool abrir_workflow — abra o procedimento ANTES de executá-lo; em multi-etapa, abra etapa por etapa)\n"+strings.Join(linhas, "\n"))
	}

	if decl := c.RecursosDoAgente(a); len(decl) > 0 {
		var leitura, escrita []string
		for _, d := range decl {
			if !d.OK && d.Conferido {
				continue
			}
			l := "- `" + d.Nome + "`"
			if d.Recurso != nil {
				l += fmt.Sprintf(" (%s, %s) — %s", d.Recurso.Kind, d.Recurso.Visibility, d.Recurso.Description)
				if len(d.Recurso.Params) > 0 {
					var ps []string
					for _, p := range d.Recurso.Params {
						s := p.Name
						if p.Type != "" {
							s += ":" + p.Type
						}
						if p.Required {
							s += "*"
						}
						ps = append(ps, s)
					}
					l += " · params " + strings.Join(ps, ", ")
				}
			}
			if d.Recurso != nil && d.Recurso.Write {
				escrita = append(escrita, l)
			} else {
				leitura = append(leitura, l)
			}
		}
		if len(leitura) > 0 {
			recursos = append(recursos, "## Recursos do Nexus que você pode chamar (tool chamar_recurso; só estes, e só os que a identidade do morador permitir)\n"+strings.Join(leitura, "\n"))
		}
		if len(escrita) > 0 {
			recursos = append(recursos, "## Recursos de escrita (NUNCA chame diretamente: o harness os executa pelos passos do caso de uso, com confirmação)\n"+strings.Join(escrita, "\n"))
		}
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
		partes = append(partes, "# Recursos\n\n"+strings.Join(recursos, "\n\n"))
	}
	partes = append(partes, regrasGerais)
	return strings.Join(partes, "\n\n")
}

const regrasGerais = `# Regras gerais
- Responda em pt-BR.
- NUNCA afirme conteúdo de conhecimento ou de recurso sem antes lê-lo com as tools.
- Cite a fonte dos fatos que vierem de conhecimento ou de recurso.
- Se existe um contrato de saída, a resposta final segue o contrato À RISCA.
- Se uma tool retornar {"error": ...}, NÃO repita a mesma chamada: corrija os
  argumentos ou, se o que falta é um dado do usuário, PERGUNTE a ele em uma frase
  e pare — pedir é melhor que insistir ou inventar.`

// Roteador compila só o que decide a rota (routers/*.md concatenados).
func (c *Catalogo) Roteador(a *Agent) string {
	var pedacos []string
	for _, f := range a.Arquivos["routers"] {
		if t := c.lerArquivo(a, f); t != "" {
			pedacos = append(pedacos, pedaco(t))
		}
	}
	return strings.Join(pedacos, "\n\n")
}

// Politicas devolve o texto das políticas do agente (rules/*.md menos output.md),
// para quem precisa delas fora da projeção inteira (a entrega do orquestrador).
func (c *Catalogo) Politicas(a *Agent) string { return c.politicas(a) }

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
	raw, err := c.Ler(a, f)
	if err != nil {
		return ""
	}
	return string(raw)
}

// Indice: o que o prompt vê como dados: knowledge/ e assets/, próprios e herdados,
// com a progressive disclosure dos pacotes OKF (só o index.md de uma pasta que o tem;
// log.md nunca). Para localizar um arquivo a ler, use IndiceCompleto.
func (c *Catalogo) Indice(a *Agent) []Arquivo {
	return podarIndice(c.IndiceCompleto(a))
}

// IndiceCompleto: todos os dados do agente, inclusive o que o prompt não lista
// (irmãos de um index.md, log.md); é o que ler_dado aceita abrir.
func (c *Catalogo) IndiceCompleto(a *Agent) []Arquivo {
	out := append([]Arquivo{}, a.Arquivos["knowledge"]...)
	out = append(out, a.Arquivos["assets"]...)
	out = append(out, a.Herdados["knowledge"]...)
	out = append(out, a.Herdados["assets"]...)
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].HerdadoDe == "") != (out[j].HerdadoDe == "") {
			return out[i].HerdadoDe == ""
		}
		return out[i].Caminho < out[j].Caminho
	})
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
			out = append(out, Procedimento{Nome: strings.TrimSuffix(e.Name(), ".md"), Resumo: Resumo(filepath.Join(base, e.Name())), Etapas: []string{}})
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
