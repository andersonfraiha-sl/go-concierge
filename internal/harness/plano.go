package harness

// Modos de orquestração ART (Automatic Reasoning and Tool-use), ao lado do roteador:
//
//	router   — uma rota por mensagem (o padrão de sempre: routers/rotas.md).
//	planning — uma rodada monta o TODO inteiro: cada item vinculado a um caso de uso
//	           (subagente), a um recurso de leitura declarado ou a "self"; os itens
//	           rodam em ordem; o que o caso perguntar ao morador pausa o plano (o próximo
//	           turno vai direto ao item pendente); no fim, uma resposta consolidada.
//	react    — loop pensar → agir → observar com um checklist: uma ação por passo (caso,
//	           recurso, perguntar, responder, encerrar), observação a cada ação.
//
// Diferença para o roteador: vários pedidos numa mensagem viram vários itens, e a
// pendência (qual item espera o morador) fica no Plano, guardado pela sessão, em vez
// de depender de o roteador reescolher a mesma rota a cada turno.
//
// Configuração no agent.yaml do orquestrador (ORQUESTRACAO_MODO no ambiente vence):
//
//	orquestracao:
//	  modo: planning          # router | planning | react
//	  bind_confidence: 0.8    # abaixo disso o item vira pergunta ao morador
//	  max_itens: 5
//	  max_iteracoes: 6        # react

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/okf"
)

const (
	ModoRouter   = "router"
	ModoPlanning = "planning"
	ModoReAct    = "react"

	AlvoSelf   = "self"
	AlvoNenhum = "nenhum"

	TipoCaso    = "caso"
	TipoRecurso = "recurso"
	TipoSelf    = "self"
	TipoNenhum  = "nenhum"

	StatusPendente   = "pendente"
	StatusAguardando = "aguardando" // o item perguntou algo ao morador
	StatusFeito      = "feito"
	StatusFalhou     = "falhou"
	StatusBloqueado  = "bloqueado" // sem vínculo aceito: vira pergunta ou fica para o fim

	PlanoExecutando = "executando"
	PlanoAguardando = "aguardando_morador"
	PlanoConcluido  = "concluido"
)

// Orquestracao é a configuração do modo.
type Orquestracao struct {
	Modo           string  `json:"modo"`
	BindConfidence float64 `json:"bind_confidence"`
	MaxItens       int     `json:"max_itens"`
	MaxIteracoes   int     `json:"max_iteracoes"`
}

// OrquestracaoDe lê `orquestracao:` do agent.yaml; ORQUESTRACAO_MODO do ambiente vence.
func OrquestracaoDe(a *okf.Agent, env func(string) string) Orquestracao {
	o := Orquestracao{Modo: ModoRouter, BindConfidence: 0.8, MaxItens: 5, MaxIteracoes: 6}
	if a != nil {
		if m, ok := a.Config["orquestracao"].(map[string]any); ok {
			if v, ok := m["modo"].(string); ok && v != "" {
				o.Modo = strings.ToLower(strings.TrimSpace(v))
			}
			if v, ok := numero(m["bind_confidence"]); ok && v >= 0.5 && v <= 1 {
				o.BindConfidence = v
			}
			if v, ok := numero(m["max_itens"]); ok && v >= 1 && v <= 10 {
				o.MaxItens = int(v)
			}
			if v, ok := numero(m["max_iteracoes"]); ok && v >= 1 && v <= 15 {
				o.MaxIteracoes = int(v)
			}
		}
	}
	if env != nil {
		if v := strings.ToLower(strings.TrimSpace(env("ORQUESTRACAO_MODO"))); v != "" {
			o.Modo = v
		}
	}
	if o.Modo != ModoPlanning && o.Modo != ModoReAct {
		o.Modo = ModoRouter
	}
	return o
}

