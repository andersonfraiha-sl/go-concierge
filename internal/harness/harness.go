// Package harness é o adaptador do Eino: transforma uma pasta OKF carregada por
// internal/okf num agente executável. O Eino entra com dois papéis, e só dois:
// o loop de tools (ChatModelAgent + Runner do adk) e a interpretação dos agentes e
// subagentes da pasta. A orquestração de negócio (passada de rota do orquestrador,
// profundidade, trace, uso) é código deste pacote.
//
//	ag, _ := harness.Build(ctx, cat, agente, harness.Config{})
//	res, _ := harness.Run(ctx, ag, "posso fazer barulho depois das 22h?", nil)
//	res.Resposta, res.Rota, res.Uso
//
// Sem chave (GEMINI_API_KEY ou OPENROUTER_API_KEY, conforme LLM_MODE) o modelo é o
// Mock: escolhe a rota por palavras-chave do roteador e do glossário, lê o primeiro
// dado com ler_dado e responde no contrato de saída. Com chave, o modelo é o
// OpenAI-compatível do eino-ext apontando para o Gemini ou para o OpenRouter.
package harness

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel/metric/noop"

	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/mcp"
	"go-concierge/internal/nexus"
	"go-concierge/internal/okf"
)

const (
	// ProfundidadeMax limita a cadeia de flips orquestrador → subagente (e de
	// invocar_agente): no nível 4 o agente responde como `self`.
	ProfundidadeMax = 4
	// PaginaChars é o tamanho da página de ler_dado, em caracteres.
	PaginaChars = 6000
	// ModeloPadrao e BaseURLPadrao valem quando o provedor não diz (ver provedor.go:
	// LLM_MODE=gemini usa o endpoint do Google e gemini-2.5-flash-lite).
	ModeloPadrao  = ModeloOpenRouter
	BaseURLPadrao = BaseURLOpenRouter

	maxIteracoesPadrao = 12
)

// Config ajusta a construção do agente. O zero funciona: provedor e chave do
// ambiente (LLM_MODE, GEMINI_API_KEY, OPENROUTER_API_KEY), e sem chave o Mock.
type Config struct {
	// Modelo é o id no provedor; vazio usa `modelo:` do agent.yaml, senão o padrão do provedor.
	Modelo string
	// APIKey do provedor; vazio lê a chave do provedor resolvido em Env. Sem chave = Mock.
	APIKey string
	// BaseURL do endpoint OpenAI-compatível; vazio = a do provedor resolvido.
	BaseURL string
	// Mock força o modelo falso mesmo com chave (inspeção e testes).
	Mock bool
	// Env resolve variáveis (chave do provedor, DEFAULT_ACCOUNT_ID); nil = os.Getenv.
	Env func(string) string
	// Nexus é o cliente dos recursos da account. nil ou sem BaseURL: chamar_recurso
	// devolve {"error": "Nexus indisponível"}.
	Nexus *nexus.Client
	// MCP executa os recursos que vieram do catálogo MCP (Recurso.Origem == "mcp") por
	// tools/call, com o token do cliente; nil = esses recursos ficam indisponíveis.
	MCP           mcp.Caller
	MCPResultados map[string]any
	// Identidade do morador nesta execução. Sem AccountID nenhum recurso do Nexus é
	// chamado: quem monta a Config (o atendimento, pelo condomínio; o okf-run) a define.
	Identidade Identidade
	// Sessao identifica a conversa (protocolo) na chave de idempotência; vazio = "local".
	Sessao string
	// Contexto é um bloco extra na instrução de toda a família (a memória sobre o
	// morador e o contexto do condomínio); vazio = nada.
	Contexto string
	// Casos filtra os casos de uso (subagentes do orquestrador de primeiro nível) que
	// este condomínio pode usar; nil = todos. Recebe o nome da pasta do subagente e o
	// id do caso.yaml (vazio se não há caso).
	Casos func(nome, casoID string) bool
	// Payloads são as definições do condomínio por recurso do Nexus (Admin): valem
	// como parâmetros padrão em chamar_recurso quando o agente não os passa.
	Payloads map[string]map[string]any
	// Orientacoes por caso de uso (nome da pasta do subagente → texto da
	// administradora): entram só na instrução daquele subagente.
	Orientacoes map[string]string
	// Locais são recursos que o próprio canal resolve, sem Nexus.
	Locais map[string]func(ctx context.Context, params map[string]any) (any, error)
	// MaxIteracoes do loop de tools por execução; <=0 = 12.
	MaxIteracoes int
	// ChatModel injeta um modelo pronto (testes); tem precedência sobre tudo.
	ChatModel model.ToolCallingChatModel
	// Ganchos avaliam toda chamada a recurso; nil = só os fixos, montados no Build.
	Ganchos *ganchos.Conjunto
	// Turno é o turno de quem chama, dono do cofre de PII; nil = o Build abre um com
	// Ganchos, e toda a família do agente o compartilha.
	Turno *ganchos.Turno
	// AdministradoraID e CondominioID são o alvo dos ganchos nas ferramentas: sem eles só
	// as regras globais valem.
	AdministradoraID, CondominioID string
}

