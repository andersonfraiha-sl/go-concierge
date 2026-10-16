package okf

import (
	"fmt"
	"strings"
)

// Arquivos-modelo: o que a interface cria quando o usuário marca uma caixinha no
// cadastro ou pede "novo arquivo" dentro dela. Adaptados dos templates do
// grill-agente (references/templates.md); o texto entre <> é para substituir.

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
			"rules/politicas.md": "# Políticas\n\n- <a fronteira principal: o que ele NÃO faz / não decide>\n- Só afirme conteúdo de dados depois de ler pela tool; cite o arquivo fonte.\n- Se a resposta não está nas fontes, diga que não está — não improvise.\n- Nunca use termos internos (caso de uso, recurso, plano, catálogo, sistema, modelo).\n",
			"rules/output.md":    "# Contrato de saída\n\n## Resposta\n(até 4 linhas, direto ao ponto, em português próprio para WhatsApp)\n\n## Fonte\n(o arquivo consultado e/ou a tool usada)\n",
		}
	case "workflows":
		if n.Tipo == Orquestrador {
			return map[string]string{
				"workflows/principal.md": fmt.Sprintf("# Workflow — %s\n\nVocê delega: o roteamento (routers/rotas.md) escolhe o subagente antes de você.\nSe a mensagem chegou até você, nenhuma rota servia:\n\n1. Diga, em uma frase, o que você consegue atender (sem citar rotas ou estrutura interna).\n2. Se o pedido mistura dois assuntos, peça para o usuário dizer qual vem primeiro.\n", nome),
			}
		}
		return map[string]string{
			"workflows/principal.md": fmt.Sprintf("# Workflow — %s\n\n1. Leia `data/<arquivo>.md` (ler_dado) para <o que esse arquivo resolve>.\n2. <Ação com a tool nominal: buscar_rag / chamar_api / rodar_script / mcp_x_y>.\n3. <Cruzamento/decisão: o que fazer com o que voltou>.\n4. Responda no contrato de saída, citando a fonte.\n", nome),
		}
	case "routers":
		return map[string]string{
			"routers/rotas.md":         "# Roteamento\n\nEscolha a rota pela linha que melhor descreve a mensagem.\n\n- Qualquer outra coisa → rota `self` (responda você mesmo).\n",
			"routers/desambiguacao.md": "# Desambiguação\n\n- Se a mensagem junta duas intenções, escolha a do **entregável final**.\n- Na dúvida real entre dois destinos, prefira `self` e peça a precisão que falta.\n",
		}
	case "evals":
		return map[string]string{
			"evals/casos.yaml": "- nome: <o defeito que este caso impede de voltar>\n  pergunta: \"<pergunta típica do usuário>\"\n  # rota: <subagente esperado>        # barato: só a decisão de rota\n  contem: [\"<trecho obrigatório>\"]\n  nao_contem: [\"<o erro real que você viu>\"]\n",
		}
	case "hooks":
		return map[string]string{
			"hooks/registrar.yaml": "nome: registrar\nevento: apos_resposta        # apos_rota | apos_tool | apos_resposta | ao_falhar\nquando:\n  contem: \"<gatilho no texto>\"\nacao:\n  tipo: script               # script | bash | api | agente\n  nome: <arquivo-ou-recurso>\n  argumentos: \"<args com {resposta}>\"\n",
		}
	case "data":
		return map[string]string{
			"data/okf/glossary.yaml":     fmt.Sprintf("# Glossário OKF — UMA definição por termo, com sinônimos: o roteador reconhece\n# a intenção por eles e a projeção leva os termos ao prompt do agente.\nterms:\n  - id: <termo>\n    display_name: <Termo>\n    synonyms: [<sinônimo 1>, <sinônimo 2>]\n    definition: <o que é, na linguagem do domínio de %s>\n    labels: [<rótulo>]\n", nome),
			"data/okf/domain/dominio.md": fmt.Sprintf("---\nid: okf-%s-dominio\ntype: Domain\ntitle: Domínio — %s\nversion: 1.0.0\nstatus: ativo\ndescription: %s\ntags: [okf, dominio]\n---\n\n# Domínio\n\n<os conceitos, invariantes e limites do domínio deste agente>\n", nome, nome, desc),
		}
	case "assets":
		return map[string]string{
			"assets/rodape.md": "<trecho fixo que a resposta pode incluir no fim — referencie como `assets/rodape.md`>\n",
		}
	case "schemas":
		return map[string]string{
			"schemas/dados.yaml": "# Estrutura dos dados (tabelas, colunas, domínios) → tool consultar_schema\ntabelas:\n  <tabela>:\n    descricao: <o que responde>\n    colunas:\n      <coluna>: {tipo: <tipo>, descricao: <significado, unidade>}\n",
		}
	case "mcps":
		return map[string]string{
			"mcps/servidor.yaml": "nome: <servidor>\ntransporte: stdio          # ou http\ncomando: ${PYTHON}         # placeholders: ${PYTHON}, ${PASTA}\nargs: [\"${PASTA}/mcps/<servidor_local>.py\"]   # SEMPRE dentro da própria pasta\n# http:\n# url: https://…/mcp\n# header_env: {Authorization: MINHA_VAR}\n",
		}
	case "bash":
		return map[string]string{
			"bash/python.md": "---\ncomando: python\ndescricao: cálculos rápidos quando a skill não cobrir\n---\n\n# python\n\nUse `rodar_bash(nome=\"python\", argumentos='-c \"print(...)\"')` só para aritmética simples.\n",
		}
	case "scripts":
		return map[string]string{
			"scripts/exemplo.sh": "#!/bin/sh\n# <primeira linha de comentário vira o resumo do script no prompt>\necho \"<saída legível — é isso que o modelo recebe>\"\n",
		}
	case "skills":
		return map[string]string{
			"skills/README.md": "---\ndescription: funções desta pasta que viram tools (nome, parâmetros, quando usar)\n---\n\n# Skills\n\n- `<nome_da_tool>(<arg>)` — <o que faz e quando usar>\n",
		}
	case "rag":
		return map[string]string{
			"rag/README.md": "---\ndescription: documentos para busca por semelhança (buscar_rag); títulos ## por assunto, parágrafos autocontidos\n---\n\n## <assunto>\n\n<parágrafo autocontido>\n",
		}
	}
	return nil
}

