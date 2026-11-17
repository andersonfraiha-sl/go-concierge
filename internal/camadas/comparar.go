package camadas

import "slices"

// Comparar faz a comparação de três vias, trecho a trecho. antes é a versão de cima que a
// pessoa usou como ponto de partida (o merge-base), depois é a versão de cima nova e minha é a
// versão da pessoa. Só aparece o que a atualização muda e a pessoa precisa ver:
//
//	antes == depois           nada (a base não mexeu)
//	antes == minha            so_base (a pessoa nunca mexeu)
//	minha == depois           nada (as duas mudaram igual)
//	senão                     conflito (apagar × alterar inclusive)
//
// Obrigatorio fica false: quem marca é aplicar.go.
func Comparar(antes, depois, minha Documento) []Diferenca {
	a, d, m := indexar(antes), indexar(depois), indexar(minha)
	var diferencas []Diferenca
	for _, id := range ordemDosTrechos(depois, minha, antes) {
		situacao, muda := situacaoDe(a.conteudo(id), d.conteudo(id), m.conteudo(id))
		if !muda {
			continue
		}
		referencia := primeiroQueTem(id, d, m, a)
		diferencas = append(diferencas, Diferenca{
			Trecho: id, Tipo: referencia.Tipo, Rotulo: referencia.Rotulo, Situacao: situacao,
			Antes: a.conteudo(id), Depois: d.conteudo(id), Minha: m.conteudo(id),
		})
	}
	return diferencas
}

func situacaoDe(antes, depois, minha *string) (Situacao, bool) {
	switch {
	case iguais(antes, depois), iguais(minha, depois):
		return "", false
	case iguais(antes, minha):
		return SoBase, true
	default:
		return Conflito, true
	}
}

// iguais trata ausente (nil) como diferente de vazio: apagar uma etapa não é esvaziá-la.
func iguais(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// trechosPorID é o documento já decomposto, para consulta por id.
type trechosPorID map[TrechoID]Trecho

func indexar(d Documento) trechosPorID {
	trechos := Trechos(d)
	porID := make(trechosPorID, len(trechos))
	for _, t := range trechos {
		porID[t.ID] = t
	}
	return porID
}

func (t trechosPorID) conteudo(id TrechoID) *string {
	trecho, ok := t[id]
	if !ok {
		return nil
	}
	return &trecho.Conteudo
}

// ordemDosTrechos junta os ids na ordem dada pelos documentos: o primeiro manda, os
// seguintes só acrescentam o que ele não tem.
func ordemDosTrechos(documentos ...Documento) []TrechoID {
	var ordem []TrechoID
	vistos := map[TrechoID]bool{}
	for _, d := range documentos {
		for _, t := range Trechos(d) {
			if !vistos[t.ID] {
				vistos[t.ID] = true
				ordem = append(ordem, t.ID)
			}
		}
	}
	return ordem
}

// primeiroQueTem devolve o trecho de rótulo e tipo mais atuais: o da versão nova, senão o da
// pessoa, senão o antigo.
func primeiroQueTem(id TrechoID, versoes ...trechosPorID) Trecho {
	i := slices.IndexFunc(versoes, func(v trechosPorID) bool { _, ok := v[id]; return ok })
	if i < 0 {
		return Trecho{ID: id}
	}
	return versoes[i][id]
}
