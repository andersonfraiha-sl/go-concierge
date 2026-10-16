package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Vinculo é uma referência interna da pasta: um asset citado num workflow, uma
// rota apontando para um subagente, uma fonte do pai, uma API do registro…
// A interface mostra todos, com o estado de cada um; o validador transforma os
// quebrados em Problemas.
type Vinculo struct {
	Tipo      string `json:"tipo"`                // asset | dado | schema | script | skill | mcp | rag | bash | fonte | rota | subagente | api | segredo | hook
	Origem    string `json:"origem"`              // arquivo (relativo ao agente) ou campo do agent.yaml
	Alvo      string `json:"alvo"`                // como está escrito
	Resolvido string `json:"resolvido,omitempty"` // onde foi encontrado (próprio, herdado de X, registro)
	OK        bool   `json:"ok"`
	Problema  string `json:"problema,omitempty"`
}

var (
	reCrase = regexp.MustCompile("`([^`\n]+)`")
	reRef   = regexp.MustCompile(`^(data|assets|schemas|scripts|skills|mcps|rag|bash|workflows|fonte:[A-Za-z0-9_.-]+)/[^\s` + "`" + `]+$`)
	reRota  = regexp.MustCompile("rota\\s+`([^`]+)`")
)

// Vinculos varre rules/, workflows/, routers/, hooks/ e o agent.yaml.
func (c *Catalogo) Vinculos(a *Agent) []Vinculo {
	var out []Vinculo

	// 1. referências em crase nos .md das caixinhas de identidade
	for _, cx := range []string{"rules", "workflows", "routers"} {
		for _, f := range a.Arquivos[cx] {
			if !strings.HasSuffix(f.Caminho, ".md") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(a.Path, filepath.FromSlash(f.Caminho)))
			if err != nil {
				continue
			}
			texto := string(raw)
			vistos := map[string]bool{}
			for _, m := range reCrase.FindAllStringSubmatch(texto, -1) {
				ref := strings.TrimSpace(m[1])
				if !reRef.MatchString(ref) || vistos[ref] {
					continue
				}
				vistos[ref] = true
				out = append(out, c.resolveRef(a, f.Caminho, ref))
			}
			if cx == "routers" {
				for _, m := range reRota.FindAllStringSubmatch(texto, -1) {
					nome := strings.TrimSpace(m[1])
					if vistos["rota:"+nome] {
						continue
					}
					vistos["rota:"+nome] = true
					out = append(out, c.resolveRota(a, f.Caminho, nome))
				}
			}
		}
	}

	// 2. agent.yaml: fontes, apis, segredos
	for _, fonte := range a.Fontes {
		v := Vinculo{Tipo: "fonte", Origem: AgentYAML + " → fontes", Alvo: fonte}
		alvo := filepath.Clean(filepath.Join(a.Path, filepath.FromSlash(fonte)))
		switch {
		case !Permitido(a.Path, alvo):
			v.Problema = "aponta para fora do contexto do agente (irmão, sobrinho ou fora da árvore)"
		case !dirExists(alvo):
			v.Problema = "a pasta não existe"
		default:
			v.OK = true
			rel, _ := filepath.Rel(c.Root, alvo)
			v.Resolvido = filepath.ToSlash(rel)
		}
		out = append(out, v)
	}
	registro := map[string]API{}
	for _, api := range c.ListarAPIs() {
		registro[api.Nome] = api
	}
	for _, nome := range a.Apis {
		v := Vinculo{Tipo: "api", Origem: AgentYAML + " → apis", Alvo: nome}
		if api, ok := registro[nome]; ok {
			v.OK = true
			v.Resolvido = "apis/" + api.Arquivo
		} else {
			v.Problema = "não está no registro apis/ do catálogo"
		}
		out = append(out, v)
	}
	for _, s := range a.Segredos {
		out = append(out, Vinculo{Tipo: "segredo", Origem: AgentYAML + " → segredos", Alvo: s, OK: true, Resolvido: "variável de ambiente obrigatória"})
	}

	// 3. subagentes
	for _, s := range a.Subagentes {
		v := Vinculo{Tipo: "subagente", Origem: DirAgentes + "/", Alvo: filepath.Base(s.Path), OK: true, Resolvido: s.Rel}
		if strings.TrimSpace(s.Descricao) == "" {
			v.OK = false
			v.Problema = "subagente sem descricao: o roteador do pai não tem critério"
		}
		out = append(out, v)
	}

	// 4. hooks: a ação aponta para script/bash/api/agente
	for _, f := range a.Arquivos["hooks"] {
		raw, err := os.ReadFile(filepath.Join(a.Path, filepath.FromSlash(f.Caminho)))
		if err != nil {
			continue
		}
		var h struct {
			Nome string `yaml:"nome"`
			Acao struct {
				Tipo string `yaml:"tipo"`
				Nome string `yaml:"nome"`
			} `yaml:"acao"`
		}
		if yaml.Unmarshal(raw, &h) != nil || h.Acao.Tipo == "" {
			continue
		}
		v := Vinculo{Tipo: "hook", Origem: f.Caminho, Alvo: h.Acao.Tipo + ":" + h.Acao.Nome}
		switch h.Acao.Tipo {
		case "script":
			r := c.resolveRef(a, f.Caminho, "scripts/"+h.Acao.Nome)
			v.OK, v.Resolvido, v.Problema = r.OK, r.Resolvido, r.Problema
		case "agente":
			r := c.resolveRota(a, f.Caminho, h.Acao.Nome)
			v.OK, v.Resolvido, v.Problema = r.OK, r.Resolvido, r.Problema
		case "api":
			if api, ok := registro[h.Acao.Nome]; ok {
				v.OK, v.Resolvido = true, "apis/"+api.Arquivo
			} else {
				v.Problema = "API fora do registro"
			}
		case "bash":
			v.OK = true
		default:
			v.Problema = "tipo de ação desconhecido (script | bash | api | agente)"
		}
		out = append(out, v)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].OK != out[j].OK {
			return !out[i].OK
		}
		return out[i].Tipo < out[j].Tipo
	})
	return out
}

