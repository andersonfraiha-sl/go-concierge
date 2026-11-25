package arquivos

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"

	"go-concierge/internal/harness/conteudo"
)

// Classes, na ordem em que a interface as mostra. A classe é o que o recurso É para o
// agente; `write` é ortogonal (um recurso de qualquer classe com write: true só entra
// por um passo do caso.yaml, depois do confirm).
var Classes = []string{"leitura", "escrita", "conhecimento", "memoria", "acao", "agente", "form"}

// SalvarRecursoFixture grava (cria ou substitui, pelo nome original) um recurso no
// instantâneo YAML da account, preservando o cabeçalho de comentários do arquivo.
func SalvarRecursoFixture(p, nomeOriginal string, r Recurso) error {
	if err := r.Validar(); err != nil {
		return err
	}
	lista, err := LerRecursosFixture(p)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if nomeOriginal == "" {
		nomeOriginal = r.Name
	}
	trocou := false
	for i := range lista {
		if lista[i].Name == nomeOriginal {
			lista[i] = r
			trocou = true
		} else if lista[i].Name == r.Name {
			return fmt.Errorf("já existe um recurso chamado %s", r.Name)
		}
	}
	if !trocou {
		lista = append(lista, r)
	}
	return gravarRecursosFixture(p, lista)
}

// RemoverRecursoFixture tira um recurso do instantâneo.
func RemoverRecursoFixture(p, nome string) error {
	lista, err := LerRecursosFixture(p)
	if err != nil {
		return err
	}
	var resto []Recurso
	for _, r := range lista {
		if r.Name != nome {
			resto = append(resto, r)
		}
	}
	if len(resto) == len(lista) {
		return fmt.Errorf("recurso não encontrado: %s", nome)
	}
	return gravarRecursosFixture(p, resto)
}

func gravarRecursosFixture(p string, lista []Recurso) error {
	var cab strings.Builder
	if raw, err := os.ReadFile(p); err == nil {
		for _, l := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(l, "#") {
				break
			}
			cab.WriteString(l + "\n")
		}
	}
	if lista == nil {
		lista = []Recurso{}
	}
	corpo, err := yaml.Marshal(struct {
		Recursos []Recurso `yaml:"recursos"`
	}{lista})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(cab.String()+string(corpo)), 0o644)
}

// LerRecursosFixture lê um arquivo `recursos: [...]` (o instantâneo de uma account,
// usado quando o Nexus não está no ar).
func LerRecursosFixture(p string) ([]Recurso, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	recursos, err := conteudo.InterpretarRecursosFixture(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return recursos, nil
}

// RecursosNexus devolve a lista da fonte configurada (vazia se não há fonte).
func (c *Catalogo) RecursosNexus() []Recurso {
	if c.Recursos == nil {
		return nil
	}
	return c.Recursos()
}

// TemNexus diz se há uma fonte de recursos para conferir declarações.
func (c *Catalogo) TemNexus() bool { return c.Recursos != nil }

func (c *Catalogo) recursoPorNome() map[string]Recurso {
	m := map[string]Recurso{}
	for _, r := range c.RecursosNexus() {
		m[r.Name] = r
	}
	return m
}

func (c *Catalogo) RecursosDoAgente(a *Agent) []RecursoDeclarado {
	tem := c.TemNexus()
	reg := c.recursoPorNome()
	var out []RecursoDeclarado
	vistos := map[string]bool{}
	add := func(nome, origem string) {
		if vistos[origem+":"+nome] {
			return
		}
		vistos[origem+":"+nome] = true
		d := RecursoDeclarado{Nome: nome, Origem: origem, Conferido: tem}
		if !tem {
			d.OK = true
			out = append(out, d)
			return
		}
		r, ok := reg[nome]
		switch {
		case !ok:
			d.Problema = "não existe na account do Nexus"
		case r.Write && origem == AgentYAML:
			d.Problema = "recurso de ESCRITA: só pode ser chamado por um passo do caso.yaml"
			d.Recurso = &r
		default:
			d.OK = true
			d.Recurso = &r
		}
		out = append(out, d)
	}
	for _, n := range a.Recursos {
		add(n, AgentYAML)
	}
	if a.Caso != nil {
		for _, n := range a.Caso.Recursos {
			add(n, CasoYAML)
		}
	}
	if out == nil {
		out = []RecursoDeclarado{}
	}
	return out
}
