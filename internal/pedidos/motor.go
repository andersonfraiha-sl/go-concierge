package pedidos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
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
			if passo.SkipIf != "" && p.Dados[passo.SkipIf] == true {
				continue
			}
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
			ops := opcoesDoPedido(p.Dados[slot.De], slot, p)
			if slot.Strict && len(ops) == 0 {
				msg := slot.Vazio
				if msg == "" {
					msg = "não há opções para " + strings.ToLower(slot.Titulo)
				}
				return falha(p, msg)
			}
			if condicaoEscolha(slot.AutoFirstIf, p) && len(ops) > 1 {
				ops = ops[:1]
			}
			if v, ok := p.Valores[slot.Nome]; ok {
				if len(ops) == 0 {
					continue
				}
				if o, ok := casarOpcao(fmt.Sprint(v), ops); ok {
					valor := valorDaOpcao(o)
					if p.Rotulos[slot.Nome] != o.Rotulo || fmt.Sprint(p.Valores[slot.Nome]) != fmt.Sprint(valor) {
						p.Valores[slot.Nome], p.Rotulos[slot.Nome] = valor, o.Rotulo
					}
					continue
				}
				delete(p.Valores, slot.Nome)
				return pergunta(p, slot, ops, "não é uma das opções")
			}
			if slot.Reuse == "always" && cart != nil {
				if v, _, ok := cart.ValorReutilizavel(slot.Nome); ok {
					if o, ok := casarOpcao(fmt.Sprint(v), ops); ok {
						p.Valor(slot.Nome, valorDaOpcao(o), o.Rotulo, OrigemMemoria)
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
				if condicaoEscolha(slot.AskIfSingleIf, p) {
					return pergunta(p, slot, ops, "")
				}
				p.Valor(slot.Nome, valorDaOpcao(ops[0]), ops[0].Rotulo, OrigemSistema)
			default:
				return pergunta(p, slot, ops, "")
			}
		case "reutilizar":
			if precisaPerguntarReutilizacao(ct, p, cart, passo) {
				decision := ct.Slots[passo.Slot]
				copy := *decision
				texto, err := Render(passo.Texto, ct.Vars(p))
				if err != nil {
					return falha(p, "pergunta de reutilização: "+err.Error())
				}
				copy.Pergunta = texto
				return pergunta(p, &copy, decision.Opcoes, "")
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
			if ct.Caso.ID == "liberacao_visitante" {
				if fmt.Sprint(p.Valores["categoria_motivo"]) == "GUEST" && !nomeVisitanteValido(fmt.Sprint(p.Valores["visitante_nome"])) {
					slot := ct.Slots["visitante_nome"]
					delete(p.Valores, "visitante_nome")
					delete(p.Rotulos, "visitante_nome")
					copy := *slot
					copy.Pergunta = "Qual o nome completo do visitante (nome e sobrenome)?"
					return pergunta(p, &copy, nil, "")
				}
				if !dataVisitaPermitida(p.Valores["data_visita"]) {
					return falha(p, "liberações para depois de amanhã ou datas futuras devem ser feitas pelo app Gruvi: https://gruvi.link/guest")
				}
			}
			texto, err := Render(passo.Texto, ct.Vars(p))
			if err != nil {
				return falha(p, "confirmação: "+err.Error())
			}
			valores, err := json.Marshal(p.Valores)
			if err != nil {
				return falha(p, "valores da confirmação: "+err.Error())
			}
			valoresMudaram := false
			if p.Confirmacao != nil && len(p.Confirmacao.Valores) > 0 {
				var anteriores bytes.Buffer
				if err := json.Compact(&anteriores, p.Confirmacao.Valores); err != nil {
					return falha(p, "valores anteriores da confirmação: "+err.Error())
				}
				valoresMudaram = !bytes.Equal(anteriores.Bytes(), valores)
			}
			if p.Confirmacao != nil && p.Confirmacao.Resposta != "nao" &&
				(p.Confirmacao.Texto != "" && p.Confirmacao.Texto != texto || valoresMudaram) {
				entregue := false
				p.Confirmacao.Resposta = ""
				p.Confirmacao.Por = ""
				p.Confirmacao.Quando = time.Time{}
				p.Confirmacao.Entregue = &entregue
			}
			if p.Confirmacao == nil || p.Confirmacao.Resposta == "" {
				if p.Confirmacao == nil {
					entregue := false
					p.Confirmacao = &Confirmacao{Texto: texto, Pedida: time.Now(), Vezes: 1, Entregue: &entregue, Valores: append(json.RawMessage(nil), valores...)}
					p.registrar("confirmacao", "pedida: "+texto, "")
				} else {
					if p.Confirmacao.Texto != texto {
						entregue := false
						p.Confirmacao.Entregue = &entregue
					}
					p.Confirmacao.Texto = texto
					p.Confirmacao.Valores = append(json.RawMessage(nil), valores...)
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
	if dados != nil {
		if raw, err := json.Marshal(dados); err == nil {
			var generico any
			if json.Unmarshal(raw, &generico) == nil {
				dados = generico
			}
		}
	}
	lista := localizarLista(dados, 0)
	var out []Opcao
	for _, it := range lista {
		m, ok := it.(map[string]any)
		if !ok {
			if slot.Strict {
				continue
			}
			t := fmt.Sprint(it)
			out = append(out, Opcao{Valor: t, Rotulo: t})
			continue
		}
		if !slot.Strict {
			val := str(m[slot.Valor])
			if val == "" {
				val = str(m["id"])
			}
			rot := str(m[slot.Rotulo])
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
			continue
		}
		campoValor := slot.Valor
		if campoValor == "" {
			campoValor = "id"
		}
		raw, existe := m[campoValor]
		if !existe || raw == nil {
			continue
		}
		if campoValor == "id_fastpass" && !idFastpassElegivel(raw) {
			continue
		}
		if slot.Rotulo == "reason" && !numeroID(raw) {
			continue
		}
		val := str(raw)
		rot := str(m[slot.Rotulo])
		if rot == "" {
			for _, k := range []string{"label", "unit", "reason", "description", "package_size", "tracking_code", "nome", "name", "titulo", "title"} {
				if rot = str(m[k]); rot != "" {
					break
				}
			}
		}
		if rot == "" {
			switch {
			case campoValor == "id_fastpass":
				rot = "Unidade"
			case slot.De == "encomendas":
				rot = fmt.Sprintf("Encomenda %d", len(out)+1)
			case slot.Rotulo == "reason":
				continue
			default:
				rot = val
			}
		}
		if val != "" {
			out = append(out, Opcao{Valor: val, Rotulo: rot, Dados: raw, Campos: m})
		}
	}
	return out
}

func localizarLista(value any, depth int) []any {
	if depth > 8 {
		return nil
	}
	if lista, ok := value.([]any); ok {
		return lista
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	for _, key := range []string{"units", "packages", "reasons", "visit_reasons", "items", "itens", "data", "resultado", "result", "content"} {
		if lista := localizarLista(m[key], depth+1); lista != nil {
			return lista
		}
	}
	for _, child := range m {
		if lista := localizarLista(child, depth+1); lista != nil {
			return lista
		}
	}
	return nil
}

func idFastpassElegivel(value any) bool {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case json.Number:
		numero, ok := new(big.Rat).SetString(string(v))
		return ok && numero.IsInt() && numero.Cmp(big.NewRat(-9007199254740991, 1)) >= 0 && numero.Cmp(big.NewRat(9007199254740991, 1)) <= 0
	}
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return false
	}
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n := rv.Int()
		return n >= -9007199254740991 && n <= 9007199254740991
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() <= 9007199254740991
	case reflect.Float32, reflect.Float64:
		n := rv.Float()
		return !math.IsNaN(n) && !math.IsInf(n, 0) && math.Trunc(n) == n && math.Abs(n) <= 9007199254740991
	default:
		return false
	}
}

func numeroID(value any) bool {
	switch v := value.(type) {
	case float64:
		return !math.IsNaN(v) && !math.IsInf(v, 0)
	case int, int64:
		return true
	default:
		return false
	}
}

func opcoesDoPedido(dados any, slot *Slot, p *Pedido) []Opcao {
	ops := OpcoesDeDados(dados, slot)
	if len(slot.Filtro) == 0 {
		return ops
	}
	var out []Opcao
	for _, op := range ops {
		valido := true
		for campo, regra := range slot.Filtro {
			esperado := fmt.Sprint(regra)
			if strings.HasPrefix(esperado, "{{.") && strings.HasSuffix(esperado, "}}") {
				chave := strings.TrimSuffix(strings.TrimPrefix(esperado, "{{."), "}}")
				esperado = fmt.Sprint(p.Valores[chave])
			}
			if esperado == "" || normalizar(str(op.Campos[campo])) != normalizar(esperado) {
				valido = false
				break
			}
		}
		if valido {
			out = append(out, op)
		}
	}
	return out
}

func valorDaOpcao(op Opcao) any {
	if op.Dados != nil {
		return op.Dados
	}
	return op.Valor
}

func condicaoEscolha(condicao string, p *Pedido) bool {
	partes := strings.SplitN(condicao, "=", 2)
	return len(partes) == 2 && strings.TrimSpace(fmt.Sprint(p.Valores[strings.TrimSpace(partes[0])])) == strings.TrimSpace(partes[1])
}

func precisaPerguntarReutilizacao(ct *Contrato, p *Pedido, cart *Carteira, passo Passo) bool {
	resolvido := "reutilizacao_resolvida_" + passo.Slot
	if p.Dados[resolvido] == true {
		return false
	}
	if cart == nil {
		p.Dados[resolvido] = true
		return false
	}
	if _, respondeu := p.Valores[passo.Slot]; respondeu {
		valores := p.Dados["reutilizacao_valores_"+passo.Slot]
		if strings.EqualFold(fmt.Sprint(p.Valores[passo.Slot]), "sim") {
			if mapa, ok := valores.(map[string]any); ok {
				for nome, dado := range mapa {
					if strings.HasSuffix(nome, "_label") {
						continue
					}
					label := fmt.Sprint(mapa[nome+"_label"])
					p.Valor(nome, dado, label, OrigemSistema)
				}
				for _, nome := range passo.Coleta {
					if nome == "motivo" {
						p.Dados["reutilizou_motivo"] = true
					}
				}
			}
		}
		p.Dados[resolvido] = true
		return false
	}
	valores := map[string]any{}
	labelPrincipal := ""
	for _, nome := range passo.Coleta {
		if _, existe := p.Valores[nome]; existe {
			p.Dados[resolvido] = true
			return false
		}
		valor, label, existe := cart.ValorReutilizavel(nome)
		if !existe {
			continue
		}
		if passo.De != "" {
			ops := opcoesDoPedido(p.Dados[passo.De], ct.Slots[nome], p)
			op, ok := casarOpcao(fmt.Sprint(valor), ops)
			if !ok || len(ops) < 2 {
				continue
			}
			valor, label = valorDaOpcao(op), op.Rotulo
		}
		valores[nome] = valor
		valores[nome+"_label"] = label
		if labelPrincipal == "" || nome == "motivo" {
			labelPrincipal = label
		}
	}
	if len(valores) == 0 {
		p.Dados[resolvido] = true
		return false
	}
	p.Dados["reutilizacao_valores_"+passo.Slot] = valores
	p.Dados["reuso_label"] = labelPrincipal
	if labelPrincipal == "" {
		p.Dados["reuso_label"] = "anterior"
	}
	return true
}

func nomeVisitanteValido(nome string) bool {
	palavras := strings.Fields(strings.TrimSpace(nome))
	if len(palavras) < 2 || strings.Contains(nome, " - ") {
		return false
	}
	for _, p := range palavras {
		switch normalizar(strings.Trim(p, ".,")) {
		case "empresa", "ltda", "eireli", "construtora", "comercial", "servicos", "servico", "s.a":
			return false
		}
	}
	return true
}

func dataVisitaPermitida(value any) bool {
	hoje := agoraSaoPaulo()
	data, err := time.ParseInLocation("2006-01-02", fmt.Sprint(value), hoje.Location())
	if err != nil {
		return false
	}
	base := time.Date(hoje.Year(), hoje.Month(), hoje.Day(), 0, 0, 0, 0, hoje.Location())
	dia := time.Date(data.Year(), data.Month(), data.Day(), 0, 0, 0, 0, hoje.Location())
	distancia := int(dia.Sub(base).Hours() / 24)
	return distancia == 0 || distancia == 1
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
			ops = opcoesDoPedido(p.Dados[slot.De], slot, p)
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
		if slot.Tipo == "choose" {
			if opcao, ok := casarOpcao(fmt.Sprint(v), ops); ok {
				val, rot = valorDaOpcao(opcao), opcao.Rotulo
			}
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