// Identidade é o que o AUTH (Nexus) disse do morador: identificado ou não, em
// qual account e com qual papel. Recurso `private` só com Authenticated.
type Identidade struct {
	Authenticated bool   `json:"authenticated"`
	AccountID     string `json:"account_id"`
	Role          string `json:"role,omitempty"`
}

func (c Config) env(k string) string {
	if c.Env != nil {
		return c.Env(k)
	}
	return os.Getenv(k)
}

func (c Config) sessao() string {
	if c.Sessao != "" {
		return c.Sessao
	}
	return "local"
}

// Agente é a pasta OKF pronta para rodar: instrução (projeção), tools nativas,
// roteador (se orquestrador) e subagentes construídos com a mesma Config.
type Agente struct {
	Cat        *okf.Catalogo
	Def        *okf.Agent
	Instrucao  string
	Roteador   string
	Tools      []tool.BaseTool
	Subagentes map[string]*Agente
	// NomeModelo é o id resolvido; Provedor é gemini | openrouter | mock; Mock diz se o modelo é o falso.
	NomeModelo string
	Provedor   string
	Mock       bool

	cfg    Config
	modelo model.ToolCallingChatModel
	runner *adk.Runner
}

// Build monta o agente Eino da pasta: instrução = cat.Projecao(a), tools = as
// nativas do formato que a pasta pede (ler_dado, abrir_workflow, invocar_agente,
// chamar_recurso), subagentes recursivos com a mesma Config.
func Build(ctx context.Context, cat *okf.Catalogo, a *okf.Agent, cfg Config) (*Agente, error) {
	if cat == nil || a == nil {
		return nil, errors.New("harness: catálogo e agente são obrigatórios")
	}
	if cfg.MaxIteracoes <= 0 {
		cfg.MaxIteracoes = maxIteracoesPadrao
	}
	turno, err := turnoDoAgente(cfg)
	if err != nil {
		return nil, err
	}
	cfg.Turno = turno
	instrucao := cat.Projecao(a)
	if c := strings.TrimSpace(cfg.Contexto); c != "" {
		instrucao += "\n\n" + c
	}
	if o := strings.TrimSpace(cfg.Orientacoes[a.Nome]); o != "" && len(okf.Ancestrais(a.Path)) > 0 {
		instrucao += "\n\n# Orientações da administradora para este caso de uso\n\nValem neste condomínio, por cima das políticas gerais:\n\n" + o
	}
	ag := &Agente{
		Cat:        cat,
		Def:        a,
		Instrucao:  instrucao,
		Subagentes: map[string]*Agente{},
		cfg:        cfg,
	}
	if a.Tipo == okf.Orquestrador {
		ag.Roteador = cat.Roteador(a)
	}
	var excluidos []string
	raiz := len(okf.Ancestrais(a.Path)) == 0
	for _, s := range a.Subagentes {
		if raiz && cfg.Casos != nil {
			casoID := ""
			if s.Caso != nil {
				casoID = s.Caso.ID
			}
			if !cfg.Casos(s.Nome, casoID) {
				excluidos = append(excluidos, s.Nome)
				continue
			}
		}
		sub, err := Build(ctx, cat, s, cfg)
		if err != nil {
			return nil, err
		}
		ag.Subagentes[s.Nome] = sub
	}
	if len(excluidos) > 0 {
		ag.Roteador = semRotas(ag.Roteador, excluidos)
		ag.Instrucao = semLinhasDeSubagente(ag.Instrucao, excluidos)
	}

	tools, err := ag.toolsNativas()
	if err != nil {
		return nil, err
	}
	ag.Tools = tools

	if err := ag.montarModelo(ctx); err != nil {
		return nil, err
	}

	cma, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          a.Nome,
		Description:   a.Descricao,
		Instruction:   ag.Instrucao,
		Model:         ag.modelo,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		GenModelInput: semTemplate,
		MaxIterations: cfg.MaxIteracoes,
	})
	if err != nil {
		return nil, fmt.Errorf("harness: %s: %w", a.Rel, err)
	}
	ag.runner = adk.NewRunner(ctx, adk.RunnerConfig{Agent: cma})
	return ag, nil
}

