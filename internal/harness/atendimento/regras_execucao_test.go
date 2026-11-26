package atendimento

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"go-concierge/internal/escopo"
	"go-concierge/internal/harness/ganchos"
	"go-concierge/internal/harness/ganchos/regras"
)

type fonteRegrasExecucaoTeste struct {
	mu        sync.Mutex
	mensagens map[string]string
}

func (f *fonteRegrasExecucaoTeste) AtivasDoEscopo(_ context.Context, empresa string, _ *int) ([]regras.RegraGravada, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	definicao := fmt.Sprintf(`{"nome":"regra teste","evento":"antes_de_responder","alvo":{},"condicoes":[{"tipo":"texto_contem","termos":["alerta"]}],"decisao":{"tipo":"bloquear","mensagem":%q},"ativa":true}`, f.mensagens[empresa])
	return []regras.RegraGravada{{ID: empresa, Versao: 1, Ativa: true, Definicao: []byte(definicao)}}, nil
}

func TestRegrasPorExecucaoMantemRetratosIsolados(t *testing.T) {
	fonte := &fonteRegrasExecucaoTeste{mensagens: map[string]string{"a": "primeira", "b": "outra"}}
	svc := &Servico{Regras: fonte}
	a := &Sessao{escopo: escopo.Contato{CompanyID: "a", Telefone: "5511999999999"}}
	b := &Sessao{escopo: escopo.Contato{CompanyID: "b", Telefone: "5511888888888"}}
	primeira, err := svc.abrirTurnoDaExecucao(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	outra, err := svc.abrirTurnoDaExecucao(t.Context(), b)
	if err != nil {
		t.Fatal(err)
	}
	fonte.mu.Lock()
	fonte.mensagens["a"] = "publicada"
	fonte.mu.Unlock()
	proxima, err := svc.abrirTurnoDaExecucao(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		turno    ganchos.Turno
		esperado string
	}{{primeira, "primeira"}, {outra, "outra"}, {proxima, "publicada"}} {
		r := tc.turno.AntesDeResponder(t.Context(), ganchos.AntesDeResponder{Texto: "alerta"})
		if r.Decisao.Tipo != ganchos.Bloquear || r.Decisao.Motivo != tc.esperado {
			t.Fatalf("decisão=%+v esperado=%q", r.Decisao, tc.esperado)
		}
	}
}
