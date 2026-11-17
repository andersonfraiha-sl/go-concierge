package okf

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"go-concierge/internal/catalogo"
)

const yamlQuebrado = "nome: [\n"

func TestValidarCaminhoDeArquivoAceitaSoArquivoLimpoSobAgentes(t *testing.T) {
	casos := []struct {
		nome, caminho string
		valido        bool
	}{
		{"arquivo de agente", "agentes/concierge/persona.md", true},
		{"arquivo fundo", "agentes/a/agentes/b/rules/output.md", true},
		{"vazio", "", false},
		{"só a pasta", "agentes", false},
		{"pasta com barra final", "agentes/a/", false},
		{"ponto-ponto no meio", "agentes/a/../b/x.md", false},
		{"ponto-ponto para o nexus", "agentes/../nexus/x.yaml", false},
		{"ponto-ponto no começo", "../fora.md", false},
		{"absoluto", "/etc/passwd", false},
		{"unidade do windows", "C:/Windows/x", false},
		{"unidade no meio", "agentes/C:/x", false},
		{"barra invertida", `agentes\a\x.md`, false},
		{"barra invertida e ponto-ponto", `agentes/..\..\x.md`, false},
		{"byte nulo", "agentes/a/x\x00.md", false},
		{"ponto no meio", "agentes/./a/x.md", false},
		{"barra dupla", "agentes//a/x.md", false},
		{"nexus", "nexus/cond-001.yaml", false},
		{"fora das pastas do catálogo", "outra/x.md", false},
		{"arquivo oculto", "agentes/a/.env", false},
		{"pasta oculta", "agentes/a/.git/config", false},
		{"ponto-ponto colado ao nome", "agentes/a/..x/y.md", false},
		{"parecido com agentes", "agentes-x/a.md", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			err := ValidarCaminhoDeArquivo(c.caminho)
			if (err == nil) != c.valido {
				t.Fatalf("ValidarCaminhoDeArquivo(%q) = %v, valido=%v", c.caminho, err, c.valido)
			}
			if err != nil && !errors.Is(err, catalogo.ErrCaminhoInvalido) {
				t.Errorf("erro = %v, quer ErrCaminhoInvalido", err)
			}
		})
	}
}

func TestValidarLoteRecusaLoteVazioRepetidoEConflitante(t *testing.T) {
	arq := func(c string) ArquivoBruto { return ArquivoBruto{Caminho: c} }
	casos := []struct {
		nome    string
		gravar  []ArquivoBruto
		remover []string
		valido  bool
	}{
		{"só gravar", []ArquivoBruto{arq("agentes/a/x.md")}, nil, true},
		{"só remover", nil, []string{"agentes/a/x.md"}, true},
		{"gravar e remover diferentes", []ArquivoBruto{arq("agentes/a/x.md")}, []string{"agentes/a/y.md"}, true},
		{"lote vazio", nil, nil, false},
		{"repetido em gravar", []ArquivoBruto{arq("agentes/a/x.md"), arq("agentes/a/x.md")}, nil, false},
		{"repetido em remover", nil, []string{"agentes/a/x.md", "agentes/a/x.md"}, false},
		{"repetido entre os dois", []ArquivoBruto{arq("agentes/a/x.md")}, []string{"agentes/a/x.md"}, false},
		{"arquivo e pasta do mesmo lote", []ArquivoBruto{arq("agentes/a/x"), arq("agentes/a/x/y.md")}, nil, false},
		{"pasta de um que se remove", []ArquivoBruto{arq("agentes/a/x/y.md")}, []string{"agentes/a/x"}, false},
		{"irmãos com prefixo parecido", []ArquivoBruto{arq("agentes/a/x"), arq("agentes/a/x-y")}, nil, true},
		{"um caminho ruim entre bons", []ArquivoBruto{arq("agentes/a/x.md"), arq("nexus/y.yaml")}, nil, false},
		{"caminho ruim em remover", nil, []string{"agentes/../nexus/x.yaml"}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			err := ValidarLote(c.gravar, c.remover)
			if (err == nil) != c.valido {
				t.Fatalf("ValidarLote = %v, valido=%v", err, c.valido)
			}
			if err != nil && !errors.Is(err, ErrLoteInvalido) {
				t.Errorf("erro = %v, quer ErrLoteInvalido", err)
			}
		})
	}
}

