package pedidos

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

// CasoResumo é o que o extrator precisa saber de um caso de uso disponível.
type CasoResumo struct {
	ID        string   `json:"id"`
	Nome      string   `json:"nome"`
	Descricao string   `json:"descricao"`
	Exemplos  []string `json:"exemplos,omitempty"`
	Keywords  []string `json:"keywords,omitempty"`
	Slots     []*Slot  `json:"slots"`
	Agente    string   `json:"-"`
}

// Entrada de uma extração: a mensagem, o que veio antes, os casos possíveis e os
// pedidos abertos do contato.
type Entrada struct {
	Mensagem  string
	Historico []string // "morador: …" / "concierge: …", os últimos turnos
	Casos     []CasoResumo
	Abertos   []*Pedido
	Agora     time.Time
}

// PedidoExtraido é um pedido novo (Protocolo vazio) ou valores para um aberto.
type PedidoExtraido struct {
	Protocolo string         `json:"protocolo,omitempty"`
	Caso      string         `json:"caso"`
	Valores   map[string]any `json:"valores"`
}

type Extracao struct {
	Pedidos      []PedidoExtraido `json:"pedidos"`
	Confirmacoes []Confirmada     `json:"confirmacoes,omitempty"`
	Cancelar     []string         `json:"cancelar,omitempty"`
	Atendente    bool             `json:"atendente"`
	// Restante é o que não é pedido (dúvida, saudação, conversa): vai ao orquestrador.
	Restante string `json:"restante,omitempty"`
}

type Confirmada struct {
	Protocolo string `json:"protocolo"`
	Resposta  string `json:"resposta"` // sim | nao
}

// UnmarshalJSON aceita "resposta" como texto (sim/nao/yes/no) ou booleano.
func (c *Confirmada) UnmarshalJSON(raw []byte) error {
	var bruto struct {
		Protocolo string `json:"protocolo"`
		Resposta  any    `json:"resposta"`
	}
	if err := json.Unmarshal(raw, &bruto); err != nil {
		return err
	}
	c.Protocolo = bruto.Protocolo
	switch v := bruto.Resposta.(type) {
	case bool:
		if v {
			c.Resposta = "sim"
		} else {
			c.Resposta = "nao"
		}
	case string:
		switch normalizar(v) {
		case "sim", "yes", "true", "s", "confirmado", "confirma":
			c.Resposta = "sim"
		case "nao", "no", "false", "n", "cancela", "cancelado":
			c.Resposta = "nao"
		default:
			c.Resposta = normalizar(v)
		}
	}
	return nil
}

// Extrator mapeia uma mensagem nos pedidos. O LLM (harness) ou as Regras (sem modelo).
type Extrator interface {
	Extrair(ctx context.Context, in Entrada) (Extracao, error)
}

// Regras é o extrator sem modelo: confirmações por palavra, resposta à única pendência
// em aberto, caso de uso por palavra-chave do caso.yaml, data dd/mm no texto.
type Regras struct{}

var rePedeAtendente = regexp.MustCompile(`atendente|humano|falar com (alguem|uma pessoa|a administracao)|pessoa de verdade`)
var reData = regexp.MustCompile(`\b(\d{1,2}/\d{1,2}(?:/\d{2,4})?|\d{4}-\d{2}-\d{2})\b`)

