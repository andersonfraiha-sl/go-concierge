package camadas

import (
	"errors"
	"fmt"
	"strings"

	"go-concierge/internal/okf"
)

var (
	ErrVersaoDesatualizada    = errors.New("versão desatualizada")
	ErrNadaAPublicar          = errors.New("nada a publicar")
	ErrVersaoNaoEncontrada    = errors.New("versão não encontrada")
	ErrAtualizacaoSubstituida = errors.New("atualização substituída por outra mais nova")
	ErrCamadaNaoEncontrada    = errors.New("camada não encontrada")
	ErrMotivoObrigatorio      = errors.New("motivo obrigatório ao marcar trecho como obrigatório")
	ErrDecisaoInvalida        = errors.New("decisão inválida")
	ErrConflitoConcorrente    = errors.New("conflito de escrita concorrente")
)

// ErrConflitoSemEscolha recusa uma decisão que deixou conflito sem escolha: o servidor nunca
// escolhe por quem tem personalização no trecho.
type ErrConflitoSemEscolha struct{ Trechos []TrechoID }

func (e *ErrConflitoSemEscolha) Error() string {
	ids := make([]string, len(e.Trechos))
	for i, t := range e.Trechos {
		ids[i] = string(t)
	}
	return "conflito sem escolha: " + strings.Join(ids, ", ")
}

// ErrComposicaoInvalida recusa um documento cujo fluxo composto tem erro (ou liga ferramenta
// que a base não usa).
type ErrComposicaoInvalida struct{ Problemas []okf.ProblemaFluxo }

func (e *ErrComposicaoInvalida) Error() string {
	return fmt.Sprintf("composição inválida: %d problema(s)", len(e.Problemas))
}

// CamadaQuebrada é uma camada cuja skill composta ficaria inválida com a base nova.
type CamadaQuebrada struct {
	Camada    Camada
	Problemas []okf.ProblemaFluxo
}

// ErrCamadasQuebram recusa publicar a base: as camadas listadas precisam de ajuste antes.
type ErrCamadasQuebram struct{ Camadas []CamadaQuebrada }

func (e *ErrCamadasQuebram) Error() string {
	return fmt.Sprintf("a publicação quebra %d camada(s)", len(e.Camadas))
}
