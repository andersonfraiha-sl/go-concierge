package pedidos

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Ambiente executa recursos em nome do subagente do caso (permissões e guarda de
// escrita do harness): confirmacao é a mensagem que confirmou (vazia = não confirmou).
type Ambiente interface {
	Chamar(ctx context.Context, agente, recurso string, params map[string]any, confirmacao string) (any, error)
}

// Saida é o que o motor devolve depois de avançar um pedido até onde dá.
type Saida struct {
	Tipo   string  `json:"tipo"` // pergunta | confirmar | concluido | cancelado | formulario | erro
	Texto  string  `json:"texto"`
	Slot   string  `json:"slot,omitempty"`
	Opcoes []Opcao `json:"opcoes,omitempty"`
}

// Avancar percorre os passos do caso do início (idempotente: listagens são refeitas,
// execuções já feitas ficam em Dados) e para na primeira coisa que depende de alguém:
// uma pergunta ao morador, a confirmação, ou o fim.
func Avancar(ctx context.Context, ct *Contrato, p *Pedido, cart *Carteira, amb Ambiente) Saida {
	if !p.Aberto() {
		return Saida{Tipo: p.Estado, Texto: textoFinal(p)}
	}
	if ct.TemFormulario() {
		p.Formulario = true
		return Saida{Tipo: "formulario", Texto: "o caso tem um formulário dinâmico: segue pelo subagente"}
	}
	if p.Dados == nil {
		p.Dados = map[string]any{}
	}
	for _, passo := range ct.Passos {
		switch passo.Tipo {
		case "recurso":
			if passo.With == nil {
				// listagem: sempre refeita (as opções podem ter mudado)
				res, err := amb.Chamar(ctx, p.Agente, passo.Recurso, nil, "")
				if err != nil {
					return falha(p, fmt.Sprintf("%s: %v", passo.Recurso, err))
				}
				if passo.SaveAs != "" {
					p.Dados[passo.SaveAs] = res
				}
				continue
			}
			if passo.SaveAs != "" {
				if _, feito := p.Dados[passo.SaveAs]; feito {
					continue // já executado (idempotente)
				}
			}
			params, err := ct.ParamsDe(passo.With, p)
			if err != nil {
				return falha(p, err.Error())
			}
			confirmacao := ""
			if p.Confirmacao != nil && p.Confirmacao.Resposta == "sim" {
				confirmacao = "sim"
			}
			p.mudar(Executando, "")
			res, err := amb.Chamar(ctx, p.Agente, passo.Recurso, params, confirmacao)
			if err != nil {
				return falha(p, fmt.Sprintf("%s: %v", passo.Recurso, err))
			}
			chave := passo.SaveAs
			if chave == "" {
				chave = passo.Recurso
			}
			p.Dados[chave] = res
			p.registrar("executado", passo.Recurso, "")
			if m, ok := res.(map[string]any); ok {
				if prot, ok := m["protocolo"].(string); ok && prot != "" {
					if p.Resultado == nil {
						p.Resultado = &Resultado{}
					}
					p.Resultado.Protocolo = prot
				}
			}
		case "escolher":
			slot := ct.Slots[passo.Slot]
			ops := OpcoesDeDados(p.Dados[slot.De], slot)
			if v, ok := p.Valores[slot.Nome]; ok {
				if len(ops) == 0 {
					continue
				}
				if o, ok := casarOpcao(fmt.Sprint(v), ops); ok {
					if p.Rotulos[slot.Nome] != o.Rotulo || p.Valores[slot.Nome] != o.Valor {
						p.Valores[slot.Nome], p.Rotulos[slot.Nome] = o.Valor, o.Rotulo
					}
					continue
				}
				delete(p.Valores, slot.Nome)
				return pergunta(p, slot, ops, "não é uma das opções")
			}
			if slot.Reuse == "always" && cart != nil {
				if v, _, ok := cart.ValorReutilizavel(slot.Nome); ok {
					if o, ok := casarOpcao(fmt.Sprint(v), ops); ok {
						p.Valor(slot.Nome, o.Valor, o.Rotulo, OrigemMemoria)
						continue
					}
				}
			}
			switch len(ops) {
			case 0:
				msg := slot.Vazio
				if msg == "" {
					msg = "não há opções para " + strings.ToLower(slot.Titulo)
				}
				return falha(p, msg)
			case 1:
				p.Valor(slot.Nome, ops[0].Valor, ops[0].Rotulo, OrigemSistema)
			default:
				return pergunta(p, slot, ops, "")
			}
		case "coletar":
			for _, nome := range passo.Coleta {
				slot, ok := ct.Slots[nome]
				if !ok {
					continue
				}
				if _, tem := p.Valores[nome]; tem || !slot.Obrigatorio {
					continue
				}
				return pergunta(p, slot, slot.Opcoes, "")
			}
		case "confirmar":
			if p.Confirmacao == nil || p.Confirmacao.Resposta == "" {
				texto, err := Render(passo.Texto, ct.Vars(p))
				if err != nil {
					return falha(p, "confirmação: "+err.Error())
				}
				if p.Confirmacao == nil {
					p.Confirmacao = &Confirmacao{Texto: texto, Pedida: time.Now(), Vezes: 1}
					p.registrar("confirmacao", "pedida: "+texto, "")
				} else {
					p.Confirmacao.Texto = texto
					p.Confirmacao.Vezes++
				}
				p.mudar(Confirmando, "")
				return Saida{Tipo: "confirmar", Texto: texto}
			}
			if p.Confirmacao.Resposta == "nao" {
				p.mudar(Cancelado, "o morador não confirmou")
				return Saida{Tipo: "cancelado", Texto: "pedido cancelado a seu pedido"}
			}
		case "redigir", "formulario":
			// redigir: a persona escreve a partir do resultado; formulario já saiu acima
		}
	}
	texto := ""
	if strings.TrimSpace(ct.Output) != "" {
		t, err := Render(ct.Output, ct.Vars(p))
		if err != nil {
			return falha(p, "output: "+err.Error())
		}
		texto = t
	}
	if texto == "" {
		texto = p.CasoNome + " concluído."
	}
	if p.Resultado == nil {
		p.Resultado = &Resultado{}
	}
	p.Resultado.Texto, p.Resultado.Quando = texto, time.Now()
	p.Pendencia = nil
	p.mudar(Concluido, "")
	return Saida{Tipo: "concluido", Texto: texto}
}

func falha(p *Pedido, motivo string) Saida {
	p.Erro = motivo
	p.mudar(Falhou, motivo)
	return Saida{Tipo: "erro", Texto: motivo}
}

// pergunta registra a pendência (repetida conta as vezes) e devolve a pergunta pronta.
func pergunta(p *Pedido, slot *Slot, ops []Opcao, motivo string) Saida {
	if p.Pendencia != nil && p.Pendencia.Slot == slot.Nome {
		p.Pendencia.Vezes++
		if motivo == "" {
			motivo = p.Pendencia.Motivo // a rejeição registrada por Aplicar segue na pergunta
		}
	} else {
		p.Pendencia = &Pendencia{Slot: slot.Nome, Pergunta: slot.Pergunta, Vezes: 1, Desde: time.Now()}
	}
	p.Pendencia.Opcoes = ops
	p.Pendencia.Motivo = motivo
	p.mudar(Coletando, "")
	p.registrar("pergunta", slot.Nome+orMotivo(motivo), "")
	return Saida{Tipo: "pergunta", Texto: TextoDaPergunta(p.Pendencia), Slot: slot.Nome, Opcoes: ops}
}

// TextoDaPergunta monta a pergunta com as opções numeradas e, se houve erro, a correção.
func TextoDaPergunta(pd *Pendencia) string {
	var b strings.Builder
	if pd.Motivo != "" {
		b.WriteString("Não consegui usar a resposta (" + pd.Motivo + "). ")
	}
	b.WriteString(pd.Pergunta)
	if len(pd.Opcoes) > 0 && len(pd.Opcoes) <= 12 {
		var ops []string
		for i, o := range pd.Opcoes {
			ops = append(ops, fmt.Sprintf("%d) %s", i+1, o.Rotulo))
		}
		b.WriteString(" Opções: " + strings.Join(ops, ", ") + ".")
	}
	return b.String()
}

// OpcoesDeDados transforma a lista devolvida por um recurso nas opções de um choose.
func OpcoesDeDados(dados any, slot *Slot) []Opcao {
	// o retorno pode ser []map[string]any, []any ou um struct: o JSON nivela tudo
	if dados != nil {
		if raw, err := json.Marshal(dados); err == nil {
			var generico any
			if json.Unmarshal(raw, &generico) == nil {
				dados = generico
			}
		}
	}
	lista, ok := dados.([]any)
	if !ok {
		if m, ok := dados.(map[string]any); ok {
			for _, k := range []string{"itens", "items", "data", "resultado"} {
				if l, ok := m[k].([]any); ok {
					lista = l
					break
				}
			}
		}
	}
	var out []Opcao
	for _, it := range lista {
		m, ok := it.(map[string]any)
		if !ok {
			t := fmt.Sprint(it)
			out = append(out, Opcao{Valor: t, Rotulo: t})
			continue
		}
		val := str(m[slot.Valor])
		rot := str(m[slot.Rotulo])
		if val == "" {
			val = str(m["id"])
		}
		if rot == "" {
			for _, k := range []string{"label", "nome", "name", "titulo", "title"} {
				if rot = str(m[k]); rot != "" {
					break
				}
			}
		}
		if rot == "" {
			rot = val
		}
		if val != "" {
			out = append(out, Opcao{Valor: val, Rotulo: rot})
		}
	}
	return out
}

func textoFinal(p *Pedido) string {
	switch {
	case p.Resultado != nil && p.Resultado.Texto != "":
		return p.Resultado.Texto
	case p.Erro != "":
		return p.Erro
	}
	return p.Estado
}

// Política de pendências: nenhum pedido aberto dorme. A cada turno em que o morador não
// responde a uma pendência, ela volta à conversa; depois de AvisoAposVezes o morador é
// convidado a responder, cancelar ou falar com um atendente; depois de AtendenteAposVezes
// o pedido vai ao atendente com o resumo. Quem aplica é o pipeline do canal (atende).
const (
	AvisoAposVezes     = 3
	AtendenteAposVezes = 5
)

// Lembrete devolve a pergunta (ou confirmação) em aberto para repetir ao morador, já
// contando mais uma vez; vazio se o pedido não espera nada dele. Vezes é o total.
func (p *Pedido) Lembrete() (texto string, vezes int) {
	switch {
	case p.Estado == Confirmando && p.Confirmacao != nil && p.Confirmacao.Resposta == "":
		p.Confirmacao.Vezes++
		p.registrar("pergunta", "confirmação lembrada", "")
		return p.Confirmacao.Texto, p.Confirmacao.Vezes
	case p.Estado == Coletando && p.Pendencia != nil:
		p.Pendencia.Vezes++
		p.Pendencia.Motivo = ""
		p.registrar("pergunta", p.Pendencia.Slot+" lembrado", "")
		return TextoDaPergunta(p.Pendencia), p.Pendencia.Vezes
	}
	return "", 0
}

// Aplicar põe no pedido os valores extraídos de uma mensagem, validando cada um pelo
// slot. Devolve os rejeitados (slot → motivo). Valores de choose antes da listagem ficam
// crus e são conferidos no passo escolher.
func Aplicar(ct *Contrato, p *Pedido, valores map[string]any, origem string) map[string]string {
	rejeitados := map[string]string{}
	pendente := ""
	if p.Pendencia != nil {
		pendente = p.Pendencia.Slot
	}
	for _, nome := range ct.Ordem {
		v, ok := valores[nome]
		if !ok || v == nil {
			continue
		}
		slot := ct.Slots[nome]
		// o que o morador (ou o atendente) já informou não é trocado por extração de
		// outra mensagem: só o slot em aberto recebe valor. Evita o modelo "lembrar"
		// errado um valor antigo e sobrescrever o certo.
		if atual, ja := p.Origens[nome]; ja && (atual == OrigemMorador || atual == OrigemAtendente) && nome != pendente && origem != OrigemAtendente {
			continue
		}
		var ops []Opcao
		if slot.Tipo == "choose" {
			ops = OpcoesDeDados(p.Dados[slot.De], slot)
			if len(ops) == 0 {
				t := strings.TrimSpace(fmt.Sprint(v))
				if t == "" {
					continue
				}
				p.Valor(nome, t, t, origem)
				continue
			}
		}
		val, rot, motivo := slot.Validar(v, ops)
		if motivo != "" {
			rejeitados[nome] = motivo
			if p.Pendencia != nil && p.Pendencia.Slot == nome {
				p.Pendencia.Motivo = motivo
			}
			continue
		}
		p.Valor(nome, val, rot, origem)
	}
	return rejeitados
}

// Confirmar registra a resposta à confirmação pendente.
func Confirmar(p *Pedido, resposta, por string) {
	if p.Confirmacao == nil {
		p.Confirmacao = &Confirmacao{Pedida: time.Now()}
	}
	p.Confirmacao.Resposta, p.Confirmacao.Por, p.Confirmacao.Quando = resposta, por, time.Now()
	p.registrar("confirmacao", resposta, por)
}

// Cancelar encerra o pedido a pedido do morador ou do atendente.
func Cancelar(p *Pedido, por, motivo string) {
	if !p.Aberto() {
		return
	}
	p.Pendencia = nil
	p.registrar("estado", "cancelado por "+por+orMotivo(motivo), por)
	p.Estado = Cancelado
	p.AtualizadoEm = time.Now()
}

// AoAtendente leva o pedido ao humano com o resumo do contexto.
func AoAtendente(p *Pedido, resumo string) {
	p.Resumo = resumo
	p.mudar(ComAtendente, "")
	p.registrar("atendente", "pendência encaminhada ao atendente", "")
}
