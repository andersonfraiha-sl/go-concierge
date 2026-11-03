package okf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const passosIntro = "A ordem do acontecimento (os passos de `caso.yaml`, que o harness executa de forma determinística):"

// GerarPrincipal escreve o workflows/principal.md de uma skill a partir do fluxo: um
// parágrafo por bloco, na ordem em que o harness executa.
func GerarPrincipal(titulo, descricao string, blocos []Bloco) string {
	linhas := []string{"# Workflow — " + titulo, ""}
	if d := colapsar(descricao); d != "" {
		linhas = append(linhas, d, "")
	}
	linhas = append(linhas, passosIntro, "")
	for i, b := range blocos {
		linhas = append(linhas, fmt.Sprintf("%d. %s", i+1, linhaDoBloco(b)))
	}
	return strings.Join(linhas, "\n") + "\n"
}

var redacaoDoBloco = map[TipoBloco]func(Bloco) string{
	BlocoBuscar:    redigirBuscar,
	BlocoGravar:    redigirGravar,
	BlocoEscolha:   redigirEscolha,
	BlocoPerguntar: func(b Bloco) string { return "Colete, um a um e só o que ainda falta: " + crases(b.Campos) + "." },
	BlocoFormulario: func(b Bloco) string {
		return "Monte o formulário dinâmico com os campos de `" + b.De + "` e guarde as alterações em `" + b.GuardarEm + "`."
	},
	BlocoConfirmar: func(b Bloco) string {
		return `Confirme com o morador (sim/não): "` + colapsar(b.Texto) + `". Não vale sem o sim.`
	},
	BlocoMontarTexto:  func(b Bloco) string { return `Redija com o modelo, seguindo: "` + colapsar(b.Texto) + `".` },
	BlocoReaproveitar: redigirReaproveitar,
	BlocoResponder:    func(b Bloco) string { return "Responda no formato: " + colapsar(b.Texto) },
}

// linhaDoBloco devolve texto vazio para um tipo desconhecido; ValidarFluxo recusa esse caso antes.
func linhaDoBloco(b Bloco) string {
	redigir, ok := redacaoDoBloco[b.Tipo]
	if !ok {
		return ""
	}
	return redigir(b)
}

func redigirBuscar(b Bloco) string {
	return "Chame `" + b.Recurso + "` (chamar_recurso)" + clausulaCom(b.Com) + sufixoGuardar(" e guarde em ", b.GuardarEm) + "."
}

func redigirGravar(b Bloco) string {
	return "ESCRITA: chame `" + b.Recurso + "`" + clausulaCom(b.Com) + sufixoGuardar("; guarde o retorno em ", b.GuardarEm) +
		". Só depois do confirm; o protocolo devolvido vai na resposta."
}

func redigirEscolha(b Bloco) string {
	rotulo := b.Rotulo
	if rotulo == "" {
		rotulo = "label"
	}
	pergunta := b.Pergunta
	if pergunta == "" {
		pergunta = "Qual " + strings.ToLower(primeiroNaoVazio(b.Titulo, b.Param)) + "?"
	}
	linha := "Defina `" + b.Param + "` a partir de `" + b.De + "` (mostre `" + rotulo + "`): com UMA só opção, use-a sem perguntar; " +
		`com várias, pergunte "` + pergunta + `" oferecendo as opções.`
	if b.Reaproveitar != "" {
		linha += " Se o morador já disse ou a memória/identidade já responde, use sem perguntar."
	}
	if b.SeVazio != "" {
		linha += ` Se vazio: "` + b.SeVazio + `"`
	}
	return linha
}

func redigirReaproveitar(b Bloco) string {
	pergunta := "pergunte se pode reaproveitar"
	if b.Pergunta != "" {
		pergunta = `pergunte "` + b.Pergunta + `"`
	}
	return "Se a conversa já tem " + crases(b.Campos) + ", " + pergunta + " (sim/não, guardado em `" + b.Confirmacao +
		"`); com o sim, reaproveite sem perguntar de novo."
}

// clausulaCom devolve " com `campo` = valor, ..." ou vazio quando não há campos.
func clausulaCom(com []CampoValor) string {
	if len(com) == 0 {
		return ""
	}
	partes := make([]string, len(com))
	for i, c := range com {
		partes[i] = "`" + c.Campo + "` = " + valorEmTexto(c.Valor)
	}
	return " com " + strings.Join(partes, ", ")
}

func sufixoGuardar(prefixo, nome string) string {
	if nome == "" {
		return ""
	}
	return prefixo + "`" + nome + "`"
}

func crases(nomes []string) string {
	return "`" + strings.Join(nomes, "`, `") + "`"
}

// valorEmTexto devolve texto como está e qualquer outro valor como JSON compacto.
func valorEmTexto(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Sprint(v)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func colapsar(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func primeiroNaoVazio(preferido, reserva string) string {
	if preferido != "" {
		return preferido
	}
	return reserva
}
