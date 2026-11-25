// Package camadas guarda as regras puras das skills em camadas: a base da Superlógica, a
// camada da administradora e a do condomínio. Este arquivo e os vizinhos (trechos, comparar,
// aplicar, validar) não fazem I/O nem leem o relógio; quem persiste e orquestra é o serviço.
package camadas

import (
	camadasruntime "go-concierge/internal/harness/camadas"
)

type Tipo = camadasruntime.Tipo
type Camada = camadasruntime.Camada
type Documento = camadasruntime.Documento

const (
	TipoBase           = camadasruntime.TipoBase
	TipoAdministradora = camadasruntime.TipoAdministradora
	TipoCondominio     = camadasruntime.TipoCondominio
)

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

type Cabeca = camadasruntime.Cabeca
type VersaoBase = camadasruntime.VersaoBase
type Retrato = camadasruntime.Retrato

// MudouTextos diz se algum trecho de texto difere entre os dois documentos.
func MudouTextos(antes, depois Documento) bool {
	return !mesmoConteudo(antes, depois, func(t Trecho) bool { return t.Tipo == TrechoTexto })
}

// MudouEtapas diz se o fluxo (etapas, informações, ordem) ou a orientação difere.
func MudouEtapas(antes, depois Documento) bool {
	return !mesmoConteudo(antes, depois, func(t Trecho) bool { return t.Tipo != TrechoTexto })
}
