package fixos

import (
	"slices"
	"testing"

	"go-concierge/internal/ganchos"
)

func TestTodosJuntaChecagemESaneamento(t *testing.T) {
	nomes := ganchos.NovoConjunto(Todos(), nil).NomesDosFixos()
	quer := map[ganchos.NomeDoEvento][]string{
		ganchos.EventoMensagemRecebida:         {EscopoDaConversa, nomePIIEntrada},
		ganchos.EventoAntesDaFerramenta:        {FerramentaPermitida, EscritaConfirmada, ContextoB2C},
		ganchos.EventoDepoisDaFerramenta:       {nomePIIResultado},
		ganchos.EventoAntesDeResponder:         {nomePIIResposta},
		ganchos.EventoTransbordoOuEncerramento: {ganchos.GanchoTransbordoSemPII},
	}
	for evento, esperados := range quer {
		if !slices.Equal(nomes[evento], esperados) {
			t.Errorf("%s: fixos %v, quer %v", evento, nomes[evento], esperados)
		}
	}
}
