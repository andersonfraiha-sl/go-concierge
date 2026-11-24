// Package arquitetura guarda fitness functions: regras de dependência entre pacotes que
// nenhum linter comum confere, só lendo "go list -deps" de verdade.
package arquitetura

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

type pacote struct {
	ImportPath string
	Deps       []string
	Imports    []string
}

func listarPacotes(t *testing.T, padrao string) []pacote {
	t.Helper()
	saida, err := exec.Command("go", "list", "-deps", "-json", padrao).Output()
	if err != nil {
		t.Fatalf("go list -deps -json %s: %v", padrao, err)
	}
	var pacotes []pacote
	dec := json.NewDecoder(strings.NewReader(string(saida)))
	for dec.More() {
		var p pacote
		if err := dec.Decode(&p); err != nil {
			t.Fatalf("decodificar saída do go list: %v", err)
		}
		pacotes = append(pacotes, p)
	}
	return pacotes
}

// TestInternalNaoImportaCmd: internal/ é a regra de negócio; cmd/ é o fiapo que liga nela
// num binário. Se algo em internal/ importar cmd/, a dependência virou bidirecional e cmd/
// deixou de ser descartável.
func TestInternalNaoImportaCmd(t *testing.T) {
	for _, p := range listarPacotes(t, "go-concierge/internal/...") {
		for _, d := range p.Deps {
			if strings.HasPrefix(d, "go-concierge/cmd/") || strings.HasPrefix(d, "go-concierge/apps/") {
				t.Errorf("%s importa %s (internal/ não pode depender de cmd/)", p.ImportPath, d)
			}
		}
	}
}

func TestAplicacoesVisuaisNaoImportamPersistenciaOuRuntime(t *testing.T) {
	for _, app := range []string{"backdoor", "console"} {
		for _, p := range listarPacotes(t, "go-concierge/apps/"+app+"/cmd/...") {
			for _, d := range p.Deps {
				for _, proibido := range []string{"go-concierge/internal/core/banco", "go-concierge/internal/core/dados", "go-concierge/internal/harness/", "github.com/jackc/pgx"} {
					if strings.HasPrefix(d, proibido) {
						t.Errorf("%s depende de %s", p.ImportPath, d)
					}
				}
			}
		}
	}
}

func TestHarnessNaoImportaCoreOuPersistencia(t *testing.T) {
	for _, p := range listarPacotes(t, "go-concierge/internal/harness/...") {
		if !strings.HasPrefix(p.ImportPath, "go-concierge/internal/harness/") {
			continue
		}
		for _, d := range p.Deps {
			if strings.HasPrefix(d, "go-concierge/internal/core/") || strings.HasPrefix(d, "github.com/jackc/pgx") || strings.HasPrefix(d, "entgo.io/ent") || d == "database/sql" {
				t.Errorf("%s depende de infraestrutura %s", p.ImportPath, d)
			}
		}
	}
}

// TestSemCicloEntrePacotesInternos: o compilador já recusa ciclo de import, então isso nunca
// deveria disparar — é a fitness function documentando a regra e travando se algum dia um
// ciclo aparecer disfarçado (ex.: via um pacote intermediário que o go list não reporte à
// primeira vista). go list -deps falha com código de erro quando há ciclo, e listarPacotes já
// chama t.Fatalf nesse caso; aqui só confirmamos que listar o módulo inteiro não falha.
func TestSemCicloEntrePacotesInternos(t *testing.T) {
	if pacotes := listarPacotes(t, "go-concierge/..."); len(pacotes) == 0 {
		t.Fatal("go list não devolveu nenhum pacote")
	}
}