func numero(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

// Item é uma linha do TODO (planning) ou do checklist (react).
type Item struct {
	ID        string         `json:"id"`
	Texto     string         `json:"texto"`
	Alvo      string         `json:"alvo"` // nome do subagente | nome do recurso | self | nenhum
	Tipo      string         `json:"tipo"` // caso | recurso | self | nenhum
	Confianca float64        `json:"confianca,omitempty"`
	Params    map[string]any `json:"params,omitempty"`
	Status    string         `json:"status"`
	Resultado string         `json:"resultado,omitempty"`
}

// Plano é o estado da orquestração entre turnos: a sessão o guarda enquanto houver
// item esperando o morador.
type Plano struct {
	Modo        string   `json:"modo"`
	Pedido      string   `json:"pedido"`
	Itens       []Item   `json:"itens"`
	Perguntas   []string `json:"perguntas,omitempty"`
	Instrucao   string   `json:"instrucao_final,omitempty"`
	Pendente    string   `json:"pendente,omitempty"` // id do item que espera o morador
	Iteracao    int      `json:"iteracao"`
	Status      string   `json:"status"`
	Observacoes []string `json:"observacoes,omitempty"` // react: o que cada passo observou
	Parciais    []string `json:"parciais,omitempty"`    // react: respostas parciais já dadas
	// Contexto: a última pergunta do próprio orquestrador e a resposta do morador, que os
	// casos de uso executados neste turno precisam ver (o morador respondeu ao plano, não
	// ao caso). Zerado a cada retomada.
	Contexto string `json:"contexto,omitempty"`
}

func (p *Plano) item(id string) *Item {
	for i := range p.Itens {
		if p.Itens[i].ID == id {
			return &p.Itens[i]
		}
	}
	return nil
}

// candidato é uma opção de vínculo: um caso de uso (subagente) ou um recurso de leitura.
type candidato struct {
	Nome   string
	Tipo   string
	Texto  string
	Params []string
}

// candidatos lista o que o orquestrador pode vincular: os subagentes (casos de uso) e os
// recursos de leitura que ele declara e tem permissão de usar. Escrita nunca é vínculo
// direto: só pelo caso de uso, com a confirmação do morador.
func (ag *Agente) candidatos() []candidato {
	var out []candidato
	for _, nome := range ag.nomesDeRotas() {
		sub := ag.Subagentes[nome]
		if sub.Def.Nome == "observador-sessao" {
			continue // observa, nunca atende
		}
		texto := strings.TrimSpace(sub.Def.Descricao)
		var params []string
		if c := sub.Def.Caso; c != nil {
			if c.Description != "" {
				texto = strings.TrimSpace(c.Description)
			}
			if len(c.Examples) > 0 {
				texto += " Exemplos: " + strings.Join(c.Examples, "; ") + "."
			}
			for _, p := range c.Params {
				params = append(params, p.Name)
			}
		}
		out = append(out, candidato{Nome: nome, Tipo: TipoCaso, Texto: texto, Params: params})
	}
	for _, p := range ag.Permissoes() {
		if !p.Permitido || p.Recurso == nil || p.Recurso.Write {
			continue
		}
		var req []string
		for _, pm := range p.Recurso.Params {
			if pm.Required {
				req = append(req, pm.Name)
			}
		}
		out = append(out, candidato{Nome: p.Nome, Tipo: TipoRecurso, Texto: strings.TrimSpace(p.Recurso.Description), Params: req})
	}
	return out
}

func (ag *Agente) candidatoPorNome(nome string) (candidato, bool) {
	for _, c := range ag.candidatos() {
		if c.Nome == nome {
			return c, true
		}
	}
	return candidato{}, false
}

func opcoes(cs []candidato) string {
	var b strings.Builder
	for _, c := range cs {
		params := ""
		if len(c.Params) > 0 {
			params = " | params: " + strings.Join(c.Params, ", ")
		}
		fmt.Fprintf(&b, "- [%s] %s: %s%s\n", c.Tipo, c.Nome, c.Texto, params)
	}
	return b.String()
}

func (ag *Agente) cabecalhoPersona() string {
	if p := strings.TrimSpace(okf.StripFrontmatter(ag.Def.Persona)); p != "" {
		return "# Persona\n" + p + "\n\n"
	}
	return ""
}

// ---- planning --------------------------------------------------------------------

const promptPlanning = `Você é o ORQUESTRADOR do concierge de um condomínio (WhatsApp), em modo PLANNING.
Transforme o pedido do morador num TODO completo de uma vez, vinculando cada item à opção que o resolve,
para atender com o MÍNIMO de idas e vindas. Responda SÓ com JSON, neste formato:
{"itens":[{"id":"t1","texto":"o que fazer","alvo":"nome de uma opção ou self ou nenhum","confianca":0.0,"params":{}}],
 "perguntas":["o que falta perguntar ao morador para escolher uma opção (vazio se nada)"],
 "instrucao_final":"como redigir a resposta consolidada"}
Regras:
- no máximo %d itens; alvo é EXATAMENTE o nome de uma opção abaixo, ou "self" (dúvida que você mesmo responde
  com o que sabe), ou "nenhum" (nada atende; explique no texto);
- confianca (0 a 1) é quanto você tem certeza de que a opção atende o item;
- casos de uso perguntam os próprios dados (unidade, data, período): NUNCA pergunte isso aqui; só pergunte o
  que é preciso para escolher a opção ou para um recurso de leitura;
- vários pedidos numa mensagem viram vários itens, na ordem do morador; escrita só por caso de uso;
- não invente dados; saudação ou conversa sem pedido: itens vazios e instrucao_final diz como responder.

## Opções (casos de uso e recursos de leitura)
%s`

type rawPlano struct {
	Itens []struct {
		ID        string         `json:"id"`
		Texto     string         `json:"texto"`
		Alvo      string         `json:"alvo"`
		Confianca float64        `json:"confianca"`
		Params    map[string]any `json:"params"`
	} `json:"itens"`
	Perguntas []string `json:"perguntas"`
	Instrucao string   `json:"instrucao_final"`
}

// planejar monta o plano numa chamada e valida os vínculos em código.
func (ag *Agente) planejar(ctx context.Context, o Orquestracao, mensagem string, historico []*schema.Message) (*Plano, Uso, error) {
	cs := ag.candidatos()
	sys := ag.cabecalhoPersona() + fmt.Sprintf(promptPlanning, o.MaxItens, opcoes(cs))
	msgs := []*schema.Message{schema.SystemMessage(sys)}
	msgs = append(msgs, ultimas(historico, 6)...)
	msgs = append(msgs, schema.UserMessage("Pedido: "+mensagem))
	out, err := ag.modelo.Generate(ctx, msgs)
	if err != nil {
		return nil, Uso{}, fmt.Errorf("harness: planning de %s: %w", ag.Def.Rel, err)
	}
	var uso Uso
	uso.registra(out)
	var raw rawPlano
	if err := json.Unmarshal([]byte(extrairJSON(out.Content)), &raw); err != nil {
		return nil, uso, fmt.Errorf("harness: planning de %s: plano inválido: %w", ag.Def.Rel, err)
	}
	p := &Plano{Modo: ModoPlanning, Pedido: mensagem, Instrucao: strings.TrimSpace(raw.Instrucao), Status: PlanoExecutando}
	for i, it := range raw.Itens {
		if i >= o.MaxItens {
			break
		}
		item := Item{ID: it.ID, Texto: strings.TrimSpace(it.Texto), Alvo: strings.TrimSpace(it.Alvo), Confianca: it.Confianca, Params: it.Params, Status: StatusPendente}
		if item.ID == "" {
			item.ID = "t" + strconv.Itoa(i+1)
		}
		ag.vincular(&item, o.BindConfidence)
		if item.Status == StatusBloqueado && item.Alvo != AlvoNenhum && item.Confianca > 0 {
			p.Perguntas = append(p.Perguntas, "Você quer "+minusculaInicial(strings.TrimRight(item.Texto, ".?!"))+"?")
		}
		p.Itens = append(p.Itens, item)
	}
	// As perguntas do modelo só valem se há algo a decidir: item sem vínculo aceito ou
	// recurso sem parâmetro obrigatório. Com tudo vinculado, os casos de uso perguntam o
	// que lhes falta (data, unidade…) e o plano não pode duplicar a pergunta.
	if ag.precisaDecidir(p) {
		for _, q := range raw.Perguntas {
			if q = strings.TrimSpace(q); q != "" {
				p.Perguntas = append(p.Perguntas, q)
			}
		}
	} else if len(raw.Perguntas) > 0 {
		p.Perguntas = nil
	}
	return p, uso, nil
}

func (ag *Agente) precisaDecidir(p *Plano) bool {
	for _, it := range p.Itens {
		if it.Status == StatusBloqueado && it.Alvo != AlvoNenhum {
			return true
		}
		if it.Tipo == TipoRecurso {
			c, _ := ag.candidatoPorNome(it.Alvo)
			for _, req := range c.Params {
				if _, ok := it.Params[req]; !ok {
					return true
				}
			}
		}
	}
	return false
}

// vincular valida o alvo em código: o modelo propõe, o harness confere o nome e a confiança.
func (ag *Agente) vincular(it *Item, bind float64) {
	switch strings.ToLower(it.Alvo) {
	case AlvoSelf:
		it.Alvo, it.Tipo, it.Status = AlvoSelf, TipoSelf, StatusFeito
		return
	case AlvoNenhum, "":
		it.Alvo, it.Tipo, it.Status = AlvoNenhum, TipoNenhum, StatusBloqueado
		return
	}
	c, ok := ag.candidatoPorNome(it.Alvo)
	if !ok {
		it.Tipo, it.Status = TipoNenhum, StatusBloqueado
		it.Resultado = "opção desconhecida: " + it.Alvo
		it.Alvo = AlvoNenhum
		return
	}
	it.Tipo = c.Tipo
	if it.Confianca > 0 && it.Confianca < bind {
		it.Status = StatusBloqueado // o morador confirma antes
		return
	}
	it.Status = StatusPendente
}

// ---- react -----------------------------------------------------------------------

const promptReAct = `Você é o ORQUESTRADOR do concierge de um condomínio (WhatsApp), em modo ReAct.
A cada passo: pense, mantenha um CHECKLIST do que o morador precisa e escolha UMA ação. Responda SÓ com JSON:
{"pensamento":"...","checklist":[{"id":"t1","texto":"...","status":"pendente|feito|falhou|bloqueado"}],
 "acao":{"tipo":"caso|recurso|perguntar|responder|encerrar","alvo":"nome da opção","item":"id do item","params":{},"texto":"pergunta ou resposta"}}
Ações:
- caso: executar um caso de uso (alvo = nome). Ele conduz as próprias perguntas e confirmações: se um caso
  resolve o item, escolha caso JÁ, sem perguntar antes (unidade, data, período…).
- recurso: consultar um recurso de leitura (alvo = nome; params obrigatórios).
- perguntar: perguntar algo ao morador (texto), só quando falta informação para escolher a opção.
- responder: dar agora parte da resposta (texto) e seguir para o próximo item.
- encerrar: a resposta final ao morador (texto), quando o checklist estiver resolvido.
Regras: use SÓ as opções abaixo; não repita uma ação já observada; não invente dados; escrita só por caso;
no máximo %d itens; respostas curtas, em português, próprias para WhatsApp.

## Opções (casos de uso e recursos de leitura)
%s`

type rawReAct struct {
	Pensamento string `json:"pensamento"`
	Checklist  []struct {
		ID     string `json:"id"`
		Texto  string `json:"texto"`
		Status string `json:"status"`
	} `json:"checklist"`
	Acao struct {
		Tipo   string         `json:"tipo"`
		Alvo   string         `json:"alvo"`
		Item   string         `json:"item"`
		Params map[string]any `json:"params"`
		Texto  string         `json:"texto"`
	} `json:"acao"`
}

func (ag *Agente) passoReAct(ctx context.Context, o Orquestracao, p *Plano, historico []*schema.Message) (rawReAct, Uso, error) {
	sys := ag.cabecalhoPersona() + fmt.Sprintf(promptReAct, o.MaxItens, opcoes(ag.candidatos()))
	var b strings.Builder
	fmt.Fprintf(&b, "Pedido original: %s\n\n", p.Pedido)
	if len(p.Itens) > 0 {
		cl, _ := json.Marshal(p.Itens)
		fmt.Fprintf(&b, "Checklist atual: %s\n\n", cl)
	}
	if len(p.Observacoes) > 0 {
		b.WriteString("Observações:\n- " + strings.Join(p.Observacoes, "\n- ") + "\n\n")
	}
	fmt.Fprintf(&b, "Passo %d de no máximo %d.", p.Iteracao+1, o.MaxIteracoes)
	msgs := []*schema.Message{schema.SystemMessage(sys)}
	msgs = append(msgs, ultimas(historico, 6)...)
	msgs = append(msgs, schema.UserMessage(b.String()))
	out, err := ag.modelo.Generate(ctx, msgs)
	if err != nil {
		return rawReAct{}, Uso{}, fmt.Errorf("harness: react de %s: %w", ag.Def.Rel, err)
	}
	var uso Uso
	uso.registra(out)
	var raw rawReAct
	if err := json.Unmarshal([]byte(extrairJSON(out.Content)), &raw); err != nil {
		return rawReAct{}, uso, fmt.Errorf("harness: react de %s: passo inválido: %w", ag.Def.Rel, err)
	}
	return raw, uso, nil
}

// ---- execução comum ----------------------------------------------------------------

// executarItem roda um item: caso de uso inline (a pergunta do caso pausa o plano) ou
// recurso de leitura pela mesma guarda de chamar_recurso. Devolve a pergunta ao
// morador quando o item ficou aguardando.
func (ag *Agente) executarItem(ctx context.Context, col *coletor, p *Plano, it *Item, mensagem string, historico []*schema.Message) (pergunta string, err error) {
	rel := ag.Def.Rel
	switch it.Tipo {
	case TipoCaso:
		sub, ok := ag.Subagentes[it.Alvo]
		if !ok {
			it.Status, it.Resultado = StatusFalhou, "caso de uso desconhecido"
			return "", nil
		}
		// O caso vê SÓ o seu pedido: na primeira execução a mensagem é o texto do item (com
		// o escopo); nas retomadas a mensagem do morador vai limpa ("sim", "dia 15") e o
		// pedido original, no histórico, é trocado pelo texto do item.
		escopo := escopoDoItem(p, it)
		hist := historico
		if mensagem != col.mensagem {
			if p.Contexto != "" {
				mensagem += "\n\n" + p.Contexto
			}
			mensagem += escopo
		} else {
			hist = make([]*schema.Message, 0, len(historico))
			for _, m := range historico {
				if m.Role == schema.User && strings.TrimSpace(m.Content) == strings.TrimSpace(p.Pedido) {
					m = schema.UserMessage(it.Texto + escopo)
				}
				hist = append(hist, m)
			}
		}
		r, err := run(ctx, sub, mensagem, hist, col.profundidade+1)
		if err != nil {
			return "", err
		}
		col.absorve(r)
		if r.Transbordo != "" {
			*col.transbordo = r.Transbordo
		}
		if aguardaMorador(r.Resposta) {
			it.Status, it.Resultado = StatusAguardando, r.Resposta
			p.Pendente, p.Status = it.ID, PlanoAguardando
			col.passo(rel, "item", it.ID+" aguardando o morador: "+corta(r.Resposta, 120))
			return r.Resposta, nil
		}
		it.Status, it.Resultado = StatusFeito, strings.TrimSpace(r.Resposta)
		col.passo(rel, "item", it.ID+" feito por "+it.Alvo)
	case TipoRecurso:
		out, err := ag.chamarRecurso(ctx, &argsRecurso{Nome: it.Alvo, Params: it.Params})
		if err != nil {
			return "", err
		}
		if e := erroDeSaida(out); e != "" {
			it.Status, it.Resultado = StatusFalhou, e
			col.passo(rel, "item", it.ID+" falhou: "+corta(e, 120))
			return "", nil
		}
		raw, _ := json.Marshal(out["resultado"])
		it.Status, it.Resultado = StatusFeito, string(raw)
		col.passo(rel, "item", it.ID+" consultou "+it.Alvo)
	default:
		if it.Status == StatusPendente {
			it.Status = StatusFeito
		}
	}
	return "", nil
}

// escopoDoItem lembra ao caso de uso que ele atende SÓ o seu item: a mensagem original
// (no histórico) pode ter outros pedidos, e eles estão com outros itens do plano. Sem
// isso o caso tenta atender o resto ou transfere a conversa "por estar fora do escopo".
func escopoDoItem(p *Plano, it *Item) string {
	var outros []string
	for _, o := range p.Itens {
		if o.ID != it.ID && o.Tipo != "pergunta" && strings.TrimSpace(o.Texto) != "" {
			outros = append(outros, strings.TrimRight(strings.TrimSpace(o.Texto), ".?!"))
		}
	}
	if len(outros) == 0 {
		return ""
	}
	return "\n\n(Atenda apenas este pedido: " + strings.TrimRight(strings.TrimSpace(it.Texto), ".?!") + ". O restante da mensagem do morador — " + strings.Join(outros, "; ") + " — já está com outro especialista: não atenda, não comente nem transfira por causa disso.)"
}

func erroDeSaida(out saida) string {
	if out == nil {
		return "sem resultado"
	}
	if e, ok := out["error"].(string); ok {
		return e
	}
	return ""
}

// consolidar redige a resposta final a partir dos itens (na persona do orquestrador).
func (ag *Agente) consolidar(ctx context.Context, p *Plano, continuacao bool) (string, Uso, error) {
	var uso Uso
	var feitos, abertos []string
	for _, it := range p.Itens {
		switch it.Status {
		case StatusFeito:
			if it.Tipo == TipoSelf {
				feitos = append(feitos, it.Texto+" (responda você, com o que sabe)")
			} else if it.Resultado != "" {
				feitos = append(feitos, it.Texto+": "+it.Resultado)
			}
		case StatusFalhou, StatusBloqueado:
			abertos = append(abertos, it.Texto+" ("+it.Status+orDetalhe(it.Resultado)+")")
		}
	}
	if ag.Mock || ag.modelo == nil {
		partes := append([]string{}, feitos...)
		if len(abertos) > 0 {
			partes = append(partes, "Não consegui: "+strings.Join(abertos, "; "))
		}
		if len(partes) == 0 {
			partes = append(partes, respostaVazia)
		}
		return strings.Join(partes, "\n"), uso, nil
	}
	var b strings.Builder
	b.WriteString(ag.preambuloPersona())
	turno := "Esta é a PRIMEIRA mensagem da conversa: pode se apresentar em uma linha."
	if continuacao {
		turno = "A conversa já começou: NÃO cumprimente nem se apresente de novo; vá direto ao ponto."
	}
	fmt.Fprintf(&b, "# Tarefa\n\nRedija a resposta final ao morador, consolidando o atendimento abaixo numa mensagem só:\n- curta, cordial, em português do Brasil, própria para WhatsApp;\n- %s\n- preserve fatos, protocolos, horários, valores e links; não acrescente informação;\n- nada de JSON, nomes de campo, termos internos (item, caso de uso, recurso, plano);\n- o que não foi possível resolver, diga com clareza e ofereça a administração.\n", turno)
	if p.Instrucao != "" {
		b.WriteString("Instrução do plano: " + p.Instrucao + "\n")
	}
	var u strings.Builder
	fmt.Fprintf(&u, "Pedido do morador: %s\n\nResolvido:\n", p.Pedido)
	if len(feitos) == 0 {
		u.WriteString("- (nada)\n")
	}
	for _, f := range feitos {
		u.WriteString("- " + f + "\n")
	}
	if len(abertos) > 0 {
		u.WriteString("\nNão resolvido:\n- " + strings.Join(abertos, "\n- ") + "\n")
	}
	m, err := ag.modelo.Generate(ctx, []*schema.Message{schema.SystemMessage(b.String()), schema.UserMessage(u.String())})
	if err != nil {
		return "", uso, err
	}
	uso.registra(m)
	return strings.TrimSpace(m.Content), uso, nil
}

func orDetalhe(s string) string {
	if s == "" {
		return ""
	}
	return ": " + corta(s, 80)
}

// ---- entrada: nova mensagem ou continuação ---------------------------------------------

// orquestrar é o caminho do orquestrador nos modos planning e react (chamado por run
// quando o modo não é router).
func (ag *Agente) orquestrar(ctx context.Context, col *coletor, o Orquestracao, mensagem string, historico []*schema.Message) (*Resultado, error) {
	rel := ag.Def.Rel
	var p *Plano
	var uso Uso
	var err error
	switch o.Modo {
	case ModoPlanning:
		p, uso, err = ag.planejar(ctx, o, mensagem, historico)
		if err != nil {
			return nil, err
		}
		col.uso.soma(uso)
		col.passo(rel, "plano", fmt.Sprintf("TODO com %d item(ns), %d pergunta(s)", len(p.Itens), len(p.Perguntas)))
	default:
		p = &Plano{Modo: ModoReAct, Pedido: mensagem, Status: PlanoExecutando}
	}
	return ag.avancar(ctx, col, o, p, mensagem, historico, len(historico) > 0)
}

// Continuar retoma um plano guardado pela sessão com a nova mensagem do morador: ela
// vai ao item pendente (o caso de uso que perguntou) ou vira observação (react).
func Continuar(ctx context.Context, ag *Agente, p *Plano, mensagem string, historico []*schema.Message) (*Resultado, error) {
	if ag == nil || p == nil {
		return nil, fmt.Errorf("harness: plano ou agente nulo")
	}
	col := &coletor{mensagem: mensagem, transbordo: new(string)}
	ctx = context.WithValue(ctx, chaveColetor{}, col)
	o := OrquestracaoDe(ag.Def, ag.cfg.env)
	if p.Modo == ModoReAct {
		o.Modo = ModoReAct
	} else {
		o.Modo = ModoPlanning
	}
	rel := ag.Def.Rel
	it := p.item(p.Pendente)
	p.Pendente, p.Status, p.Contexto = "", PlanoExecutando, ""
	if it != nil {
		switch {
		case it.Tipo == TipoCaso:
			col.passo(rel, "item", it.ID+" recebe a resposta do morador")
			it.Status = StatusPendente
			if q, err := ag.executarItem(ctx, col, p, it, mensagem, historico); err != nil {
				return nil, err
			} else if q != "" {
				return ag.resultadoPendente(col, p, q, historico), nil
			}
		default: // pergunta do próprio orquestrador: vira observação (react) e contexto dos casos
			it.Status = StatusFeito
			p.Observacoes = append(p.Observacoes, "morador respondeu a "+it.ID+": "+corta(mensagem, 160))
			it.Resultado = corta(mensagem, 160)
			p.Contexto = "Contexto: o concierge perguntou ao morador \"" + corta(it.Texto, 200) + "\" e ele respondeu: \"" + corta(mensagem, 300) + "\"."
		}
	}
	return ag.avancar(ctx, col, o, p, mensagem, historico, true)
}

// avancar executa o que falta no plano: planning segue os itens em ordem; react roda
// passos até encerrar. Para quando um item fica aguardando o morador.
func (ag *Agente) avancar(ctx context.Context, col *coletor, o Orquestracao, p *Plano, mensagem string, historico []*schema.Message, continuacao bool) (*Resultado, error) {
	rel := ag.Def.Rel
	if o.Modo == ModoPlanning {
		// perguntas de escolha, todas de uma vez, antes de executar (ask once)
		if len(p.Perguntas) > 0 && p.Iteracao == 0 {
			p.Iteracao++
			q := "Para resolver tudo de uma vez, preciso de:\n" + numerado(p.Perguntas)
			if len(p.Perguntas) == 1 {
				q = p.Perguntas[0]
			}
			p.Perguntas = nil
			for i := range p.Itens {
				if p.Itens[i].Status == StatusBloqueado && p.Itens[i].Alvo != AlvoNenhum {
					p.Itens[i].Status = StatusPendente // a resposta do morador confirma; sem ela, o item roda do mesmo jeito
				}
			}
			it := Item{ID: "pergunta", Texto: q, Tipo: "pergunta", Status: StatusAguardando}
			p.Itens = append([]Item{it}, p.Itens...)
			p.Pendente, p.Status = it.ID, PlanoAguardando
			return ag.resultadoPendente(col, p, q, historico), nil
		}
		for i := range p.Itens {
			it := &p.Itens[i]
			if it.Status != StatusPendente {
				continue
			}
			texto := it.Texto
			if texto == "" {
				texto = mensagem
			}
			q, err := ag.executarItem(ctx, col, p, it, texto, historico)
			if err != nil {
				return nil, err
			}
			if q != "" {
				return ag.resultadoPendente(col, p, q, historico), nil
			}
		}
		return ag.concluir(ctx, col, p, historico, continuacao)
	}

	// react
	for p.Iteracao < o.MaxIteracoes {
		p.Iteracao++
		raw, uso, err := ag.passoReAct(ctx, o, p, historico)
		if err != nil {
			return nil, err
		}
		col.uso.soma(uso)
		col.passo(rel, "plano", fmt.Sprintf("passo %d: %s", p.Iteracao, corta(raw.Pensamento, 140)))
		ag.mesclarChecklist(p, raw)
		a := raw.Acao
		switch strings.ToLower(a.Tipo) {
		case TipoCaso, TipoRecurso:
			it := p.item(a.Item)
			if it == nil {
				it = &Item{ID: a.Item, Texto: corta(raw.Pensamento, 80)}
				if it.ID == "" {
					it.ID = "t" + strconv.Itoa(len(p.Itens)+1)
				}
				p.Itens = append(p.Itens, *it)
				it = p.item(it.ID)
			}
			it.Alvo, it.Params, it.Confianca = a.Alvo, a.Params, 1
			ag.vincular(it, 0)
			if it.Status == StatusBloqueado {
				p.Observacoes = append(p.Observacoes, "opção desconhecida: "+a.Alvo)
				continue
			}
			texto := it.Texto
			if texto == "" {
				texto = mensagem
			}
			q, err := ag.executarItem(ctx, col, p, it, texto, historico)
			if err != nil {
				return nil, err
			}
			if q != "" {
				return ag.resultadoPendente(col, p, q, historico), nil
			}
			p.Observacoes = append(p.Observacoes, it.ID+" ("+it.Alvo+") "+it.Status+": "+corta(it.Resultado, 200))
		case "perguntar":
			it := Item{ID: "pergunta" + strconv.Itoa(p.Iteracao), Texto: a.Texto, Tipo: "pergunta", Status: StatusAguardando}
			p.Itens = append(p.Itens, it)
			p.Pendente, p.Status = it.ID, PlanoAguardando
			return ag.resultadoPendente(col, p, a.Texto, historico), nil
		case "responder":
			if t := strings.TrimSpace(a.Texto); t != "" {
				p.Parciais = append(p.Parciais, t)
				p.Observacoes = append(p.Observacoes, "respondeu ao morador: "+corta(t, 120))
			}
		default: // encerrar
			resposta := strings.TrimSpace(a.Texto)
			if resposta == "" {
				resposta = respostaVazia
			}
			if len(p.Parciais) > 0 {
				resposta = strings.Join(append(p.Parciais, resposta), "\n\n")
			}
			p.Status = PlanoConcluido
			col.passo(rel, "resposta", corta(resposta, 160))
			return ag.resultadoFinal(col, p, resposta, historico), nil
		}
	}
	col.passo(rel, "aviso", "react: máximo de passos atingido; consolidando o que há")
	return ag.concluir(ctx, col, p, historico, continuacao)
}

func (ag *Agente) mesclarChecklist(p *Plano, raw rawReAct) {
	for _, c := range raw.Checklist {
		if c.ID == "" {
			continue
		}
		if it := p.item(c.ID); it != nil {
			if it.Status != StatusAguardando && it.Status != StatusFeito && c.Status != "" {
				it.Status = normalizarStatus(c.Status)
			}
			if c.Texto != "" {
				it.Texto = c.Texto
			}
			continue
		}
		p.Itens = append(p.Itens, Item{ID: c.ID, Texto: c.Texto, Status: normalizarStatus(c.Status)})
	}
}

func normalizarStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case StatusFeito, "done":
		return StatusFeito
	case StatusFalhou, "failed":
		return StatusFalhou
	case StatusBloqueado, "blocked":
		return StatusBloqueado
	}
	return StatusPendente
}