// fotoDoDisco lista tudo sob a raiz, pastas inclusas, com o conteúdo dos arquivos: duas fotos
// iguais querem dizer disco idêntico.
func fotoDoDisco(t *testing.T, raiz string) map[string]string {
	t.Helper()
	foto := map[string]string{}
	err := filepath.WalkDir(raiz, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(raiz, p)
		if d.IsDir() {
			foto[filepath.ToSlash(rel)+"/"] = ""
			return nil
		}
		b, err := os.ReadFile(p)
		foto[filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return foto
}

func lerNoDisco(t *testing.T, c *Catalogo, rel string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(c.Root, filepath.FromSlash(rel)))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

func bruto(caminho, conteudo string) ArquivoBruto {
	return ArquivoBruto{Caminho: caminho, Conteudo: []byte(conteudo)}
}

func TestAplicarArquivosGravaAlteraERemoveEPersisteUmaVez(t *testing.T) {
	c := catalogoDeTeste(t)
	persistencias := 0
	c.Persistir = func() error { persistencias++; return nil }
	existente := relConcierge + "/persona.md"
	removido := relMudanca + "/temporario/vazio.yaml"
	if err := os.MkdirAll(filepath.Join(c.Root, filepath.FromSlash(relMudanca), "temporario"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.Root, filepath.FromSlash(removido)), []byte("a: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := c.AplicarArquivos(
		[]ArquivoBruto{
			bruto(relConcierge+"/knowledge/novo.md", "# novo\n"),
			bruto(existente, "# outra persona\n"),
			{Caminho: relConcierge + "/assets/logo.png", Conteudo: []byte{0x89, 'P', 'N', 'G', 0xff, 0x00}},
		},
		[]string{removido, relConcierge + "/knowledge/nunca-existiu.md"},
	)
	if err != nil || len(res.Problemas) > 0 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if res.Gravados != 3 || res.Removidos != 1 {
		t.Errorf("gravados/removidos = %d/%d, quer 3/1", res.Gravados, res.Removidos)
	}
	if persistencias != 1 {
		t.Errorf("Persistir rodou %d vez(es), quer 1", persistencias)
	}
	if got, _ := lerNoDisco(t, c, relConcierge+"/knowledge/novo.md"); got != "# novo\n" {
		t.Errorf("arquivo criado = %q", got)
	}
	if got, _ := lerNoDisco(t, c, existente); got != "# outra persona\n" {
		t.Errorf("arquivo alterado = %q", got)
	}
	if got, _ := lerNoDisco(t, c, relConcierge+"/assets/logo.png"); got != "\x89PNG\xff\x00" {
		t.Errorf("binário não preservou os bytes: %q", got)
	}
	if _, existe := lerNoDisco(t, c, removido); existe {
		t.Error("arquivo removido continua no disco")
	}
	if dirExists(filepath.Join(c.Root, filepath.FromSlash(relMudanca), "temporario")) {
		t.Error("pasta que ficou vazia com a remoção continua no disco")
	}
}

func TestAplicarArquivosReenviadoNaoMudaNada(t *testing.T) {
	c := catalogoDeTeste(t)
	gravar := []ArquivoBruto{bruto(relConcierge+"/knowledge/novo.md", "# novo\n")}
	remover := []string{relConcierge + "/knowledge/velho.md"}
	if _, err := c.AplicarArquivos(gravar, remover); err != nil {
		t.Fatal(err)
	}
	depoisDoPrimeiro := fotoDoDisco(t, c.Root)

	res, err := c.AplicarArquivos(gravar, remover)
	if err != nil || len(res.Problemas) > 0 {
		t.Fatalf("reenvio: res = %+v, err = %v", res, err)
	}
	if res.Gravados != 1 || res.Removidos != 0 {
		t.Errorf("gravados/removidos = %d/%d, quer 1/0", res.Gravados, res.Removidos)
	}
	if !maps.Equal(depoisDoPrimeiro, fotoDoDisco(t, c.Root)) {
		t.Error("o reenvio mudou o disco")
	}
}

func TestAplicarArquivosQueQuebraOCatalogoDevolveOsProblemasENaoMudaNada(t *testing.T) {
	casos := []struct {
		nome    string
		gravar  []ArquivoBruto
		remover []string
	}{
		{
			"yaml quebrado num agente existente",
			[]ArquivoBruto{bruto(relMudanca+"/agent.yaml", yamlQuebrado), bruto(relConcierge+"/knowledge/ok.md", "x")},
			[]string{relConcierge + "/persona.md"},
		},
		{
			"agente novo com yaml quebrado cria pastas que somem no desfazer",
			[]ArquivoBruto{bruto("agentes/agente-novo/agent.yaml", yamlQuebrado), bruto("agentes/agente-novo/knowledge/x.md", "x")},
			nil,
		},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := catalogoDeTeste(t)
			c.Persistir = func() error { t.Error("Persistir rodou num lote que quebra o catálogo"); return nil }
			antes := fotoDoDisco(t, c.Root)

			res, err := c.AplicarArquivos(caso.gravar, caso.remover)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Problemas) == 0 {
				t.Fatal("quer problemas, veio nenhum")
			}
			for _, p := range res.Problemas {
				if p.Nivel != "erro" {
					t.Errorf("problema de nível %q na resposta: %+v", p.Nivel, p)
				}
			}
			if depois := fotoDoDisco(t, c.Root); !maps.Equal(antes, depois) {
				t.Errorf("o disco mudou:\nantes:  %d entradas\ndepois: %d entradas", len(antes), len(depois))
			}
		})
	}
}

