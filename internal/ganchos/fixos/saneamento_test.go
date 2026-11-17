package fixos

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	turno := turnoDeSaneamento()
	r := turno.MensagemRecebida(context.Background(), ganchos.MensagemRecebida{
		Texto: "meu cpf " + cpfValido + " e tel " + telefone,
	})
	if strings.Contains(r.Evento.Texto, "529") || strings.Contains(r.Evento.Texto, "98765") {
		t.Fatalf("texto vazou PII: %q", r.Evento.Texto)
	}
	if r.Decisao.Tipo != ganchos.Ajustar || r.Decisao.Quantidade != 2 {
		t.Fatalf("decisao = %+v", r.Decisao)
	}
	if got := len(turno.Cofre().Ler()); got != 2 {
		t.Fatalf("cofre do turno guarda %d, quer 2", got)
	}
}

func TestSaneamentoContinuaNumeracaoDoCofre(t *testing.T) {
	turno := turnoDeSaneamento()
	turno.Cofre().Mascarar("tel " + telefone)
	r := turno.MensagemRecebida(context.Background(), ganchos.MensagemRecebida{Texto: "cpf " + cpfValido})
	if !strings.Contains(r.Evento.Texto, "«cpf:2»") {
		t.Fatalf("texto = %q", r.Evento.Texto)
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
	turno := turnoDeSaneamento()
	turno.Cofre().Mascarar("cpf " + cpfValido)
	r := turno.MensagemRecebida(context.Background(), ganchos.MensagemRecebida{
		Texto: "me mostra «cpf:1» e «tel:2»",
	})
	if strings.Contains(r.Evento.Texto, "«") {
		t.Fatalf("marcador sobreviveu: %q", r.Evento.Texto)
	}
	if r.Decisao.Tipo != ganchos.Ajustar || r.Decisao.Quantidade != 0 {
		t.Fatalf("decisao = %+v", r.Decisao)
	}
	if got := len(turno.Cofre().Ler()); got != 1 {
		t.Fatalf("cofre mudou: %d", got)
	}
}

// Review Focus 1, saída: marcador forjado que não está no cofre do turno não é revelado.
func TestSaneamentoNaoRevelaMarcadorForaDoCofreNaResposta(t *testing.T) {
	turno := turnoDeSaneamento()
	turno.Cofre().Mascarar("cpf " + cpfValido)
	r := turno.AntesDeResponder(context.Background(), ganchos.AntesDeResponder{Texto: "veja «cpf:1» e «cpf:7»"})
	if want := "veja " + cpfValido + " e «cpf:7»"; r.Evento.Texto != want {
		t.Fatalf("texto = %q, quer %q", r.Evento.Texto, want)
	}
	if r.Decisao.Quantidade != 1 {
		t.Fatalf("quantidade = %d, quer 1", r.Decisao.Quantidade)
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
	if r.Decisao.Quantidade != 2 {
		t.Fatalf("decisao = %+v", r.Decisao)
	}
}

func TestSaneamentoDevolveMarcadorDoCofreNaResposta(t *testing.T) {
	turno := turnoDeSaneamento()
	turno.Cofre().Mascarar("cpf " + cpfValido)
	r := turno.AntesDeResponder(context.Background(), ganchos.AntesDeResponder{Texto: "seu cpf é «cpf:1»"})
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

// cpfsValidos gera CPFs com dígitos verificadores corretos, um por semente.
func cpfsValidos(n int) []string {
	cpfs := make([]string, n)
	for i := range cpfs {
		base := []int{1, 2, 3, 4, 5, 6, 7, 8, i + 1}
		base = append(base, digitoVerificador(base))
		base = append(base, digitoVerificador(base))
		cpfs[i] = strings.Trim(strings.ReplaceAll(fmt.Sprint(base), " ", ""), "[]")
	}
	return cpfs
}

func digitoVerificador(digitos []int) int {
	soma := 0
	for i, d := range digitos {
		soma += d * (len(digitos) + 1 - i)
	}
	if resto := soma * 10 % 11; resto < 10 {
		return resto
	}
	return 0
}

func TestSaneamentoNumeraMarcadoresDistintosEmFerramentasParalelas(t *testing.T) {
	const paralelas = 8
	turno := turnoDeSaneamento()
	cpfs := cpfsValidos(paralelas)
	resultados := make([]ganchos.Resultado[ganchos.DepoisDaFerramenta], paralelas)
	var espera sync.WaitGroup
	for i, cpf := range cpfs {
		espera.Add(1)
		go func() {
			defer espera.Done()
			resultados[i] = turno.DepoisDaFerramenta(context.Background(), ganchos.DepoisDaFerramenta{
				Resultado: map[string]any{"cpf": cpf},
			})
		}()
	}
	espera.Wait()
	vistos := map[string]bool{}
	for _, r := range resultados {
		if r.Decisao.Quantidade != 1 {
			t.Fatalf("quantidade = %d, quer 1 (decisao %+v)", r.Decisao.Quantidade, r.Decisao)
		}
		marcador, _ := r.Evento.Resultado.(map[string]any)["cpf"].(string)
		if !strings.HasPrefix(marcador, "«cpf:") || vistos[marcador] {
			t.Fatalf("marcador repetido ou ausente: %q (resultados %v)", marcador, resultados)
		}
		vistos[marcador] = true
	}
	if got := len(turno.Cofre().Ler()); got != paralelas {
		t.Fatalf("cofre guarda %d, quer %d", got, paralelas)
	}
}

func TestSaneamentoNeutralizaMarcadorForjadoNoResultadoEmValorEChave(t *testing.T) {
	turno := turnoDeSaneamento()
	turno.Cofre().Mascarar("cpf " + cpfValido)
	r := turno.DepoisDaFerramenta(context.Background(), ganchos.DepoisDaFerramenta{
		Resultado: map[string]any{"«cpf:1»": "chave", "nota": []any{"veja «cpf:1»"}},
	})
	saida, ok := r.Evento.Resultado.(map[string]any)
	if !ok {
		t.Fatalf("resultado = %T", r.Evento.Resultado)
	}
	if _, achou := saida[`"cpf:1"`]; !achou || len(saida) != 2 {
		t.Fatalf("chave forjada sobreviveu: %v", saida)
	}
	if got := saida["nota"].([]any)[0]; got != `veja "cpf:1"` {
		t.Fatalf("valor forjado sobreviveu: %v", got)
	}
}

func bloqueadoraDeMensagem() ganchos.Gancho[ganchos.MensagemRecebida] {
	return ganchos.Gancho[ganchos.MensagemRecebida]{
		Nome: "bloqueia",
		Avaliar: func(e ganchos.MensagemRecebida) (ganchos.MensagemRecebida, ganchos.Decisao) {
			return e, ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: "fora do escopo"}
		},
	}
}

func TestSaneamentoRodaMesmoDepoisDeBloqueio(t *testing.T) {
	fixos := Saneamento()
	fixos.MensagemRecebida.Checagem = []ganchos.Gancho[ganchos.MensagemRecebida]{bloqueadoraDeMensagem()}
	r := ganchos.NovoConjunto(fixos, nil).Turno().MensagemRecebida(context.Background(), ganchos.MensagemRecebida{
		Texto: "meu cpf " + cpfValido + " e «cpf:9»",
	})
	if r.Decisao.Tipo != ganchos.Bloquear {
		t.Fatalf("decisao = %+v", r.Decisao)
	}
	if strings.Contains(r.Evento.Texto, "529") || !strings.Contains(r.Evento.Texto, "«cpf:1»") || strings.Contains(r.Evento.Texto, "«cpf:9»") {
		t.Fatalf("texto sem sanear: %q", r.Evento.Texto)
	}
}

func TestSaneamentoOmiteMotivoQuandoOSaneadorDoTransbordoFalha(t *testing.T) {
	fixos := ganchos.Fixos{TransbordoOuEncerramento: ganchos.Pontos[ganchos.TransbordoOuEncerramento]{
		Saneamento: []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{{
			Nome: ganchos.GanchoTransbordoSemPII, Falha: ganchos.Aberta,
			Avaliar: func(ganchos.TransbordoOuEncerramento) (ganchos.TransbordoOuEncerramento, ganchos.Decisao) {
				panic("quebrou")
			},
		}},
	}}
	r := ganchos.NovoConjunto(fixos, nil).Turno().TransbordoOuEncerramento(context.Background(), ganchos.TransbordoOuEncerramento{
		Motivo: "morador " + cpfValido,
	})
	if r.Evento.Motivo != ganchos.MotivoTransbordoSolicitado {
		t.Fatalf("motivo = %q", r.Evento.Motivo)
	}
	if r.Decisao.Tipo == ganchos.Bloquear {
		t.Fatalf("transbordo bloqueado: %+v", r.Decisao)
	}
}