func (ag *Agente) concluir(ctx context.Context, col *coletor, p *Plano, historico []*schema.Message, continuacao bool) (*Resultado, error) {
	rel := ag.Def.Rel
	p.Status = PlanoConcluido
	resposta, uso, err := ag.consolidar(ctx, p, continuacao)
	if err != nil {
		col.passo(rel, "aviso", "consolidação falhou: "+err.Error())
		var partes []string
		for _, it := range p.Itens {
			if it.Status == StatusFeito && it.Resultado != "" && it.Tipo != TipoSelf {
				partes = append(partes, it.Resultado)
			}
		}
		resposta = strings.Join(partes, "\n")
		if resposta == "" {
			resposta = respostaVazia
		}
	} else {
		col.uso.soma(uso)
	}
	col.passo(rel, "entrega", corta(resposta, 160))
	return ag.resultadoFinal(col, p, resposta, historico), nil
}

func (ag *Agente) resultadoPendente(col *coletor, p *Plano, pergunta string, historico []*schema.Message) *Resultado {
	return ag.resultadoFinal(col, p, pergunta, historico)
}

func (ag *Agente) resultadoFinal(col *coletor, p *Plano, resposta string, historico []*schema.Message) *Resultado {
	msgs := make([]*schema.Message, 0, len(historico)+2)
	msgs = append(msgs, historico...)
	msgs = append(msgs, schema.UserMessage(col.mensagem), schema.AssistantMessage(resposta, nil))
	transbordo := guardaTransbordo(col, ag.Def.Rel, resposta)
	col.mu.Lock()
	defer col.mu.Unlock()
	return &Resultado{Resposta: resposta, Agente: ag.Def.Rel, Rota: col.passos, Uso: col.uso, Mensagens: msgs, Transbordo: transbordo, Plano: p, Registros: col.registros}
}

