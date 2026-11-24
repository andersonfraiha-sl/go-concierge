package conteudo

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
)

var (
	ErrSemFluxo             = errors.New("skill sem fluxo")
	ErrSkillNaoExiste       = errors.New("skill não existe")
	ErrVersaoDesatualizada  = errors.New("versão desatualizada")
	ErrNomeInvalido         = errors.New("nome inválido")
	ErrSkillJaExiste        = errors.New("skill já existe")
	ErrTituloVazio          = errors.New("título vazio")
	ErrVisibilidadeInvalida = errors.New("visibilidade inválida")
)

// Textos são as partes da skill que o Studio edita fora do fluxo.
type Textos struct {
	Titulo          string   `json:"titulo"`          // caso.name (sem caso: agent.yaml nome)
	QuandoUsar      string   `json:"quandoUsar"`      // caso.description (sem caso: agent.yaml descricao)
	FrasesDeExemplo []string `json:"frasesDeExemplo"` // caso.examples (sem caso: agent.yaml frases_de_exemplo)
	PalavrasChave   []string `json:"palavrasChave"`   // caso.keywords (sem caso: agent.yaml palavras_chave)
	Resumo          string   `json:"resumo"`          // caso.summary (sem caso: agent.yaml resumo)
	Visibilidade    string   `json:"visibilidade"`    // caso.visibility (sem caso: "")
	TemFluxo        bool     `json:"temFluxo"`        // só leitura
}

// TextosNoCaso devolve o caso.yaml com os campos de texto trocados e o resto (passos, params,
// ids) como estava. Visibilidade vazia mantém a atual, e fora de private|public é recusada.
func TextosNoCaso(raw []byte, atual, t Textos) ([]byte, error) {
	visibilidade := primeiroNaoVazio(t.Visibilidade, atual.Visibilidade)
	if visibilidade != "private" && visibilidade != "public" {
		return nil, ErrVisibilidadeInvalida
	}
	frases := make(Seq, len(t.FrasesDeExemplo))
	for i, frase := range t.FrasesDeExemplo {
		frases[i] = frase
	}
	chaves, err := ChavesDoCaso([]Par{
		{"name", t.Titulo}, {"description", t.QuandoUsar}, {"examples", frases}, {"keywords", Lista(t.PalavrasChave)},
		{"summary", resumoOuApagar(t.Resumo)}, {"visibility", visibilidade},
	})
	if err != nil {
		return nil, err
	}
	return SubstituirChaves(raw, chaves)
}

// resumoOuApagar devolve o resumo como está, ou um *yaml.Node nil (que definirChave apaga)
// quando vazio.
func resumoOuApagar(resumo string) any {
	if resumo == "" {
		return (*yaml.Node)(nil)
	}
	return resumo
}

func ChavesDoCaso(pares []Par) ([]chaveNo, error) {
	chaves := make([]chaveNo, len(pares))
	for i, p := range pares {
		no, err := NoDe(p.Valor)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Chave, err)
		}
		chaves[i] = chaveNo{p.Chave, no}
	}
	return chaves, nil
}