// turnoDoAgente é o turno da Config ou um aberto no conjunto dela; sem conjunto, no dos
// fixos. Recusa turno sem algum fixo: as guardas de permissão, escrita e contexto B2C
// moram neles, e fixo nunca some por configuração.
func turnoDoAgente(cfg Config) (*ganchos.Turno, error) {
	turno := cfg.Turno
	if turno == nil {
		conjunto, err := conjuntoDe(cfg.Ganchos)
		if err != nil {
			return nil, err
		}
		t := conjunto.Turno()
		turno = &t
	}
	if faltam := fixos.Ausentes(turno.NomesDosFixos()); len(faltam) > 0 {
		return nil, fmt.Errorf("harness: ganchos sem os fixos %s", strings.Join(faltam, ", "))
	}
	return turno, nil
}

func conjuntoDe(conjunto *ganchos.Conjunto) (*ganchos.Conjunto, error) {
	if conjunto != nil {
		return conjunto, nil
	}
	return conjuntoDosFixos(fixos.Todos())
}

// conjuntoDosFixos é o conjunto de quem não injetou um: só fixos, com log e métrica noop,
// para falha de fixo sair em ERROR.
func conjuntoDosFixos(f ganchos.Fixos) (*ganchos.Conjunto, error) {
	observar, err := ganchos.ComObservabilidade(slog.Default(), noop.NewMeterProvider().Meter("ganchos"))
	if err != nil {
		return nil, fmt.Errorf("harness: observabilidade dos ganchos: %w", err)
	}
	return ganchos.NovoConjunto(f, nil, observar), nil
}

// semTemplate põe a projeção como system message SEM formatação: ela tem JSON e
// chaves literais, e o padrão do adk tentaria interpretá-las quando há session values.
func semTemplate(_ context.Context, instrucao string, in *adk.AgentInput) ([]adk.Message, error) {
	msgs := make([]adk.Message, 0, len(in.Messages)+1)
	if instrucao != "" {
		msgs = append(msgs, schema.SystemMessage(instrucao))
	}
	return append(msgs, in.Messages...), nil
}

func (ag *Agente) montarModelo(ctx context.Context) error {
	cfg := ag.cfg
	ag.NomeModelo = cfg.Modelo
	if ag.NomeModelo == "" {
		ag.NomeModelo = ag.Def.Modelo
	}
	if ag.NomeModelo == "" {
		ag.NomeModelo = ModeloPadrao
	}
	if cfg.ChatModel != nil {
		ag.modelo = cfg.ChatModel
		return nil
	}
	prov := ResolverProvedor(cfg.env)
	chave := cfg.APIKey
	if chave == "" {
		chave = prov.Chave
	}
	base := cfg.BaseURL
	if base == "" {
		base = prov.BaseURL
	}
	if cfg.Modelo == "" && ag.Def.Modelo == "" {
		ag.NomeModelo = prov.Modelo
	}
	ag.Provedor = prov.Nome
	if prov.Nome == "luna" && chave == "" && !cfg.Mock {
		return errors.New("harness: configure LLM_LUNA_GATEWAY_TOKEN para o gateway Luna")
	}
	if cfg.Mock || chave == "" || prov.Nome == "mock" && cfg.APIKey == "" {
		ag.Mock = true
		ag.Provedor = "mock"
		ag.modelo = NewMock(ag.Cat, ag.Def)
		return nil
	}
	if base == "" {
		base = BaseURLPadrao
	}
	m, err := novoModelo(ctx, prov, chave, base, ag.NomeModelo, 90*time.Second)
	if err != nil {
		return fmt.Errorf("harness: modelo %s: %w", ag.NomeModelo, err)
	}
	ag.modelo = m
	return nil
}

