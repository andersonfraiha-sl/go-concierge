// Package camadas guarda as regras puras das skills em camadas: a base da Superlógica, a
// camada da administradora e a do condomínio. Este arquivo e os vizinhos (trechos, comparar,
// aplicar, validar) não fazem I/O nem leem o relógio; quem persiste e orquestra é o serviço.
package camadas

import (
	"time"

	"go-concierge/internal/okf"
)

// Tipo diz de que nível é uma camada.
type Tipo string

const (
	TipoBase           Tipo = "base"
	TipoAdministradora Tipo = "administradora"
	TipoCondominio     Tipo = "condominio"
)

// Camada identifica uma camada de uma skill. ID é vazio na base.
type Camada struct {
	Skill string `json:"skill"`
	Tipo  Tipo   `json:"tipo"`
	ID    string `json:"id"`
}

// Documento é o que uma camada guarda: a skill composta inteira daquele nível. As ferramentas
// são sempre as da base; a camada de baixo só herda.
type Documento struct {
	Textos      okf.Textos `json:"textos"`
	Fluxo       okf.Fluxo  `json:"fluxo"`
	Ferramentas []string   `json:"ferramentas"`
	Orientacao  string     `json:"orientacao"`
}

// TrechoID é a identidade estável de um trecho: sobrevive a reordenar e a renomear.
type TrechoID string

// TipoTrecho agrupa os trechos pelo que a tela mostra.
type TipoTrecho string

const (
	TrechoTexto      TipoTrecho = "texto"
	TrechoEtapa      TipoTrecho = "etapa"
	TrechoInformacao TipoTrecho = "informacao"
	TrechoOrdem      TipoTrecho = "ordem"
	TrechoOrientacao TipoTrecho = "orientacao"
)

// Trecho é a menor parte de um documento que se compara e se decide sozinha. Conteudo é a
// forma canônica: texto puro, ou JSON compacto (etapa, informação e ordem).
type Trecho struct {
	ID       TrechoID
	Tipo     TipoTrecho
	Rotulo   string
	Conteudo string
}

// Situacao diz por que uma diferença precisa de atenção.
type Situacao string

const (
	// SoBase: só a base mudou; a pessoa nunca mexeu no trecho.
	SoBase Situacao = "so_base"
	// Conflito: a base e a pessoa mudaram o trecho de jeitos diferentes (apagar × alterar inclusive).
	Conflito Situacao = "conflito"
)

// Diferenca é um trecho que a atualização muda e que pede decisão (ou aviso). Antes, Depois e
// Minha são nil quando o trecho não existe naquela versão.
type Diferenca struct {
	Trecho      TrechoID   `json:"trecho"`
	Tipo        TipoTrecho `json:"tipo"`
	Rotulo      string     `json:"rotulo"`
	Situacao    Situacao   `json:"situacao"`
	Obrigatorio bool       `json:"obrigatorio"`
	Antes       *string    `json:"antes"`
	Depois      *string    `json:"depois"`
	Minha       *string    `json:"minha"`
}

// Cabeca é a versão mais recente de uma camada.
type Cabeca struct {
	VersaoID  int
	Documento Documento
}

// VersaoBase é a cabeça da base: além do documento, o caso.yaml cru (para o turno sem
// reler o disco) e a data em que ela deixa de valer, se foi encerrada.
type VersaoBase struct {
	Cabeca
	CasoYAML  []byte
	EncerraEm time.Time
}

// Retrato é tudo o que a composição precisa, lido do banco de uma vez.
type Retrato struct {
	Bases           map[string]VersaoBase        // skill → cabeça da base
	Administradoras map[string]map[string]Cabeca // skill → administradora → cabeça
	Condominios     map[string]map[string]Cabeca // skill → condomínio → cabeça
}

// MudouTextos diz se algum trecho de texto difere entre os dois documentos.
func MudouTextos(antes, depois Documento) bool {
	return !mesmoConteudo(antes, depois, func(t Trecho) bool { return t.Tipo == TrechoTexto })
}

// MudouEtapas diz se o fluxo (etapas, informações, ordem) ou a orientação difere.
func MudouEtapas(antes, depois Documento) bool {
	return !mesmoConteudo(antes, depois, func(t Trecho) bool { return t.Tipo != TrechoTexto })
}
