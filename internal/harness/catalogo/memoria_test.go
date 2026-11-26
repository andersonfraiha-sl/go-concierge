package catalogo_test

import (
	"bytes"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	corecatalogo "go-concierge/internal/core/catalogo"
	arquivos "go-concierge/internal/core/catalogo/arquivos"
	"go-concierge/internal/harness/catalogo"
	"go-concierge/internal/harness/conteudo"
)

func TestCatalogoMemoriaEquivaleALeituraDaBase(t *testing.T) {
	_, arquivo, _, _ := runtime.Caller(0)
	raiz := filepath.Join(filepath.Dir(arquivo), "..", "..", "..", "okf")
	lista, err := corecatalogo.LerDisco(raiz)
	if err != nil {
		t.Fatal(err)
	}
	originais := make(map[string][]byte, len(lista))
	for _, a := range lista {
		originais[a.Caminho] = a.Conteudo
	}
	recursos, err := arquivos.LerRecursosFixture(filepath.Join(raiz, "nexus", "cond-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := arquivos.NewCatalogo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	base.Recursos = func() []conteudo.Recurso { return recursos }
	memoria, err := catalogo.Novo(originais, recursos, true)
	if err != nil {
		t.Fatal(err)
	}
	baseDir, _ := base.ResolveAgente("agentes/concierge")
	memDir, _ := memoria.ResolveAgente("agentes/concierge")
	baseRaiz, err := base.Load(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	memRaiz, err := memoria.Load(memDir)
	if err != nil {
		t.Fatal(err)
	}
	var comparar func(*conteudo.Agent, *conteudo.Agent)
	comparar = func(a, b *conteudo.Agent) {
		t.Helper()
		if a.Rel != b.Rel || a.Nome != b.Nome || a.Descricao != b.Descricao || a.Tipo != b.Tipo || !reflect.DeepEqual(a.Caso, b.Caso) || len(a.Subagentes) != len(b.Subagentes) {
			t.Fatalf("agentes diferentes %s: base=%+v memória=%+v", a.Rel, a, b)
		}
		if p, q := base.ProjecaoCom(a, nil), memoria.ProjecaoCom(b, nil); p != q {
			t.Fatalf("projeção divergente em %s", a.Rel)
		}
		if p, q := base.Roteador(a), memoria.Roteador(b); p != q {
			t.Fatalf("roteador divergente em %s", a.Rel)
		}
		if p, q := base.Politicas(a), memoria.Politicas(b); p != q {
			t.Fatalf("políticas divergentes em %s", a.Rel)
		}
		if p, q := base.Indice(a), memoria.Indice(b); !reflect.DeepEqual(p, q) {
			t.Fatalf("índice divergente em %s", a.Rel)
		}
		if p, q := base.Procedimentos(a), memoria.Procedimentos(b); !reflect.DeepEqual(p, q) {
			t.Fatalf("procedimentos divergentes em %s", a.Rel)
		}
		if p, q := base.Conhecimento(a), memoria.Conhecimento(b); !reflect.DeepEqual(p, q) {
			t.Fatalf("conhecimento divergente em %s", a.Rel)
		}
		if p, q := base.RecursosDoAgente(a), memoria.RecursosDoAgente(b); !reflect.DeepEqual(p, q) {
			t.Fatalf("recursos divergentes em %s", a.Rel)
		}
		for _, f := range base.IndiceCompleto(a) {
			x, errX := base.LerDado(a, f.Caminho)
			y, errY := memoria.LerDado(b, f.Caminho)
			if (errX != nil) != (errY != nil) || !bytes.Equal(x, y) {
				t.Fatalf("leitura tardia divergente em %s/%s: %v / %v", a.Rel, f.Caminho, errX, errY)
			}
		}
		for i := range a.Subagentes {
			comparar(a.Subagentes[i], b.Subagentes[i])
		}
	}
	comparar(baseRaiz, memRaiz)
	chave := "agentes/concierge/workflows/principal.md"
	originais[chave] = []byte("# Novo\nconteúdo publicado")
	seguinte, err := catalogo.Novo(originais, recursos, true)
	if err != nil {
		t.Fatal(err)
	}
	velho, _ := memoria.LerArquivo("agentes/concierge", "workflows/principal.md")
	novo, _ := seguinte.LerArquivo("agentes/concierge", "workflows/principal.md")
	if bytes.Equal(velho, novo) || !strings.Contains(string(novo), "publicado") {
		t.Fatal("retratos de execuções não ficaram isolados")
	}
}

func TestCatalogoMemoriaRecusaCaminhosForaDoEscopo(t *testing.T) {
	for _, nome := range []string{"../segredo", "agentes/x/../../segredo", "agentes/x\\y", "outro/arquivo"} {
		if _, err := catalogo.Novo(map[string][]byte{nome: []byte("x")}, nil, false); err == nil {
			t.Fatalf("aceitou caminho %q", nome)
		}
	}
}
