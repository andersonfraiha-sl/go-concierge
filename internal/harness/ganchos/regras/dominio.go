package regras

import (
	"encoding/json"
	"time"

	"go-concierge/internal/harness/ganchos"
)

type RegraGravada struct {
	ID               string
	Versao           int
	Nome             string
	Evento           ganchos.NomeDoEvento
	AdministradoraID string
	Condominios      []string
	Ativa            bool
	Definicao        json.RawMessage
	AlteradaPor      string
	AlteradaEm       time.Time
}

type ErroDeCarga struct {
	RegraID string
	Err     error
}

type Compilador func([]RegraGravada) (*ganchos.Indice, []ErroDeCarga)
