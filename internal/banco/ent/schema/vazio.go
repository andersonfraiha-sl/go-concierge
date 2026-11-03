package schema

import "entgo.io/ent"

// Vazio existe só porque o ent não gera cliente para um esquema sem entidades. É uma view
// externa: o ent não a cria nem a migra, então nenhuma tabela nasce dela.
//
// Apague este arquivo junto com a chegada da primeira entidade real: o ent v0.14.6 gera
// client.go inválido (hooks struct { X, }) com exatamente uma entidade e esta view.
type Vazio struct {
	ent.View
}
