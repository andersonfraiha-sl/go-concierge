package camadas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go-concierge/internal/okf"
)

// Escolha é o que a pessoa decidiu para um trecho.
type Escolha string

const (
	Aceitar  Escolha = "aceitar"
	Manter   Escolha = "manter"
	Misturar Escolha = "misturar"
)

// Decisao é a escolha de uma pessoa para um trecho. Mistura só vale com Misturar: é o conteúdo
// canônico do trecho, editado por ela.
type Decisao struct {
	Trecho  TrechoID `json:"trecho"`
	Escolha Escolha  `json:"escolha"`
	Mistura string   `json:"mistura,omitempty"`
}

// Aplicar compõe o documento depois das decisões. Trecho sem decisão fica como está em minha:
// o servidor nunca muda sozinho, e por isso conflito sem decisão é erro. As ferramentas e a
// visibilidade do resultado são sempre as de depois (a base manda nelas).
func Aplicar(minha, depois Documento, difs []Diferenca, decisoes []Decisao) (Documento, error) {
	valores, err := valoresDasDecisoes(depois, difs, decisoes)
	if err != nil {
		return Documento{}, err
	}
	if semEscolha := conflitosSemEscolha(difs, decisoes); len(semEscolha) > 0 {
		return Documento{}, &ErrConflitoSemEscolha{Trechos: semEscolha}
	}
	return compor(minha, depois, valores), nil
}

// AceitarPadrao aceita todo trecho em que só a base mudou: é o que o prazo aplica.
func AceitarPadrao(difs []Diferenca) []Decisao {
	var decisoes []Decisao
	for _, d := range difs {
		if d.Situacao == SoBase {
			decisoes = append(decisoes, Decisao{Trecho: d.Trecho, Escolha: Aceitar})
		}
	}
	return decisoes
}

// MarcarObrigatorios devolve cópia das diferenças com Obrigatorio ligado nos trechos dados.
func MarcarObrigatorios(difs []Diferenca, obrigatorios []TrechoID) []Diferenca {
	marcadas := slices.Clone(difs)
	for i := range marcadas {
		marcadas[i].Obrigatorio = slices.Contains(obrigatorios, marcadas[i].Trecho)
	}
	return marcadas
}

// AplicarObrigatorio troca em minha os trechos obrigatórios pelos de depois, mesmo onde a
// pessoa tinha mexido. antes é a versão de cima que a pessoa usou: trocados lista só os
// trechos que ela tinha editado (os que ela nunca tocou entram em silêncio).
func AplicarObrigatorio(antes, minha, depois Documento, obrigatorios []TrechoID) (Documento, []TrechoID) {
	a, d, m := indexar(antes), indexar(depois), indexar(minha)
	valores := map[TrechoID]valorEscolhido{}
	var trocados []TrechoID
	for _, id := range obrigatorios {
		if iguais(m.conteudo(id), d.conteudo(id)) {
			continue
		}
		valores[id] = valorDoDocumento(depois, id)
		if !iguais(a.conteudo(id), m.conteudo(id)) {
			trocados = append(trocados, id)
		}
	}
	return compor(minha, depois, valores), trocados
}

func conflitosSemEscolha(difs []Diferenca, decisoes []Decisao) []TrechoID {
	var faltam []TrechoID
	for _, d := range difs {
		decidido := slices.ContainsFunc(decisoes, func(x Decisao) bool { return x.Trecho == d.Trecho })
		if d.Situacao == Conflito && !decidido {
			faltam = append(faltam, d.Trecho)
		}
	}
	return faltam
}

// valoresDasDecisoes valida as decisões e devolve o valor novo de cada trecho que muda;
// manter não aparece, porque manter é não mexer.
func valoresDasDecisoes(depois Documento, difs []Diferenca, decisoes []Decisao) (map[TrechoID]valorEscolhido, error) {
	valores := map[TrechoID]valorEscolhido{}
	vistos := map[TrechoID]bool{}
	for _, dec := range decisoes {
		if !slices.ContainsFunc(difs, func(d Diferenca) bool { return d.Trecho == dec.Trecho }) {
			return nil, fmt.Errorf("trecho %q não está na atualização: %w", dec.Trecho, ErrDecisaoInvalida)
		}
		if vistos[dec.Trecho] {
			return nil, fmt.Errorf("trecho %q decidido duas vezes: %w", dec.Trecho, ErrDecisaoInvalida)
		}
		vistos[dec.Trecho] = true
		valor, muda, err := valorDaDecisao(depois, dec)
		if err != nil {
			return nil, err
		}
		if muda {
			valores[dec.Trecho] = valor
		}
	}
	return valores, nil
}