// Tool devolve uma tool nativa pelo nome (para testes e para o okf-run).
func (ag *Agente) Tool(nome string) (tool.InvokableTool, bool) {
	for _, t := range ag.Tools {
		info, err := t.Info(context.Background())
		if err != nil || info.Name != nome {
			continue
		}
		if it, ok := t.(tool.InvokableTool); ok {
			return it, true
		}
	}
	return nil, false
}

// NomesDeTools lista as tools na ordem de registro.
func (ag *Agente) NomesDeTools() []string {
	var out []string
	for _, t := range ag.Tools {
		if info, err := t.Info(context.Background()); err == nil {
			out = append(out, info.Name)
		}
	}
	return out
}

// ---- execução -------------------------------------------------------------

// Passo é um evento do trace: rota escolhida, tool chamada, resultado, resposta.
type Passo struct {
	Agente  string `json:"agente"`
	Tipo    string `json:"tipo"` // rota | tool | resultado | resposta | aviso | entrega | transbordo | modo | plano | item
	Detalhe string `json:"detalhe"`
}

// Uso soma os tokens das chamadas ao modelo (inclusive a passada de rota).
type Uso struct {
	Entrada  int `json:"entrada"`
	Saida    int `json:"saida"`
	Total    int `json:"total"`
	Chamadas int `json:"chamadas"`
}

// Soma acumula o uso de outra execução (para quem monta a resposta fora do harness).
func (u *Uso) Soma(o Uso) { u.soma(o) }

func (u *Uso) soma(o Uso) {
	u.Entrada += o.Entrada
	u.Saida += o.Saida
	u.Total += o.Total
	u.Chamadas += o.Chamadas
}

func (u *Uso) registra(m *schema.Message) {
	if m == nil || m.ResponseMeta == nil || m.ResponseMeta.Usage == nil {
		return
	}
	us := m.ResponseMeta.Usage
	u.Entrada += us.PromptTokens
	u.Saida += us.CompletionTokens
	u.Total += us.TotalTokens
	u.Chamadas++
}

// Resultado de uma execução (uma "vez do robô").
type Resultado struct {
	// Resposta é o último texto do assistente sem tool calls.
	Resposta string `json:"resposta"`
	// Agente que respondeu (caminho relativo), depois dos flips de rota.
	Agente string `json:"agente"`
	// Rota é o trace percorrido, do orquestrador ao último passo.
	Rota []Passo `json:"rota"`
	Uso  Uso     `json:"uso"`
	// Mensagens é o histórico completo da execução final (histórico + usuário +
	// assistente/tools), para continuar a conversa na próxima vez.
	Mensagens []*schema.Message `json:"-"`
	// Transbordo é o motivo quando algum agente pediu um atendente humano
	// (tool transferir_atendente); vazio = o concierge seguiu.
	Transbordo string `json:"transbordo,omitempty"`
	// Plano é o estado da orquestração nos modos planning e react (nil no router). A
	// sessão o guarda enquanto Pendente != "" e retoma com Continuar.
	Plano *Plano `json:"plano,omitempty"`
	// Registros são os ganchos que ajustaram, não deixaram passar ou falharam nas
	// chamadas a recurso. Fora do JSON: o motivo pode citar a mensagem do morador.
	Registros []ganchos.Registro `json:"-"`
}

// coletor acompanha uma execução (e as aninhadas via invocar_agente) pelo contexto.
type coletor struct {
	mu           sync.Mutex
	profundidade int
	mensagem     string // a mensagem do morador nesta execução (a guarda de escrita lê o "sim")
	// transbordo é o motivo registrado pela tool transferir_atendente; compartilhado
	// entre o orquestrador e os subagentes da mesma execução.
	transbordo *string
	passos     []Passo
	uso        Uso
	invocacoes int // chamadas a recursos do Nexus nesta execução (chave de idempotência)
	registros  []ganchos.Registro
}

func (c *coletor) registrar(rs []ganchos.Registro) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.registros = append(c.registros, rs...)
}

func (c *coletor) proximaInvocacao() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.invocacoes++
	return c.invocacoes
}

type chaveColetor struct{}

