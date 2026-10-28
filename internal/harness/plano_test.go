package harness

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/okf"
)

// roteiro é um modelo com script: responde pelo system prompt e pela última mensagem
// do usuário, para testar os modos planning e react sem chave e sem o Mock.
type roteiro struct {
	tools    []*schema.ToolInfo
	chamadas []string
	resposta func(sistema, usuario, primeiroUsuario string) string
}

func (r *roteiro) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	c := *r
	c.tools = tools
	return &c, nil
}

func (r *roteiro) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	out, err := r.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{out}), nil
}

func (r *roteiro) Generate(_ context.Context, in []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	var sistema, usuario, primeiro string
	for _, m := range in {
		switch m.Role {
		case schema.System:
			sistema = m.Content
		case schema.User:
			usuario = m.Content
			if primeiro == "" {
				primeiro = m.Content
			}
		}
	}
	texto := r.resposta(sistema, usuario, primeiro)
	r.chamadas = append(r.chamadas, corta(texto, 40))
	return schema.AssistantMessage(texto, nil), nil
}

func envModo(modo string) func(string) string {
	return func(k string) string {
		if k == "ORQUESTRACAO_MODO" {
			return modo
		}
		return ""
	}
}

func TestOrquestracaoDe(t *testing.T) {
	a := &okf.Agent{Config: map[string]any{"orquestracao": map[string]any{"modo": "Planning", "bind_confidence": 0.7, "max_itens": 3, "max_iteracoes": "4"}}}
	o := OrquestracaoDe(a, semEnv)
	if o.Modo != ModoPlanning || o.BindConfidence != 0.7 || o.MaxItens != 3 || o.MaxIteracoes != 4 {
		t.Fatalf("agent.yaml devia valer: %+v", o)
	}
	if o := OrquestracaoDe(a, envModo("react")); o.Modo != ModoReAct {
		t.Fatalf("ORQUESTRACAO_MODO devia vencer o agent.yaml: %+v", o)
	}
	if o := OrquestracaoDe(&okf.Agent{Config: map[string]any{"orquestracao": map[string]any{"modo": "xpto", "bind_confidence": 5}}}, semEnv); o.Modo != ModoRouter || o.BindConfidence != 0.8 {
		t.Fatalf("modo desconhecido e valor fora da faixa caem no padrão: %+v", o)
	}
	if o := OrquestracaoDe(nil, nil); o.Modo != ModoRouter {
		t.Fatalf("sem agente é router: %+v", o)
	}
}

// Os candidatos de vínculo são os casos de uso (menos o observador) e os recursos de
// leitura permitidos; escrita nunca entra.
func TestCandidatos(t *testing.T) {
	cat, a := catalogoExemplo(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv, Identidade: Identidade{Authenticated: true, AccountID: "cond-001"}})
	if err != nil {
		t.Fatal(err)
	}
	var casos, recursos []string
	for _, c := range ag.candidatos() {
		if c.Tipo == TipoCaso {
			casos = append(casos, c.Nome)
		} else {
			recursos = append(recursos, c.Nome)
		}
	}
	if len(casos) < 5 || strings.Contains(strings.Join(casos, ","), "observador-sessao") {
		t.Fatalf("casos de uso sem o observador; veio %v", casos)
	}
	sort.Strings(recursos)
	if strings.Join(recursos, ",") != "faq.buscar,unidades.listar" {
		t.Fatalf("recursos de leitura declarados pelo concierge; veio %v", recursos)
	}
	if _, ok := ag.candidatoPorNome("reserva-area-comum"); !ok {
		t.Fatal("reserva-area-comum devia ser candidato")
	}
}

