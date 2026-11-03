// Package copiloto decide, sem efeito colateral, quais propostas do Copiloto a pessoa pode
// aplicar na tela em que está.
package copiloto

import (
	"errors"
	"fmt"
	"slices"

	"go-concierge/internal/okf"
)

type TipoDeOperacao string

const (
	PreencherTextos TipoDeOperacao = "preencher_textos"
	TrocarFluxo     TipoDeOperacao = "trocar_fluxo" // inclui pôr, tirar e reordenar blocos
	CriarSkill      TipoDeOperacao = "criar_skill"
	ExcluirSkill    TipoDeOperacao = "excluir_skill" // o front pede confirmação
	HabilitarSkill  TipoDeOperacao = "habilitar_skill"
	Orientar        TipoDeOperacao = "orientar" // Skill vazia = orientação geral
	Ajustar         TipoDeOperacao = "ajustar"
	EditarPersona   TipoDeOperacao = "editar_persona"
	EditarArquivo   TipoDeOperacao = "editar_arquivo"
)

// Operacao é uma união discriminada por Tipo; só os campos do tipo vêm preenchidos.
type Operacao struct {
	Tipo       TipoDeOperacao  `json:"tipo"`
	Skill      string          `json:"skill,omitempty"`
	Textos     *TextosParciais `json:"textos,omitempty"`
	Fluxo      *okf.Fluxo      `json:"fluxo,omitempty"`
	Titulo     string          `json:"titulo,omitempty"`
	Condominio string          `json:"condominio,omitempty"`
	Habilitado *bool           `json:"habilitado,omitempty"`
	Papeis     []string        `json:"papeis,omitempty"`
	Recurso    string          `json:"recurso,omitempty"`
	Campos     map[string]any  `json:"campos,omitempty"` // payload dinâmico do recurso (exceção da constituição)
	Texto      string          `json:"texto,omitempty"`
	Agente     string          `json:"agente,omitempty"`
	Caminho    string          `json:"caminho,omitempty"`
	Conteudo   string          `json:"conteudo,omitempty"`
}

// TextosParciais traz só os textos que o Copiloto quer trocar; nil = mantém.
type TextosParciais struct {
	Titulo          *string  `json:"titulo,omitempty"`
	Resumo          *string  `json:"resumo,omitempty"`
	QuandoUsar      *string  `json:"quandoUsar,omitempty"`
	PalavrasChave   []string `json:"palavrasChave,omitempty"`
	FrasesDeExemplo []string `json:"frasesDeExemplo,omitempty"`
}

// Quem pede: Administradora "" = Superlógica.
type Quem struct {
	Permissoes     []string
	Administradora string
}

// Recusa explica, em PT-BR simples, por que a proposta de índice Indice não foi aplicada.
type Recusa struct {
	Indice int    `json:"indice"`
	Motivo string `json:"motivo"`
}

const (
	motivoForaDaTela    = "Esta tela não faz isso."
	motivoSemPermissao  = "Você não tem permissão para isso."
	motivoForaDoEscopo  = "Esse condomínio não está com você."
	motivoSoSuperlogica = "Só a Superlógica muda isso."
	motivoIncompleta    = "Proposta incompleta."
)

var permissaoPorTipo = map[TipoDeOperacao]string{
	PreencherTextos: "skills.editar_textos",
	TrocarFluxo:     "skills.editar_fluxo",
	CriarSkill:      "skills.criar",
	ExcluirSkill:    "skills.excluir",
	HabilitarSkill:  "condominios.habilitar",
	Orientar:        "condominios.habilitar",
	Ajustar:         "condominios.habilitar",
	EditarPersona:   "skills.editar_textos",
	EditarArquivo:   "studio.avancado",
}

// doCatalogo são as operações no catálogo global, mesma regra de podeNoCatalogo no front.
var doCatalogo = []TipoDeOperacao{PreencherTextos, TrocarFluxo, CriarSkill, ExcluirSkill, EditarPersona}

// doCondominio são as operações que mexem num condomínio e por isso passam pelo escopo.
var doCondominio = []TipoDeOperacao{HabilitarSkill, Orientar, Ajustar}

// PermissaoDe devolve a permissão exigida pelo tipo; "" para tipo desconhecido.
func PermissaoDe(t TipoDeOperacao) string {
	return permissaoPorTipo[t]
}

