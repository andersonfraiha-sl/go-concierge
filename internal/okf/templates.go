package okf

import (
	"fmt"
	"strings"
)

// Arquivos-modelo: o que a interface cria quando o usuário marca uma caixinha no
// cadastro ou pede "novo arquivo". O texto entre <> é para substituir.

// ModeloPersona: persona.md de quem fala com o morador.
func ModeloPersona(n NovoAgente) string {
	return "# Persona\n\nVocê é o concierge do condomínio, falando pelo WhatsApp em nome da administração.\n\n- Trate o morador pelo primeiro nome quando ele estiver identificado.\n- Tom cordial, direto e prestativo: frases curtas, sem gírias e sem formalidade excessiva.\n- Nunca use termos internos (caso de uso, recurso, plano, catálogo, sistema, modelo, rota).\n- Sem informação, diga isso com clareza e oriente a procurar a administração; não invente.\n- Apresente-se só na primeira mensagem da conversa; depois, vá direto ao ponto.\n"
}

// ModeloCaso: caso.yaml no formato do catálogo (passos determinísticos).
func ModeloCaso(n NovoAgente) string {
	id := strings.ReplaceAll(n.Nome, "-", "_")
	desc := strings.TrimSpace(n.Descricao)
	if desc == "" {
		desc = "<quando este caso é usado, na linguagem do morador>"
	}
	return fmt.Sprintf(`id: %s
name: %s
visibility: private          # public | private
description: >
  %s
keywords: [<palavra>, <palavra>]
examples:
  - <frase típica do morador>
params:
  - name: <param>
    title: <Título>
    ask: <pergunta ao morador>
    # type: text | textarea | number | date | phone | select | checkbox | list
steps:
  - resource: unidades.listar          # leitura (precisa estar em agent.yaml → recursos ou aqui)
    save_as: unidades
  - choose:
      param: unidade
      reuse: always
      title: Unidade
      from: unidades
      label: label
      value: id
      ask: Para qual unidade?
  - collect: [<param>]
  - confirm: "Confirma <a ação> para {{.unidade_label}}? (sim/não)"
  - resource: <recurso.de.escrita>     # ESCRITA: só aqui, depois do confirm
    with:
      unidade_id: "{{.unidade}}"
      <campo>: "{{.<param>}}"
    save_as: resultado
output: "Pronto! Protocolo: {{.resultado.protocolo}}."
`, id, titulo(n.Nome), desc)
}

// ModelosDaCaixinha devolve nome-relativo → conteúdo para uma caixinha na criação.
func ModelosDaCaixinha(cx string, n NovoAgente) map[string]string {
	nome := n.Nome
	desc := strings.TrimSpace(n.Descricao)
	if desc == "" {
		desc = "<o que este agente faz, em uma linha>"
	}
	switch cx {
	case "rules":
		return map[string]string{
			"rules/politicas.md": "# Políticas\n\n- <a fronteira principal: o que ele NÃO faz / não decide>\n- Só afirme conteúdo de conhecimento ou de recurso depois de ler pela tool; cite a fonte.\n- Se a resposta não está nas fontes, diga que não está — não improvise.\n- Escrita (liberar, reservar, agendar, atualizar) só pelo caso de uso, com confirmação.\n",
			"rules/output.md":    "# Contrato de saída\n\n## Resposta\n(até 4 linhas, direto ao ponto, em português próprio para WhatsApp)\n\n## Fonte\n(o conhecimento consultado e/ou o recurso usado)\n",
		}
	case "workflows":
		if n.Tipo == Orquestrador {
			return map[string]string{
				"workflows/principal.md": fmt.Sprintf("# Workflow — %s\n\nVocê delega: o roteamento (routers/rotas.md) escolhe o subagente antes de você.\nSe a mensagem chegou até você, nenhuma rota servia:\n\n1. Leia os termos de `knowledge/glossary.yaml` para reconhecer o assunto pelos sinônimos.\n2. Se é um caso de uso de um subagente, encaminhe; se é dúvida, responda com o conhecimento.\n3. Se não há como atender, diga em uma frase o que você consegue fazer e transfira ao atendente.\n", nome),
			}
		}
		return map[string]string{
			"workflows/principal.md": fmt.Sprintf("# Workflow — %s\n\n1. Leia `knowledge/<arquivo>.md` (ler_dado) para <o que esse arquivo resolve>.\n2. <Chame o recurso do Nexus com chamar_recurso(nome, params), se precisar de dado vivo>.\n3. <Cruzamento/decisão: o que fazer com o que voltou>.\n4. Responda no contrato de saída, citando a fonte.\n", nome),
		}
	case "routers":
		return map[string]string{
			"routers/rotas.md":         "# Roteamento\n\nEscolha a rota pela linha que melhor descreve a mensagem.\n\n- Qualquer outra coisa → rota `self` (responda você mesmo).\n",
			"routers/desambiguacao.md": "# Desambiguação\n\n- Se a mensagem junta duas intenções, escolha a do **entregável final**.\n- Na dúvida real entre dois destinos, prefira `self` e peça a precisão que falta.\n",
		}
	case "knowledge":
		return map[string]string{
			"knowledge/glossary.yaml":     fmt.Sprintf("# Glossário OKF — UMA definição por termo, com sinônimos: o roteador reconhece a\n# intenção por eles e a projeção leva os termos ao prompt do agente.\nterms:\n  - id: <termo>\n    display_name: <Termo>\n    synonyms: [<sinônimo 1>, <sinônimo 2>]\n    definition: <o que é, na linguagem do domínio de %s>\n    labels: [<rótulo>]\n", nome),
			"knowledge/domain/dominio.md": fmt.Sprintf("---\nid: okf-%s-dominio\ntype: Domain\ntitle: Domínio — %s\nversion: 1.0.0\nstatus: ativo\ndescription: %s\ntags: [okf, dominio]\n---\n\n# Domínio\n\n<os conceitos, invariantes e limites do domínio deste agente>\n", nome, nome, desc),
		}
	case "assets":
		return map[string]string{
			"assets/rodape.md": "<trecho fixo que a resposta pode incluir no fim — referencie como `assets/rodape.md`>\n",
		}
	case "evals":
		return map[string]string{
			"evals/casos.yaml": "- nome: <o defeito que este caso impede de voltar>\n  pergunta: \"<pergunta típica do morador>\"\n  # rota: <subagente esperado>        # barato: só a decisão de rota\n  contem: [\"<trecho obrigatório>\"]\n  nao_contem: [\"<o erro real que você viu>\"]\n",
		}
	}
	return nil
}