func (c *coletor) passo(agente, tipo, detalhe string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.passos = append(c.passos, Passo{Agente: agente, Tipo: tipo, Detalhe: detalhe})
}

func (c *coletor) absorve(r *Resultado) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.passos = append(c.passos, r.Rota...)
	c.registros = append(c.registros, r.Registros...)
	c.uso.soma(r.Uso)
}

func coletorDe(ctx context.Context) *coletor {
	if c, ok := ctx.Value(chaveColetor{}).(*coletor); ok {
		return c
	}
	return nil
}

// Run executa o agente para a mensagem, com o histórico anterior (pode ser nil).
// Orquestrador: passada de rota com o roteador; se a rota é um subagente, flipa
// para ele (recursivo até ProfundidadeMax); `self` ou rota desconhecida respondem
// com o próprio agente.
func Run(ctx context.Context, ag *Agente, mensagem string, historico []*schema.Message) (*Resultado, error) {
	if ag == nil {
		return nil, errors.New("harness: agente nulo")
	}
	prof := 0
	if pai := coletorDe(ctx); pai != nil {
		prof = pai.profundidade
	}
	return run(ctx, ag, mensagem, historico, prof)
}

func run(ctx context.Context, ag *Agente, mensagem string, historico []*schema.Message, profundidade int) (*Resultado, error) {
	col := &coletor{profundidade: profundidade, mensagem: mensagem}
	if pai := coletorDe(ctx); pai != nil && pai.transbordo != nil {
		col.transbordo = pai.transbordo
	} else {
		col.transbordo = new(string)
	}
	ctx = context.WithValue(ctx, chaveColetor{}, col)
	rel := ag.Def.Rel

	if ag.Roteador != "" && len(ag.Subagentes) > 0 {
		// Modos ART (plano.go): no primeiro nível, planning e react substituem a passada de
		// rota. O Mock não planeja (responde rotas por palavra-chave): nele é sempre router.
		if o := OrquestracaoDe(ag.Def, ag.cfg.env); profundidade == 0 && o.Modo != ModoRouter && !ag.Mock {
			col.passo(rel, "modo", o.Modo)
			return ag.orquestrar(ctx, col, o, mensagem, historico)
		}
		if profundidade >= ProfundidadeMax {
			col.passo(rel, "aviso", fmt.Sprintf("profundidade %d atingida; respondo como self", profundidade))
		} else {
			rota, uso, err := ag.rotear(ctx, mensagem, historico)
			if err != nil {
				return nil, err
			}
			col.uso.soma(uso)
			col.passo(rel, "rota", rota)
			if sub, ok := ag.Subagentes[rota]; ok {
				r, err := run(ctx, sub, mensagem, historico, profundidade+1)
				if err != nil {
					return nil, err
				}
				if strings.TrimSpace(r.Resposta) == "" && r.Transbordo == "" {
					col.passo(rel, "aviso", "subagente devolveu resposta vazia; vai o aviso padrão")
					r.Resposta = respostaVazia
				}
				// O orquestrador de primeiro nível é quem fala com o morador: ele entrega
				// o resultado do subagente na sua persona (o subagente devolve um contrato,
				// não a resposta final). Pergunta curta e simples do subagente ao morador
				// ("Para qual data?") vai direto: não precisa de persona nem de assets.
				if profundidade == 0 && strings.TrimSpace(ag.Def.Persona) != "" && strings.TrimSpace(r.Resposta) != "" && !perguntaSimples(r.Resposta) {
					texto, uso, err := ag.entregar(ctx, mensagem, sub, r.Resposta, len(historico) > 0)
					switch {
					case err != nil:
						col.passo(rel, "aviso", "entrega falhou, resposta do subagente vai crua: "+err.Error())
					case texto != "":
						col.uso.soma(uso)
						r.Resposta = texto
						if n := len(r.Mensagens); n > 0 && r.Mensagens[n-1].Role == schema.Assistant && len(r.Mensagens[n-1].ToolCalls) == 0 {
							r.Mensagens[n-1] = schema.AssistantMessage(texto, nil)
						}
						r.Rota = append(r.Rota, Passo{Agente: rel, Tipo: "entrega", Detalhe: corta(texto, 160)})
					}
				}
				r.Transbordo = guardaTransbordo(col, rel, r.Resposta)
				r.Rota = append(col.passos, r.Rota...)
				r.Registros = append(col.registros, r.Registros...)
				r.Uso.soma(col.uso)
				return r, nil
			}
		}
	}

	msgs := make([]*schema.Message, 0, len(historico)+1)
	msgs = append(msgs, historico...)
	msgs = append(msgs, schema.UserMessage(mensagem))

	iter := ag.runner.Run(ctx, msgs)
	var resposta string
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			return nil, fmt.Errorf("harness: %s: %w", rel, ev.Err)
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil {
			continue
		}
		m, err := mensagemDe(ev.Output.MessageOutput)
		if err != nil {
			return nil, fmt.Errorf("harness: %s: %w", rel, err)
		}
		if m == nil {
			continue
		}
		msgs = append(msgs, m)
		switch m.Role {
		case schema.Assistant:
			col.mu.Lock()
			col.uso.registra(m)
			col.mu.Unlock()
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					col.passo(rel, "tool", tc.Function.Name+" "+corta(tc.Function.Arguments, 200))
				}
			} else if strings.TrimSpace(m.Content) != "" {
				resposta = m.Content
				col.passo(rel, "resposta", corta(m.Content, 160))
			}
		case schema.Tool:
			nome := m.ToolName
			if nome == "" {
				nome = ev.Output.MessageOutput.ToolName
			}
			col.passo(rel, "resultado", strings.TrimSpace(nome+": "+corta(m.Content, 200)))
		}
	}

	if strings.TrimSpace(resposta) == "" && *col.transbordo == "" {
		col.passo(rel, "aviso", "modelo não redigiu resposta; vai o aviso padrão")
		resposta = respostaVazia
		msgs = append(msgs, schema.AssistantMessage(resposta, nil))
	}
	transbordo := guardaTransbordo(col, rel, resposta)
	col.mu.Lock()
	defer col.mu.Unlock()
	return &Resultado{
		Resposta:   resposta,
		Agente:     rel,
		Rota:       col.passos,
		Uso:        col.uso,
		Mensagens:  msgs,
		Transbordo: transbordo,
		Registros:  col.registros,
	}, nil
}