func valorDaDecisao(depois Documento, dec Decisao) (valorEscolhido, bool, error) {
	switch dec.Escolha {
	case Manter:
		return valorEscolhido{}, false, nil
	case Aceitar:
		return valorDoDocumento(depois, dec.Trecho), true, nil
	case Misturar:
		valor, err := valorDaMistura(dec.Trecho, dec.Mistura)
		return valor, err == nil, err
	default:
		return valorEscolhido{}, false, fmt.Errorf("escolha %q do trecho %q: %w", dec.Escolha, dec.Trecho, ErrDecisaoInvalida)
	}
}

// valorEscolhido é o conteúdo novo de um trecho já no tipo dele; removido vale para etapa e
// informação que deixam de existir.
type valorEscolhido struct {
	removido   bool
	texto      string
	ordem      []string
	bloco      okf.Bloco
	informacao okf.Informacao
}

var definidoresDeTexto = map[TrechoID]func(*okf.Textos, string){
	IDTitulo:          func(t *okf.Textos, v string) { t.Titulo = v },
	IDResumo:          func(t *okf.Textos, v string) { t.Resumo = v },
	IDQuandoUsar:      func(t *okf.Textos, v string) { t.QuandoUsar = v },
	IDFrasesDeExemplo: func(t *okf.Textos, v string) { t.FrasesDeExemplo = linhas(v) },
	IDPalavrasChave:   func(t *okf.Textos, v string) { t.PalavrasChave = linhas(v) },
}

func linhas(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// valorDoDocumento lê o trecho como ele está em d, no tipo dele.
func valorDoDocumento(d Documento, id TrechoID) valorEscolhido {
	if blocoID, ok := BlocoDoTrecho(id); ok {
		i := slices.IndexFunc(d.Fluxo.Blocos, func(b okf.Bloco) bool { return b.ID == blocoID })
		if i < 0 {
			return valorEscolhido{removido: true}
		}
		return valorEscolhido{bloco: d.Fluxo.Blocos[i]}
	}
	if nome, ok := InformacaoDoTrecho(id); ok {
		i := slices.IndexFunc(d.Fluxo.Informacoes, func(x okf.Informacao) bool { return x.Nome == nome })
		if i < 0 {
			return valorEscolhido{removido: true}
		}
		return valorEscolhido{informacao: d.Fluxo.Informacoes[i]}
	}
	if id == IDOrdem {
		return valorEscolhido{ordem: idsDosBlocos(d.Fluxo.Blocos)}
	}
	conteudo := indexar(d).conteudo(id)
	if conteudo == nil {
		return valorEscolhido{}
	}
	return valorEscolhido{texto: *conteudo}
}

// valorDaMistura lê o que a pessoa escreveu no tipo do trecho; o que não faz parse é decisão inválida.
func valorDaMistura(id TrechoID, mistura string) (valorEscolhido, error) {
	if blocoID, ok := BlocoDoTrecho(id); ok {
		var b okf.Bloco
		if err := lerEstrito(mistura, &b); err != nil || b.Tipo == "" {
			return valorEscolhido{}, erroDeMistura(id, err, "a etapa precisa de um tipo")
		}
		b.ID = blocoID
		return valorEscolhido{bloco: b}, nil
	}
	if nome, ok := InformacaoDoTrecho(id); ok {
		var info okf.Informacao
		if err := lerEstrito(mistura, &info); err != nil {
			return valorEscolhido{}, erroDeMistura(id, err, "")
		}
		info.Nome = nome
		return valorEscolhido{informacao: info}, nil
	}
	if id == IDOrdem {
		var ordem []string
		if err := lerEstrito(mistura, &ordem); err != nil {
			return valorEscolhido{}, erroDeMistura(id, err, "")
		}
		return valorEscolhido{ordem: ordem}, nil
	}
	return valorEscolhido{texto: mistura}, nil
}

func erroDeMistura(id TrechoID, causa error, motivo string) error {
	if causa == nil {
		return fmt.Errorf("mistura do trecho %q: %s: %w", id, motivo, ErrDecisaoInvalida)
	}
	return fmt.Errorf("mistura do trecho %q: %w: %w", id, causa, ErrDecisaoInvalida)
}

func lerEstrito(texto string, destino any) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(texto)))
	dec.DisallowUnknownFields()
	return dec.Decode(destino)
}

