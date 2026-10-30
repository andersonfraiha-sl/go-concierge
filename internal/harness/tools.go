package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"go-concierge/internal/mcp"
	"go-concierge/internal/okf"
)

// As quatro tools nativas do formato. O que é DO agente está na pasta (ler_dado
// sobre knowledge/ e assets/, abrir_workflow sobre workflows/, invocar_agente sobre
// agentes/); o que é infraestrutura vem do Nexus (chamar_recurso sobre os recursos
// que o agent.yaml declara). Erro de uso nunca é erro Go: volta como {"error": ...}
// para o modelo corrigir os argumentos (regras gerais da projeção); só falha de
// infraestrutura derruba o loop.

type saida = map[string]any

var toolsMCPObrigatorias = map[string]bool{
	"unidades.listar": true,
	"gruvi.bff.building-operation.get-company-by-document-controller-handle":   true,
	"gruvi.bff.building-operation.fetch-unit-packages-controller-handle":       true,
	"gruvi.bff.building-operation.get-package-delivery-code-controller-handle": true,
	"gruvi.bff.building-operation.fetch-visit-reasons-controller-handle":       true,
	"gruvi.bff.building-operation.create-visit-controller-handle":              true,
}

var toolsMCPComContextoB2C = map[string]bool{
	"gruvi.bff.building-operation.get-company-by-document-controller-handle":   true,
	"gruvi.bff.building-operation.fetch-unit-packages-controller-handle":       true,
	"gruvi.bff.building-operation.get-package-delivery-code-controller-handle": true,
	"gruvi.bff.building-operation.fetch-visit-reasons-controller-handle":       true,
	"gruvi.bff.building-operation.create-visit-controller-handle":              true,
}

func nomeToolMCP(nome string) (string, bool) {
	if nome == "unidades.listar" {
		return "gruvi.bff.building-operation.get-company-by-document-controller-handle", true
	}
	return nome, false
}

func fixarContextoB2C(nome string, params, contexto map[string]any) error {
	if !toolsMCPComContextoB2C[nome] {
		return nil
	}
	companyID, ok := contexto["companyId"]
	if !ok || strings.TrimSpace(fmt.Sprint(companyID)) == "" {
		return fmt.Errorf("companyId B2C ausente")
	}
	params["companyId"] = companyID
	switch nome {
	case "gruvi.bff.building-operation.get-company-by-document-controller-handle",
		"gruvi.bff.building-operation.fetch-unit-packages-controller-handle",
		"gruvi.bff.building-operation.get-package-delivery-code-controller-handle":
		telefone, existe := contexto["filter_mobile_number"]
		if !existe || strings.TrimSpace(fmt.Sprint(telefone)) == "" {
			return fmt.Errorf("telefone B2C ausente")
		}
		params["filter_mobile_number"] = telefone
		if filtro, ok := params["filter"].(map[string]any); ok {
			filtro["mobile-number"] = telefone
		}
	case "gruvi.bff.building-operation.create-visit-controller-handle":
		residentContext, ok := contexto["resident"].(map[string]any)
		telefone, existe := residentContext["mobile_number"]
		if !ok || !existe || strings.TrimSpace(fmt.Sprint(telefone)) == "" {
			return fmt.Errorf("resident.mobile_number B2C ausente")
		}
		resident, ok := params["resident"].(map[string]any)
		if !ok {
			if _, existe := params["resident"]; existe {
				return fmt.Errorf("resident deve ser objeto")
			}
			resident = map[string]any{}
		}
		resident["mobile_number"] = telefone
		params["resident"] = resident
	}
	return nil
}

func (ag *Agente) recursoMCP(nome string) *okf.Recurso {
	for _, r := range ag.Cat.RecursosNexus() {
		if r.Name == nome && r.Origem == okf.OrigemMCP {
			r := r
			return &r
		}
	}
	return nil
}

func erro(formato string, args ...any) saida {
	return saida{"error": fmt.Sprintf(formato, args...)}
}

