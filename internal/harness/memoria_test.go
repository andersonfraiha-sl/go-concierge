package harness

import (
	"context"
	"errors"
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

func TestExtratorLLMFiltraConfiancaBaixa(t *testing.T) {
	resposta := "```json\n{\"fatos\":[{\"chave\":\"pets\",\"texto\":\"tem um cachorro\",\"confianca\":0.9},{\"chave\":\"veiculo\",\"texto\":\"talvez um carro\",\"confianca\":0.3}]}\n```"

	fatos, err := extratorQueResponde(resposta).Extrair(t.Context(), "tenho um cachorro", "anotado", memoria.Memoria{})

	if err != nil || len(fatos) != 1 || fatos[0].Chave != "pets" {
		t.Fatalf("Extrair = %+v %v", fatos, err)
	}
}
