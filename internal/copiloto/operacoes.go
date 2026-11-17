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
	Conteudo   *string         `json:"conteudo,omitempty"` // nil = faltou; "" = esvaziar o arquivo
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

// alcance diz que regra extra a operação segue além da permissão. O zero é indefinido e recusa.
type alcance int

const (
	alcanceIndefinido alcance = iota
	alcanceCatalogo           // catálogo global: só a Superlógica muda (podeNoCatalogo no front)
	alcanceCondominio         // mexe num condomínio: precisa estar no escopo de quem pede
)

type campo struct {
	rotulo     string
	preenchido func(Operacao) bool
}

// regra é tudo que decide se um tipo de operação passa. Tipo sem regra completa é recusado.
type regra struct {
	permissao string
	alcance   alcance
	campos    []campo
}

func (r regra) completa() bool {
	return r.permissao != "" && r.alcance != alcanceIndefinido && len(r.campos) > 0
}

func texto(get func(Operacao) string) func(Operacao) bool {
	return func(op Operacao) bool { return get(op) != "" }
}

var (
	campoSkill      = campo{"skill", texto(func(op Operacao) string { return op.Skill })}
	campoCondominio = campo{"condominio", texto(func(op Operacao) string { return op.Condominio })}
	campoTexto      = campo{"texto", texto(func(op Operacao) string { return op.Texto })}
)

var regras = map[TipoDeOperacao]regra{
	PreencherTextos: {"skills.editar_textos", alcanceCatalogo, []campo{campoSkill, {"textos", func(op Operacao) bool { return op.Textos != nil }}}},
	TrocarFluxo:     {"skills.editar_fluxo", alcanceCatalogo, []campo{campoSkill, {"fluxo", func(op Operacao) bool { return op.Fluxo != nil }}}},
	CriarSkill:      {"skills.criar", alcanceCatalogo, []campo{{"titulo", texto(func(op Operacao) string { return op.Titulo })}}},
	ExcluirSkill:    {"skills.excluir", alcanceCatalogo, []campo{campoSkill}},
	HabilitarSkill: {"condominios.habilitar", alcanceCondominio, []campo{
		campoSkill, campoCondominio, {"habilitado", func(op Operacao) bool { return op.Habilitado != nil }},
	}},
	Orientar: {"condominios.habilitar", alcanceCondominio, []campo{campoCondominio, campoTexto}},
	Ajustar: {"condominios.habilitar", alcanceCondominio, []campo{
		campoCondominio,
		{"recurso", texto(func(op Operacao) string { return op.Recurso })},
		{"campos", func(op Operacao) bool { return len(op.Campos) > 0 }},
	}},
	EditarPersona: {"skills.editar_textos", alcanceCatalogo, []campo{{"agente", texto(func(op Operacao) string { return op.Agente })}, campoTexto}},
	// Os arquivos do Avançado valem para todas as administradoras; Conteudo vazio explícito é permitido.
	EditarArquivo: {"studio.avancado", alcanceCatalogo, []campo{
		{"caminho", texto(func(op Operacao) string { return op.Caminho })},
		{"conteudo", func(op Operacao) bool { return op.Conteudo != nil }},
	}},
}

// PermissaoDe devolve a permissão exigida pelo tipo; "" para tipo desconhecido.
func PermissaoDe(t TipoDeOperacao) string {
	return regras[t].permissao
}

// ValidarForma confere os campos obrigatórios do tipo.
func ValidarForma(op Operacao) error {
	r := regras[op.Tipo]
	if !r.completa() {
		return fmt.Errorf("operação %q: tipo desconhecido", op.Tipo)
	}
	var faltando []error
	for _, c := range r.campos {
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

// motivoDaRecusa devolve "" quando a operação pode ser aplicada. Forma vem antes do escopo
// porque o escopo precisa do condomínio preenchido.
func motivoDaRecusa(op Operacao, ctx Contexto, quem Quem, doEscopo func(string) bool) string {
	r := regras[op.Tipo]
	switch {
	case !telaFaz(ctx.Tela, op.Tipo):
		return motivoForaDaTela
	case !r.completa():
		return motivoIncompleta
	case !slices.Contains(quem.Permissoes, r.permissao):
		return motivoSemPermissao
	case r.alcance == alcanceCatalogo && quem.Administradora != "":
		return motivoSoSuperlogica
	case ValidarForma(op) != nil:
		return motivoIncompleta
	case r.alcance == alcanceCondominio && !condominioPermitido(op.Condominio, ctx, doEscopo):
		return motivoForaDoEscopo
	}
	return ""
}

// condominioPermitido: na ficha de um condomínio, só ele vale, além de estar no escopo.
// doEscopo nil = nenhum condomínio é de quem pede.
func condominioPermitido(id string, ctx Contexto, doEscopo func(string) bool) bool {
	if ctx.Tela == TelaCondominio && id != ctx.Condominio {
		return false
	}
	return doEscopo != nil && doEscopo(id)
}

// ValidarFluxoProposto decide no servidor quais blocos gravam e devolve o que impede gravar.
func ValidarFluxoProposto(f okf.Fluxo, escrita func(string) bool) []okf.ProblemaFluxo {
	return okf.ValidarFluxo(okf.Reclassificar(f, escrita))
}
