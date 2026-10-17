package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"go-concierge/internal/okf"
)

// As quatro tools nativas do formato. O que é DO agente está na pasta (ler_dado
// sobre knowledge/ e assets/, abrir_workflow sobre workflows/, invocar_agente sobre
// agentes/); o que é infraestrutura vem do Nexus (chamar_recurso sobre os recursos
// que o agent.yaml declara). Erro de uso nunca é erro Go: volta como {"error": ...}
// para o modelo corrigir os argumentos (regras gerais da projeção); só falha de
// infraestrutura derruba o loop.

type saida = map[string]any

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
	nome := strings.TrimSpace(in.Nome)
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
		perm := ag.recursosPermitidos()
		if len(perm) == 0 {
			return erro("recurso %q não está declarado por este agente e nenhum recurso está permitido agora", nome), nil
		}
		return erro("recurso %q não está declarado por este agente; permitidos: %s", nome, strings.Join(perm, ", ")), nil
	}
	if !achado.Permitido {
		return erro("recurso %s recusado: %s", nome, achado.Motivo), nil
	}
	// escrita: a guarda determinística — só com o "sim" do morador na mensagem atual
	if achado.Recurso != nil && achado.Recurso.Write {
		msg := ""
		if col := coletorDe(ctx); col != nil {
			msg = col.mensagem
		}
		if !Confirmado(msg) {
			return erro("%s é escrita e exige a confirmação explícita do morador: a mensagem atual (%q) não é um sim. Resuma o que vai registrar, pergunte \"confirma? (sim/não)\" e pare", nome, corta(msg, 60)), nil
		}
	}

	// recurso resolvido pelo canal (identidade do morador): não depende do Nexus
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
