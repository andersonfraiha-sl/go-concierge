package okf

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Conhecimento OKF: os documentos tipados (frontmatter `type: Question | Condition |
// Vocabulary | Domain | Policy | KnowledgeIndex…`) e os glossários (`glossary.yaml`,
// com termos, sinônimos e definição) que vivem em data/okf/ do agente (como no adam)
// ou em okf/ na raiz da pasta (como nos bundles do stt4). Entram no prompt como
// dados indexados, e a interface os mostra como "conhecimento", não como arquivo solto.

// Doc é um documento OKF em Markdown com frontmatter.
type Doc struct {
	Caminho   string   `json:"caminho"` // relativo ao agente dono
	Tipo      string   `json:"tipo"`
	ID        string   `json:"id,omitempty"`
	Titulo    string   `json:"titulo"`
	Descricao string   `json:"descricao,omitempty"`
	Status    string   `json:"status,omitempty"`
	Versao    string   `json:"versao,omitempty"`
	Dominio   string   `json:"dominio,omitempty"`
	Tags      []string `json:"tags"`
	Respostas []string `json:"respostas,omitempty"` // type Question: o espaço de respostas
	HerdadoDe string   `json:"herdado_de,omitempty"`
}

// Termo é uma entrada de glossário.
type Termo struct {
	ID        string   `json:"id"`
	Nome      string   `json:"display_name"`
	Sinonimos []string `json:"synonyms"`
	Definicao string   `json:"definition"`
	Labels    []string `json:"labels"`
	Arquivo   string   `json:"arquivo"`
	HerdadoDe string   `json:"herdado_de,omitempty"`
}

type Conhecimento struct {
	Docs   []Doc   `json:"docs"`
	Termos []Termo `json:"termos"`
}

// TiposOKF são os tipos reconhecidos (outros valem, mas a interface sugere estes).
var TiposOKF = []string{"Domain", "Vocabulary", "Question", "Condition", "Policy", "Procedure", "KnowledgeIndex"}

// Conhecimento lista os docs e termos OKF do agente, próprios e herdados.
func (c *Catalogo) Conhecimento(a *Agent) Conhecimento {
	out := Conhecimento{Docs: []Doc{}, Termos: []Termo{}}
	raizes := []struct{ dir, herdadoDe string }{{a.Path, ""}}
	for _, anc := range Ancestrais(a.Path) {
		raizes = append(raizes, struct{ dir, herdadoDe string }{anc, filepath.Base(anc)})
	}
	vistos := map[string]bool{}
	for _, r := range raizes {
		for _, sub := range []string{filepath.Join("data", "okf"), "okf"} {
			base := filepath.Join(r.dir, sub)
			if !dirExists(base) {
				continue
			}
			_ = filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(r.dir, p)
				rel = filepath.ToSlash(rel)
				if vistos[rel] {
					return nil
				}
				vistos[rel] = true
				nome := strings.ToLower(d.Name())
				switch {
				case strings.HasSuffix(nome, ".md"):
					if doc, ok := lerDoc(p, rel); ok {
						doc.HerdadoDe = r.herdadoDe
						out.Docs = append(out.Docs, doc)
					}
				case nome == "glossary.yaml" || nome == "glossario.yaml" || nome == "glossary.yml":
					for _, t := range lerGlossario(p, rel) {
						t.HerdadoDe = r.herdadoDe
						out.Termos = append(out.Termos, t)
					}
				}
				return nil
			})
		}
	}
	sort.Slice(out.Docs, func(i, j int) bool {
		if out.Docs[i].Tipo != out.Docs[j].Tipo {
			return out.Docs[i].Tipo < out.Docs[j].Tipo
		}
		return out.Docs[i].Caminho < out.Docs[j].Caminho
	})
	sort.Slice(out.Termos, func(i, j int) bool { return out.Termos[i].ID < out.Termos[j].ID })
	return out
}

func lerDoc(p, rel string) (Doc, bool) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return Doc{}, false
	}
	fm := Frontmatter(string(raw))
	doc := Doc{Caminho: rel, Tags: []string{}}
	doc.Tipo = str(fm["type"], str(fm["tipo"], "Doc"))
	doc.ID = str(fm["id"], "")
	doc.Titulo = str(fm["title"], str(fm["titulo"], ""))
	if doc.Titulo == "" {
		doc.Titulo = Resumo(p)
	}
	doc.Descricao = str(fm["description"], str(fm["descricao"], ""))
	doc.Status = str(fm["status"], "")
	doc.Versao = str(fm["version"], str(fm["versao"], ""))
	doc.Dominio = str(fm["domain"], str(fm["dominio"], ""))
	doc.Tags = stringList(fm["tags"])
	doc.Respostas = stringList(fm["answers"])
	return doc, true
}

func lerGlossario(p, rel string) []Termo {
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var g struct {
		Terms []struct {
			ID         string   `yaml:"id"`
			Display    string   `yaml:"display_name"`
			Synonyms   []string `yaml:"synonyms"`
			Definition string   `yaml:"definition"`
			Labels     []string `yaml:"labels"`
		} `yaml:"terms"`
	}
	if yaml.Unmarshal(raw, &g) != nil {
		return nil
	}
	var out []Termo
	for _, t := range g.Terms {
		if t.Synonyms == nil {
			t.Synonyms = []string{}
		}
		if t.Labels == nil {
			t.Labels = []string{}
		}
		out = append(out, Termo{ID: t.ID, Nome: t.Display, Sinonimos: t.Synonyms, Definicao: strings.TrimSpace(t.Definition), Labels: t.Labels, Arquivo: rel})
	}
	return out
}

func str(v any, padrao string) string {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return padrao
}