// resolveRef procura `data/x`, `assets/y`, `fonte:nome/z`… na própria pasta, nos
// ancestrais (só caixinhas que herdam) e nas fontes declaradas.
func (c *Catalogo) resolveRef(a *Agent, origem, ref string) Vinculo {
	v := Vinculo{Origem: origem, Alvo: ref}
	cx, resto, _ := strings.Cut(ref, "/")
	switch {
	case cx == "assets":
		v.Tipo = "asset"
	case cx == "data":
		v.Tipo = "dado"
	case strings.HasPrefix(cx, "fonte:"):
		v.Tipo = "fonte"
	case cx == "schemas":
		v.Tipo = "schema"
	case cx == "scripts":
		v.Tipo = "script"
	case cx == "skills":
		v.Tipo = "skill"
	case cx == "mcps":
		v.Tipo = "mcp"
	case cx == "rag":
		v.Tipo = "rag"
	case cx == "bash":
		v.Tipo = "bash"
	case cx == "workflows":
		v.Tipo = "procedimento"
	}

	if strings.HasPrefix(cx, "fonte:") {
		nome := strings.TrimPrefix(cx, "fonte:")
		for _, fonte := range a.Fontes {
			alvo := filepath.Clean(filepath.Join(a.Path, filepath.FromSlash(fonte)))
			if filepath.Base(alvo) != nome {
				continue
			}
			if fileExists(filepath.Join(alvo, filepath.FromSlash(resto))) {
				v.OK = true
				rel, _ := filepath.Rel(c.Root, alvo)
				v.Resolvido = "fonte " + filepath.ToSlash(rel)
				return v
			}
		}
		v.Problema = "não existe na fonte declarada (agent.yaml → fontes)"
		return v
	}

	// próprio (aceita arquivo ou pasta, p.ex. um procedimento multi-etapa)
	proprio := filepath.Join(a.Path, filepath.FromSlash(ref))
	if fileExists(proprio) || dirExists(proprio) {
		v.OK = true
		v.Resolvido = "próprio"
		return v
	}
	def, _ := CaixinhaPorNome(cx)
	if def.Herda {
		for _, anc := range Ancestrais(a.Path) {
			p := filepath.Join(anc, filepath.FromSlash(ref))
			if fileExists(p) || dirExists(p) {
				v.OK = true
				v.Resolvido = "herdado de " + filepath.Base(anc)
				return v
			}
		}
	}
	// existe num irmão ou em outro lugar? Dá a dica certa.
	if cx != "" && resto != "" {
		if achado := c.procurarEmOutroAgente(a, ref); achado != "" {
			v.Problema = fmt.Sprintf("não existe aqui; existe em %s (fronteira: copie para dentro ou mova para o ancestral)", achado)
			return v
		}
	}
	v.Problema = "arquivo não existe na pasta" + map[bool]string{true: " nem nos ancestrais", false: ""}[def.Herda]
	return v
}