// respostaVazia é o que vai ao morador quando o modelo não redige nada.
const respostaVazia = "Não encontrei isso por aqui. Posso te ajudar com outra coisa, ou passo para a administração."

var anuncios = []string{"transferido ao atendente", "vou transferir", "vou te transferir", "transferir você", "transferindo você", "um atendente vai assumir", "um atendente humano vai assumir", "atendente irá assumir", "atendente vai entrar em contato", "vou encaminhar para um atendente", "encaminhar você para um atendente", "passar para um atendente", "repassar para um atendente"}

// AnunciaTransferencia diz se o texto promete ao morador uma transferência a humano.
func AnunciaTransferencia(texto string) bool {
	t := strings.ToLower(texto)
	for _, a := range anuncios {
		if strings.Contains(t, a) {
			return true
		}
	}
	return false
}

// guardaTransbordo: a transferência acontece pela tool; se a resposta a anuncia sem a tool
// ter sido chamada, o canal transfere mesmo assim (o morador não pode ficar esperando um
// humano que nunca vem). Devolve o motivo do transbordo (vazio = nenhum).
func guardaTransbordo(col *coletor, rel, resposta string) string {
	if col.transbordo == nil {
		return ""
	}
	if *col.transbordo == "" && AnunciaTransferencia(resposta) {
		*col.transbordo = "anunciado na resposta sem a tool: " + corta(strings.TrimSpace(resposta), 120)
		col.passo(rel, "transbordo", *col.transbordo)
	}
	return *col.transbordo
}

// perguntaSimples: texto curto, sem marcação, que termina perguntando ao morador.
func perguntaSimples(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > 220 || !strings.HasSuffix(s, "?") {
		return false
	}
	if strings.ContainsAny(s, "#{}`|[]") || strings.Count(s, "\n") > 1 {
		return false
	}
	return true
}

