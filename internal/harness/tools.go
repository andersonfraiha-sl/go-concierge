package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/pii"
	"go-concierge/internal/mcp"
	"go-concierge/internal/nexus"
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

const ferramentaCriarVisita = "gruvi.bff.building-operation.create-visit-controller-handle"

func nomeToolMCP(nome string) (string, bool) {
	if nome == "unidades.listar" {
		return "gruvi.bff.building-operation.get-company-by-document-controller-handle", true
	}
	return nome, false
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
		"Registra a intenção de solicitar um atendente HUMANO para este protocolo. Use quando não há como resolver: pedido fora do que você e seus subagentes fazem, dado que nenhum recurso traz, erro repetido, morador pedindo gente, reclamação séria. Esta ferramenta não confirma a transferência; aguarde a confirmação do sistema antes de afirmar que um atendente assumiu.",
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
	return saida{"intencao_registrada": true, "motivo": motivo, "instrucao": "a solicitação foi registrada; não afirme que a transferência ocorreu ou que um atendente assumiu"}, nil
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

// chamada é um pedido de recurso resolvido: o nome que o modelo pediu, a tool MCP por
// trás, a permissão desta identidade e os params já com o payload do condomínio.
type chamada struct {
	nome, nomeMCP            string
	aliasMCP, obrigatoriaMCP bool
	achado                   *Permissao
	// pedidos são os params do chamador com os valores do cofre devolvidos; params
	// somam o payload do condomínio e saem do antes_da_ferramenta.
	pedidos, params map[string]any
}

// invocar é o ponto único de toda chamada a recurso: antes_da_ferramenta decide se roda
// (permissão, confirmação, contexto B2C e regras), depois_da_ferramenta saneia o resultado.
func (ag *Agente) invocar(ctx context.Context, in *argsRecurso, confirmacao string) (saida, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c := ag.resolverChamada(in)
	antes := ag.cfg.Turno.AntesDaFerramenta(ctx, ag.eventoAntes(ctx, c, confirmacao))
	registrarGanchos(ctx, antes.Registros)
	if recusa := recusaDeAntes(antes); recusa != "" {
		return erro("%s", recusa), nil
	}
	c.params = antes.Evento.Params
	out, err := ag.executar(ctx, c)
	if err != nil {
		return nil, err
	}
	if texto, falhou := out["error"].(string); falhou {
		// o texto pode trazer o que o MCP ou a ferramenta devolveu: sai mascarado
		out["error"], _ = ag.cfg.Turno.Cofre().Mascarar(texto)
		return out, nil
	}
	return ag.depoisDe(ctx, c, out), nil
}

func (ag *Agente) resolverChamada(in *argsRecurso) chamada {
	nome := strings.TrimSpace(in.Nome)
	nomeMCP, alias := nomeToolMCP(nome)
	pedidos, _ := pii.DevolverEmValor(in.Params, ag.cfg.Turno.Cofre().Ler()).(map[string]any)
	return chamada{
		nome: nome, nomeMCP: nomeMCP, aliasMCP: alias, obrigatoriaMCP: toolsMCPObrigatorias[nome],
		achado: ag.permissao(nome), pedidos: pedidos,
		// sem alias, nomeMCP == nome: o payload é o mesmo e vale uma vez só
		params: comPadroes(pedidos, ag.cfg.Payloads[nomeMCP], ag.cfg.Payloads[nome]),
	}
}

// permissao é a declaração do recurso para esta identidade; com duas, vale a permitida.
func (ag *Agente) permissao(nome string) *Permissao {
	var achado *Permissao
	for _, p := range ag.Permissoes() {
		if p.Nome != nome {
			continue
		}
		if achado == nil || (p.Permitido && !achado.Permitido) {
			achado = &p
		}
	}
	return achado
}

// comPadroes copia params e completa as chaves ausentes com cada payload, na ordem.
func comPadroes(params map[string]any, payloads ...map[string]any) map[string]any {
	out := maps.Clone(params)
	if out == nil {
		out = map[string]any{}
	}
	for _, payload := range payloads {
		for k, v := range payload {
			if _, tem := out[k]; !tem {
				out[k] = v
			}
		}
	}
	return out
}

func (ag *Agente) eventoAntes(ctx context.Context, c chamada, confirmacao string) ganchos.AntesDaFerramenta {
	_, subagente := ag.Subagentes[c.nome]
	e := ganchos.AntesDaFerramenta{
		Agora: time.Now(), Ferramenta: c.nomeMCP, Pedida: c.nome, Caso: ag.casoID(),
		Params: c.params, ContextoB2C: ag.contextoB2C(c),
		Declarada: c.achado != nil, Subagente: subagente,
		MCPObrigatoria: c.obrigatoriaMCP, MCPDisponivel: ag.cfg.MCP != nil,
		PeloMotorDoCaso: ctx.Value(chaveExecucaoCaso{}) == true,
		Confirmacao:     confirmacao, Identidade: ag.identidadeDosGanchos(),
		AdministradoraID: ag.cfg.AdministradoraID, CondominioID: ag.cfg.CondominioID,
	}
	if c.achado == nil {
		e.Permitidas = ag.recursosPermitidos()
		return e
	}
	e.Permitida, e.MotivoRecusa = c.achado.Permitido, c.achado.Motivo
	e.Escrita = c.achado.Recurso != nil && c.achado.Recurso.Write
	return e
}

// contextoB2C é o payload do pedido com o da tool MCP por cima: a empresa e o telefone
// do morador vêm do canal, nunca do modelo.
func (ag *Agente) contextoB2C(c chamada) map[string]any {
	contexto := map[string]any{}
	maps.Copy(contexto, ag.cfg.Payloads[c.nome])
	maps.Copy(contexto, ag.cfg.Payloads[c.nomeMCP])
	return contexto
}

func (ag *Agente) casoID() string {
	if ag.Def.Caso == nil {
		return ""
	}
	return ag.Def.Caso.ID
}

func (ag *Agente) identidadeDosGanchos() ganchos.Identidade {
	id := ganchos.Identidade{Cadastrado: ag.cfg.Identidade.Authenticated}
	if ag.cfg.Identidade.Role != "" {
		id.Papeis = []string{ag.cfg.Identidade.Role}
	}
	return id
}

// recusaDeAntes é o texto que o agente lê quando antes_da_ferramenta não deixa rodar;
// vazio = roda.
func recusaDeAntes(r ganchos.Resultado[ganchos.AntesDaFerramenta]) string {
	switch r.Decisao.Tipo {
	case ganchos.Bloquear:
		return r.Decisao.Motivo
	case ganchos.Confirmar:
		return strings.Join(r.Perguntas, "\n")
	default:
		return ""
	}
}

func registrarGanchos(ctx context.Context, registros []ganchos.Registro) {
	if col := coletorDe(ctx); col != nil && len(registros) > 0 {
		col.registrar(registros)
	}
}

// depoisDe passa o resultado por depois_da_ferramenta: sai saneado, ou não sai.
func (ag *Agente) depoisDe(ctx context.Context, c chamada, out saida) saida {
	depois := ag.cfg.Turno.DepoisDaFerramenta(ctx, ganchos.DepoisDaFerramenta{
		Agora: time.Now(), Ferramenta: c.nomeMCP, Caso: ag.casoID(), Params: c.params,
		Resultado: out["resultado"], Identidade: ag.identidadeDosGanchos(),
		AdministradoraID: ag.cfg.AdministradoraID, CondominioID: ag.cfg.CondominioID,
	})
	registrarGanchos(ctx, depois.Registros)
	if depois.Decisao.Tipo == ganchos.Bloquear {
		return erro("%s indisponível", c.nome)
	}
	out["resultado"] = depois.Evento.Resultado
	return out
}

// executar roda o recurso que antes_da_ferramenta liberou, pela origem dele.
func (ag *Agente) executar(ctx context.Context, c chamada) (saida, error) {
	if c.achado == nil { // inalcançável: o Build exige ferramenta_permitida, que barra antes
		return erro("recurso %q não está declarado por este agente", c.nome), nil
	}
	// recurso resolvido pelo canal (identidade do morador): não depende do Nexus
	if local := ag.cfg.Locais[c.nome]; local != nil && !c.obrigatoriaMCP {
		return executarLocal(ctx, c, local), nil
	}
	if c.obrigatoriaMCP || c.achado.Recurso != nil && c.achado.Recurso.Origem == okf.OrigemMCP {
		return ag.executarMCP(ctx, c)
	}
	cli := ag.cfg.Nexus
	if cli == nil || cli.BaseURL == "" {
		return ag.executarMock(c), nil
	}
	return ag.executarNexus(ctx, c, cli)
}

func executarLocal(ctx context.Context, c chamada, local func(context.Context, map[string]any) (any, error)) saida {
	res, err := local(ctx, c.params)
	if err != nil {
		return erro("falha ao resolver %s: %v", c.nome, err)
	}
	return saida{"recurso": c.nome, "resultado": res, "origem": "identidade"}
}

func (ag *Agente) executarMCP(ctx context.Context, c chamada) (saida, error) {
	recurso, recusa := ag.recursoMCPDaChamada(c)
	if recusa != nil {
		return recusa, nil
	}
	if faltam := faltamParams(recurso, c.params); len(faltam) > 0 {
		return erro("faltam params obrigatórios de %s: %s", c.nome, strings.Join(faltam, ", ")), nil
	}
	if err := ajustarParamsSchema(recurso, c.params); err != nil {
		return erro("params inválidos para %s: %v", c.nome, err), nil
	}
	if resultado, ok := ag.cfg.MCPResultados[c.nomeMCP]; ok {
		return saida{"recurso": c.nome, "resultado": resultado, "origem": "mcp"}, nil
	}
	if c.nomeMCP == ferramentaCriarVisita {
		normalizarTelefonesVisitantes(c.params)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	res, err := ag.cfg.MCP.CallTool(ctx, c.nomeMCP, c.params)
	return saidaMCP(c, res, err), nil
}

// recursoMCPDaChamada acha a tool MCP do catálogo; sem ela, a saida de erro vem preenchida.
func (ag *Agente) recursoMCPDaChamada(c chamada) (*okf.Recurso, saida) {
	if ag.cfg.MCP == nil {
		return nil, erro("catálogo MCP indisponível: não há como chamar %s agora; avise o morador", c.nome)
	}
	recurso := c.achado.Recurso
	if c.obrigatoriaMCP && (c.aliasMCP || recurso == nil || recurso.Origem != okf.OrigemMCP) {
		recurso = ag.recursoMCP(c.nomeMCP)
	}
	if recurso == nil || recurso.Origem != okf.OrigemMCP {
		return nil, erro("tool MCP %s ausente no catálogo; o fluxo não pode usar dados simulados", c.nomeMCP)
	}
	return recurso, nil
}

func saidaMCP(c chamada, res *mcp.CallResult, err error) saida {
	if err != nil {
		if c.nomeMCP == ferramentaCriarVisita {
			return erro("resultado da escrita %s incerto após o envio; não repita automaticamente e verifique o estado antes de tentar de novo: %v", c.nome, err)
		}
		return erro("falha ao chamar %s: %v", c.nomeMCP, err)
	}
	if res != nil && res.IsError {
		return erro("%s recusou: %s", c.nomeMCP, corta(res.Texto(), 400))
	}
	resultado, err := valorMCP(c.nomeMCP, res)
	if err != nil {
		var incerto *resultadoMCPIncerto
		if c.achado.Recurso != nil && c.achado.Recurso.Write && errors.As(err, &incerto) {
			return erro("resultado da escrita %s incerto após o envio; não repita automaticamente e verifique o estado antes de tentar de novo: %v", c.nome, err)
		}
		return erro("falha funcional ao chamar %s: %v", c.nomeMCP, err)
	}
	return saida{"recurso": c.nome, "resultado": resultado, "origem": "mcp"}
}

// executarMock: sem Nexus no ar, o mock do concierge responde pela amostra do catálogo.
func (ag *Agente) executarMock(c chamada) saida {
	if c.achado.Recurso == nil {
		return erro("Nexus indisponível: não há como chamar %s agora; avise o morador", c.nome)
	}
	return saida{"recurso": c.nome, "resultado": amostraDe(*c.achado.Recurso, c.params, ag.cfg.sessao()), "origem": "mock"}
}

func (ag *Agente) executarNexus(ctx context.Context, c chamada, cli *nexus.Client) (saida, error) {
	if faltam := ag.faltamNoNexus(c); len(faltam) > 0 {
		return erro("faltam params obrigatórios de %s: %s", c.nome, strings.Join(faltam, ", ")), nil
	}
	n := 1
	if col := coletorDe(ctx); col != nil {
		n = col.proximaInvocacao()
	}
	idem := fmt.Sprintf("%s:%s:%s:%d", ag.cfg.sessao(), ag.Def.Nome, c.nome, n)
	account := ag.cfg.Identidade.AccountID
	if account == "" {
		return erro("sem account do condomínio: não há como chamar %s agora; avise o morador", c.nome), nil
	}
	raw, err := cli.Invoke(ctx, c.nome, account, c.params, idem)
	if err != nil {
		return erro("falha ao chamar %s: %v", c.nome, err), nil
	}
	var resultado any
	if json.Unmarshal(raw, &resultado) != nil {
		resultado = string(raw)
	}
	return saida{"recurso": c.nome, "resultado": resultado}, nil
}

// faltamNoNexus lista os obrigatórios vazios no pedido que o payload do condomínio não preenche.
func (ag *Agente) faltamNoNexus(c chamada) []string {
	if c.achado.Recurso == nil {
		return nil
	}
	var faltam []string
	for _, pm := range c.achado.Recurso.Params {
		if pm.Required && vazio(c.pedidos, pm.Name) && vazio(ag.cfg.Payloads[c.nome], pm.Name) {
			faltam = append(faltam, pm.Name)
		}
	}
	return faltam
}

func vazio(params map[string]any, nome string) bool {
	v, ok := params[nome]
	return !ok || v == nil || fmt.Sprint(v) == ""
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
	if nome == ferramentaCriarVisita {
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