func (c *Catalogo) procurarEmOutroAgente(a *Agent, ref string) string {
	var achado string
	_ = filepath.WalkDir(filepath.Join(c.Root, DirAgentes), func(p string, d os.DirEntry, err error) error {
		if err != nil || achado != "" {
			return nil
		}
		if d.IsDir() && IsAgent(p) && p != a.Path {
			if fileExists(filepath.Join(p, filepath.FromSlash(ref))) {
				rel, _ := filepath.Rel(c.Root, p)
				achado = filepath.ToSlash(rel)
			}
		}
		return nil
	})
	return achado
}

func (c *Catalogo) resolveRota(a *Agent, origem, nome string) Vinculo {
	v := Vinculo{Tipo: "rota", Origem: origem, Alvo: nome}
	if nome == "self" {
		v.OK = true
		v.Resolvido = "o próprio agente responde"
		return v
	}
	for _, s := range a.Subagentes {
		if filepath.Base(s.Path) == nome || s.Nome == nome {
			v.OK = true
			v.Resolvido = s.Rel
			return v
		}
	}
	v.Problema = "não há subagente com esse nome em agentes/"
	return v
}

// validar transforma vínculos quebrados e lacunas de anatomia em Problemas.
func (c *Catalogo) validar(a *Agent) []Problema {
	var ps []Problema
	for _, v := range c.Vinculos(a) {
		if v.OK {
			continue
		}
		p := Problema{Nivel: "erro", Onde: v.Origem, Texto: fmt.Sprintf("%s `%s`: %s", v.Tipo, v.Alvo, v.Problema)}
		if v.Tipo == "subagente" || v.Tipo == "segredo" {
			p.Nivel = "aviso"
		}
		ps = append(ps, p)
	}
	temRouters := len(a.Arquivos["routers"]) > 0
	temPrincipal := false
	temOutput := false
	for _, f := range a.Arquivos["workflows"] {
		if f.Caminho == "workflows/principal.md" {
			temPrincipal = true
		}
	}
	for _, f := range a.Arquivos["rules"] {
		if f.Caminho == "rules/output.md" {
			temOutput = true
		}
	}
	switch a.Tipo {
	case Orquestrador:
		if len(a.Subagentes) == 0 {
			ps = append(ps, Problema{Nivel: "erro", Onde: AgentYAML, Texto: "orquestrador sem subagentes: não há para quem delegar", Solucao: "crie um subagente ou mude o tipo para operador"})
		}
		if !temRouters {
			ps = append(ps, Problema{Nivel: "aviso", Onde: "routers/", Texto: "sem routers/: roteia só pelas descrições dos subagentes", Solucao: "crie routers/rotas.md com um critério por rota e a saída `self`"})
		}
	case Operador:
		if !temPrincipal {
			ps = append(ps, Problema{Nivel: "aviso", Onde: "workflows/principal.md", Texto: "operador sem passo a passo projetado", Solucao: "escreva workflows/principal.md citando as tools e os arquivos"})
		}
	}
	if !temOutput {
		ps = append(ps, Problema{Nivel: "aviso", Onde: "rules/output.md", Texto: "sem contrato de saída: a resposta não tem forma garantida"})
	}
	if strings.TrimSpace(a.Descricao) == "" {
		ps = append(ps, Problema{Nivel: "aviso", Onde: AgentYAML + " → descricao", Texto: "sem descrição: o roteador do pai e o índice de subagentes ficam sem critério"})
	}
	return ps
}