// Planning: dois pedidos numa mensagem viram dois itens; o caso de uso do primeiro
// pergunta a data e o plano PAUSA; a resposta do morador vai direto ao item pendente
// (Continuar), o segundo item consulta um recurso e a resposta sai consolidada.
func TestPlanningComPausaEContinuacao(t *testing.T) {
	cat, a := catalogoExemplo(t)
	script := &roteiro{resposta: func(sistema, usuario, primeiroUsuario string) string {
		switch {
		case strings.Contains(sistema, "modo PLANNING"):
			return "```json\n" + `{"itens":[{"id":"t1","texto":"reservar o salão de festas","alvo":"reserva-area-comum","confianca":0.95},{"id":"t2","texto":"saber o horário de obras","alvo":"faq.buscar","confianca":0.9,"params":{"pergunta":"horário de obras"}}],"perguntas":[],"instrucao_final":"responda os dois pontos"}` + "\n```"
		case strings.Contains(sistema, "Redija a resposta final"):
			return "CONSOLIDADO: " + corta(usuario, 60)
		case strings.TrimSpace(usuario) == "dia 15":
			if !strings.Contains(primeiroUsuario, "Atenda apenas este pedido: reservar o salão de festas") || !strings.Contains(primeiroUsuario, "saber o horário de obras — já está com outro especialista") || strings.Contains(primeiroUsuario, "quero reservar") {
				return "ERRO: na retomada o caso devia ver só o seu pedido no histórico: " + primeiroUsuario
			}
			return "Reserva do salão registrada para o dia 15, protocolo R-1."
		case strings.HasPrefix(usuario, "reservar"):
			if !strings.Contains(usuario, "Atenda apenas este pedido") {
				return "ERRO: o caso devia receber o escopo do item: " + usuario
			}
			return "Para qual data?"
		}
		return "não sei o que responder a: " + usuario
	}}
	faq := func(_ context.Context, params map[string]any) (any, error) {
		return map[string]any{"resposta": "Obras só em dias úteis, das 8h às 18h", "pergunta": params["pergunta"]}, nil
	}
	cfg := Config{ChatModel: script, Env: envModo("planning"), Identidade: Identidade{Authenticated: true, AccountID: "cond-001"},
		Locais: map[string]func(context.Context, map[string]any) (any, error){"faq.buscar": faq}}
	ag, err := Build(context.Background(), cat, a, cfg)
	if err != nil {
		t.Fatal(err)
	}

	r1, err := Run(context.Background(), ag, "quero reservar o salão e saber o horário de obras", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Resposta != "Para qual data?" || r1.Plano == nil || r1.Plano.Pendente != "t1" || r1.Plano.Status != PlanoAguardando {
		t.Fatalf("o caso perguntou e o plano devia pausar em t1; veio %q plano=%+v", r1.Resposta, r1.Plano)
	}
	if len(r1.Plano.Itens) != 2 || r1.Plano.Itens[1].Status != StatusPendente || r1.Plano.Itens[1].Tipo != TipoRecurso {
		t.Fatalf("dois itens, o segundo (recurso) ainda pendente: %+v", r1.Plano.Itens)
	}
	if !temPasso(r1.Rota, "modo", "planning") || !temPasso(r1.Rota, "plano", "2 item") || !temPasso(r1.Rota, "item", "t1 aguardando") {
		t.Fatalf("trace devia ter modo, plano e item: %+v", r1.Rota)
	}

	r2, err := Continuar(context.Background(), ag, r1.Plano, "dia 15", r1.Mensagens)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Plano == nil || r2.Plano.Status != PlanoConcluido || r2.Plano.Pendente != "" {
		t.Fatalf("plano devia concluir: %+v", r2.Plano)
	}
	t1, t2 := r2.Plano.Itens[0], r2.Plano.Itens[1]
	if t1.Status != StatusFeito || !strings.Contains(t1.Resultado, "R-1") {
		t.Fatalf("t1 devia terminar pelo caso de uso: %+v", t1)
	}
	if t2.Status != StatusFeito || !strings.Contains(t2.Resultado, "dias úteis") {
		t.Fatalf("t2 devia consultar faq.buscar pelo canal: %+v", t2)
	}
	if !strings.HasPrefix(r2.Resposta, "CONSOLIDADO:") || !temPasso(r2.Rota, "entrega", "CONSOLIDADO") {
		t.Fatalf("resposta consolidada na persona: %q", r2.Resposta)
	}
	if n := len(r2.Mensagens); n < 4 || r2.Mensagens[n-1].Content != r2.Resposta || r2.Mensagens[n-2].Content != "dia 15" {
		t.Fatalf("histórico devia terminar com a troca deste turno: %d msgs", n)
	}
}

// Planning: vínculo abaixo de bind_confidence (ou pergunta do plano) vira UMA pergunta
// ao morador antes de executar; a resposta libera os itens.
func TestPlanningPerguntaAntesDeExecutar(t *testing.T) {
	cat, a := catalogoExemplo(t)
	script := &roteiro{resposta: func(sistema, usuario, primeiroUsuario string) string {
		switch {
		case strings.Contains(sistema, "modo PLANNING"):
			return `{"itens":[{"id":"t1","texto":"Liberar um visitante","alvo":"liberacao-visitante","confianca":0.5},{"id":"t2","texto":"saber se pode churrasco","alvo":"self","confianca":1}],"perguntas":["Qual o nome do visitante?"]}`
		case strings.Contains(sistema, "Redija a resposta final"):
			return "CONSOLIDADO"
		}
		if strings.Contains(usuario, "Contexto: o concierge perguntou") && strings.Contains(usuario, "sim, o João") {
			return "Visitante João liberado, código V-9."
		}
		return "ERRO: o caso devia receber a resposta do morador no contexto: " + usuario
	}}
	ag, err := Build(context.Background(), cat, a, Config{ChatModel: script, Env: envModo("planning"), Identidade: Identidade{Authenticated: true, AccountID: "cond-001"}})
	if err != nil {
		t.Fatal(err)
	}
	r1, err := Run(context.Background(), ag, "vem um amigo aqui, pode churrasco?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r1.Resposta, "Você quer liberar um visitante?") || !strings.Contains(r1.Resposta, "Qual o nome do visitante?") || r1.Plano.Pendente != "pergunta" {
		t.Fatalf("as perguntas deviam sair juntas, uma vez só: %q %+v", r1.Resposta, r1.Plano)
	}
	r2, err := Continuar(context.Background(), ag, r1.Plano, "sim, o João", r1.Mensagens)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Plano.Status != PlanoConcluido || r2.Resposta != "CONSOLIDADO" {
		t.Fatalf("depois da resposta o plano roda até o fim: %+v %q", r2.Plano, r2.Resposta)
	}
	var feitos int
	for _, it := range r2.Plano.Itens {
		if it.Status == StatusFeito {
			feitos++
		}
	}
	if feitos != 3 { // pergunta (feita ao receber a resposta), t1 e t2 (self)
		t.Fatalf("itens feitos: %+v", r2.Plano.Itens)
	}
	if it := r2.Plano.item("t1"); it == nil || !strings.Contains(it.Resultado, "João liberado") {
		t.Fatalf("o caso devia ver a resposta do morador: %+v", r2.Plano.Itens)
	}
}

// ReAct: um passo por ação com observação; caso de uso inline e encerramento com a
// resposta final; alvo desconhecido vira observação em vez de quebrar.
func TestReActPassoAPasso(t *testing.T) {
	cat, a := catalogoExemplo(t)
	passo := 0
	script := &roteiro{resposta: func(sistema, usuario, primeiroUsuario string) string {
		switch {
		case strings.Contains(sistema, "modo ReAct"):
			passo++
			switch passo {
			case 1:
				return `{"pensamento":"preciso do regimento","checklist":[{"id":"t1","texto":"horário de obras","status":"pendente"}],"acao":{"tipo":"caso","alvo":"inexistente","item":"t1"}}`
			case 2:
				if !strings.Contains(usuario, "opção desconhecida: inexistente") {
					return `{"acao":{"tipo":"encerrar","texto":"ERRO: a observação do alvo desconhecido não chegou"}}`
				}
				return `{"pensamento":"vou ao regimento","checklist":[{"id":"t1","texto":"horário de obras","status":"pendente"}],"acao":{"tipo":"caso","alvo":"regimento","item":"t1"}}`
			default:
				if !strings.Contains(usuario, "t1 (regimento) feito: Obras só") {
					return `{"acao":{"tipo":"encerrar","texto":"ERRO: a observação do caso não chegou: ` + usuario + `"}}`
				}
				return `{"pensamento":"resolvido","checklist":[{"id":"t1","texto":"horário de obras","status":"feito"}],"acao":{"tipo":"encerrar","texto":"Obras só em dias úteis, das 8h às 18h."}}`
			}
		}
		return "Obras só em dias úteis, das 8h às 18h (art. 12)."
	}}
	ag, err := Build(context.Background(), cat, a, Config{ChatModel: script, Env: envModo("react"), Identidade: Identidade{Authenticated: true, AccountID: "cond-001"}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Run(context.Background(), ag, "pode fazer obra no sábado?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Resposta != "Obras só em dias úteis, das 8h às 18h." || r.Plano == nil || r.Plano.Status != PlanoConcluido || r.Plano.Iteracao != 3 {
		t.Fatalf("react devia encerrar no 3º passo: %q %+v", r.Resposta, r.Plano)
	}
	if it := r.Plano.item("t1"); it == nil || it.Status != StatusFeito || it.Alvo != "regimento" {
		t.Fatalf("t1 devia estar feito pelo regimento: %+v", r.Plano.Itens)
	}
	if !temPasso(r.Rota, "modo", "react") || !temPasso(r.Rota, "plano", "passo 2") || !temPasso(r.Rota, "item", "t1 feito por regimento") {
		t.Fatalf("trace: %+v", r.Rota)
	}
}

// ReAct: a pergunta do próprio orquestrador pausa; a resposta do morador vira
// observação; o limite de passos consolida o que há em vez de rodar para sempre.
func TestReActPerguntaELimite(t *testing.T) {
	cat, a := catalogoExemplo(t)
	passo := 0
	script := &roteiro{resposta: func(sistema, usuario, primeiroUsuario string) string {
		switch {
		case strings.Contains(sistema, "modo ReAct"):
			passo++
			if passo == 1 {
				return `{"checklist":[{"id":"t1","texto":"descobrir a unidade","status":"pendente"}],"acao":{"tipo":"perguntar","texto":"De qual unidade você fala?"}}`
			}
			if passo == 2 && !strings.Contains(usuario, "morador respondeu a pergunta1: 101") {
				return `{"acao":{"tipo":"encerrar","texto":"ERRO: resposta do morador não virou observação"}}`
			}
			return `{"checklist":[{"id":"t1","texto":"descobrir a unidade","status":"feito"}],"acao":{"tipo":"responder","texto":"anotado"}}` // nunca encerra
		case strings.Contains(sistema, "Redija a resposta final"):
			return "CONSOLIDADO"
		}
		return "?"
	}}
	a.Config = map[string]any{"orquestracao": map[string]any{"modo": "react", "max_iteracoes": 3}}
	ag, err := Build(context.Background(), cat, a, Config{ChatModel: script, Env: semEnv, Identidade: Identidade{Authenticated: true, AccountID: "cond-001"}})
	if err != nil {
		t.Fatal(err)
	}
	r1, err := Run(context.Background(), ag, "tem encomenda pra mim?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Resposta != "De qual unidade você fala?" || r1.Plano.Pendente != "pergunta1" {
		t.Fatalf("a pergunta do orquestrador devia pausar: %q %+v", r1.Resposta, r1.Plano)
	}
	r2, err := Continuar(context.Background(), ag, r1.Plano, "101", r1.Mensagens)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Plano.Status != PlanoConcluido || r2.Resposta != "CONSOLIDADO" || !temPasso(r2.Rota, "aviso", "máximo de passos") {
		t.Fatalf("no limite de passos consolida: %q %+v %+v", r2.Resposta, r2.Plano, r2.Rota)
	}
}

// No modo router nada muda: Resultado sem plano e a passada de rota de sempre.
func TestRouterSegueIgual(t *testing.T) {
	cat, a := catalogoExemplo(t)
	ag, err := Build(context.Background(), cat, a, Config{Mock: true, Env: semEnv})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Run(context.Background(), ag, "posso fazer barulho depois das 22h?", nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Plano != nil || !temPasso(r.Rota, "rota", "") || temPasso(r.Rota, "modo", "") {
		t.Fatalf("router não produz plano: %+v", r.Rota)
	}
}

func TestAguardaMorador(t *testing.T) {
	sim := []string{
		"Para qual data?",
		"Confirma a reserva de Salão de festas em 15/11/2026 para Operacao Predial? (sim/não)",
		"Resumo:\n- salão, 15/11\n- unidade 109\nConfirma? (sim/não)",
		"Posso registrar assim? Responda sim ou não.",
		"Confirma a reserva para Operacao Predial? Por favor, responda apenas \"sim\" ou \"não\".",
		"Qual a unidade?\n\n[assets/rodape-administracao.md]",
	}
	nao := []string{
		"Reserva registrada, protocolo R-1.",
		"Obras só em dias úteis, das 8h às 18h.",
		`{"resposta": "pode?"}`,
		"",
	}
	for _, s := range sim {
		if !aguardaMorador(s) {
			t.Errorf("devia aguardar o morador: %q", s)
		}
	}
	for _, s := range nao {
		if aguardaMorador(s) {
			t.Errorf("não devia aguardar o morador: %q", s)
		}
	}
}

func temPasso(rota []Passo, tipo, trecho string) bool {
	for _, p := range rota {
		if p.Tipo == tipo && strings.Contains(p.Detalhe, trecho) {
			return true
		}
	}
	return false
}
