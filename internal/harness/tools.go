package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"go-concierge/internal/okf"
)

// As tools nativas do formato. Cada caixinha de capacidade da pasta vira uma tool
// do Eino, só quando a pasta tem algo nela. Erro de uso nunca é erro Go: volta
// como {"error": ...} para o modelo corrigir os argumentos (regras gerais da
// projeção), e só falhas de infraestrutura derrubam o loop.

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

	if len(ag.Cat.Dados(a)) > 0 {
		if err := add(utils.InferTool("ler_dado",
			"Lê um arquivo do índice de dados do agente, em páginas de "+fmt.Sprint(PaginaChars)+" caracteres. Quando tem_mais=true, chame de novo com pagina+1.",
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
	if len(a.Arquivos["schemas"])+len(a.Herdados["schemas"]) > 0 {
		if err := add(utils.InferTool("consultar_schema",
			"Devolve a estrutura (tabelas, colunas, domínios) de um schema da pasta.",
			ag.consultarSchema)); err != nil {
			return nil, err
		}
	}
	if len(a.Apis) > 0 {
		if err := add(utils.InferTool("chamar_api",
			"Chama uma API do registro vinculada ao agente (ver manual na projeção). Recusa quando falta um segredo.",
			ag.chamarAPI)); err != nil {
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

	// stubs: a pasta pede, o runtime ainda não entrega
	if len(a.Arquivos["scripts"])+len(a.Herdados["scripts"]) > 0 {
		if err := add(utils.InferTool("rodar_script", "Roda um script da pasta em subprocesso.", stub[*argsScript]("rodar_script"))); err != nil {
			return nil, err
		}
	}
	if len(a.Arquivos["bash"])+len(a.Herdados["bash"]) > 0 {
		if err := add(utils.InferTool("rodar_bash", "Roda um comando da allowlist bash da pasta.", stub[*argsBash]("rodar_bash"))); err != nil {
			return nil, err
		}
	}
	if len(a.Arquivos["rag"]) > 0 {
		if err := add(utils.InferTool("buscar_rag", "Busca semântica na base RAG da pasta.", stub[*argsRag]("buscar_rag"))); err != nil {
			return nil, err
		}
	}
	mcps := append(append([]okf.Arquivo{}, a.Arquivos["mcps"]...), a.Herdados["mcps"]...)
	sort.Slice(mcps, func(i, j int) bool { return mcps[i].Caminho < mcps[j].Caminho })
	for _, m := range mcps {
		nome := strings.TrimSuffix(filepath.Base(m.Caminho), filepath.Ext(m.Caminho))
		if err := add(utils.InferTool("mcp_"+nome, "Tools do servidor MCP "+nome+" (descritor em "+m.Caminho+").", stub[*argsMCP]("mcp_"+nome))); err != nil {
			return nil, err
		}
	}
	return tools, nil
}

// ---- ler_dado -------------------------------------------------------------

type argsLerDado struct {
	Caminho string `json:"caminho" jsonschema:"required" jsonschema_description:"caminho exatamente como no índice de dados (ex.: data/regimento.md, assets/rodape.md, fonte:data/okf/glossary.yaml)"`
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

// ---- consultar_schema -----------------------------------------------------

type argsSchema struct {
	Nome string `json:"nome" jsonschema:"required" jsonschema_description:"nome do schema sem extensão, como listado na projeção"`
}

func (ag *Agente) consultarSchema(_ context.Context, in *argsSchema) (saida, error) {
	todos := append(append([]okf.Arquivo{}, ag.Def.Arquivos["schemas"]...), ag.Def.Herdados["schemas"]...)
	var nomes []string
	for _, f := range todos {
		nome := strings.TrimSuffix(filepath.Base(f.Caminho), filepath.Ext(f.Caminho))
		nomes = append(nomes, nome)
		if nome != strings.TrimSpace(in.Nome) {
			continue
		}
		raw, err := ag.Cat.Ler(ag.Def, f)
		if err != nil {
			return erro("%v", err), nil
		}
		return saida{"nome": nome, "caminho": f.Caminho, "conteudo": string(raw)}, nil
	}
	return erro("schema %q não existe; disponíveis: %s", in.Nome, strings.Join(nomes, ", ")), nil
}

// ---- chamar_api -----------------------------------------------------------

type argsAPI struct {
	Nome   string `json:"nome" jsonschema:"required" jsonschema_description:"nome da API no registro (ex.: nexus)"`
	Metodo string `json:"metodo" jsonschema:"required" jsonschema_description:"GET, POST, PUT, PATCH ou DELETE"`
	Rota   string `json:"rota" jsonschema:"required" jsonschema_description:"caminho a partir da base_url, começando com / (pode ter query string)"`
	Corpo  string `json:"corpo" jsonschema_description:"corpo JSON da requisição, quando houver"`
}

var metodosHTTP = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}

const maxCorpoResposta = 64 * 1024

func (ag *Agente) chamarAPI(ctx context.Context, in *argsAPI) (saida, error) {
	nome := strings.TrimSpace(in.Nome)
	vinculada := false
	for _, n := range ag.Def.Apis {
		if n == nome {
			vinculada = true
		}
	}
	if !vinculada {
		return erro("api %q não está vinculada a este agente; vinculadas: %s", nome, strings.Join(ag.Def.Apis, ", ")), nil
	}
	var api *okf.API
	for _, x := range ag.Cat.ListarAPIs() {
		if x.Nome == nome {
			x := x
			api = &x
		}
	}
	if api == nil {
		return erro("api %q não existe no registro apis/", nome), nil
	}

	var faltam []string
	for _, r := range api.Requer {
		if ag.cfg.env(r) == "" {
			faltam = append(faltam, r)
		}
	}
	headers := map[string]string{}
	for k, v := range api.Headers {
		headers[k] = os.Expand(v, func(variavel string) string {
			val := ag.cfg.env(variavel)
			if val == "" {
				faltam = append(faltam, variavel)
			}
			return val
		})
	}
	if len(faltam) > 0 {
		sort.Strings(faltam)
		return erro("recusado: segredo(s) %s não definido(s) no ambiente; a api %s exige. Não insista: avise que o acesso não está configurado.", strings.Join(unicos(faltam), ", "), nome), nil
	}

	metodo := strings.ToUpper(strings.TrimSpace(in.Metodo))
	if !metodosHTTP[metodo] {
		return erro("método %q inválido; use GET, POST, PUT, PATCH ou DELETE", in.Metodo), nil
	}
	rota := strings.TrimSpace(in.Rota)
	if !strings.HasPrefix(rota, "/") {
		return erro("rota deve começar com / (recebi %q)", in.Rota), nil
	}
	if api.BaseURL == "" {
		return erro("api %s não tem base_url no registro", nome), nil
	}
	url := strings.TrimRight(api.BaseURL, "/") + rota

	var corpo io.Reader
	if strings.TrimSpace(in.Corpo) != "" {
		corpo = strings.NewReader(in.Corpo)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, url, corpo)
	if err != nil {
		return erro("requisição inválida: %v", err), nil
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if corpo != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	cliente := ag.cfg.HTTPClient
	if cliente == nil {
		cliente = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := cliente.Do(req)
	if err != nil {
		return erro("falha ao chamar %s %s: %v", metodo, url, err), nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxCorpoResposta))
	out := saida{"status": resp.StatusCode, "corpo": corta(string(raw), PaginaChars)}
	if resp.StatusCode >= 400 {
		out["error"] = fmt.Sprintf("HTTP %d em %s %s", resp.StatusCode, metodo, rota)
	}
	return out, nil
}

func unicos(xs []string) []string {
	vistos := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !vistos[x] {
			vistos[x] = true
			out = append(out, x)
		}
	}
	return out
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

// ---- stubs ----------------------------------------------------------------

type argsScript struct {
	Nome string `json:"nome" jsonschema:"required" jsonschema_description:"arquivo do script"`
	Args string `json:"args" jsonschema_description:"argumentos"`
}
type argsBash struct {
	Comando string `json:"comando" jsonschema:"required" jsonschema_description:"comando da allowlist"`
	Args    string `json:"args" jsonschema_description:"argumentos"`
}
type argsRag struct {
	Consulta string `json:"consulta" jsonschema:"required" jsonschema_description:"o que procurar"`
}
type argsMCP struct {
	Tool       string `json:"tool" jsonschema:"required" jsonschema_description:"tool do servidor"`
	Argumentos string `json:"argumentos" jsonschema_description:"argumentos em JSON"`
}

// stub devolve o erro claro de capacidade ainda não implementada no runtime Go.
func stub[T any](nome string) func(context.Context, T) (saida, error) {
	return func(_ context.Context, in T) (saida, error) {
		args, _ := json.Marshal(in)
		return erro("%s ainda não suportado neste runtime (pedido: %s); siga sem essa capacidade ou avise o morador", nome, string(args)), nil
	}
}