func (Regras) Extrair(_ context.Context, in Entrada) (Extracao, error) {
	var out Extracao
	msg := strings.TrimSpace(in.Mensagem)
	n := normalizar(msg)
	if rePedeAtendente.MatchString(n) {
		out.Atendente = true
	}
	// confirmação pendente: sim/não vale para ela
	for _, p := range in.Abertos {
		if p.Estado == Confirmando {
			switch {
			case Confirmado(msg):
				out.Confirmacoes = append(out.Confirmacoes, Confirmada{Protocolo: p.Protocolo, Resposta: "sim"})
				return out, nil
			case Negado(msg):
				out.Confirmacoes = append(out.Confirmacoes, Confirmada{Protocolo: p.Protocolo, Resposta: "nao"})
				return out, nil
			}
		}
	}
	// cancelamento: "cancela", "desisto" (com o protocolo, ou o único pedido aberto)
	if ps := tokens(msg); len(ps) > 0 && (ps[0] == "cancela" || ps[0] == "cancelar" || ps[0] == "desisto" || ps[0] == "desiste") {
		for _, p := range in.Abertos {
			if strings.Contains(strings.ToUpper(msg), p.Protocolo) {
				out.Cancelar = append(out.Cancelar, p.Protocolo)
			}
		}
		if len(out.Cancelar) == 0 && len(in.Abertos) == 1 {
			out.Cancelar = append(out.Cancelar, in.Abertos[0].Protocolo)
		}
		if len(out.Cancelar) > 0 {
			return out, nil
		}
	}
	// uma só pendência em aberto: a mensagem responde a ela
	var pendentes []*Pedido
	for _, p := range in.Abertos {
		if p.Pendencia != nil && (p.Estado == Coletando) {
			pendentes = append(pendentes, p)
		}
	}
	if len(pendentes) == 1 && !out.Atendente {
		p := pendentes[0]
		resposta := true
		if len(p.Pendencia.Opcoes) > 0 {
			// pergunta com opções: uma frase que não casa com nenhuma (e não é curta) é outro assunto
			if _, ok := casarOpcao(msg, p.Pendencia.Opcoes); !ok && len(strings.Fields(msg)) > 2 {
				resposta = false
			}
		}
		if resposta {
			out.Pedidos = append(out.Pedidos, PedidoExtraido{Protocolo: p.Protocolo, Caso: p.Caso, Valores: map[string]any{p.Pendencia.Slot: msg}})
			return out, nil
		}
	}
	// caso novo por palavra-chave
	melhor, pontos := -1, 0
	for i, c := range in.Casos {
		s := 0
		for _, k := range c.Keywords {
			if k = normalizar(k); k != "" && strings.Contains(n, k) {
				s++
			}
		}
		if s > pontos {
			melhor, pontos = i, s
		}
	}
	if melhor >= 0 {
		c := in.Casos[melhor]
		vals := map[string]any{}
		if m := reData.FindString(msg); m != "" {
			for _, s := range c.Slots {
				if s.Tipo == "date" {
					vals[s.Nome] = m
					break
				}
			}
		}
		pe := PedidoExtraido{Caso: c.ID, Valores: vals}
		for _, p := range in.Abertos {
			if p.Caso == c.ID {
				pe.Protocolo = p.Protocolo // o mesmo caso aberto recebe os valores
				break
			}
		}
		out.Pedidos = append(out.Pedidos, pe)
		return out, nil
	}
	if !out.Atendente {
		out.Restante = msg
	}
	return out, nil
}

var palavrasDeSim = map[string]bool{"sim": true, "s": true, "ss": true, "ok": true, "okay": true, "pode": true, "confirmo": true, "confirma": true, "confirmado": true, "confirmar": true, "isso": true, "claro": true, "positivo": true, "beleza": true, "bora": true, "fechado": true, "vai": true, "vamos": true, "certo": true, "correto": true, "exato": true, "yes": true, "manda": true, "aceito": true}
var palavrasDeNao = map[string]bool{"nao": true, "n": true, "nunca": true, "cancela": true, "cancelar": true, "espera": true, "ainda": true, "negativo": true, "deixa": true, "desisto": true}

func tokens(mensagem string) []string {
	return strings.FieldsFunc(normalizar(mensagem), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
}

// Confirmado: a mensagem é um sim explícito (mesma regra da guarda de escrita do harness).
func Confirmado(mensagem string) bool {
	ps := tokens(mensagem)
	if len(ps) == 0 {
		return false
	}
	for _, p := range ps {
		if palavrasDeNao[p] {
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

// Negado: a mensagem começa com um não (ou cancela).
func Negado(mensagem string) bool {
	ps := tokens(mensagem)
	return len(ps) > 0 && palavrasDeNao[ps[0]]
}
