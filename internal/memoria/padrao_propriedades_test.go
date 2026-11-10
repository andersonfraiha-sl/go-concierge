package memoria

import (
	"slices"
	"testing"
	"testing/quick"
	"time"
)

// padraoSemente é um padrão descrito só por primitivos, que o testing/quick sabe gerar.
type padraoSemente struct {
	Caso, Condominio, Unidade uint8
	Valor                     uint8
	MinutosAtras              int16
	SuprimidoMinutos          int16 // relativo a agora; <= 0 não suprime
}

func (s padraoSemente) padrao() Padrao {
	p := Padrao{
		Caso:         string(rune('a' + s.Caso%4)),
		CondominioID: string(rune('x' + s.Condominio%2)),
		Valor:        float64(s.Valor) / 8,
		AtualizadoEm: agoraDoTeste.Add(-time.Duration(s.MinutosAtras) * time.Minute),
	}
	if s.Unidade%4 != 0 {
		p.UnidadeID = "u" + string(rune('0'+s.Unidade%4))
	}
	if s.SuprimidoMinutos > 0 {
		p.SuprimidoAte = agoraDoTeste.Add(time.Duration(s.SuprimidoMinutos) * time.Minute)
	}
	return p
}

func padroesDe(sementes []padraoSemente) []Padrao {
	out := make([]Padrao, 0, len(sementes))
	for _, s := range sementes {
		out = append(out, s.padrao())
	}
	return out
}

func TestPropriedadeValorEmNaoCresceComOTempo(t *testing.T) {
	naoCresce := func(s padraoSemente, d1, d2 uint16) bool {
		p := s.padrao()
		t1 := agoraDoTeste.Add(time.Duration(min(d1, d2)) * time.Hour)
		t2 := agoraDoTeste.Add(time.Duration(max(d1, d2)) * time.Hour)
		return ValorEm(p, t2, AjustesPadrao()) <= ValorEm(p, t1, AjustesPadrao())
	}
	if err := quick.Check(naoCresce, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

func TestPropriedadeOfertaRespeitaOAlvo(t *testing.T) {
	alvo := Alvo{CondominioID: "x", Unidades: map[string]string{"u1": "Apto 1", "u2": "Apto 2"}}
	respeita := func(sementes []padraoSemente) bool {
		p, ofereceu := EscolherOferta(padroesDe(sementes), alvo, agoraDoTeste, AjustesPadrao())
		if !ofereceu {
			return true
		}
		_, vinculada := alvo.Unidades[p.UnidadeID]
		return p.CondominioID == alvo.CondominioID &&
			(p.UnidadeID == "" || vinculada) &&
			!agoraDoTeste.Before(p.SuprimidoAte)
	}
	if err := quick.Check(respeita, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

func TestPropriedadeRanquearEhPermutacaoOrdenada(t *testing.T) {
	aj := AjustesPadrao()
	ehPermutacaoOrdenada := func(sementes []padraoSemente) bool {
		in := padroesDe(sementes)
		out := Ranquear(in, agoraDoTeste, aj)

		ordenado := slices.IsSortedFunc(out, func(a, b Padrao) int {
			va, vb := ValorEm(a, agoraDoTeste, aj), ValorEm(b, agoraDoTeste, aj)
			switch {
			case va > vb:
				return -1
			case va < vb:
				return 1
			}
			return 0
		})
		return len(out) == len(in) && ordenado && slices.Equal(ordenadas(in), ordenadas(out))
	}
	if err := quick.Check(ehPermutacaoOrdenada, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

// ordenadas compara por conteúdo: as chaves podem repetir nas sementes aleatórias.
func ordenadas(ps []Padrao) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Chave()+"|"+p.AtualizadoEm.String()+"|"+p.SuprimidoAte.String())
	}
	slices.Sort(out)
	return out
}