// compor aplica os valores sobre minha. Função total: os valores já estão no tipo certo.
func compor(minha, depois Documento, valores map[TrechoID]valorEscolhido) Documento {
	composto := Documento{
		Textos:      minha.Textos,
		Orientacao:  minha.Orientacao,
		Ferramentas: slices.Clone(depois.Ferramentas),
		Fluxo: okf.Fluxo{
			Blocos:      slices.Clone(minha.Fluxo.Blocos),
			Informacoes: slices.Clone(minha.Fluxo.Informacoes),
		},
	}
	composto.Textos.Visibilidade = depois.Textos.Visibilidade
	ordem := idsDosBlocos(minha.Fluxo.Blocos)
	for _, id := range slices.Sorted(maps.Keys(valores)) {
		ordem = aplicarValor(&composto, ordem, id, valores[id])
	}
	composto.Fluxo.Blocos = ordenarBlocos(composto.Fluxo.Blocos, ordem, depois, minha)
	return composto
}

// aplicarValor muda um trecho do composto e devolve a lista de ordem vigente.
func aplicarValor(composto *Documento, ordem []string, id TrechoID, v valorEscolhido) []string {
	if blocoID, ok := BlocoDoTrecho(id); ok {
		composto.Fluxo.Blocos = definirPorChave(composto.Fluxo.Blocos, blocoID, v.removido, v.bloco, func(b okf.Bloco) string { return b.ID })
		return ordem
	}
	if nome, ok := InformacaoDoTrecho(id); ok {
		composto.Fluxo.Informacoes = definirPorChave(composto.Fluxo.Informacoes, nome, v.removido, v.informacao, func(x okf.Informacao) string { return x.Nome })
		return ordem
	}
	switch id {
	case IDOrdem:
		return v.ordem
	case IDOrientacao:
		composto.Orientacao = v.texto
	default:
		if definir, ok := definidoresDeTexto[id]; ok {
			definir(&composto.Textos, v.texto)
		}
	}
	return ordem
}

// definirPorChave troca o item de mesma chave, acrescenta se não existe ou remove.
func definirPorChave[T any](itens []T, chave string, remover bool, novo T, chaveDe func(T) string) []T {
	i := slices.IndexFunc(itens, func(x T) bool { return chaveDe(x) == chave })
	switch {
	case remover && i >= 0:
		return slices.Delete(itens, i, i+1)
	case remover:
		return itens
	case i >= 0:
		itens[i] = novo
		return itens
	default:
		return append(itens, novo)
	}
}

// ordenarBlocos segue a lista escolhida, depois acrescenta o que ficou fora dela (primeiro na
// ordem de depois, depois na de minha) e deixa a resposta por último.
func ordenarBlocos(blocos []okf.Bloco, lista []string, depois, minha Documento) []okf.Bloco {
	presentes := map[string]okf.Bloco{}
	for _, b := range blocos {
		presentes[b.ID] = b
	}
	vistos := map[string]bool{}
	var ordenados []okf.Bloco
	acrescentar := func(id string) {
		if b, ok := presentes[id]; ok && !vistos[id] {
			vistos[id] = true
			ordenados = append(ordenados, b)
		}
	}
	for _, id := range lista {
		acrescentar(id)
	}
	for _, origem := range [][]okf.Bloco{depois.Fluxo.Blocos, minha.Fluxo.Blocos, blocos} {
		for _, b := range origem {
			acrescentar(b.ID)
		}
	}
	slices.SortStableFunc(ordenados, func(a, b okf.Bloco) int {
		return boolParaInt(a.Tipo == okf.BlocoResponder) - boolParaInt(b.Tipo == okf.BlocoResponder)
	})
	return ordenados
}

func boolParaInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
