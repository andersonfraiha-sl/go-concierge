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

func TestAusentesListaOsFixosQueFaltam(t *testing.T) {
	tests := []struct {
		nome   string
		fixos  ganchos.Fixos
		faltam []string
	}{
		{"todos", Todos(), nil},
		{"só checagem", Checagem(), []string{
			"antes_de_responder/" + nomePIIResposta, "depois_da_ferramenta/" + nomePIIResultado,
			"mensagem_recebida/" + nomePIIEntrada, "transbordo_ou_encerramento/" + ganchos.GanchoTransbordoSemPII,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			if got := Ausentes(ganchos.NovoConjunto(tt.fixos, nil).NomesDosFixos()); !slices.Equal(got, tt.faltam) {
				t.Fatalf("Ausentes = %v, want %v", got, tt.faltam)
			}
		})
	}
}
