// Package regras guarda as regras declarativas dos ganchos no Postgres e entrega ao turno o
// índice compilado das ativas, sem ir ao banco a cada mensagem.
//
// O histórico (VersaoGravada) guarda só definição, autor e data de cada versão; nome, evento
// e alvo de uma versão antiga estão dentro da Definicao. Restaurar uma versão é decodificar a
// Definicao dela (Task 13), nunca juntá-la com as colunas atuais da regra.
package regras

import (
	"encoding/json"
	"errors"
	"time"
)

// VersaoGravada é uma linha de ganchos_regras_versoes: a regra como foi gravada naquela versão.
type VersaoGravada struct {
	RegraID     string
	Versao      int
	Definicao   json.RawMessage
	AlteradaPor string
	AlteradaEm  time.Time
}

var (
	ErrVersaoVelha       = errors.New("regra alterada por outra pessoa")
	ErrRegraDesconhecida = errors.New("regra não encontrada")
)
