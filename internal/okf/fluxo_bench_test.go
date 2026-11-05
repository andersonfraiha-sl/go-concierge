package okf

import (
	"testing"
	"time"
)

// yamlFluxoRealista é o steps: do caso.yaml de agendar-mudanca (okf/agentes/concierge/
// agentes/agendar-mudanca/caso.yaml): resource + choose + collect + confirm + resource,
// o tamanho e a mistura de blocos de um caso real.
const yamlFluxoRealista = `
steps:
  - resource: unidades.listar
    save_as: unidades
  - choose:
      param: unidade
      reuse: always
      title: Unidade
      from: unidades
      label: label
      value: id
      ask: Para qual unidade é a mudança?
      empty: Não encontrei unidades vinculadas ao seu cadastro.
  - collect: [data_mudanca, periodo_mudanca]
  - confirm: "Confirma o agendamento da mudança para a unidade {{.unidade_label}}?"
  - resource: mudancas.agendar
    with:
      unidade_id: "{{.unidade}}"
      data: "{{.data_mudanca}}"
      periodo: "{{.periodo_mudanca}}"
    save_as: agendamento
output: "Sua mudança foi agendada. Protocolo: {{.agendamento.protocolo}}."
`

// BenchmarkFluxoDeYAML mede o parse do fluxo (função quente: roda a cada leitura/validação
// de caso.yaml no Studio e no boot do turno).
func BenchmarkFluxoDeYAML(b *testing.B) {
	raw := []byte(yamlFluxoRealista)
	escrita := func(recurso string) bool { return recurso == "mudancas.agendar" }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := FluxoDeYAML(raw, escrita); err != nil {
			b.Fatal(err)
		}
	}
}

// TestFluxoDeYAMLNaoRegride é um teste de regressão de performance para o CI (não um
// benchmark): parsear este caso repetidas vezes não deveria passar de um limite bem
// folgado. ponytail: limite ~40x o ~120µs/op medido em dev, para não quebrar por ruído.
func TestFluxoDeYAMLNaoRegride(t *testing.T) {
	raw := []byte(yamlFluxoRealista)
	escrita := func(recurso string) bool { return recurso == "mudancas.agendar" }

	const n = 1_000
	const limite = 5 * time.Second

	inicio := time.Now()
	for i := 0; i < n; i++ {
		if _, err := FluxoDeYAML(raw, escrita); err != nil {
			t.Fatal(err)
		}
	}
	if decorrido := time.Since(inicio); decorrido > limite {
		t.Fatalf("FluxoDeYAML: %d chamadas levaram %s, limite %s (regressão de performance?)", n, decorrido, limite)
	}
}
