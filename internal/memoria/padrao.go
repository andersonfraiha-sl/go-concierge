package memoria

import (
	"cmp"
	"math"
	"slices"
	"time"
)

// Padrao é um contador de uso com decaimento: quantas vezes o contato concluiu um caso de
// uso, num condomínio e numa unidade, com peso maior para o recente. Guarda só o valor e
// quando foi gravado: atualizar e ler são O(1), sem log de eventos.
type Padrao struct {
	Caso         string `json:"caso"` // id do caso.yaml
	CasoNome     string `json:"caso_nome"`
	CondominioID string `json:"condominio_id"`
	UnidadeID    string `json:"unidade_id,omitempty"`
	// Valor é o contador como estava em AtualizadoEm; ValorEm o decai até um instante.
	Valor        float64   `json:"valor"`
	AtualizadoEm time.Time `json:"atualizado_em"`
	// SuprimidoAte silencia a oferta deste padrão depois de uma recusa.
	SuprimidoAte time.Time `json:"suprimido_ate,omitzero"`
}

// Chave identifica o padrão dentro do contato: caso, condomínio e unidade.
func (p Padrao) Chave() string {
	return p.Caso + "|" + p.CondominioID + "|" + p.UnidadeID
}

// Ocorrencia é um pedido concluído que conta para um padrão.
type Ocorrencia struct{ Caso, CasoNome, CondominioID, UnidadeID string }

func (o Ocorrencia) chave() string {
	return Padrao{Caso: o.Caso, CondominioID: o.CondominioID, UnidadeID: o.UnidadeID}.Chave()
}

// Alvo é o vínculo do turno: o condomínio da conversa e as unidades da identidade (id → rótulo).
// A memória só guarda referência; quem confirma o vínculo é a identidade do turno.
type Alvo struct {
	CondominioID string
	Unidades     map[string]string
}

// ValorEm é o valor do padrão decaído até agora: Valor · 0,5^(Δ/meia-vida). Gravado no
// futuro (relógio adiantado de outra réplica), não cresce: o decaimento é de no mínimo zero.
func ValorEm(p Padrao, agora time.Time, aj Ajustes) float64 {
	decorrido := max(0, agora.Sub(p.AtualizadoEm))
	return p.Valor * math.Pow(0.5, float64(decorrido)/float64(aj.MeiaVida))
}

// Registrar conta a ocorrência: decai o padrão existente até agora e soma 1, ou cria o
// padrão com valor 1. O segundo resultado é false quando a ocorrência não conta (a
// anterior foi há menos de IntervaloMinimo, inclusive gravada no futuro), e o padrão
// devolvido é o que já existia.
func Registrar(atuais []Padrao, o Ocorrencia, agora time.Time, aj Ajustes) (Padrao, bool) {
	i := slices.IndexFunc(atuais, func(p Padrao) bool { return p.Chave() == o.chave() })
	if i < 0 {
		return Padrao{
			Caso: o.Caso, CasoNome: o.CasoNome, CondominioID: o.CondominioID, UnidadeID: o.UnidadeID,
			Valor: 1, AtualizadoEm: agora,
		}, true
	}
	p := atuais[i]
	if agora.Sub(p.AtualizadoEm) < aj.IntervaloMinimo {
		return p, false
	}
	p.Valor, p.AtualizadoEm = ValorEm(p, agora, aj)+1, agora
	if o.CasoNome != "" {
		p.CasoNome = o.CasoNome
	}
	return p, true
}

// Recusar divide o valor por 2 (já decaído até agora) e suprime a oferta do padrão por
// Supressao. Dividir, e não zerar, mantém a linha com a supressão até a limpeza.
func Recusar(p Padrao, agora time.Time, aj Ajustes) Padrao {
	p.Valor, p.AtualizadoEm = ValorEm(p, agora, aj)/2, agora
	p.SuprimidoAte = agora.Add(aj.Supressao)
	return p
}

// Ranquear ordena do maior para o menor valor decaído até agora; no empate, o mais
// recentemente atualizado, depois a Chave. Não altera ps.
func Ranquear(ps []Padrao, agora time.Time, aj Ajustes) []Padrao {
	out := slices.Clone(ps)
	slices.SortStableFunc(out, func(a, b Padrao) int {
		return cmp.Or(
			cmp.Compare(ValorEm(b, agora, aj), ValorEm(a, agora, aj)),
			b.AtualizadoEm.Compare(a.AtualizadoEm),
			cmp.Compare(a.Chave(), b.Chave()),
		)
	})
	return out
}

// DoAlvo fica com os padrões do condomínio da conversa cuja unidade, se houver, está no
// vínculo do turno. Padrão de unidade fora do vínculo é ignorado, nunca revelado.
func DoAlvo(ps []Padrao, alvo Alvo) []Padrao {
	out := make([]Padrao, 0, len(ps))
	for _, p := range ps {
		if p.CondominioID != alvo.CondominioID {
			continue
		}
		if _, vinculada := alvo.Unidades[p.UnidadeID]; p.UnidadeID != "" && !vinculada {
			continue
		}
		out = append(out, p)
	}
	return out
}

// EscolherOferta devolve o padrão a oferecer, se houver: o primeiro do alvo, fora da
// supressão, com valor de pelo menos OfertaMinima e OfertaRazao vezes o segundo.
func EscolherOferta(ps []Padrao, alvo Alvo, agora time.Time, aj Ajustes) (Padrao, bool) {
	livres := slices.DeleteFunc(DoAlvo(ps, alvo), func(p Padrao) bool { return agora.Before(p.SuprimidoAte) })
	ranque := Ranquear(livres, agora, aj)
	if len(ranque) == 0 {
		return Padrao{}, false
	}
	primeiro := ValorEm(ranque[0], agora, aj)
	if primeiro < aj.OfertaMinima {
		return Padrao{}, false
	}
	if len(ranque) > 1 && primeiro < aj.OfertaRazao*ValorEm(ranque[1], agora, aj) {
		return Padrao{}, false
	}
	return ranque[0], true
}
