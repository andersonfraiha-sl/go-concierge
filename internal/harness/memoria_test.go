package harness

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/memoria"
)

func extratorQueResponde(texto string) *ExtratorLLM {
	return &ExtratorLLM{gerar: func(context.Context, []*schema.Message) (*schema.Message, error) {
		return schema.AssistantMessage(texto, nil), nil
	}}
}

func TestExtratorLLMRespostaForaDoFormato(t *testing.T) {
	fatos, err := extratorQueResponde("não sei").Extrair(t.Context(), "oi", "olá", memoria.Memoria{})

	if !errors.Is(err, memoria.ErrRespostaForaDoFormato) || fatos != nil {
		t.Fatalf("Extrair = %v %v, want nil e ErrRespostaForaDoFormato", fatos, err)
	}
}

func fatosDaResposta(t *testing.T, resposta string) []memoria.Fato {
	t.Helper()
	fatos, err := extratorQueResponde(resposta).Extrair(t.Context(), "oi", "olá", memoria.Memoria{})
	if err != nil {
		t.Fatalf("Extrair: %v", err)
	}
	return fatos
}

func TestExtratorMantemCorteDeConfianca(t *testing.T) {
	resposta := `{"fatos":[{"chave":"tratamento","texto":"formal","confianca":0.9},{"chave":"tamanho_resposta","texto":"curta","confianca":0.5}]}`

	fatos := fatosDaResposta(t, resposta)

	if len(fatos) != 1 || fatos[0].Chave != "tratamento" {
		t.Fatalf("Extrair = %+v, want só tratamento", fatos)
	}
}

func TestExtratorDescartaChaveForaDaLista(t *testing.T) {
	resposta := `{"fatos":[{"chave":"pets","texto":"um cachorro","confianca":0.9},{"chave":"tamanho_resposta","texto":"curta","confianca":0.9}]}`

	fatos := fatosDaResposta(t, resposta)

	if len(fatos) != 1 || fatos[0].Chave != "tamanho_resposta" || fatos[0].Texto != "curta" {
		t.Fatalf("Extrair = %+v, want só tamanho_resposta=curta", fatos)
	}
}

func TestExtratorDescartaValorForaDaLista(t *testing.T) {
	resposta := `{"fatos":[{"chave":"tamanho_resposta","texto":"curta mas com detalhes","confianca":0.9}]}`

	if fatos := fatosDaResposta(t, resposta); len(fatos) != 0 {
		t.Fatalf("Extrair = %+v, want nenhum", fatos)
	}
}

func TestExtratorNormalizaValor(t *testing.T) {
	resposta := `{"fatos":[{"chave":"formato_boleto","texto":"Linha digitável","confianca":0.9}]}`

	fatos := fatosDaResposta(t, resposta)

	if len(fatos) != 1 || fatos[0].Texto != "linha_digitavel" {
		t.Fatalf("Extrair = %+v, want formato_boleto=linha_digitavel", fatos)
	}
}

func TestExtratorDescartaNomePreferidoEmFrase(t *testing.T) {
	resposta := `{"fatos":[{"chave":"nome_preferido","texto":"prefere ser chamado de Van","confianca":0.9},{"chave":"nome_preferido","texto":"Van","confianca":0.9}]}`

	fatos := fatosDaResposta(t, resposta)

	if len(fatos) != 1 || fatos[0].Texto != "Van" {
		t.Fatalf("Extrair = %+v, want só o nome curto", fatos)
	}
}

func TestPromptDoExtratorListaAsChaves(t *testing.T) {
	for _, chave := range memoria.ChavesDePreferencia() {
		if !strings.Contains(promptExtrator, chave) {
			t.Errorf("promptExtrator não lista %q", chave)
		}
	}
}

func TestExtratorAceitaJSONEmBlocoDeCodigo(t *testing.T) {
	resposta := "```json\n" + `{"fatos":[{"chave":"tratamento","texto":"formal","confianca":0.9}]}` + "\n```"

	if fatos := fatosDaResposta(t, resposta); len(fatos) != 1 {
		t.Fatalf("Extrair = %+v, want 1 fato", fatos)
	}
}
