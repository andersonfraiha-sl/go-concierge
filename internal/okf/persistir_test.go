package okf

import (
	"errors"
	"slices"
	"testing"
)

// escritaDoCatalogo é uma operação pública que escreve no disco, pronta para rodar no
// catálogo de teste.
type escritaDoCatalogo struct {
	nome string
	roda func(t *testing.T, c *Catalogo) error
}

func escritasDoCatalogo() []escritaDoCatalogo {
	return []escritaDoCatalogo{
		{"CriarAgente", func(_ *testing.T, c *Catalogo) error {
			_, _, err := c.CriarAgente(NovoAgente{Nome: "novo-agente", Descricao: "teste"})
			return err
		}},
		{"SalvarArquivo", func(_ *testing.T, c *Catalogo) error {
			return c.SalvarArquivo(relConcierge, PersonaMD, []byte("# Persona\n"))
		}},
		{"ExcluirArquivo", func(_ *testing.T, c *Catalogo) error {
			return c.ExcluirArquivo(relConcierge, PersonaMD)
		}},
		{"SalvarConfig", func(_ *testing.T, c *Catalogo) error {
			return c.SalvarConfig(relMudanca, map[string]any{"descricao": "outra descrição"})
		}},
		{"VincularRecurso", func(t *testing.T, c *Catalogo) error {
			_, err := c.VincularRecurso(relMudanca, umRecursoDeLeituraNaoVinculado(t, c), true)
			return err
		}},
		{"ImportarZip", func(t *testing.T, c *Catalogo) error {
			dados, err := c.ExportarZip(relMudanca)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.ImportarZip("agentes/copia-zip", dados)
			return err
		}},
		{"SalvarFluxo", func(t *testing.T, c *Catalogo) error {
			lido, err := c.LerFluxo(relMudanca)
			if err != nil {
				t.Fatal(err)
			}
			f := lido.Fluxo
			f.Blocos = slices.Clone(f.Blocos)
			f.Blocos[indiceDoTipo(t, f, BlocoConfirmar)].Texto = "Posso agendar? (sim/não)"
			_, err = c.SalvarFluxo(relMudanca, lido.Versao, f)
			return err
		}},
		{"SalvarTextos", func(t *testing.T, c *Catalogo) error {
			textos, versao, err := c.LerTextos(relMudanca)
			if err != nil {
				t.Fatal(err)
			}
			textos.QuandoUsar = "Quando o morador quer marcar a mudança."
			_, err = c.SalvarTextos(relMudanca, versao, textos)
			return err
		}},
		{"CriarSkill", func(_ *testing.T, c *Catalogo) error {
			_, err := c.CriarSkill(relConcierge, "skill-nova", "Skill nova")
			return err
		}},
		{"DuplicarSkill", func(_ *testing.T, c *Catalogo) error {
			_, err := c.DuplicarSkill(relMudanca, "mudanca-copia", "Mudança cópia")
			return err
		}},
		{"ExcluirSkill", func(_ *testing.T, c *Catalogo) error {
			return c.ExcluirSkill(relMudanca)
		}},
		{"Escrever", func(_ *testing.T, c *Catalogo) error {
			return c.Escrever(func() error { return nil })
		}},
	}
}

func umRecursoDeLeituraNaoVinculado(t *testing.T, c *Catalogo) string {
	t.Helper()
	atuais := c.RecursosDoAgente(mustLoad(t, c, relMudanca))
	for _, r := range c.RecursosNexus() {
		jaDeclarado := slices.ContainsFunc(atuais, func(d RecursoDeclarado) bool { return d.Nome == r.Name })
		if !r.Write && !jaDeclarado {
			return r.Name
		}
	}
	t.Fatal("o catálogo de teste não tem recurso de leitura livre para vincular")
	return ""
}

func mustLoad(t *testing.T, c *Catalogo, rel string) *Agent {
	t.Helper()
	dir, err := c.ResolveAgente(rel)
	if err != nil {
		t.Fatal(err)
	}
	a, err := c.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestEscritaPersisteUmaVezAposDarCerto(t *testing.T) {
	for _, e := range escritasDoCatalogo() {
		t.Run(e.nome, func(t *testing.T) {
			c := catalogoDeTeste(t)
			if e.nome == "ExcluirArquivo" {
				if err := c.SalvarArquivo(relConcierge, PersonaMD, []byte("x")); err != nil {
					t.Fatal(err)
				}
			}
			chamadas := 0
			c.Persistir = func() error { chamadas++; return nil }

			if err := e.roda(t, c); err != nil {
				t.Fatal(err)
			}

			if chamadas != 1 {
				t.Fatalf("Persistir rodou %d vez(es), quer 1", chamadas)
			}
		})
	}
}

func TestEscritaDevolveOErroDoGancho(t *testing.T) {
	errBanco := errors.New("banco fora do ar")
	for _, e := range escritasDoCatalogo() {
		t.Run(e.nome, func(t *testing.T) {
			c := catalogoDeTeste(t)
			if e.nome == "ExcluirArquivo" {
				if err := c.SalvarArquivo(relConcierge, PersonaMD, []byte("x")); err != nil {
					t.Fatal(err)
				}
			}
			c.Persistir = func() error { return errBanco }

			if err := e.roda(t, c); !errors.Is(err, errBanco) {
				t.Fatalf("erro = %v, quer o erro do gancho", err)
			}
		})
	}
}

func TestEscritaQueFalhaNaoPersiste(t *testing.T) {
	c := catalogoDeTeste(t)
	c.Persistir = func() error { t.Error("Persistir rodou numa escrita que falhou"); return nil }

	if err := c.SalvarArquivo(relConcierge, "../fora.md", []byte("x")); err == nil {
		t.Fatal("caminho fora do agente deveria ser recusado")
	}
	if _, err := c.CriarSkill(relConcierge, "Nome Inválido", "x"); err == nil {
		t.Fatal("nome inválido deveria ser recusado")
	}
	if err := c.ExcluirSkill("agentes/nao-existe"); err == nil {
		t.Fatal("skill inexistente deveria falhar")
	}
	if err := c.Escrever(func() error { return errors.New("falhou") }); err == nil {
		t.Fatal("Escrever deveria devolver o erro de fn")
	}
}

func TestEscritaSemGanchoSoGravaNoDisco(t *testing.T) {
	c := catalogoDeTeste(t)
	if _, err := c.CriarSkill(relConcierge, "skill-nova", "Skill nova"); err != nil {
		t.Fatal(err)
	}
}