// ModeloDeArquivo: conteúdo inicial para um arquivo novo criado pela interface.
func ModeloDeArquivo(rel string) string {
	if rel == PersonaMD {
		return ModeloPersona(NovoAgente{})
	}
	if rel == CasoYAML {
		return ModeloCaso(NovoAgente{Nome: "novo-caso"})
	}
	cx, resto, _ := strings.Cut(rel, "/")
	nome := strings.TrimSuffix(resto, extDe(resto))
	switch {
	case rel == "rules/output.md":
		return "# Contrato de saída\n\n## Resposta\n(até 4 linhas)\n\n## Fonte\n(conhecimento e/ou recurso)\n"
	case cx == "rules":
		return "# " + titulo(nome) + "\n\n- <regra em imperativo, uma por linha>\n"
	case rel == "workflows/principal.md":
		return "# Workflow\n\n1. <ação com a tool nominal e o arquivo exato>\n2. <critério de parada>\n"
	case cx == "workflows":
		return "---\ndescription: <o que o procedimento faz — aparece no índice do prompt>\n---\n\n# " + titulo(nome) + "\n\n1. <ação concreta com tool>\n2. <critério de parada>\n"
	case cx == "routers" && strings.HasSuffix(rel, "rotas.md"):
		return "# Roteamento\n\n- <critério observável> → rota `<subagente>`.\n- Qualquer outra coisa → rota `self`.\n"
	case cx == "routers":
		return "# Desambiguação\n\n- <quando dois destinos servem, qual ganha e por quê>\n"
	case cx == "evals":
		return "- nome: <defeito que este caso impede de voltar>\n  pergunta: \"<pergunta>\"\n  contem: [\"<trecho>\"]\n"
	case strings.HasPrefix(rel, "knowledge/questions/"):
		return "---\ntype: Question\ntitle: Pergunta — " + titulo(nome) + "\nquestion: " + nome + "\nanswers: [<RESPOSTA_A>, <RESPOSTA_B>, NONE]\n---\n\n## Answers (o espaço de leituras)\n\n<o que cada resposta significa>\n\n## Method\n\n1. <passo>\n\n## Tie-breaking\n\n- <como desempatar>\n"
	case strings.HasPrefix(rel, "knowledge/conditions/"):
		return "---\ntype: Condition\ntitle: " + titulo(nome) + "\n---\n\n## <estado 1>\n\n<quando vale e o que impede de concluir>\n"
	case strings.HasPrefix(rel, "knowledge/policies/"):
		return "---\ntype: Policy\ntitle: " + titulo(nome) + "\n---\n\n- <regra>\n"
	case cx == "knowledge" && strings.HasSuffix(rel, "glossary.yaml"):
		return "terms:\n  - id: <termo>\n    display_name: <Termo>\n    synonyms: []\n    definition: <definição>\n    labels: []\n"
	case cx == "knowledge":
		return "---\nid: okf-" + nome + "\ntype: Domain\ntitle: " + titulo(nome) + "\nversion: 1.0.0\nstatus: ativo\ndescription: <uma linha — é isso que aparece no índice do prompt>\ntags: [okf]\n---\n\n# " + titulo(nome) + "\n\n<conteúdo>\n"
	case cx == "assets":
		return "<conteúdo fixo que a resposta pode usar — referencie como `" + rel + "`>\n"
	}
	return ""
}

func extDe(nome string) string {
	if i := strings.LastIndex(nome, "."); i >= 0 {
		return nome[i:]
	}
	return ""
}

func titulo(slug string) string {
	s := strings.NewReplacer("-", " ", "_", " ").Replace(slug)
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}