func TestAplicarArquivosNaoCulpaOLotePorProblemaQueOCatalogoJaTinha(t *testing.T) {
	c := catalogoDeTeste(t)
	if err := os.WriteFile(filepath.Join(c.Root, filepath.FromSlash(relMudanca), "agent.yaml"), []byte(yamlQuebrado), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := c.AplicarArquivos([]ArquivoBruto{bruto(relConcierge+"/knowledge/ok.md", "x")}, nil)
	if err != nil || len(res.Problemas) > 0 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
}

func TestAplicarArquivosComFalhaDoGanchoRestauraODisco(t *testing.T) {
	errBanco := errors.New("banco fora do ar")
	casos := []struct {
		nome          string
		falhasDoBanco int // chamadas seguidas do gancho que falham
		quer          []error
	}{
		{"banco volta na segunda tentativa", 1, []error{ErrPersistir, errBanco}},
		{"banco continua fora", 2, []error{ErrPersistir, errBanco}},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := catalogoDeTeste(t)
			chamadas := 0
			c.Persistir = func() error {
				chamadas++
				if chamadas <= caso.falhasDoBanco {
					return errBanco
				}
				return nil
			}
			antes := fotoDoDisco(t, c.Root)

			_, err := c.AplicarArquivos(
				[]ArquivoBruto{bruto(relConcierge+"/persona.md", "# outra\n"), bruto("agentes/pasta-nova/agent.yaml", "nome: pasta-nova\n")},
				[]string{relMudanca + "/agent.yaml"},
			)
			for _, quer := range caso.quer {
				if !errors.Is(err, quer) {
					t.Errorf("erro = %v, quer %v na cadeia", err, quer)
				}
			}
			if depois := fotoDoDisco(t, c.Root); !maps.Equal(antes, depois) {
				t.Error("o disco não voltou ao estado anterior")
			}
			if chamadas != 2 {
				t.Errorf("gancho rodou %d vez(es), quer 2 (a escrita e a ressincronização do banco)", chamadas)
			}
		})
	}
}

func TestAplicarArquivosRecusaConflitoComODiscoSemTocarNele(t *testing.T) {
	casos := []struct {
		nome    string
		gravar  []ArquivoBruto
		remover []string
	}{
		{"gravar sobre uma pasta", []ArquivoBruto{bruto(relConcierge+"/knowledge", "x")}, nil},
		{"gravar sob um arquivo", []ArquivoBruto{bruto(relConcierge+"/persona.md/filho.md", "x")}, nil},
		{"remover uma pasta", nil, []string{relConcierge + "/agentes"}},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			c := catalogoDeTeste(t)
			if err := os.MkdirAll(filepath.Join(c.Root, filepath.FromSlash(relConcierge), "knowledge"), 0o755); err != nil {
				t.Fatal(err)
			}
			c.Persistir = func() error { t.Error("Persistir rodou com conflito"); return nil }
			antes := fotoDoDisco(t, c.Root)

			res, err := c.AplicarArquivos(caso.gravar, caso.remover)
			if err != nil || len(res.Problemas) == 0 {
				t.Fatalf("res = %+v, err = %v; quer problemas", res, err)
			}
			if !maps.Equal(antes, fotoDoDisco(t, c.Root)) {
				t.Error("o disco mudou")
			}
		})
	}
}

func TestAplicarArquivosNuncaEscreveForaDeAgentes(t *testing.T) {
	c := catalogoDeTeste(t)
	c.Persistir = func() error { t.Error("Persistir rodou com caminho inválido"); return nil }
	fora := filepath.Join(c.Root, "fora.txt")
	if err := os.WriteFile(fora, []byte("intacto"), 0o644); err != nil {
		t.Fatal(err)
	}
	antes := fotoDoDisco(t, c.Root)

	for _, ruim := range []string{
		"../fora.txt", "fora.txt", "agentes/../fora.txt", "agentes/../nexus/cond-001.yaml",
		"nexus/cond-001.yaml", `agentes\..\fora.txt`, "/fora.txt", "agentes/.oculto/x.md",
	} {
		_, errGravar := c.AplicarArquivos([]ArquivoBruto{bruto(ruim, "invasor")}, nil)
		_, errRemover := c.AplicarArquivos(nil, []string{ruim})
		if !errors.Is(errGravar, ErrLoteInvalido) || !errors.Is(errRemover, ErrLoteInvalido) {
			t.Errorf("%q: gravar=%v remover=%v, quer ErrLoteInvalido nos dois", ruim, errGravar, errRemover)
		}
	}
	if !maps.Equal(antes, fotoDoDisco(t, c.Root)) {
		t.Error("um caminho inválido mudou o disco")
	}
	if lerOuVazio(t, fora) != "intacto" {
		t.Error("arquivo fora de agentes/ foi tocado")
	}
}

func lerOuVazio(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