func (ag *Agente) toolsNativas() ([]tool.BaseTool, error) {
	var tools []tool.BaseTool
	add := func(t tool.BaseTool, err error) error {
		if err != nil {
			return err
		}
		tools = append(tools, t)
		return nil
	}
	a := ag.Def

	if len(ag.Cat.Indice(a)) > 0 {
		if err := add(utils.InferTool("ler_dado",
			"Lê um arquivo de conhecimento ou asset do agente, em páginas de "+fmt.Sprint(PaginaChars)+" caracteres. Quando tem_mais=true, chame de novo com pagina+1. Um index.md lista a pasta dele; os links [x](knowledge/...) de um documento são caminhos que você pode passar aqui.",
			ag.lerDado)); err != nil {
			return nil, err
		}
	}
	if len(ag.Cat.Procedimentos(a)) > 0 {
		if err := add(utils.InferTool("abrir_workflow",
			"Abre um procedimento da pasta (workflows/) para seguir; nos multi-etapa, abre uma etapa por vez.",
			ag.abrirWorkflow)); err != nil {
			return nil, err
		}
	}
	if a.Tipo == okf.Operador && len(ag.Subagentes) > 0 {
		if err := add(utils.InferTool("invocar_agente",
			"Delega uma subtarefa a um subagente da pasta e devolve a resposta dele; você continua dono da resposta final.",
			ag.invocarAgente)); err != nil {
			return nil, err
		}
	}
	if err := add(utils.InferTool("transferir_atendente",
		"Transfere este protocolo a um atendente HUMANO, que assume a conversa daqui em diante. Use quando não há como resolver: pedido fora do que você e seus subagentes fazem, dado que nenhum recurso traz, erro repetido, morador pedindo gente, reclamação séria. Depois de chamar, responda em uma frase dizendo que um atendente vai assumir e pare.",
		ag.transferirAtendente)); err != nil {
		return nil, err
	}
	if len(ag.Cat.RecursosDoAgente(a)) > 0 {
		desc := "Chama um recurso do Nexus declarado pelo agente, com os params do manual. Um recurso de ESCRITA só roda se for passo do caso.yaml deste agente e a mensagem atual do morador for a confirmação explícita (sim); antes disso, pergunte a confirmação em uma frase e pare."
		if perm := ag.recursosPermitidos(); len(perm) > 0 {
			desc += " Permitidos agora: " + strings.Join(perm, ", ") + "."
		} else {
			desc += " Nenhum recurso está permitido para esta identidade."
		}
		if err := add(utils.InferTool("chamar_recurso", desc, ag.chamarRecurso)); err != nil {
			return nil, err
		}
	}
	return tools, nil
}

// ---- ler_dado -------------------------------------------------------------

type argsLerDado struct {
	Caminho string `json:"caminho" jsonschema:"required" jsonschema_description:"caminho exatamente como no índice (ex.: knowledge/regimento.md, assets/rodape.md)"`
	Pagina  int    `json:"pagina" jsonschema_description:"página a partir de 1 (padrão 1)"`
}

func (ag *Agente) lerDado(_ context.Context, in *argsLerDado) (saida, error) {
	raw, err := ag.Cat.LerDado(ag.Def, strings.TrimSpace(in.Caminho))
	if err != nil {
		return erro("%v", err), nil
	}
	return paginar(in.Caminho, string(raw), in.Pagina), nil
}

// paginar corta o texto em páginas de PaginaChars caracteres (runas).
func paginar(caminho, texto string, pagina int) saida {
	if pagina <= 0 {
		pagina = 1
	}
	r := []rune(texto)
	paginas := (len(r) + PaginaChars - 1) / PaginaChars
	if paginas == 0 {
		paginas = 1
	}
	if pagina > paginas {
		return erro("página %d não existe: %s tem %d página(s)", pagina, caminho, paginas)
	}
	ini := (pagina - 1) * PaginaChars
	fim := ini + PaginaChars
	if fim > len(r) {
		fim = len(r)
	}
	return saida{
		"caminho":  caminho,
		"pagina":   pagina,
		"paginas":  paginas,
		"tem_mais": pagina < paginas,
		"conteudo": string(r[ini:fim]),
	}
}

var palavrasDeSim = map[string]bool{"sim": true, "s": true, "ss": true, "ok": true, "okay": true, "pode": true, "confirmo": true, "confirma": true, "confirmado": true, "confirmar": true, "isso": true, "claro": true, "positivo": true, "beleza": true, "bora": true, "fechado": true, "vai": true, "vamos": true, "certo": true, "correto": true, "exato": true, "yes": true, "manda": true, "aceito": true}

