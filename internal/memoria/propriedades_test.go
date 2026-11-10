package memoria

import (
	"testing"
	"testing/quick"
	"time"
)

// Propriedades de Upsert/Esquecer/NormalizarChave: invariantes que valem para
// qualquer fato, não só os exemplos fixos de nucleo_test.go.

// upsertMantemNoMaximoUmaOcorrenciaPorChave: depois de Upsert com preferência válida, a
// memória tem exatamente um fato daquela chave, com o valor novo.
func upsertMantemNoMaximoUmaOcorrenciaPorChave(indiceDaChave, indiceDoValor uint8, confianca float64) bool {
	chaves := ChavesDePreferencia()
	chave := chaves[int(indiceDaChave)%len(chaves)]
	valores := valoresValidos(chave)
	texto := valores[int(indiceDoValor)%len(valores)]
	base := Memoria{Fatos: []Fato{
		{Chave: chave, Texto: "valor antigo"},
		{Chave: "outra_chave_fixa", Texto: "intacto"},
	}}
	f := Fato{Chave: chave, Texto: texto, Confianca: confianca, Quando: time.Now()}
	novo, mudou := base.Upsert(f)
	if !mudou {
		return false
	}
	ocorrencias, achouTexto := 0, false
	for _, ft := range novo.Fatos {
		if ft.Chave == chave {
			ocorrencias++
			achouTexto = ft.Texto == texto
		}
	}
	return ocorrencias == 1 && achouTexto
}

func TestPropriedadeUpsertSemDuplicarChave(t *testing.T) {
	if err := quick.Check(upsertMantemNoMaximoUmaOcorrenciaPorChave, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

// upsertEhIdempotente: aplicar o mesmo fato duas vezes dá o mesmo resultado da
// primeira vez (a segunda não muda nada).
func upsertEhIdempotente(indiceDaChave, indiceDoValor uint8) bool {
	chaves := ChavesDePreferencia()
	chave := chaves[int(indiceDaChave)%len(chaves)]
	valores := valoresValidos(chave)
	f := Fato{Chave: chave, Texto: valores[int(indiceDoValor)%len(valores)], Quando: time.Now()}
	uma, mudouUma := Memoria{}.Upsert(f)
	duas, mudouDuas := uma.Upsert(f)
	return mudouUma && !mudouDuas && len(duas.Fatos) == len(uma.Fatos)
}

func TestPropriedadeUpsertIdempotente(t *testing.T) {
	if err := quick.Check(upsertEhIdempotente, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

// esquecerRemoveTodasAsOcorrenciasDaChave: depois de Esquecer, nenhum fato
// daquela chave sobrevive, e os fatos de outras chaves não são tocados.
func esquecerRemoveTodasAsOcorrenciasDaChave(chave string) bool {
	chave = NormalizarChave(chave)
	if chave == "" {
		return true
	}
	base := Memoria{Fatos: []Fato{
		{Chave: chave, Texto: "a"},
		{Chave: "intacta", Texto: "b"},
	}}
	novo, existia := base.Esquecer(chave)
	if !existia {
		return false
	}
	for _, f := range novo.Fatos {
		if f.Chave == chave {
			return false
		}
	}
	return len(novo.Fatos) == 1 && novo.Fatos[0].Chave == "intacta"
}

func TestPropriedadeEsquecerLimpaAChave(t *testing.T) {
	if err := quick.Check(esquecerRemoveTodasAsOcorrenciasDaChave, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}

// normalizarChaveEhIdempotenteEEstavel: normalizar duas vezes dá o mesmo
// resultado da primeira, e o resultado só tem [a-z0-9_] (nunca maiúscula, espaço
// ou acento, que quebrariam a comparação de chave do Upsert).
func normalizarChaveEhIdempotenteEEstavel(s string) bool {
	uma := NormalizarChave(s)
	duas := NormalizarChave(uma)
	if uma != duas {
		return false
	}
	for _, r := range uma {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
		if !ok {
			return false
		}
	}
	return true
}

func TestPropriedadeNormalizarChaveIdempotenteEEstavel(t *testing.T) {
	if err := quick.Check(normalizarChaveEhIdempotenteEEstavel, &quick.Config{MaxCount: 500}); err != nil {
		t.Error(err)
	}
}

// semChavesRepetidasPreservaAUltima: para qualquer lista de fatos, o resultado
// não tem chave repetida e cada chave que sobra é a última ocorrência dela na
// lista original (a extração de memória roda em ordem cronológica).
func semChavesRepetidasPreservaAUltima(chaveA, textoA, textoB string) bool {
	chave := NormalizarChave(chaveA)
	if chave == "" {
		return true
	}
	fatos := []Fato{{Chave: chave, Texto: textoA}, {Chave: chave, Texto: textoB}}
	out := semChavesRepetidas(fatos)
	return len(out) == 1 && out[0].Texto == textoB
}

func TestPropriedadeSemChavesRepetidas(t *testing.T) {
	if err := quick.Check(semChavesRepetidasPreservaAUltima, &quick.Config{MaxCount: 300}); err != nil {
		t.Error(err)
	}
}