// entregar redige a resposta final ao morador, na persona do orquestrador, a partir
// do resultado do subagente. No Mock, devolve o resultado como veio.
func (ag *Agente) entregar(ctx context.Context, mensagem string, sub *Agente, resultado string, continuacao bool) (string, Uso, error) {
	var uso Uso
	if ag.Mock || ag.modelo == nil {
		return resultado, uso, nil
	}
	var b strings.Builder
	b.WriteString(ag.preambuloPersona())
	turno := "Esta é a PRIMEIRA mensagem da conversa: pode se apresentar em uma linha."
	if continuacao {
		turno = "A conversa já começou: NÃO cumprimente nem se apresente de novo; vá direto ao ponto."
	}
	fmt.Fprintf(&b, "\n# Tarefa\n\nO especialista `%s` (%s) produziu o RESULTADO abaixo para a mensagem do morador. Redija a resposta final ao morador:\n- curta, cordial, em português do Brasil, própria para WhatsApp, na persona acima;\n- %s\n- preserve os fatos, artigos citados, horários, valores, protocolos e links do resultado; não acrescente informação;\n- nada de JSON, chaves, nomes de campo, caminhos de arquivo, termos internos (subagente, rota, contrato, recurso) nem menção a que houve um especialista;\n- se o resultado diz que não encontrou, não há dado ou houve erro, diga isso com clareza e ofereça a administração.\nResponda só com o texto da mensagem.", sub.Def.Nome, sub.Def.Descricao, turno)
	msgs := []*schema.Message{
		schema.SystemMessage(b.String()),
		schema.UserMessage("Mensagem do morador:\n" + mensagem + "\n\nResultado:\n" + resultado),
	}
	m, err := ag.modelo.Generate(ctx, msgs)
	if err != nil {
		return "", uso, err
	}
	uso.registra(m)
	return strings.TrimSpace(m.Content), uso, nil
}

// Redigir escreve ao morador, na persona do orquestrador, a partir de um conteúdo já
// decidido em código (o motor de pedidos): a tarefa diz o que preservar. No Mock
// devolve o conteúdo como veio.
func (ag *Agente) Redigir(ctx context.Context, tarefa, conteudo string, continuacao bool) (string, Uso, error) {
	var uso Uso
	if ag.Mock || ag.modelo == nil {
		return conteudo, uso, nil
	}
	var b strings.Builder
	b.WriteString(ag.preambuloPersona())
	turno := "Esta é a PRIMEIRA mensagem da conversa: pode se apresentar em uma linha."
	if continuacao {
		turno = "A conversa já começou: NÃO cumprimente nem se apresente de novo; vá direto ao ponto."
	}
	fmt.Fprintf(&b, "\n# Tarefa\n\n%s\n- curta, cordial, em português do Brasil, própria para WhatsApp, na persona acima;\n- %s\n- nada de JSON, chaves, nomes de campo, caminhos de arquivo nem termos internos;\n- não acrescente informação nem prometa nada além do conteúdo.\nResponda só com o texto da mensagem.", strings.TrimSpace(tarefa), turno)
	m, err := ag.modelo.Generate(ctx, []*schema.Message{schema.SystemMessage(b.String()), schema.UserMessage(conteudo)})
	if err != nil {
		return "", uso, err
	}
	uso.registra(m)
	return strings.TrimSpace(m.Content), uso, nil
}

// preambuloPersona abre o prompt de redação ao morador: quem fala, a persona, as
// políticas e os assets do orquestrador (rodapés, templates: o texto, nunca o caminho).
func (ag *Agente) preambuloPersona() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Você é o agente %q, quem fala com o morador pelo WhatsApp em nome da administração do condomínio.\n\n# Persona\n\n%s\n", ag.Def.Nome, strings.TrimSpace(okf.StripFrontmatter(ag.Def.Persona)))
	if pol := ag.Cat.Politicas(ag.Def); pol != "" {
		b.WriteString("\n# Políticas\n\n" + pol + "\n")
	}
	if assets := ag.Def.Arquivos["assets"]; len(assets) > 0 {
		b.WriteString("\n# Assets (quando a persona mandar usar um, use o TEXTO dele, nunca o caminho)\n")
		for _, f := range assets {
			if f.Bytes > 4096 {
				continue
			}
			if raw, err := ag.Cat.Ler(ag.Def, f); err == nil {
				fmt.Fprintf(&b, "\n## %s\n%s\n", f.Caminho, strings.TrimSpace(string(raw)))
			}
		}
	}
	return b.String()
}