// ---- util --------------------------------------------------------------------------

var reAguarda = regexp.MustCompile(`\?|\bsim\s*(/|ou)\s*n[ãa]o\b`)

// aguardaMorador diz se a resposta do caso de uso espera o morador: termina perguntando
// ("Para qual data?") ou pedindo confirmação ("Confirma? (sim/não)"), mesmo depois de um
// resumo de algumas linhas. É mais largo que perguntaSimples, porque aqui o que importa
// é não fechar o item (e o plano) com uma pergunta ou confirmação em aberto.
func aguardaMorador(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len([]rune(s)) > 800 || strings.ContainsAny(s, "{}") {
		return false
	}
	linhas := strings.Split(s, "\n")
	for i := len(linhas) - 1; i >= 0; i-- {
		l := strings.TrimSpace(linhas[i])
		if l == "" || strings.HasPrefix(l, "[") { // assets/rodapés citados pelo caminho
			continue
		}
		return reAguarda.MatchString(l)
	}
	return false
}

// extrairJSON tolera texto em volta do JSON (cercas de código, frases).
func extrairJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			return s[i : j+1]
		}
	}
	return s
}

func numerado(itens []string) string {
	var b strings.Builder
	for i, it := range itens {
		fmt.Fprintf(&b, "%d. %s\n", i+1, it)
	}
	return strings.TrimRight(b.String(), "\n")
}

func minusculaInicial(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) == 0 {
		return s
	}
	return strings.ToLower(string(r[0])) + string(r[1:])
}
