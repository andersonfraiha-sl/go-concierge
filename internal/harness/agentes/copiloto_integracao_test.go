package agentes_test

import (
	"testing"

	administracaollm "go-concierge/internal/core/administracao/llm"
	"go-concierge/internal/core/clientes/llm"
	harness "go-concierge/internal/harness/agentes"
)

func TestCopilotoVertexSemAPIKey(t *testing.T) {
	valores := map[string]string{"LLM_MODE": "vertex", "GOOGLE_CLOUD_PROJECT": "projeto-teste", "GOOGLE_APPLICATION_CREDENTIALS": harness.CredencialVertexDeTeste(t, "http://127.0.0.1/token")}
	cfg := harness.Config{Modelos: llm.Fabrica{Env: func(k string) string { return valores[k] }}}
	if _, ok := administracaollm.NovoCopiloto(cfg); !ok {
		t.Error("Vertex deve habilitar o copiloto")
	}
}
