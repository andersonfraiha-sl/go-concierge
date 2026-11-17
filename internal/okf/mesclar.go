package okf

import "gopkg.in/yaml.v3"

// mesclarNo leva para `atual` (o nó lido do arquivo) só o que difere em `novo` e devolve o nó
// a gravar. O que não mudou fica com o nó original: aspas, comentários e ordem das chaves.
func mesclarNo(atual, novo *yaml.Node) *yaml.Node {
	if atual.Kind != novo.Kind || atual.Tag != novo.Tag {
		return comComentariosDe(atual, novo)
	}
	switch atual.Kind {
	case yaml.ScalarNode:
		// Mesmo tag: o estilo do arquivo (aspas, bloco) continua servindo ao valor novo.
		atual.Value = novo.Value
	case yaml.MappingNode:
		atual.Content = mesclarMapa(atual.Content, novo.Content)
	case yaml.SequenceNode:
		atual.Content = mesclarLista(atual.Content, novo.Content)
	default:
		return comComentariosDe(atual, novo)
	}
	return atual
}

// mesclarMapa mantém as chaves que seguem na ordem do arquivo, tira as que saíram e põe as
// novas no fim.
func mesclarMapa(atual, novo []*yaml.Node) []*yaml.Node {
	mesclado := make([]*yaml.Node, 0, len(novo))
	for i := 0; i+1 < len(atual); i += 2 {
		if j := indiceDaChave(novo, atual[i].Value); j >= 0 {
			mesclado = append(mesclado, atual[i], mesclarNo(atual[i+1], novo[j+1]))
		}
	}
	for j := 0; j+1 < len(novo); j += 2 {
		if indiceDaChave(atual, novo[j].Value) < 0 {
			mesclado = append(mesclado, novo[j], novo[j+1])
		}
	}
	return mesclado
}

// mesclarLista casa os itens pela posição.
// ponytail: inserir um passo no meio regrava os seguintes; casar por identidade se doer.
func mesclarLista(atual, novo []*yaml.Node) []*yaml.Node {
	mesclado := make([]*yaml.Node, len(novo))
	for i, n := range novo {
		if i < len(atual) {
			mesclado[i] = mesclarNo(atual[i], n)
			continue
		}
		mesclado[i] = n
	}
	return mesclado
}

func comComentariosDe(atual, novo *yaml.Node) *yaml.Node {
	novo.HeadComment, novo.LineComment, novo.FootComment = atual.HeadComment, atual.LineComment, atual.FootComment
	return novo
}
