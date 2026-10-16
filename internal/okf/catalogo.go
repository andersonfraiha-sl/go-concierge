package okf

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Catálogos: recursos cadastrados UMA vez na raiz e aplicados aos agentes
// (na interface, arrastando o cartão sobre o agente; por API, Aplicar).
//
//	bases/<nome>/base.yaml + mcps/ schemas/ skills/ scripts/ bash/ data/   conector completo
//	tools/scripts/*  tools/mcps/*  tools/assets/*  tools/skills/*  tools/schemas/*   avulsos
//	apis/*.yaml                                                           registro de APIs (por referência)

// API é uma entrada do registro apis/*.yaml.
type API struct {
	Nome      string            `json:"nome" yaml:"nome"`
	Arquivo   string            `json:"arquivo" yaml:"-"`
	Descricao string            `json:"descricao" yaml:"descricao"`
	BaseURL   string            `json:"base_url" yaml:"base_url"`
	Headers   map[string]string `json:"headers,omitempty" yaml:"headers"`
	Requer    []string          `json:"requer" yaml:"requer"`
	Manual    string            `json:"manual,omitempty" yaml:"manual"`
}

// ListarAPIs lê apis/*.yaml.
func (c *Catalogo) ListarAPIs() []API {
	entradas, err := os.ReadDir(filepath.Join(c.Root, "apis"))
	if err != nil {
		return nil
	}
	var out []API
	for _, e := range entradas {
		if e.IsDir() || !(strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(c.Root, "apis", e.Name()))
		if err != nil {
			continue
		}
		var api API
		if yaml.Unmarshal(raw, &api) != nil {
			continue
		}
		if api.Nome == "" {
			api.Nome = strings.TrimSuffix(strings.TrimSuffix(e.Name(), ".yaml"), ".yml")
		}
		if api.Requer == nil {
			api.Requer = []string{}
		}
		api.Arquivo = e.Name()
		out = append(out, api)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nome < out[j].Nome })
	return out
}

// Base é um conector completo em bases/<nome>/.
type Base struct {
	Nome      string              `json:"nome"`
	Tipo      string              `json:"tipo"`
	Descricao string              `json:"descricao"`
	Segredos  []string            `json:"segredos"`
	Apis      []string            `json:"apis"`
	Conteudo  map[string][]string `json:"conteudo"` // caixinha → arquivos
}

var subpastasDeBase = []string{"mcps", "schemas", "skills", "scripts", "bash", "data", "assets"}

// ListarBases lê bases/*/base.yaml.
func (c *Catalogo) ListarBases() []Base {
	entradas, err := os.ReadDir(filepath.Join(c.Root, "bases"))
	if err != nil {
		return nil
	}
	var out []Base
	for _, e := range entradas {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(c.Root, "bases", e.Name())
		raw, err := os.ReadFile(filepath.Join(dir, "base.yaml"))
		if err != nil {
			continue
		}
		var b Base
		_ = yaml.Unmarshal(raw, &b)
		if b.Nome == "" {
			b.Nome = e.Name()
		}
		if b.Segredos == nil {
			b.Segredos = []string{}
		}
		if b.Apis == nil {
			b.Apis = []string{}
		}
		b.Conteudo = map[string][]string{}
		for _, sub := range subpastasDeBase {
			itens, err := os.ReadDir(filepath.Join(dir, sub))
			if err != nil {
				continue
			}
			for _, it := range itens {
				if !it.IsDir() && !strings.HasPrefix(it.Name(), ".") {
					b.Conteudo[sub] = append(b.Conteudo[sub], it.Name())
				}
			}
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nome < out[j].Nome })
	return out
}

// Recurso é um item avulso de tools/<tipo>/.
type Recurso struct {
	Tipo   string `json:"tipo"` // script | mcp | asset | skill | schema
	Nome   string `json:"nome"`
	Resumo string `json:"resumo"`
	Bytes  int64  `json:"bytes"`
}

// tiposDeTool mapeia o tipo do recurso para a subpasta de tools/ e a caixinha destino.
var tiposDeTool = map[string][2]string{
	"script": {"scripts", "scripts"},
	"mcp":    {"mcps", "mcps"},
	"asset":  {"assets", "assets"},
	"skill":  {"skills", "skills"},
	"schema": {"schemas", "schemas"},
}

// ListarTools lê tools/{scripts,mcps,assets,skills,schemas}.
func (c *Catalogo) ListarTools() map[string][]Recurso {
	out := map[string][]Recurso{}
	for tipo, par := range tiposDeTool {
		out[tipo] = []Recurso{}
		entradas, err := os.ReadDir(filepath.Join(c.Root, "tools", par[0]))
		if err != nil {
			continue
		}
		for _, e := range entradas {
			if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			info, _ := e.Info()
			p := filepath.Join(c.Root, "tools", par[0], e.Name())
			out[tipo] = append(out[tipo], Recurso{Tipo: tipo, Nome: e.Name(), Resumo: Resumo(p), Bytes: info.Size()})
		}
	}
	return out
}

