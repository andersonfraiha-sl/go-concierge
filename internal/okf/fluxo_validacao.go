package okf

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

const (
	nivelErro  = "erro"
	nivelAviso = "aviso"
)

// ProblemaFluxo é um defeito (erro) ou ressalva (aviso) de um fluxo, apontando o bloco quando há um.
type ProblemaFluxo struct {
	BlocoID  string `json:"blocoId,omitempty"`
	Codigo   string `json:"codigo"`
	Nivel    string `json:"nivel"` // "erro" | "aviso"
	Mensagem string `json:"mensagem"`
}

func erro(blocoID, codigo, mensagem string) ProblemaFluxo {
	return ProblemaFluxo{BlocoID: blocoID, Codigo: codigo, Nivel: nivelErro, Mensagem: mensagem}
}

// obrigatorio descreve o campo sem o qual um bloco não funciona.
type obrigatorio struct {
	rotulo  string
	preench func(Bloco) bool
}

func texto(get func(Bloco) string) func(Bloco) bool {
	return func(b Bloco) bool { return get(b) != "" }
}

var obrigatorios = map[TipoBloco]obrigatorio{
	BlocoBuscar:       {"a ferramenta", texto(func(b Bloco) string { return b.Recurso })},
	BlocoGravar:       {"a ferramenta", texto(func(b Bloco) string { return b.Recurso })},
	BlocoEscolha:      {"onde guardar a escolha", texto(func(b Bloco) string { return b.Param })},
	BlocoPerguntar:    {"as informações perguntadas", func(b Bloco) bool { return len(b.Campos) > 0 }},
	BlocoFormulario:   {"de onde vêm os campos", texto(func(b Bloco) string { return b.De })},
	BlocoConfirmar:    {"a pergunta de confirmação", texto(func(b Bloco) string { return b.Texto })},
	BlocoMontarTexto:  {"a instrução", texto(func(b Bloco) string { return b.Texto })},
	BlocoReaproveitar: {"as informações reaproveitadas", func(b Bloco) bool { return len(b.Campos) > 0 }},
	BlocoResponder:    {"a resposta", texto(func(b Bloco) string { return b.Texto })},
}

// chavesDoBloco são as chaves que a gravação escreve para o tipo; no Extra, repetiriam a chave no arquivo.
var chavesDoBloco = map[TipoBloco][]string{
	BlocoBuscar:       {"resource", "with", "save_as"},
	BlocoGravar:       {"resource", "with", "save_as"},
	BlocoEscolha:      {"param", "title", "from", "label", "value", "ask", "empty", "reuse"},
	BlocoFormulario:   {"fields_from", "save_as"},
	BlocoReaproveitar: {"slot", "slots", "confirm", "from", "label", "value", "ask"},
	tipoInformacao:    {"name", "title", "ask", "type", "options"},
}

// tipoInformacao reaproveita a tabela de chaves para os parâmetros do caso (não é um bloco).
const tipoInformacao TipoBloco = "informacao"

// ValidarFluxo devolve todos os erros que impedem gravar o fluxo; nunca nil.
func ValidarFluxo(f Fluxo) []ProblemaFluxo {
	problemas := []ProblemaFluxo{}
	problemas = append(problemas, problemasDeBlocos(f.Blocos)...)
	problemas = append(problemas, problemasDeResponder(f.Blocos)...)
	return append(problemas, problemasDeInformacoes(f.Informacoes)...)
}

func problemasDeBlocos(blocos []Bloco) []ProblemaFluxo {
	var problemas []ProblemaFluxo
	for _, b := range blocos {
		if o, ok := obrigatorios[b.Tipo]; ok && !o.preench(b) {
			problemas = append(problemas, erro(b.ID, "BLOCO_INCOMPLETO", "Preencha "+o.rotulo+"."))
		}
		problemas = append(problemas, extrasRepetidos(b.ID, b.Tipo, b.Extra, "Este bloco")...)
	}
	return append(problemas, gravacoesSemConfirmar(blocos)...)
}

// gravacoesSemConfirmar: cada Confirmar vale para a próxima gravação, e só para ela.
func gravacoesSemConfirmar(blocos []Bloco) []ProblemaFluxo {
	var problemas []ProblemaFluxo
	confirmado := false
	for _, b := range blocos {
		switch b.Tipo {
		case BlocoConfirmar:
			confirmado = true
		case BlocoGravar:
			if !confirmado {
				problemas = append(problemas, erro(b.ID, "GRAVAR_SEM_CONFIRMAR", "Coloque um Confirmar antes de gravar no sistema."))
			}
			confirmado = false
		}
	}
	return problemas
}

