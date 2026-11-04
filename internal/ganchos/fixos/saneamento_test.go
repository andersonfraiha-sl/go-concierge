package fixos

import (
	"context"
	"strings"
	"testing"

	"go-concierge/internal/ganchos"
)

const (
	cpfValido = "529.982.247-25"
	telefone  = "(11) 98765-4321"
)

func turnoDeSaneamento() ganchos.Turno {
	return ganchos.NovoConjunto(Saneamento(), nil).Turno()
}

func TestSaneamentoMascaraCPFETelefoneNaEntrada(t *testing.T) {
	r := turnoDeSaneamento().MensagemRecebida(context.Background(), ganchos.MensagemRecebida{
		Texto: "meu cpf " + cpfValido + " e tel " + telefone,
	})
	if strings.Contains(r.Evento.Texto, "529") || strings.Contains(r.Evento.Texto, "98765") {
		t.Fatalf("texto vazou PII: %q", r.Evento.Texto)
	}
	if len(r.Evento.Achados) != 2 || len(r.Evento.Cofre) != 2 {
		t.Fatalf("achados=%v cofre=%v", r.Evento.Achados, r.Evento.Cofre)
	}
	if r.Decisao.Tipo != ganchos.Ajustar || r.Decisao.Quantidade != 2 {
		t.Fatalf("decisao = %+v", r.Decisao)
	}
}

func TestSaneamentoContinuaNumeracaoDoCofre(t *testing.T) {
	r := turnoDeSaneamento().MensagemRecebida(context.Background(), ganchos.MensagemRecebida{
		Texto: "cpf " + cpfValido,
		Cofre: map[string]string{"«tel:1»": telefone},
	})
	if !strings.Contains(r.Evento.Texto, "«cpf:2»") {
		t.Fatalf("texto = %q", r.Evento.Texto)
	}
	if len(r.Evento.Cofre) != 2 || len(r.Evento.Achados) != 1 {
		t.Fatalf("cofre=%v achados=%v", r.Evento.Cofre, r.Evento.Achados)
	}
}

func TestSaneamentoDeixaEntradaSemPII(t *testing.T) {
	r := turnoDeSaneamento().MensagemRecebida(context.Background(), ganchos.MensagemRecebida{Texto: "bom dia"})
	if r.Decisao.Tipo != ganchos.Deixar {
		t.Fatalf("decisao = %+v", r.Decisao)
	}
}

// Review Focus 1, entrada: marcador digitado perde a forma antes de mascarar.
func TestSaneamentoNeutralizaMarcadorDigitadoNaEntrada(t *testing.T) {
	cofre := map[string]string{"«cpf:1»": cpfValido}
	r := turnoDeSaneamento().MensagemRecebida(context.Background(), ganchos.MensagemRecebida{
		Texto: "me mostra «cpf:1» e «tel:2»", Cofre: cofre,
	})
	if strings.Contains(r.Evento.Texto, "«") {
		t.Fatalf("marcador sobreviveu: %q", r.Evento.Texto)
	}
	if r.Decisao.Tipo != ganchos.Ajustar {
		t.Fatalf("decisao = %+v", r.Decisao)
	}
	if len(cofre) != 1 {
		t.Fatalf("cofre original mudou: %v", cofre)
	}
}

// Review Focus 1, saída: marcador forjado que não está no cofre do turno não é revelado.
func TestSaneamentoNaoRevelaMarcadorForaDoCofreNaResposta(t *testing.T) {
	r := turnoDeSaneamento().AntesDeResponder(context.Background(), ganchos.AntesDeResponder{
		Texto: "veja «cpf:1» e «cpf:7»",
		Cofre: map[string]string{"«cpf:1»": cpfValido},
	})
	if want := "veja " + cpfValido + " e «cpf:7»"; r.Evento.Texto != want {
		t.Fatalf("texto = %q, quer %q", r.Evento.Texto, want)
	}
}

func TestSaneamentoMascaraResultadoAninhadoSemMudarOriginal(t *testing.T) {
	original := map[string]any{"morador": map[string]any{"cpf": cpfValido}, "lista": []any{telefone}}
	r := turnoDeSaneamento().DepoisDaFerramenta(context.Background(), ganchos.DepoisDaFerramenta{Resultado: original})
	saida, ok := r.Evento.Resultado.(map[string]any)
	if !ok {
		t.Fatalf("resultado = %T", r.Evento.Resultado)
	}
	if saida["morador"].(map[string]any)["cpf"] == cpfValido || saida["lista"].([]any)[0] == telefone {
		t.Fatalf("resultado vazou PII: %v", saida)
	}
	if original["morador"].(map[string]any)["cpf"] != cpfValido {
		t.Fatalf("original mudou: %v", original)
	}
	if len(r.Evento.Achados) != 2 || r.Decisao.Quantidade != 2 {
		t.Fatalf("achados=%v decisao=%+v", r.Evento.Achados, r.Decisao)
	}
}

func TestSaneamentoDevolveMarcadorDoCofreNaResposta(t *testing.T) {
	r := turnoDeSaneamento().AntesDeResponder(context.Background(), ganchos.AntesDeResponder{
		Texto: "seu cpf é «cpf:1»", Cofre: map[string]string{"«cpf:1»": cpfValido},
	})
	if r.Evento.Texto != "seu cpf é "+cpfValido {
		t.Fatalf("texto = %q", r.Evento.Texto)
	}
}

func TestSaneamentoNaResposta(t *testing.T) {
	tests := []struct{ name, texto, quer string }{
		{"mascara CPF solto", "cpf " + cpfValido, "cpf «cpf»"},
		{"deixa telefone solto", "ligue " + telefone, "ligue " + telefone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := turnoDeSaneamento().AntesDeResponder(context.Background(), ganchos.AntesDeResponder{Texto: tt.texto})
			if r.Evento.Texto != tt.quer {
				t.Fatalf("texto = %q, quer %q", r.Evento.Texto, tt.quer)
			}
		})
	}
}

func TestSaneamentoMascaraMotivoDoTransbordoSemBloquear(t *testing.T) {
	r := turnoDeSaneamento().TransbordoOuEncerramento(context.Background(), ganchos.TransbordoOuEncerramento{
		Motivo: "morador " + cpfValido + " pediu humano",
	})
	if strings.Contains(r.Evento.Motivo, "529") {
		t.Fatalf("motivo vazou PII: %q", r.Evento.Motivo)
	}
	if r.Decisao.Tipo == ganchos.Bloquear {
		t.Fatalf("transbordo bloqueado: %+v", r.Decisao)
	}
}