// Aplicar põe um recurso do catálogo no agente: copia arquivos para a caixinha
// certa e funde segredos/apis no agent.yaml. Idempotente: aplicar de novo não
// duplica nada. Devolve a lista do que mudou.
func (c *Catalogo) Aplicar(agenteRel, tipo, nome string) ([]string, error) {
	dir, err := c.ResolveAgente(agenteRel)
	if err != nil {
		return nil, err
	}
	if !IsAgent(dir) {
		return nil, fmt.Errorf("agente não existe: %s", agenteRel)
	}
	nome = filepath.Base(nome)
	var mudancas []string

	copiar := func(origens []string, sub string) error {
		if len(origens) == 0 {
			return nil
		}
		destino := filepath.Join(dir, sub)
		if err := os.MkdirAll(destino, 0o755); err != nil {
			return err
		}
		for _, o := range origens {
			alvo := filepath.Join(destino, filepath.Base(o))
			if iguais(o, alvo) {
				continue
			}
			if err := copiarArquivo(o, alvo); err != nil {
				return err
			}
			mudancas = append(mudancas, sub+"/"+filepath.Base(o))
		}
		return nil
	}

	switch tipo {
	case "api":
		achou := false
		for _, api := range c.ListarAPIs() {
			if api.Nome == nome {
				achou = true
			}
		}
		if !achou {
			return nil, fmt.Errorf("api desconhecida: %s", nome)
		}
		m, err := c.mesclarLista(dir, "apis", []string{nome})
		if err != nil {
			return nil, err
		}
		mudancas = append(mudancas, m...)
	case "base":
		baseDir := filepath.Join(c.Root, "bases", nome)
		raw, err := os.ReadFile(filepath.Join(baseDir, "base.yaml"))
		if err != nil {
			return nil, fmt.Errorf("base desconhecida: %s", nome)
		}
		var b Base
		_ = yaml.Unmarshal(raw, &b)
		for _, sub := range subpastasDeBase {
			itens, err := os.ReadDir(filepath.Join(baseDir, sub))
			if err != nil {
				continue
			}
			var origens []string
			for _, it := range itens {
				if !it.IsDir() && !strings.HasPrefix(it.Name(), ".") {
					origens = append(origens, filepath.Join(baseDir, sub, it.Name()))
				}
			}
			if err := copiar(origens, sub); err != nil {
				return nil, err
			}
		}
		if len(b.Segredos) > 0 {
			m, err := c.mesclarLista(dir, "segredos", b.Segredos)
			if err != nil {
				return nil, err
			}
			mudancas = append(mudancas, m...)
		}
		if len(b.Apis) > 0 {
			m, err := c.mesclarLista(dir, "apis", b.Apis)
			if err != nil {
				return nil, err
			}
			mudancas = append(mudancas, m...)
		}
	default:
		par, ok := tiposDeTool[tipo]
		if !ok {
			return nil, fmt.Errorf("tipo desconhecido: %s (use base | api | script | mcp | asset | skill | schema)", tipo)
		}
		origem := filepath.Join(c.Root, "tools", par[0], nome)
		if !fileExists(origem) {
			return nil, fmt.Errorf("%s desconhecido: %s", tipo, nome)
		}
		if err := copiar([]string{origem}, par[1]); err != nil {
			return nil, err
		}
	}
	if len(mudancas) == 0 {
		mudancas = append(mudancas, "nada a fazer — o agente já tinha esse recurso")
	}
	return mudancas, nil
}

// mesclarLista acrescenta itens a uma lista do agent.yaml, sem repetir.
func (c *Catalogo) mesclarLista(dir, chave string, novos []string) ([]string, error) {
	cfg, err := lerConfig(dir)
	if err != nil {
		return nil, err
	}
	atuais := stringList(cfg[chave])
	var faltam []string
	for _, n := range novos {
		achou := false
		for _, a := range atuais {
			if a == n {
				achou = true
			}
		}
		if !achou {
			faltam = append(faltam, n)
		}
	}
	if len(faltam) == 0 {
		return nil, nil
	}
	cfg[chave] = append(atuais, faltam...)
	if err := gravarConfig(dir, cfg); err != nil {
		return nil, err
	}
	return []string{fmt.Sprintf("agent.yaml: %s += %s", chave, strings.Join(faltam, ", "))}, nil
}

func lerConfig(dir string) (map[string]any, error) {
	cfg := map[string]any{}
	raw, err := os.ReadFile(filepath.Join(dir, AgentYAML))
	if err != nil {
		if os.IsNotExist(err) {
			cfg["nome"] = filepath.Base(dir)
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("agent.yaml inválido: %w", err)
	}
	if cfg == nil {
		cfg = map[string]any{"nome": filepath.Base(dir)}
	}
	return cfg, nil
}

// gravarConfig escreve o agent.yaml com os campos principais primeiro, na ordem
// em que uma pessoa lê (nome, tipo, descricao, modelo, fontes, apis, segredos).
func gravarConfig(dir string, cfg map[string]any) error {
	ordem := []string{"nome", "tipo", "descricao", "modelo", "fontes", "apis", "segredos"}
	var doc yaml.Node
	doc.Kind = yaml.MappingNode
	add := func(k string, v any) {
		var val yaml.Node
		if err := val.Encode(v); err != nil {
			return
		}
		doc.Content = append(doc.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, &val)
	}
	vistos := map[string]bool{}
	for _, k := range ordem {
		if v, ok := cfg[k]; ok && v != nil {
			if l, isList := v.([]string); isList && len(l) == 0 {
				continue
			}
			if s, isStr := v.(string); isStr && s == "" {
				continue
			}
			add(k, v)
			vistos[k] = true
		}
	}
	var resto []string
	for k := range cfg {
		if !vistos[k] {
			resto = append(resto, k)
		}
	}
	sort.Strings(resto)
	for _, k := range resto {
		if cfg[k] != nil {
			add(k, cfg[k])
		}
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, AgentYAML), out, 0o644)
}

func copiarArquivo(origem, destino string) error {
	in, err := os.Open(origem)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(destino)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func iguais(a, b string) bool {
	ra, err1 := os.ReadFile(a)
	rb, err2 := os.ReadFile(b)
	return err1 == nil && err2 == nil && string(ra) == string(rb)
}
