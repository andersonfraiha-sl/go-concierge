package regras

import (
	"time"

	"go-concierge/internal/harness/ganchos"
	regrasruntime "go-concierge/internal/harness/ganchos/regras"
)

type Regra = regrasruntime.Regra
type Alvo = regrasruntime.Alvo
type TipoCondicao = regrasruntime.TipoCondicao
type Condicao = regrasruntime.Condicao
type CondFerramenta = regrasruntime.CondFerramenta
type CondCaso = regrasruntime.CondCaso
type CondHorario = regrasruntime.CondHorario
type CondDias = regrasruntime.CondDias
type CondNaoUtil = regrasruntime.CondNaoUtil
type CondCadastrado = regrasruntime.CondCadastrado
type CondPapeis = regrasruntime.CondPapeis
type CondTermos = regrasruntime.CondTermos
type Fonte = regrasruntime.Fonte
type DecisaoDaRegra = regrasruntime.DecisaoDaRegra
type Ajuste = regrasruntime.Ajuste
type ErroDeEsquema = regrasruntime.ErroDeEsquema
type Problema = regrasruntime.Problema
type Catalogo = regrasruntime.Catalogo
type RegraGravada = regrasruntime.RegraGravada
type ErroDeCarga = regrasruntime.ErroDeCarga
type Compilador = regrasruntime.Compilador

const (
	CondicaoFerramenta        = regrasruntime.CondicaoFerramenta
	CondicaoCaso              = regrasruntime.CondicaoCaso
	CondicaoForaDoHorario     = regrasruntime.CondicaoForaDoHorario
	CondicaoDiaDaSemana       = regrasruntime.CondicaoDiaDaSemana
	CondicaoDiaNaoUtil        = regrasruntime.CondicaoDiaNaoUtil
	CondicaoContatoCadastrado = regrasruntime.CondicaoContatoCadastrado
	CondicaoPapelDoMorador    = regrasruntime.CondicaoPapelDoMorador
	CondicaoTextoContem       = regrasruntime.CondicaoTextoContem
	FonteAgora                = regrasruntime.FonteAgora
	FonteParametro            = regrasruntime.FonteParametro
	FusoPadrao                = regrasruntime.FusoPadrao
)

func Decodificar(raw []byte) (Regra, error)       { return regrasruntime.Decodificar(raw) }
func ValidarRegra(r Regra, c Catalogo) []Problema { return regrasruntime.ValidarRegra(r, c) }
func ValidarLimites(rs []Regra, r Regra, c Catalogo) []Problema {
	return regrasruntime.ValidarLimites(rs, r, c)
}
func FeriadoNacional(d time.Time) bool                            { return regrasruntime.FeriadoNacional(d) }
func Compilar(rs []RegraGravada) (*ganchos.Indice, []ErroDeCarga) { return regrasruntime.Compilar(rs) }
