package catalogo

import (
	"errors"
	"slices"
	"testing"
)

func TestValidarCaminhoAceitaSoArquivoLimpoSobAsPastasDoCatalogo(t *testing.T) {
	casos := []struct {
		nome     string
		caminho  string
		invalido bool
	}{
		{"agente", "agentes/concierge/agent.yaml", false},
		{"instantaneo do nexus", "nexus/cond-001.yaml", false},
		{"vazio", "", true},
		{"so a pasta", "agentes", true},
		{"pasta com barra final", "agentes/", true},
		{"fora das pastas", "var/catalogo/x.md", true},
		{"prefixo parecido", "agentes-velhos/x.md", true},
		{"sobe com ponto-ponto", "agentes/../../etc/passwd", true},
		{"ponto-ponto no fim", "agentes/x/..", true},
		{"comeca com ponto-ponto", "../agentes/x.md", true},
		{"absoluto", "/agentes/x.md", true},
		{"unidade do Windows", "C:/agentes/x.md", true},
		{"unidade colada", "agentes/C:x.md", true},
		{"barra invertida", `agentes\x\..\..\y.md`, true},
		{"barra dupla", "agentes//x.md", true},
		{"ponto no meio", "agentes/./x.md", true},
		{"byte nulo", "agentes/x\x00.md", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			err := ValidarCaminho(c.caminho)
			if c.invalido != (err != nil) {
				t.Fatalf("ValidarCaminho(%q) = %v, invalido = %v", c.caminho, err, c.invalido)
			}
			if c.invalido && !errors.Is(err, ErrCaminhoInvalido) {
				t.Fatalf("ValidarCaminho(%q) = %v, quer ErrCaminhoInvalido", c.caminho, err)
			}
		})
	}
}

func TestDiferencaDizOQueGravarEOQueRemover(t *testing.T) {
	igual := Arquivo{"agentes/igual.md", []byte("igual")}
	casos := []struct {
		nome        string
		noBanco     map[string]string
		noDisco     []Arquivo
		querGravar  []string
		querRemover []string
	}{
		{"banco vazio grava tudo", nil, []Arquivo{{"agentes/b.md", []byte("b")}, {"agentes/a.md", []byte("a")}}, []string{"agentes/a.md", "agentes/b.md"}, nil},
		{"disco vazio remove tudo", map[string]string{"agentes/b.md": "x", "agentes/a.md": "y"}, nil, nil, []string{"agentes/a.md", "agentes/b.md"}},
		{"igual nao muda nada", map[string]string{igual.Caminho: SHA256(igual.Conteudo)}, []Arquivo{igual}, nil, nil},
		{"conteudo diferente regrava", map[string]string{"agentes/a.md": SHA256([]byte("velho"))}, []Arquivo{{"agentes/a.md", []byte("novo")}}, []string{"agentes/a.md"}, nil},
		{
			"mistura", map[string]string{igual.Caminho: SHA256(igual.Conteudo), "agentes/sumiu.md": "x"},
			[]Arquivo{igual, {"agentes/novo.md", []byte("n")}}, []string{"agentes/novo.md"}, []string{"agentes/sumiu.md"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			gravar, remover := Diferenca(c.noBanco, c.noDisco)
			if got := caminhos(gravar); !slices.Equal(got, c.querGravar) {
				t.Errorf("gravar = %v, quer %v", got, c.querGravar)
			}
			if !slices.Equal(remover, c.querRemover) {
				t.Errorf("remover = %v, quer %v", remover, c.querRemover)
			}
		})
	}
}

func caminhos(arquivos []Arquivo) []string {
	var out []string
	for _, a := range arquivos {
		out = append(out, a.Caminho)
	}
	return out
}
