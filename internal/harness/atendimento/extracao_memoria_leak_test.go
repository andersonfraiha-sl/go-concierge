package atendimento

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/goleak"

	okf "go-concierge/internal/core/catalogo/arquivos"
	harness "go-concierge/internal/harness/agentes"
	"go-concierge/internal/harness/memoria"
)

// TestExtracaoDeMemoriaNaoDeixaGoroutinePendente: Processar dispara a extração de
// memória (go func em servico.go) fora do caminho crítico. Aguardar precisa esperar
// essa goroutine terminar de verdade — se ela escapar do WaitGroup extracoes, o teste
// a pega como vazamento.
func TestExtracaoDeMemoriaNaoDeixaGoroutinePendente(t *testing.T) {
	// o pacote também tem testes com Postgres via testcontainers (bancotest); o Reaper
	// deles fica de prontidão pelo resto do binário de teste, de propósito, para garantir
	// o cleanup do contêiner — não é um vazamento desta goroutine aqui.
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("github.com/testcontainers/testcontainers-go.(*Reaper).connect.func1"),
		goleak.IgnoreAnyFunction("github.com/Microsoft/go-winio.ioCompletionProcessor"),
	)

	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	fix, _ := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	cat.Recursos = func() []okf.Recurso { return fix }
	adicionarToolsMCP(cat)
	svc := &Servico{
		Cat: cat, Agente: "agentes/concierge", Mapeamento: mapeamentoDeTeste(t), Sessoes: NewSessoes(NovaSessaoEmMemoria()), Sender: &Outbox{},
		Config:      harness.Config{Mock: true, Env: func(string) string { return "" }, MCP: &mcpServicoFalso{}},
		TelefoneB2C: "5519981621916", Memorias: memoria.NovoFake(),
	}
	ev, err := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "na verdade só tenho um apartamento, tenho um cachorro", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Processar(context.Background(), ev.Normalizar()); err != nil {
		t.Fatal(err)
	}
	svc.Aguardar() // sem isso, goleak pegaria a extração ainda em voo
}