func extrasRepetidos(blocoID string, tipo TipoBloco, extra map[string]any, quem string) []ProblemaFluxo {
	var problemas []ProblemaFluxo
	for _, chave := range slices.Sorted(maps.Keys(extra)) {
		if slices.Contains(chavesDoBloco[tipo], chave) {
			problemas = append(problemas, erro(blocoID, "EXTRA_REPETE_CAMPO", quem+" tem um campo extra repetido: "+chave+"."))
		}
	}
	return problemas
}

func problemasDeResponder(blocos []Bloco) []ProblemaFluxo {
	var problemas []ProblemaFluxo
	respostas, outros := 0, 0
	for i, b := range blocos {
		if b.Tipo != BlocoResponder {
			outros++
			continue
		}
		respostas++
		if i != len(blocos)-1 {
			problemas = append(problemas, erro(b.ID, "RESPONDER_FORA_DO_FIM", "O Responder precisa ser o último bloco."))
		}
	}
	switch {
	case respostas == 0:
		problemas = append(problemas, erro("", "SEM_RESPONDER", "O fluxo precisa terminar com um Responder."))
	case respostas == 1 && outros == 0:
		problemas = append(problemas, erro("", "FLUXO_VAZIO", "Adicione ao menos um bloco antes do Responder."))
	}
	return problemas
}

// problemasDeInformacoes: a gravação de opções simples só guarda o valor, então um rótulo diferente se perderia.
func problemasDeInformacoes(informacoes []Informacao) []ProblemaFluxo {
	var problemas []ProblemaFluxo
	for _, info := range informacoes {
		problemas = append(problemas, extrasRepetidos("", tipoInformacao, info.Extra, "A informação "+info.Nome)...)
		if info.OpcoesSimples && slices.ContainsFunc(info.Opcoes, func(o Opcao) bool { return o.Rotulo != o.Valor }) {
			mensagem := fmt.Sprintf("A informação %s tem opções simples com rótulo diferente do valor; o rótulo se perderia.", info.Nome)
			problemas = append(problemas, erro("", "OPCAO_ROTULO_PERDIDO", mensagem))
		}
	}
	return problemas
}

// Reclassificar decide no servidor se cada ferramenta busca ou grava; o tipo vindo do cliente é ignorado.
func Reclassificar(f Fluxo, escrita func(string) bool) Fluxo {
	blocos := slices.Clone(f.Blocos)
	for i, b := range blocos {
		if (b.Tipo != BlocoBuscar && b.Tipo != BlocoGravar) || b.Recurso == "" {
			continue
		}
		blocos[i].Tipo = BlocoBuscar
		if escrita(b.Recurso) {
			blocos[i].Tipo = BlocoGravar
		}
	}
	f.Blocos = blocos
	return f
}

// FerramentasDesconhecidas avisa dos blocos cuja ferramenta saiu do catálogo.
func FerramentasDesconhecidas(f Fluxo, conhecida func(string) bool) []ProblemaFluxo {
	var avisos []ProblemaFluxo
	for _, b := range f.Blocos {
		if (b.Tipo == BlocoBuscar || b.Tipo == BlocoGravar) && b.Recurso != "" && !conhecida(b.Recurso) {
			avisos = append(avisos, ProblemaFluxo{
				BlocoID: b.ID, Codigo: "FERRAMENTA_DESCONHECIDA", Nivel: nivelAviso,
				Mensagem: "Esta ferramenta não está no catálogo agora; o Studio deduziu pelo nome se ela grava no sistema.",
			})
		}
	}
	return avisos
}

var verbosLeitura = []string{"get-", "list-", "find-", "search-", "read-", "fetch-", "show-", "count-", "check-", "health", "hello", "status"}

// EscritaPeloNome("") é true (nome sem verbo de leitura); quem classifica trata recurso vazio antes.
// EscritaPeloNome deduz pelo último nível do nome (a operação) se a ferramenta grava no sistema.
func EscritaPeloNome(recurso string) bool {
	op := strings.ToLower(recurso[strings.LastIndex(recurso, ".")+1:])
	return !slices.ContainsFunc(verbosLeitura, func(v string) bool { return strings.HasPrefix(op, v) })
}
