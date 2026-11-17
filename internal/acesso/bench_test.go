package acesso

import (
	"testing"
	"time"

	"go-concierge/internal/escopo"
)

// TestPermitidoNaoRegride é um teste de regressão de performance para o CI (não um
// benchmark): roda Permitido várias vezes e falha só se ficar ordens de grandeza mais
// lento que o medido em dev (~0.5µs/op), nunca por ruído normal de máquina de CI.
// ponytail: limite 100x folgado de propósito (detectar regressão grosseira, não medir).
func TestPermitidoNaoRegride(t *testing.T) {
	a := base([]Atribuicao{
		{PapelID: "gestor_administradora", Escopo: admA()},
		{PapelID: "operador_administradora", Escopo: Escopo{Tipo: TipoAdministradora, ID: "B"}},
		{PapelID: "construtor_superlogica", Escopo: cond("c1")},
	}, nil, escopo.Implantacao{})
	alvo := cond("c1")

	const n = 10_000
	const limite = 500 * time.Millisecond // ~50µs/op: 100x o ~0.5µs/op medido em dev

	inicio := time.Now()
	for i := 0; i < n; i++ {
		Permitido(a, SkillsEditarFluxo, alvo)
	}
	if decorrido := time.Since(inicio); decorrido > limite {
		t.Fatalf("Permitido: %d chamadas levaram %s, limite %s (regressão de performance?)", n, decorrido, limite)
	}
}

// BenchmarkPermitido mede a checagem de RBAC (função quente: roda em toda requisição
// autenticada). Cenário realista: usuário com várias atribuições (superlógica, duas
// administradoras, um condomínio) checando um alvo de condomínio.
func BenchmarkPermitido(b *testing.B) {
	a := base([]Atribuicao{
		{PapelID: "gestor_administradora", Escopo: admA()},
		{PapelID: "operador_administradora", Escopo: Escopo{Tipo: TipoAdministradora, ID: "B"}},
		{PapelID: "construtor_superlogica", Escopo: cond("c1")},
	}, nil, escopo.Implantacao{})
	alvo := cond("c1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Permitido(a, SkillsEditarFluxo, alvo)
	}
}

// BenchmarkPermitidoEmAlgum mede a leitura global (catálogo, listas): roda em toda GET de
// skills/agentes.
func BenchmarkPermitidoEmAlgum(b *testing.B) {
	a := base([]Atribuicao{
		{PapelID: "gestor_administradora", Escopo: admA()},
		{PapelID: "operador_administradora", Escopo: Escopo{Tipo: TipoAdministradora, ID: "B"}},
	}, nil, escopo.Implantacao{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		PermitidoEmAlgum(a, SkillsLer)
	}
}