// semRotas tira do texto do roteador os itens que apontam para rotas excluídas (o
// item inteiro, com as linhas de continuação), para o modelo nem as ver.
func semRotas(texto string, excluidos []string) string {
	if texto == "" || len(excluidos) == 0 {
		return texto
	}
	ehItem := func(l string) bool {
		t := strings.TrimSpace(l)
		return strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ")
	}
	ehContinuacao := func(l string) bool {
		return !ehItem(l) && strings.TrimSpace(l) != "" && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t"))
	}
	bl := strings.Split(texto, "\n")
	var out []string
	for i := 0; i < len(bl); i++ {
		if !ehItem(bl[i]) {
			out = append(out, bl[i])
			continue
		}
		j := i + 1
		for j < len(bl) && ehContinuacao(bl[j]) {
			j++
		}
		bloco := strings.Join(bl[i:j], " ")
		excluir := false
		for _, ex := range excluidos {
			if strings.Contains(bloco, "rota `"+ex+"`") {
				excluir = true
				break
			}
		}
		if !excluir {
			out = append(out, bl[i:j]...)
		}
		i = j - 1
	}
	return strings.Join(out, "\n")
}

// semLinhasDeSubagente tira da projeção as linhas de índice dos subagentes excluídos.
func semLinhasDeSubagente(texto string, excluidos []string) string {
	linhas := strings.Split(texto, "\n")
	var out []string
	for _, l := range linhas {
		skip := false
		for _, ex := range excluidos {
			if strings.HasPrefix(strings.TrimSpace(l), "- `"+ex+"`") {
				skip = true
			}
		}
		if !skip {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func mensagemDe(mv *adk.MessageVariant) (*schema.Message, error) {
	if mv.IsStreaming {
		if mv.MessageStream == nil {
			return nil, nil
		}
		return schema.ConcatMessageStream(mv.MessageStream)
	}
	return mv.Message, nil
}

// cabecalhoRoteador abre o system prompt da passada de rota; o Mock o reconhece.
const cabecalhoRoteador = "# Roteador"

// rotear faz a passada barata do orquestrador: só o roteador, sem tools, e o
// modelo responde o nome da rota.
func (ag *Agente) rotear(ctx context.Context, mensagem string, historico []*schema.Message) (string, Uso, error) {
	nomes := ag.nomesDeRotas()
	var sb strings.Builder
	sb.WriteString(cabecalhoRoteador + " de " + ag.Def.Nome + "\n\n")
	sb.WriteString("Você decide SÓ a rota da mensagem do morador, segundo os critérios abaixo.\n\n")
	sb.WriteString(ag.Roteador)
	sb.WriteString("\n\nRotas válidas: `" + strings.Join(nomes, "`, `") + "`, `self`.\n")
	sb.WriteString("Responda SOMENTE com o nome da rota, sem explicação.")

	msgs := []*schema.Message{schema.SystemMessage(sb.String())}
	msgs = append(msgs, ultimas(historico, 6)...)
	msgs = append(msgs, schema.UserMessage(mensagem))

	out, err := ag.modelo.Generate(ctx, msgs)
	if err != nil {
		return "", Uso{}, fmt.Errorf("harness: roteador de %s: %w", ag.Def.Rel, err)
	}
	var uso Uso
	uso.registra(out)
	return extrairRota(out.Content, nomes), uso, nil
}

func (ag *Agente) nomesDeRotas() []string {
	nomes := make([]string, 0, len(ag.Subagentes))
	for n := range ag.Subagentes {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)
	return nomes
}

// extrairRota acha o nome de rota válido na resposta do modelo; nada → self.
func extrairRota(texto string, nomes []string) string {
	t := strings.ToLower(strings.TrimSpace(texto))
	t = strings.Trim(t, "`\"'. \n")
	for _, n := range nomes {
		if t == n {
			return n
		}
	}
	melhor, pos := "self", -1
	for _, n := range nomes {
		if i := strings.Index(t, n); i >= 0 && (pos == -1 || i < pos) {
			melhor, pos = n, i
		}
	}
	return melhor
}

func ultimas(h []*schema.Message, n int) []*schema.Message {
	var out []*schema.Message
	for _, m := range h {
		if m.Role == schema.User || (m.Role == schema.Assistant && len(m.ToolCalls) == 0) {
			out = append(out, m)
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

func corta(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
