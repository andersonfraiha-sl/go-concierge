package memoria

import "time"

// Ajustes são os números da memória classificada. Os valores de AjustesPadrao são ponto
// de partida, a calibrar com a taxa de aceite real; o boot pode trocá-los.
type Ajustes struct {
	MeiaVida            time.Duration // em quanto tempo um padrão perde metade do valor
	IntervaloMinimo     time.Duration // duas ocorrências do mesmo padrão dentro dele contam uma
	OfertaMinima        float64       // valor mínimo do primeiro candidato para oferecer
	OfertaRazao         float64       // quantas vezes o primeiro candidato precisa superar o segundo
	Supressao           time.Duration // quanto tempo uma recusa silencia a oferta daquele padrão
	PisoLimpeza         float64       // padrão decaído abaixo disto é apagado
	ValidadePreferencia time.Duration // preferência sem reafirmar por este tempo vence
}

// AjustesPadrao devolve os números decididos na spec.
func AjustesPadrao() Ajustes {
	const dia = 24 * time.Hour
	return Ajustes{
		MeiaVida:            45 * dia,
		IntervaloMinimo:     12 * time.Hour,
		OfertaMinima:        2.0,
		OfertaRazao:         2.0,
		Supressao:           14 * dia,
		PisoLimpeza:         0.1,
		ValidadePreferencia: 180 * dia,
	}
}
