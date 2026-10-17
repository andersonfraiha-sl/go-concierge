package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Vinculo é uma referência interna da pasta ou uma declaração ao Nexus: um asset
// citado num workflow, um doc de conhecimento, uma rota para um subagente, um
// recurso declarado. A interface mostra todos com o estado; o validador transforma
// os quebrados em Problemas.
type Vinculo struct {
	Tipo      string `json:"tipo"` // asset | conhecimento | procedimento | rota | subagente | recurso | persona
	Origem    string `json:"origem"`
	Alvo      string `json:"alvo"`
	Resolvido string `json:"resolvido,omitempty"`
	OK        bool   `json:"ok"`
	Problema  string `json:"problema,omitempty"`
}

var (
	reCrase = regexp.MustCompile("`([^`\n]+)`")
	reRef   = regexp.MustCompile(`^(knowledge|assets|workflows)/[^\s` + "`" + `]+$`)
	reRota  = regexp.MustCompile("rota\\s+`([^`]+)`")
)

// Vinculos varre rules/, workflows/, routers/, persona.md, agent.yaml e caso.yaml.
func (c *Catalogo) Vinculos(a *Agent) []Vinculo {
	var out []Vinculo

	// referências em crase nos textos de identidade
	textos := []Arquivo{}
	for _, cx := range []string{"rules", "workflows", "routers"} {
		textos = append(textos, a.Arquivos[cx]...)
	}
	if fileExists(filepath.Join(a.Path, PersonaMD)) {
		textos = append(textos, Arquivo{Caminho: PersonaMD})
	}
	for _, f := range textos {
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
		if strings.HasPrefix(f.Caminho, "routers/") {
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

	// recursos declarados ao Nexus
	for _, d := range c.RecursosDoAgente(a) {
		v := Vinculo{Tipo: "recurso", Origem: d.Origem, Alvo: d.Nome, OK: d.OK, Problema: d.Problema}
		switch {
		case !d.Conferido:
			v.Resolvido = "não conferido (Nexus indisponível)"
		case d.Recurso != nil && d.OK:
			v.Resolvido = fmt.Sprintf("nexus · %s · %s%s", d.Recurso.Kind, d.Recurso.Visibility, map[bool]string{true: " · escrita (pelo caso)", false: ""}[d.Recurso.Write])
		}
		out = append(out, v)
	}

	// persona em vigor
	switch {
	case a.PersonaDe != "":
		out = append(out, Vinculo{Tipo: "persona", Origem: PersonaMD, Alvo: "herdada", OK: true, Resolvido: "persona de " + a.PersonaDe + " (primeiro nível)"})
	case a.Persona != "":
		out = append(out, Vinculo{Tipo: "persona", Origem: PersonaMD, Alvo: "própria", OK: true, Resolvido: "próprio"})
	}

	// subagentes
	for _, s := range a.Subagentes {
		v := Vinculo{Tipo: "subagente", Origem: DirAgentes + "/", Alvo: filepath.Base(s.Path), OK: true, Resolvido: s.Rel}
		if strings.TrimSpace(s.Descricao) == "" {
			v.OK = false
			v.Problema = "subagente sem descricao: o roteador do pai não tem critério"
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

// resolveRef procura `knowledge/x`, `assets/y`, `workflows/z` na própria pasta e,
// nas caixinhas que herdam, nos ancestrais.
func (c *Catalogo) resolveRef(a *Agent, origem, ref string) Vinculo {
	v := Vinculo{Origem: origem, Alvo: ref}
	cx, _, _ := strings.Cut(ref, "/")
	switch cx {
	case "assets":
		v.Tipo = "asset"
	case "knowledge":
		v.Tipo = "conhecimento"
	case "workflows":
		v.Tipo = "procedimento"
	}
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
	if achado := c.procurarEmOutroAgente(a, ref); achado != "" {
		v.Problema = fmt.Sprintf("não existe aqui; existe em %s (fronteira: copie para dentro ou mova para o ancestral)", achado)
		return v
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
		if d.IsDir() && IsAgent(p) && p != a.Path && fileExists(filepath.Join(p, filepath.FromSlash(ref))) {
			rel, _ := filepath.Rel(c.Root, p)
			achado = filepath.ToSlash(rel)
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
		if v.Tipo == "subagente" {
			p.Nivel = "aviso"
		}
		if v.Tipo == "recurso" && strings.Contains(v.Problema, "ESCRITA") {
			p.Solucao = "tire de agent.yaml → recursos e chame pelo passo `resource` do caso.yaml, depois de um `confirm`"
		}
		ps = append(ps, p)
	}
	temRouters := len(a.Arquivos["routers"]) > 0
	temPrincipal, temOutput := false, false
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
		if a.Persona == "" && len(Ancestrais(a.Path)) == 0 {
			ps = append(ps, Problema{Nivel: "aviso", Onde: PersonaMD, Texto: "quem fala com o morador precisa de persona (tom, traços, frases)", Solucao: "crie persona.md neste agente de primeiro nível; todos os subagentes herdam"})
		}
	case Operador:
		if !temPrincipal && a.Caso == nil {
			ps = append(ps, Problema{Nivel: "aviso", Onde: "workflows/principal.md", Texto: "operador sem passo a passo nem caso.yaml", Solucao: "escreva workflows/principal.md ou defina o caso de uso em caso.yaml"})
		}
	}
	if !temOutput && a.Caso == nil {
		ps = append(ps, Problema{Nivel: "aviso", Onde: "rules/output.md", Texto: "sem contrato de saída: a resposta não tem forma garantida"})
	}
	if strings.TrimSpace(a.Descricao) == "" {
		ps = append(ps, Problema{Nivel: "aviso", Onde: AgentYAML + " → descricao", Texto: "sem descrição: o roteador do pai e o índice de subagentes ficam sem critério"})
	}
	if a.Caso != nil && len(a.Caso.Recursos) > 0 && !a.Caso.Confirma {
		if reg := c.recursoPorNome(); len(reg) > 0 {
			for _, n := range a.Caso.Recursos {
				if r, ok := reg[n]; ok && r.Write {
					ps = append(ps, Problema{Nivel: "aviso", Onde: CasoYAML, Texto: fmt.Sprintf("escrita `%s` sem passo `confirm` antes", n), Solucao: "acrescente um `confirm` mostrando os dados antes do recurso de escrita"})
					break
				}
			}
		}
	}
	return ps
}
