// Package regras guarda as regras declarativas dos ganchos no Postgres e entrega ao turno o
// índice compilado das ativas, sem ir ao banco a cada mensagem.
package regras

import (
	"encoding/json"
	"errors"
	"time"

	"go-concierge/internal/ganchos"
)

// RegraGravada é a regra como está no banco: a definição fica em JSON cru, quem a interpreta
// é o compilador.
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

var (
	ErrVersaoVelha       = errors.New("regra alterada por outra pessoa")
	ErrRegraDesconhecida = errors.New("regra não encontrada")
)

// ErroDeCarga é uma regra ativa que o compilador não conseguiu pôr no índice.
type ErroDeCarga struct {
	RegraID string
	Err     error
}

// Compilador monta o índice inteiro a partir das ativas. Regra que não compila fica de fora e
// volta como ErroDeCarga; as outras entram.
type Compilador func([]RegraGravada) (*ganchos.Indice, []ErroDeCarga)