// Confirmado diz se a mensagem do morador é um sim explícito: começa com (ou é) uma
// palavra de confirmação e não contém negação.
func Confirmado(mensagem string) bool {
	// tokens curtos contam aqui ("sim", "ok"): não usa palavras(), que exige 4 letras
	ps := strings.FieldsFunc(normalizar(mensagem), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	if len(ps) == 0 {
		return false
	}
	for _, p := range ps {
		if p == "nao" || p == "não" || p == "nunca" || p == "cancela" || p == "cancelar" || p == "espera" || p == "ainda" {
			return false
		}
	}
	if palavrasDeSim[ps[0]] {
		return true
	}
	for _, p := range ps {
		if p == "sim" || p == "confirmo" || p == "confirmado" {
			return true
		}
	}
	return false
}

// amostraDe: o retorno do mock. Leitura devolve a amostra do catálogo (ou lista vazia);
// escrita devolve a amostra ou um protocolo gerado, ecoando os params recebidos.
func amostraDe(r okf.Recurso, params map[string]any, sessao string) any {
	if r.Amostra != nil {
		return r.Amostra
	}
	if r.Write {
		h := fnv.New32a()
		h.Write([]byte(sessao + "|" + r.Name + "|" + fmt.Sprint(params)))
		out := map[string]any{"protocolo": fmt.Sprintf("MOCK-%06d", h.Sum32()%1000000), "status": "registrado", "params": params}
		return out
	}
	return []any{}
}

// ---- transferir_atendente ---------------------------------------------------

type argsTransbordo struct {
	Motivo string `json:"motivo" jsonschema:"required" jsonschema_description:"em uma frase, por que o concierge não resolve (vai para o atendente)"`
}

func (ag *Agente) transferirAtendente(ctx context.Context, in *argsTransbordo) (saida, error) {
	motivo := strings.TrimSpace(in.Motivo)
	if motivo == "" {
		motivo = "o concierge não conseguiu resolver"
	}
	if col := coletorDe(ctx); col != nil && col.transbordo != nil {
		*col.transbordo = motivo
		col.passo(ag.Def.Rel, "transbordo", motivo)
	}
	return saida{"transferido": true, "motivo": motivo, "instrucao": "avise o morador em uma frase que um atendente humano vai assumir este protocolo e pare"}, nil
}

// ---- abrir_workflow -------------------------------------------------------

type argsWorkflow struct {
	Nome  string `json:"nome" jsonschema:"required" jsonschema_description:"nome do procedimento, como listado na projeção"`
	Etapa string `json:"etapa" jsonschema_description:"etapa de um procedimento multi-etapa (padrão: a primeira)"`
}

func (ag *Agente) abrirWorkflow(_ context.Context, in *argsWorkflow) (saida, error) {
	procs := ag.Cat.Procedimentos(ag.Def)
	var nomes []string
	for _, p := range procs {
		nomes = append(nomes, p.Nome)
		if p.Nome != strings.TrimSpace(in.Nome) {
			continue
		}
		base := filepath.Join(ag.Def.Path, "workflows")
		if len(p.Etapas) == 0 {
			raw, err := os.ReadFile(filepath.Join(base, p.Nome+".md"))
			if err != nil {
				return erro("não consegui ler o procedimento %s: %v", p.Nome, err), nil
			}
			return saida{"nome": p.Nome, "conteudo": string(raw)}, nil
		}
		etapa := strings.TrimSpace(in.Etapa)
		if etapa == "" {
			etapa = p.Etapas[0]
		}
		idx := -1
		for i, e := range p.Etapas {
			if e == etapa || strings.TrimSuffix(e, ".md") == strings.TrimSuffix(etapa, ".md") {
				idx = i
				etapa = e
			}
		}
		if idx == -1 {
			return erro("etapa %q não existe em %s; etapas: %s", etapa, p.Nome, strings.Join(p.Etapas, ", ")), nil
		}
		raw, err := os.ReadFile(filepath.Join(base, p.Nome, etapa+".md"))
		if err != nil {
			return erro("não consegui ler a etapa %s de %s: %v", etapa, p.Nome, err), nil
		}
		out := saida{"nome": p.Nome, "etapa": etapa, "etapas": p.Etapas, "conteudo": string(raw)}
		if idx+1 < len(p.Etapas) {
			out["proxima"] = p.Etapas[idx+1]
		}
		return out, nil
	}
	return erro("procedimento %q não existe; disponíveis: %s", in.Nome, strings.Join(nomes, ", ")), nil
}

// ---- invocar_agente -------------------------------------------------------

type argsAgente struct {
	Nome     string `json:"nome" jsonschema:"required" jsonschema_description:"nome do subagente, como listado na projeção"`
	Mensagem string `json:"mensagem" jsonschema:"required" jsonschema_description:"a subtarefa, completa e autocontida (o subagente não vê sua conversa)"`
}

func (ag *Agente) invocarAgente(ctx context.Context, in *argsAgente) (saida, error) {
	sub, ok := ag.Subagentes[strings.TrimSpace(in.Nome)]
	if !ok {
		return erro("subagente %q não existe; disponíveis: %s", in.Nome, strings.Join(ag.nomesDeRotas(), ", ")), nil
	}
	prof := 0
	pai := coletorDe(ctx)
	if pai != nil {
		prof = pai.profundidade
	}
	if prof+1 >= ProfundidadeMax {
		return erro("profundidade máxima (%d) de delegação atingida; resolva você mesmo", ProfundidadeMax), nil
	}
	r, err := run(ctx, sub, in.Mensagem, nil, prof+1)
	if err != nil {
		return erro("subagente %s falhou: %v", sub.Def.Nome, err), nil
	}
	if pai != nil {
		pai.absorve(r)
	}
	return saida{"agente": sub.Def.Nome, "resposta": r.Resposta}, nil
}

// ---- chamar_recurso -------------------------------------------------------

type argsRecurso struct {
	Nome   string         `json:"nome" jsonschema:"required" jsonschema_description:"nome do recurso, exatamente como declarado (ex.: faq.buscar)"`
	Params map[string]any `json:"params" jsonschema_description:"parâmetros do recurso, pelo manual da projeção"`
}

type chaveExecucaoCaso struct{}

// Permissao é o veredito do harness sobre um recurso declarado para esta identidade.
type Permissao struct {
	okf.RecursoDeclarado
	Permitido bool   `json:"permitido"`
	Motivo    string `json:"motivo,omitempty"` // por que NÃO está permitido
}

// Permissoes cruza as declarações do agente (RecursosDoAgente) com a identidade da
// Config: escrita só quando é passo do caso.yaml do próprio agente (e, na chamada,
// só depois do sim do morador); private só com Authenticated; declaração com
// problema no Nexus fica bloqueada com o problema como motivo.
func (ag *Agente) Permissoes() []Permissao {
	var out []Permissao
	for _, d := range ag.Cat.RecursosDoAgente(ag.Def) {
		p := Permissao{RecursoDeclarado: d}
		switch {
		case d.Recurso != nil && d.Recurso.Write && (d.Origem != "caso.yaml" || ag.Def.Caso == nil):
			p.Motivo = "escrita só por passo do caso de uso"
		case d.Recurso != nil && d.Recurso.Write && !ag.cfg.Identidade.Authenticated:
			p.Motivo = "escrita só para morador identificado"
		case !d.OK:
			p.Motivo = d.Problema
		case d.Recurso != nil && d.Recurso.Visibility == "private" && !ag.cfg.Identidade.Authenticated:
			p.Motivo = "privado: só para morador identificado"
		default:
			p.Permitido = true
		}
		out = append(out, p)
	}
	return out
}

func (ag *Agente) recursosPermitidos() []string {
	var out []string
	vistos := map[string]bool{}
	for _, p := range ag.Permissoes() {
		if p.Permitido && !vistos[p.Nome] {
			vistos[p.Nome] = true
			out = append(out, p.Nome)
		}
	}
	sort.Strings(out)
	return out
}

func (ag *Agente) chamarRecurso(ctx context.Context, in *argsRecurso) (saida, error) {
	msg := ""
	if col := coletorDe(ctx); col != nil {
		msg = col.mensagem
	}
	return ag.invocar(ctx, in, msg)
}

func (ag *Agente) InvocarRecurso(ctx context.Context, nome string, params map[string]any, confirmacao string) (map[string]any, error) {
	return ag.invocar(ctx, &argsRecurso{Nome: nome, Params: params}, confirmacao)
}

func (ag *Agente) ExecutarPassoCaso(ctx context.Context, nome string, params map[string]any, confirmacao string) (map[string]any, error) {
	ctx = context.WithValue(ctx, chaveExecucaoCaso{}, true)
	return ag.invocar(ctx, &argsRecurso{Nome: nome, Params: params}, confirmacao)
}

func (ag *Agente) invocar(ctx context.Context, in *argsRecurso, confirmacao string) (saida, error) {
	nome := strings.TrimSpace(in.Nome)
	nomeMCP, aliasMCP := nomeToolMCP(nome)
	chamarMCP := toolsMCPObrigatorias[nome]
	var achado *Permissao
	for _, p := range ag.Permissoes() {
		if p.Nome != nome {
			continue
		}
		p := p
		if achado == nil || (p.Permitido && !achado.Permitido) {
			achado = &p
		}
	}
	if achado == nil {
		if _, eRota := ag.Subagentes[nome]; eRota {
			return erro("%q é um subagente (rota), não um recurso: não execute o caso você mesmo; peça ao morador o pedido de forma direta para a rota levar ao especialista, ou responda só o que você sabe", nome), nil
		}
		perm := ag.recursosPermitidos()
		if len(perm) == 0 {
			return erro("recurso %q não está declarado por este agente e nenhum recurso está permitido agora", nome), nil
		}
		return erro("recurso %q não está declarado por este agente; permitidos: %s", nome, strings.Join(perm, ", ")), nil
	}
	if !achado.Permitido {
		return erro("recurso %s recusado: %s", nome, achado.Motivo), nil
	}
	if chamarMCP && ag.cfg.MCP == nil {
		return erro("tool MCP %s indisponível; o fluxo não pode usar dados simulados", nome), nil
	}
	// escrita: a guarda determinística — só com o "sim" do morador na mensagem atual
	if achado.Recurso != nil && achado.Recurso.Write {
		if nomeMCP == "gruvi.bff.building-operation.create-visit-controller-handle" && ctx.Value(chaveExecucaoCaso{}) != true {
			return erro("%s só pode ser executada pelo motor determinístico do caso de liberação de visitante", nome), nil
		}
		if !Confirmado(confirmacao) {
			return erro("%s é escrita e exige a confirmação explícita do morador: a mensagem atual (%q) não é um sim. Resuma o que vai registrar, pergunte \"confirma? (sim/não)\" e pare", nome, corta(confirmacao, 60)), nil
		}
	}

	// recurso resolvido pelo canal (identidade do morador): não depende do Nexus
	if !chamarMCP {
		if local, ok := ag.cfg.Locais[nome]; ok && local != nil {
			params := in.Params
			if params == nil {
				params = map[string]any{}
			}
			for k, v := range ag.cfg.Payloads[nome] {
				if _, tem := params[k]; !tem {
					params[k] = v
				}
			}
			res, err := local(ctx, params)
			if err != nil {
				return erro("falha ao resolver %s: %v", nome, err), nil
			}
			return saida{"recurso": nome, "resultado": res, "origem": "identidade"}, nil
		}
	}
	if chamarMCP || achado.Recurso != nil && achado.Recurso.Origem == okf.OrigemMCP {
		if ag.cfg.MCP == nil {
			return erro("catálogo MCP indisponível: não há como chamar %s agora; avise o morador", nome), nil
		}
		recurso := achado.Recurso
		if chamarMCP && (aliasMCP || recurso == nil || recurso.Origem != okf.OrigemMCP) {
			recurso = ag.recursoMCP(nomeMCP)
		}
		if recurso == nil || recurso.Origem != okf.OrigemMCP {
			return erro("tool MCP %s ausente no catálogo; o fluxo não pode usar dados simulados", nomeMCP), nil
		}
		params := in.Params
		if params == nil {
			params = map[string]any{}
		}
		for k, v := range ag.cfg.Payloads[nomeMCP] {
			if _, tem := params[k]; !tem {
				params[k] = v
			}
		}
		for k, v := range ag.cfg.Payloads[nome] {
			if _, tem := params[k]; !tem {
				params[k] = v
			}
		}
		contextoMCP := map[string]any{}
		for k, v := range ag.cfg.Payloads[nome] {
			contextoMCP[k] = v
		}
		for k, v := range ag.cfg.Payloads[nomeMCP] {
			contextoMCP[k] = v
		}
		if err := fixarContextoB2C(nomeMCP, params, contextoMCP); err != nil {
			return erro("contexto B2C inválido para %s: %v", nome, err), nil
		}
		if faltam := faltamParams(recurso, params); len(faltam) > 0 {
			return erro("faltam params obrigatórios de %s: %s", nome, strings.Join(faltam, ", ")), nil
		}
		if err := ajustarParamsSchema(recurso, params); err != nil {
			return erro("params inválidos para %s: %v", nome, err), nil
		}
		if resultado, ok := ag.cfg.MCPResultados[nomeMCP]; ok {
			return saida{"recurso": nome, "resultado": resultado, "origem": "mcp"}, nil
		}
		if nomeMCP == "gruvi.bff.building-operation.create-visit-controller-handle" {
			normalizarTelefonesVisitantes(params)
		}
		res, err := ag.cfg.MCP.CallTool(ctx, nomeMCP, params)
		if err != nil {
			if nomeMCP == "gruvi.bff.building-operation.create-visit-controller-handle" {
				return erro("resultado da escrita %s incerto após o envio; não repita automaticamente e verifique o estado antes de tentar de novo: %v", nome, err), nil
			}
			return erro("falha ao chamar %s: %v", nomeMCP, err), nil
		}
		if res != nil && res.IsError {
			return erro("%s recusou: %s", nomeMCP, corta(res.Texto(), 400)), nil
		}
		resultado, err := valorMCP(nomeMCP, res)
		if err != nil {
			var incerto *resultadoMCPIncerto
			if achado.Recurso != nil && achado.Recurso.Write && errors.As(err, &incerto) {
				return erro("resultado da escrita %s incerto após o envio; não repita automaticamente e verifique o estado antes de tentar de novo: %v", nome, err), nil
			}
			return erro("falha funcional ao chamar %s: %v", nomeMCP, err), nil
		}
		return saida{"recurso": nome, "resultado": resultado, "origem": "mcp"}, nil
	}
	cli := ag.cfg.Nexus
	if cli == nil || cli.BaseURL == "" {
		// sem Nexus no ar: o mock do concierge responde pela amostra do catálogo
		if achado.Recurso == nil {
			return erro("Nexus indisponível: não há como chamar %s agora; avise o morador", nome), nil
		}
		params := in.Params
		if params == nil {
			params = map[string]any{}
		}
		for k, v := range ag.cfg.Payloads[nome] {
			if _, tem := params[k]; !tem {
				params[k] = v
			}
		}
		return saida{"recurso": nome, "resultado": amostraDe(*achado.Recurso, params, ag.cfg.sessao()), "origem": "mock"}, nil
	}
	if achado.Recurso != nil {
		var faltam []string
		for _, pm := range achado.Recurso.Params {
			if pm.Required {
				v, ok := in.Params[pm.Name]
				if !ok || v == nil || fmt.Sprint(v) == "" {
					if pv, okp := ag.cfg.Payloads[nome][pm.Name]; okp && pv != nil && fmt.Sprint(pv) != "" {
						continue // o payload do condomínio preenche
					}
					faltam = append(faltam, pm.Name)
				}
			}
		}
		if len(faltam) > 0 {
			return erro("faltam params obrigatórios de %s: %s", nome, strings.Join(faltam, ", ")), nil
		}
	}
	params := in.Params
	if params == nil {
		params = map[string]any{}
	}
	// payload do condomínio (Admin): remontado na chamada como parâmetros padrão
	for k, v := range ag.cfg.Payloads[nome] {
		if _, ok := params[k]; !ok {
			params[k] = v
		}
	}
	n := 1
	if col := coletorDe(ctx); col != nil {
		n = col.proximaInvocacao()
	}
	idem := fmt.Sprintf("%s:%s:%s:%d", ag.cfg.sessao(), ag.Def.Nome, nome, n)
	raw, err := cli.Invoke(ctx, nome, ag.cfg.account(), params, idem)
	if err != nil {
		return erro("falha ao chamar %s: %v", nome, err), nil
	}
	var resultado any
	if json.Unmarshal(raw, &resultado) != nil {
		resultado = string(raw)
	}
	return saida{"recurso": nome, "resultado": resultado}, nil
}

func faltamParams(r *okf.Recurso, params map[string]any) []string {
	if r == nil {
		return nil
	}
	var faltam []string
	for _, p := range r.Params {
		if !p.Required {
			continue
		}
		v, ok := params[p.Name]
		if !ok || v == nil || strings.TrimSpace(fmt.Sprint(v)) == "" {
			faltam = append(faltam, p.Name)
		}
	}
	return faltam
}

func ajustarParamsSchema(r *okf.Recurso, params map[string]any) error {
	if r == nil {
		return nil
	}
	for _, p := range r.Params {
		v, ok := params[p.Name]
		if !ok || v == nil {
			continue
		}
		switch p.Type {
		case "string":
			switch n := v.(type) {
			case string:
				params[p.Name] = strings.TrimSpace(n)
			case int:
				params[p.Name] = strconv.Itoa(n)
			case int64:
				params[p.Name] = strconv.FormatInt(n, 10)
			case float64:
				if math.Trunc(n) != n || math.Abs(n) > 9007199254740991 {
					return fmt.Errorf("%s deve ser texto ou inteiro seguro", p.Name)
				}
				params[p.Name] = strconv.FormatFloat(n, 'f', 0, 64)
			default:
				return fmt.Errorf("%s deve ser texto", p.Name)
			}
		case "number":
			if n, ok := v.(string); ok {
				num, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
				if err != nil || math.IsNaN(num) || math.IsInf(num, 0) {
					return fmt.Errorf("%s deve ser número", p.Name)
				}
				params[p.Name] = num
			}
		case "object":
			if _, ok := v.(map[string]any); !ok {
				return fmt.Errorf("%s deve ser objeto", p.Name)
			}
		case "array":
			switch v.(type) {
			case []any, []map[string]any:
			default:
				return fmt.Errorf("%s deve ser lista", p.Name)
			}
		}
	}
	return nil
}

func normalizarTelefonesVisitantes(params map[string]any) {
	var visitors []any
	switch v := params["visitors"].(type) {
	case []any:
		visitors = v
	case []map[string]any:
		for _, item := range v {
			visitors = append(visitors, item)
		}
	}
	for _, item := range visitors {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		phone, ok := m["mobile_number"].(string)
		if ok {
			m["mobile_number"] = normalizarTelefoneVisitante(phone)
		}
	}
}

func normalizarTelefoneVisitante(phone string) string {
	var digits strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if len(d) <= 11 {
		d = "55" + d
	}
	if len(d) == 12 && strings.HasPrefix(d, "55") {
		d = d[:4] + "9" + d[4:]
	}
	return d
}

type resultadoMCPIncerto struct {
	motivo string
}

func (e *resultadoMCPIncerto) Error() string { return e.motivo }

func valorMCP(nome string, res *mcp.CallResult) (any, error) {
	if res == nil {
		return nil, &resultadoMCPIncerto{motivo: "resposta MCP vazia"}
	}
	if res.IsError {
		return nil, fmt.Errorf("servidor MCP: %s", corta(res.Texto(), 400))
	}
	value, err := normalizarValorMCP(res.Valor())
	if err != nil {
		return nil, err
	}
	if nome == "gruvi.bff.building-operation.create-visit-controller-handle" {
		m, ok := value.(map[string]any)
		if !ok {
			return nil, &resultadoMCPIncerto{motivo: "a resposta não contém a visita criada"}
		}
		visit, ok := m["visit"].(map[string]any)
		if !ok {
			return nil, &resultadoMCPIncerto{motivo: "a resposta não confirma a visita criada"}
		}
		status := strings.ToUpper(strings.TrimSpace(fmt.Sprint(visit["clearance_status"])))
		if status != "AUTHORIZED" && status != "AUTHORIZED_FOR_TODAY" {
			return nil, &resultadoMCPIncerto{motivo: fmt.Sprintf("a resposta não confirma uma liberação autorizada (clearance_status=%q)", status)}
		}
	}
	return value, nil
}

func NormalizarResultadoMCP(nome string, res *mcp.CallResult) (any, error) {
	return valorMCP(nome, res)
}

func normalizarValorMCP(value any) (any, error) {
	if msg := falhaFuncionalMCP(value); msg != "" {
		return nil, fmt.Errorf("%s", corta(msg, 400))
	}
	switch v := value.(type) {
	case string:
		t := strings.TrimSpace(v)
		if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") {
			var decoded any
			if json.Unmarshal([]byte(t), &decoded) == nil {
				return normalizarValorMCP(decoded)
			}
		}
		return v, nil
	case []any:
		if content, ok, err := valorDoConteudoMCP(v); ok || err != nil {
			return content, err
		}
		out := make([]any, 0, len(v))
		for _, item := range v {
			n, err := normalizarValorMCP(item)
			if err != nil {
				return nil, err
			}
			out = append(out, n)
		}
		return out, nil
	case map[string]any:
		if content, ok := v["structuredContent"]; ok {
			return normalizarValorMCP(content)
		}
		if content, ok := v["content"]; ok {
			switch c := content.(type) {
			case []any:
				if result, ok, err := valorDoConteudoMCP(c); ok || err != nil {
					return result, err
				}
			case string:
				return normalizarValorMCP(c)
			}
		}
		if result, ok := v["result"]; ok {
			return normalizarValorMCP(result)
		}
		if data, ok := v["data"]; ok {
			return normalizarValorMCP(data)
		}
		return v, nil
	default:
		return value, nil
	}
}