type campo struct {
	rotulo     string
	preenchido func(Operacao) bool
}

func texto(get func(Operacao) string) func(Operacao) bool {
	return func(op Operacao) bool { return get(op) != "" }
}

var (
	campoSkill      = campo{"skill", texto(func(op Operacao) string { return op.Skill })}
	campoCondominio = campo{"condominio", texto(func(op Operacao) string { return op.Condominio })}
	campoTexto      = campo{"texto", texto(func(op Operacao) string { return op.Texto })}
)

var camposPorTipo = map[TipoDeOperacao][]campo{
	PreencherTextos: {campoSkill, {"textos", func(op Operacao) bool { return op.Textos != nil }}},
	TrocarFluxo:     {campoSkill, {"fluxo", func(op Operacao) bool { return op.Fluxo != nil }}},
	CriarSkill:      {{"titulo", texto(func(op Operacao) string { return op.Titulo })}},
	ExcluirSkill:    {campoSkill},
	HabilitarSkill:  {campoSkill, campoCondominio, {"habilitado", func(op Operacao) bool { return op.Habilitado != nil }}},
	Orientar:        {campoCondominio, campoTexto},
	Ajustar: {
		campoCondominio,
		{"recurso", texto(func(op Operacao) string { return op.Recurso })},
		{"campos", func(op Operacao) bool { return len(op.Campos) > 0 }},
	},
	EditarPersona: {{"agente", texto(func(op Operacao) string { return op.Agente })}, campoTexto},
	EditarArquivo: {{"caminho", texto(func(op Operacao) string { return op.Caminho })}},
}

// ValidarForma confere os campos obrigatórios do tipo.
func ValidarForma(op Operacao) error {
	campos, conhecido := camposPorTipo[op.Tipo]
	if !conhecido {
		return fmt.Errorf("operação %q: tipo desconhecido", op.Tipo)
	}
	var faltando []error
	for _, c := range campos {
		if !c.preenchido(op) {
			faltando = append(faltando, fmt.Errorf("falta %s", c.rotulo))
		}
	}
	if len(faltando) > 0 {
		return fmt.Errorf("operação %s: %w", op.Tipo, errors.Join(faltando...))
	}
	return nil
}

// Filtrar separa o que a pessoa pode aplicar na tela do que é recusado, com o motivo.
// condominiosDoEscopo diz se o condomínio é de quem pede.
func Filtrar(ops []Operacao, ctx Contexto, quem Quem, condominiosDoEscopo func(id string) bool) (aceitas []Operacao, recusas []Recusa) {
	for i, op := range ops {
		if motivo := motivoDaRecusa(op, ctx, quem, condominiosDoEscopo); motivo != "" {
			recusas = append(recusas, Recusa{Indice: i, Motivo: motivo})
			continue
		}
		aceitas = append(aceitas, op)
	}
	return aceitas, recusas
}

// motivoDaRecusa devolve "" quando a operação pode ser aplicada.
func motivoDaRecusa(op Operacao, ctx Contexto, quem Quem, doEscopo func(string) bool) string {
	switch {
	case !telaFaz(ctx.Tela, op.Tipo):
		return motivoForaDaTela
	case !slices.Contains(quem.Permissoes, PermissaoDe(op.Tipo)):
		return motivoSemPermissao
	case slices.Contains(doCatalogo, op.Tipo) && quem.Administradora != "":
		return motivoSoSuperlogica
	case ValidarForma(op) != nil:
		return motivoIncompleta
	case slices.Contains(doCondominio, op.Tipo) && !condominioPermitido(op.Condominio, ctx, doEscopo):
		return motivoForaDoEscopo
	}
	return ""
}

// condominioPermitido: na ficha de um condomínio, só ele vale, além de estar no escopo.
func condominioPermitido(id string, ctx Contexto, doEscopo func(string) bool) bool {
	if ctx.Tela == TelaCondominio && id != ctx.Condominio {
		return false
	}
	return doEscopo(id)
}

// ValidarFluxoProposto decide no servidor quais blocos gravam e devolve o que impede gravar.
func ValidarFluxoProposto(f okf.Fluxo, escrita func(string) bool) []okf.ProblemaFluxo {
	return okf.ValidarFluxo(okf.Reclassificar(f, escrita))
}