// ModeloDeArquivo: conteúdo inicial para um arquivo novo criado pela interface,
// pela caixinha e pelo nome escolhido.
func ModeloDeArquivo(rel string) string {
	cx, resto, _ := strings.Cut(rel, "/")
	nome := strings.TrimSuffix(resto, extDe(resto))
	switch {
	case rel == "rules/output.md":
		return "# Contrato de saída\n\n## Resposta\n(até 4 linhas)\n\n## Fonte\n(arquivo e/ou tool)\n"
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
	case cx == "hooks":
		return "nome: " + nome + "\nevento: apos_resposta\nquando:\n  contem: \"<gatilho>\"\nacao:\n  tipo: script\n  nome: <arquivo>\n"
	case strings.HasPrefix(rel, "data/okf/questions/"):
		return "---\ntype: Question\ntitle: Pergunta — " + titulo(nome) + "\nquestion: " + nome + "\nanswers: [<RESPOSTA_A>, <RESPOSTA_B>, NONE]\n---\n\n## Answers (o espaço de leituras)\n\n<o que cada resposta significa>\n\n## Method\n\n1. <passo>\n\n## Tie-breaking\n\n- <como desempatar>\n"
	case strings.HasPrefix(rel, "data/okf/conditions/"):
		return "---\ntype: Condition\ntitle: " + titulo(nome) + "\n---\n\n## <estado 1>\n\n<quando vale e o que impede de concluir>\n"
	case strings.HasPrefix(rel, "data/okf/policies/"):
		return "---\ntype: Policy\ntitle: " + titulo(nome) + "\n---\n\n- <regra>\n"
	case strings.HasPrefix(rel, "data/okf/") && strings.HasSuffix(rel, "glossary.yaml"):
		return "terms:\n  - id: <termo>\n    display_name: <Termo>\n    synonyms: []\n    definition: <definição>\n    labels: []\n"
	case strings.HasPrefix(rel, "data/okf/") || strings.HasPrefix(rel, "okf/"):
		return "---\nid: okf-" + nome + "\ntype: Domain\ntitle: " + titulo(nome) + "\nversion: 1.0.0\nstatus: ativo\ndescription: <uma linha — é isso que aparece no índice do prompt>\ntags: [okf]\n---\n\n# " + titulo(nome) + "\n\n<conteúdo>\n"
	case cx == "data" || cx == "rag":
		return "---\ntitle: " + titulo(nome) + "\ndescription: <uma linha — é isso que aparece no índice do prompt>\n---\n\n# " + titulo(nome) + "\n\n<conteúdo>\n"
	case cx == "mcps":
		return "nome: " + nome + "\ntransporte: stdio\ncomando: ${PYTHON}\nargs: [\"${PASTA}/mcps/" + nome + ".py\"]\n"
	case cx == "bash":
		return "---\ncomando: " + nome + "\ndescricao: <quando usar — e quando NÃO usar>\n---\n\n# " + nome + "\n"
	case cx == "scripts":
		return "# <primeira linha vira o resumo do script no prompt>\n"
	case cx == "schemas":
		return "# <estrutura dos dados>\n"
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