func valorDoConteudoMCP(content []any) (any, bool, error) {
	var texts []string
	for _, item := range content {
		m, ok := item.(map[string]any)
		if !ok || m["type"] != "text" {
			continue
		}
		if text, ok := m["text"].(string); ok && strings.TrimSpace(text) != "" {
			texts = append(texts, text)
		}
	}
	if len(texts) == 0 {
		return nil, false, nil
	}
	joined := strings.Join(texts, "\n")
	var decoded any
	if json.Unmarshal([]byte(joined), &decoded) == nil {
		value, err := normalizarValorMCP(decoded)
		return value, true, err
	}
	return joined, true, nil
}

func falhaFuncionalMCP(value any) string {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			var decoded any
			if json.Unmarshal([]byte(trimmed), &decoded) == nil {
				return falhaFuncionalMCP(decoded)
			}
		}
	case []any:
		for _, item := range v {
			if msg := falhaFuncionalMCP(item); msg != "" {
				return msg
			}
		}
	case map[string]any:
		if ok, exists := v["ok"].(bool); exists && !ok {
			return "MCP informou ok=false: " + detalheErroMCP(v)
		}
		if ok, exists := v["success"].(bool); exists && !ok {
			return "MCP informou success=false: " + detalheErroMCP(v)
		}
		if isError, exists := v["isError"].(bool); exists && isError {
			return "result.isError=true: " + detalheErroMCP(v)
		}
		if status, exists := numeroMCP(v["statusCode"]); exists && status >= 400 {
			return fmt.Sprintf("BFF HTTP %d: %s", int(status), detalheErroMCP(v))
		}
		if e, exists := v["error"]; exists && erroMCPPresente(e) {
			return "erro do BFF: " + detalheErroMCP(e)
		}
		for _, key := range []string{"result", "data", "content", "structuredContent"} {
			if child, exists := v[key]; exists {
				if msg := falhaFuncionalMCP(child); msg != "" {
					return msg
				}
			}
		}
	}
	return ""
}

func erroMCPPresente(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case bool:
		return v
	case map[string]any:
		return len(v) > 0
	case []any:
		return len(v) > 0
	default:
		return true
	}
}

func detalheErroMCP(value any) string {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{"message", "detail", "code", "reasons", "details", "error", "text"} {
			if child, ok := v[key]; ok && erroMCPPresente(child) {
				return detalheErroMCP(child)
			}
		}
	case []any:
		var parts []string
		for _, child := range v {
			if erroMCPPresente(child) {
				parts = append(parts, detalheErroMCP(child))
			}
		}
		return strings.Join(parts, "; ")
	}
	if value == nil {
		return "erro funcional sem detalhe"
	}
	return fmt.Sprint(value)
}

func numeroMCP(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
