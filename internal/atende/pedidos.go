package atende

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/escopo"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/pii"
	"go-concierge/internal/harness"
	"go-concierge/internal/pedidos"
)

// Pedidos no turno: com Servico.Pedidos ligado, cada mensagem passa pelo detector
// (extrator) e pelo motor de pedidos antes de qualquer agente. O que é pedido vira (ou
// alimenta) um Pedido com protocolo; o que não é (dúvida, conversa) vai ao orquestrador.
// O estado de cada pedido é decidido pelos parâmetros, em código; a resposta final é
// redigida na persona a partir do que o motor devolveu.

// PedidoTurno é o que um turno fez com um pedido (para a sessão e o Console).
type PedidoTurno struct {
	Protocolo          string `json:"protocolo"`
	Caso               string `json:"caso"`
	Estado             string `json:"estado"`
	Evento             string `json:"evento"` // aberto | valores | pergunta | confirmar | concluido | cancelado | erro | transbordo_pendente
	Resumo             string `json:"resumo,omitempty"`
	confirmacaoTexto   string
	confirmacaoValores json.RawMessage
}

// ambienteHarness executa recursos pelo subagente do caso (permissões e guarda de escrita).
type ambienteHarness struct {
	ag              *harness.Agente
	escritaIniciada bool
}

func (a *ambienteHarness) Chamar(ctx context.Context, agente, recurso string, params map[string]any, confirmacao string) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sub, ok := a.ag.Subagentes[agente]
	if !ok {
		return nil, fmt.Errorf("caso de uso %q não está disponível neste condomínio", agente)
	}
	for _, permissao := range sub.Permissoes() {
		if permissao.Nome == recurso && permissao.Recurso != nil && permissao.Recurso.Write {
			a.escritaIniciada = true
			break
		}
	}
	out, err := sub.ExecutarPassoCaso(ctx, recurso, params, confirmacao)
	if err != nil {
		return nil, err
	}
	if e, ok := out["error"].(string); ok && e != "" {
		return nil, errors.New(e)
	}
	return out["resultado"], nil
}

// guardarNoCofre põe no cofre do turno os valores dos pedidos abertos: um CPF coletado num
// turno anterior chega à ferramenta e ao morador como valor, em vez de sair «cpf».
func guardarNoCofre(cofre *pii.Cofre, cart pedidos.Carteira) {
	for _, p := range cart.Abertos() {
		cofre.MascararEmValor(p.Valores)
	}
}

type contratoDoCaso struct {
	ct     *pedidos.Contrato
	agente string // nome da pasta do subagente
}

// casosDisponiveis: os subagentes com caso.yaml que o Build deixou (filtro do Admin).
func casosDisponiveis(ag *harness.Agente) ([]pedidos.CasoResumo, map[string]contratoDoCaso) {
	var resumos []pedidos.CasoResumo
	contratos := map[string]contratoDoCaso{}
	for nome, sub := range ag.Subagentes {
		if sub.Def.Caso == nil {
			continue
		}
		ct, err := pedidos.ContratoDe(sub.Def.Caso)
		if err != nil {
			continue
		}
		contratos[ct.Caso.ID] = contratoDoCaso{ct: ct, agente: nome}
		r := pedidos.CasoResumo{ID: ct.Caso.ID, Nome: ct.Caso.Name, Descricao: ct.Caso.Description, Exemplos: ct.Caso.Examples, Keywords: ct.Caso.Keywords, Agente: nome}
		for _, n := range ct.Ordem {
			r.Slots = append(r.Slots, ct.Slots[n])
		}
		resumos = append(resumos, r)
	}
	// ordem estável (a do roteador seria melhor; alfabética basta para o prompt)
	for i := 1; i < len(resumos); i++ {
		for j := i; j > 0 && resumos[j].ID < resumos[j-1].ID; j-- {
			resumos[j], resumos[j-1] = resumos[j-1], resumos[j]
		}
	}
	return resumos, contratos
}

func ultimasFalas(sess *Sessao, n int) []string {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	var out []string
	for _, t := range sess.Turnos {
		switch t.Quem {
		case "morador", "concierge", "atendente":
			out = append(out, t.Quem+": "+corta(t.Texto, 200))
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

var reCancelar = regexp.MustCompile(`CANCELA[R]?\s+(P-[A-Z0-9]{8}|P-[A-Z0-9]{5})\b`) // 8 nos novos, 5 nos antigos
var reMencionaData = regexp.MustCompile(`\d|janeiro|fevereiro|março|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro|hoje|amanh|segunda|terça|terca|quarta|quinta|sexta|sábado|sabado|domingo|fim de semana|semana que vem|próxim|proxim`)

// mencionaData: a mensagem fala de alguma data (dígito, mês, dia da semana, hoje…).
func mencionaData(msg string) bool { return reMencionaData.MatchString(strings.ToLower(msg)) }

func normalizar(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// restanteDoMorador: a maioria das palavras significativas do restante está na mensagem.
func restanteDoMorador(mensagem, restante string) bool {
	norm := func(s string) []string {
		var out []string
		for _, w := range strings.Fields(strings.ToLower(s)) {
			w = strings.Trim(w, ".,;:!?()\"'")
			if len([]rune(w)) >= 4 {
				out = append(out, w)
			}
		}
		return out
	}
	msg := map[string]bool{}
	for _, w := range norm(mensagem) {
		msg[w] = true
	}
	palavras := norm(restante)
	if len(palavras) == 0 {
		return true
	}
	achadas := 0
	for _, w := range palavras {
		if msg[w] {
			achadas++
		}
	}
	return achadas*2 >= len(palavras)
}

func orProtocolo(p string) string {
	if p == "" {
		return " (novo)"
	}
	return " " + p
}

func corta(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// turnoComPedidos é a vez do robô com o registro de pedidos como espinha dorsal.
// cofre é o do turno: os slots extraídos da mensagem mascarada voltam aos valores reais
// antes de entrar no pedido (é dele que a ferramenta lê), e o texto que sai para o modelo,
// o turno e o histórico volta mascarado.
func (s *Servico) turnoComPedidos(ctx context.Context, sess *Sessao, ag *harness.Agente, texto string, historico []*schema.Message, cofre *pii.Cofre) (*harness.Resultado, []PedidoTurno, error) {
	contato := sess.contato()
	desbloquear := s.bloquearPedidos(contato)
	defer desbloquear()
	rel := ag.Def.Rel
	var rota []harness.Passo
	var uso harness.Uso
	var registros []ganchos.Registro
	passo := func(tipo, det string) { rota = append(rota, harness.Passo{Agente: rel, Tipo: tipo, Detalhe: det}) }
	casos, contratos := casosDisponiveis(ag)
	cart, err := s.carregarCarteira(ctx, contato)
	if err != nil {
		return nil, nil, fmt.Errorf("pedidos de %s: %w", contato.Mascarado(), err)
	}
	estadosAntes := estadosDe(cart)
	guardarNoCofre(cofre, cart)

	ex := s.ExtratorPedidos
	if ex == nil {
		ex = pedidos.Regras{}
	}
	extracao, err := ex.Extrair(ctx, pedidos.Entrada{Mensagem: texto, Historico: ultimasFalas(sess, 6), Casos: casos, Abertos: cart.Abertos(), Agora: time.Now()})
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if err != nil {
		passo("aviso", "extrator de pedidos falhou, usando regras: "+err.Error())
		extracao, _ = pedidos.Regras{}.Extrair(ctx, pedidos.Entrada{Mensagem: texto, Casos: casos, Abertos: cart.Abertos()})
		// modelo fora do ar e as regras sem nada: melhor avisar a instabilidade do que
		// responder "não entendi" a um pedido legítimo
		if FalhaTemporaria(err) && len(extracao.Pedidos) == 0 && len(extracao.Confirmacoes) == 0 && len(extracao.Cancelar) == 0 && !extracao.Atendente {
			return nil, nil, fmt.Errorf("extrator de pedidos: %w", err)
		}
	}
	{
		var itens []string
		for _, pe := range extracao.Pedidos {
			vals, _ := json.Marshal(pe.Valores)
			itens = append(itens, pe.Caso+orProtocolo(pe.Protocolo)+" "+string(vals))
		}
		passo("pedidos", fmt.Sprintf("%d pedido(s) [%s], %d confirmação(ões), atendente=%v, restante=%q", len(extracao.Pedidos), strings.Join(itens, "; "), len(extracao.Confirmacoes), extracao.Atendente, corta(extracao.Restante, 80)))
	}

	// "cancelar P-XXXXX" vale sempre, em código, por cima do extrator
	for _, m := range reCancelar.FindAllStringSubmatch(strings.ToUpper(texto), -1) {
		extracao.Cancelar = append(extracao.Cancelar, m[1])
	}

	amb := &ambienteHarness{ag: ag}
	var turnos []PedidoTurno
	consumida := false // a mensagem foi usada como resposta a um pedido (confirmação/valor)
	anota := func(p *pedidos.Pedido, evento string) {
		turno := PedidoTurno{Protocolo: p.Protocolo, Caso: p.CasoNome, Estado: p.Estado, Evento: evento}
		if (evento == "confirmar" || evento == "lembrete") && p.Confirmacao != nil && p.Estado == pedidos.Confirmando {
			turno.confirmacaoTexto = p.Confirmacao.Texto
			turno.confirmacaoValores = append(json.RawMessage(nil), p.Confirmacao.Valores...)
		}
		turnos = append(turnos, turno)
	}
	tocados := map[string]*pedidos.Pedido{}
	trechos := map[string]string{} // o que o morador disse sobre cada pedido, neste turno
	var cancelados []string

	for _, c := range extracao.Confirmacoes {
		if p := cart.Por(c.Protocolo); p != nil && p.Estado == pedidos.Confirmando {
			if p.Confirmacao != nil && p.Confirmacao.Entregue != nil && !*p.Confirmacao.Entregue {
				tocados[p.Protocolo] = p
				consumida = true
				passo("pedido", p.Protocolo+" confirmação ainda não entregue")
				continue
			}
			pedidos.Confirmar(p, c.Resposta, pedidos.OrigemMorador)
			tocados[p.Protocolo] = p
			consumida = true
			passo("pedido", p.Protocolo+" confirmação: "+c.Resposta)
		}
	}
	for _, prot := range extracao.Cancelar {
		if p := cart.Por(prot); p != nil && p.Aberto() {
			pedidos.Cancelar(p, pedidos.OrigemMorador, "")
			anota(p, "cancelado")
			passo("pedido", p.Protocolo+" cancelado pelo morador")
			cancelados = append(cancelados, "Pedido "+p.Protocolo+" ("+p.CasoNome+") cancelado.")
			consumida = true
		}
	}
	for _, pe := range extracao.Pedidos {
		cc, ok := contratos[pe.Caso]
		if !ok {
			passo("aviso", "caso desconhecido na extração: "+pe.Caso)
			continue
		}
		var p *pedidos.Pedido
		if pe.Protocolo != "" {
			p = cart.Por(pe.Protocolo)
		}
		if p == nil || !p.Aberto() {
			p = cart.AbertoDoCaso(pe.Caso)
		}
		if p == nil {
			sess.mu.Lock()
			numeroTurno := len(sess.Turnos)
			sess.mu.Unlock()
			p = cart.Novo(cc.ct, cc.agente, contato.CondominioID, pedidos.Origem{ProtocoloAtende: sess.Protocolo, Turno: numeroTurno})
			passo("pedido", p.Protocolo+" aberto: "+cc.ct.Caso.Name)
			anota(p, "aberto")
			// pedido novo sem nenhuma menção a data na mensagem: data extraída é invenção
			if !mencionaData(texto) {
				for _, sl := range cc.ct.Slots {
					if sl.Tipo == "date" {
						delete(pe.Valores, sl.Nome)
					}
				}
			}
		}
		if p.Estado == pedidos.ComAtendente {
			passo("pedido", p.Protocolo+" está com o atendente; valores ignorados")
			continue
		}
		antes := len(p.Valores)
		valores, _ := pii.DevolverEmValor(pe.Valores, cofre.Ler()).(map[string]any)
		if rej := pedidos.Aplicar(cc.ct, p, valores, pedidos.OrigemMorador); len(rej) > 0 {
			for k, v := range rej {
				passo("pedido", p.Protocolo+" rejeitou "+k+": "+v)
			}
			consumida = true
		}
		if len(p.Valores) > antes {
			consumida = true
		}
		tocados[p.Protocolo] = p
		if t := strings.TrimSpace(pe.Trecho); t != "" {
			trechos[p.Protocolo] = t
		}
	}
	// a mensagem inteira foi a resposta a um pedido ("sim", "salão"): não há restante
	if consumida && normalizar(extracao.Restante) == normalizar(texto) {
		extracao.Restante = ""
	}

	// FILA: um pedido por vez. O ativo é o mais antigo em aberto (fora os que estão com
	// o atendente); só ele avança, pergunta e lembra. Os outros ficam na fila: recebem os
	// valores que o morador der, mas não perguntam nada. Quando o ativo termina
	// (concluído, cancelado, falhou), o próximo da fila é puxado no mesmo turno, para o
	// morador já receber a pergunta dele.
	var partes, perguntas []string
	precisaRedigir := false
	transbordo := ""
	if len(cancelados) > 0 {
		partes = append(partes, cancelados...)
		precisaRedigir = true
	}
	var fila []*pedidos.Pedido
	for _, p := range cart.Abertos() {
		if p.Estado != pedidos.ComAtendente {
			if _, ok := contratos[p.Caso]; ok {
				fila = append(fila, p)
			}
		}
	}
	var ativo *pedidos.Pedido
	novosNaFila := false
	for _, p := range fila {
		if _, ok := tocados[p.Protocolo]; ok && p.Pendencia == nil && len(p.Historico) <= 1 {
			novosNaFila = true
		}
	}
	for len(fila) > 0 {
		p := fila[0]
		ativo = p
		if _, ok := tocados[p.Protocolo]; !ok && p.Pendencia != nil {
			break // a pergunta em aberto volta pelo lembrete, abaixo
		}
		cc := contratos[p.Caso]
		saida := pedidos.Avancar(ctx, cc.ct, p, &cart, amb)
		if amb.escritaIniciada {
			ctxSalvar, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			err := s.salvarCarteira(ctxSalvar, contato, cart)
			cancelar()
			if err != nil {
				return nil, nil, err
			}
			amb.escritaIniciada = false
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		passo("pedido", p.Protocolo+" "+saida.Tipo+": "+corta(mascarado(cofre, saida.Texto), 140))
		anota(p, saida.Tipo)
		switch saida.Tipo {
		case "pergunta", "confirmar":
			perguntas = append(perguntas, saida.Texto)
			precisaRedigir = true
		case "concluido":
			partes = append(partes, saida.Texto)
			precisaRedigir = true
		case "cancelado":
			partes = append(partes, "Pedido "+p.Protocolo+" ("+p.CasoNome+") cancelado.")
			precisaRedigir = true
		case "erro":
			partes = append(partes, "Não consegui concluir "+strings.ToLower(p.CasoNome)+": "+saida.Texto+".")
			precisaRedigir = true
		case "formulario":
			// o caso tem formulário dinâmico: segue pelo subagente conversacional, que
			// recebe só a parte da mensagem que é dele (numa mensagem com dois pedidos, o
			// subagente não deve ver, nem responder por, o outro)
			fala := trechos[p.Protocolo]
			if fala == "" || len(extracao.Pedidos) <= 1 {
				fala = texto
			}
			if sub, ok := ag.Subagentes[p.Agente]; ok {
				r, err := harness.Run(ctx, sub, fala, historico)
				if err != nil {
					partes = append(partes, "Não consegui seguir com "+strings.ToLower(p.CasoNome)+".")
				} else {
					if strings.TrimSpace(r.Resposta) != "" && (r.Transbordo == "" || sess.Destino.Conector == "atende-emulado") {
						partes = append(partes, r.Resposta)
					} else if r.Transbordo != "" && sess.Destino.Conector == "atende" {
						if resposta := removerPromessasTransbordo(r.Resposta); strings.TrimSpace(resposta) != "" {
							partes = append(partes, resposta)
						}
					}
					rota = append(rota, r.Rota...)
					uso.Soma(r.Uso)
					registros = append(registros, r.Registros...)
					if r.Transbordo != "" {
						transbordo = r.Transbordo
					}
				}
			}
		}
		if p.Aberto() && p.Estado != pedidos.ComAtendente {
			break // o ativo ainda está em andamento: os outros esperam
		}
		fila = fila[1:] // terminou: o próximo da fila é puxado neste mesmo turno
		ativo = nil
		if len(fila) > 0 {
			tocados[fila[0].Protocolo] = fila[0]
			passo("pedido", fila[0].Protocolo+" sai da fila: próximo da vez")
		}
	}
	// o morador sabe a ordem: quando pedidos novos entraram na fila atrás do ativo
	if ativo != nil && len(fila) > 1 && novosNaFila {
		var nomes []string
		for _, p := range fila[1:] {
			nomes = append(nomes, strings.ToLower(p.CasoNome)+" ("+p.Protocolo+")")
		}
		perguntas = append(perguntas, "Vou cuidar de um pedido por vez: agora "+strings.ToLower(ativo.CasoNome)+" ("+ativo.Protocolo+"); depois "+strings.Join(nomes, ", ")+".")
		precisaRedigir = true
	}

	// o que não é pedido vai ao orquestrador (dúvidas, conversa), em modo router. O
	// restante tem de ser fala do morador: se o modelo pôs ali uma resposta sua, cai.
	if r := strings.TrimSpace(extracao.Restante); r != "" && !restanteDoMorador(texto, r) {
		passo("aviso", "restante descartado (não é fala do morador): "+corta(r, 80))
		extracao.Restante = ""
	}
	if r := strings.TrimSpace(extracao.Restante); r != "" {
		res, err := harness.Run(ctx, ag, r, historico)
		if err != nil {
			return nil, nil, err
		}
		rota = append(rota, res.Rota...)
		uso.Soma(res.Uso)
		registros = append(registros, res.Registros...)
		if strings.TrimSpace(res.Resposta) != "" && (res.Transbordo == "" || sess.Destino.Conector == "atende-emulado") {
			partes = append(partes, res.Resposta)
		} else if res.Transbordo != "" && sess.Destino.Conector == "atende" {
			if resposta := removerPromessasTransbordo(res.Resposta); strings.TrimSpace(resposta) != "" {
				partes = append(partes, resposta)
			}
		}
		if res.Transbordo != "" {
			transbordo = res.Transbordo
		}
	}

	// pendências não dormem: todo pedido aberto que o turno não tocou volta à conversa
	// com a pergunta (ou a confirmação) em aberto; com AvisoAposVezes o morador é
	// convidado a responder, cancelar ou chamar o atendente; com AtendenteAposVezes o
	// pedido vai ao atendente (transbordo) em vez de ficar aberto para sempre.
	for _, p := range cart.Abertos() {
		if _, ok := tocados[p.Protocolo]; ok || p.Estado == pedidos.ComAtendente {
			continue
		}
		if ativo != nil && p.Protocolo != ativo.Protocolo {
			continue // na fila: espera a vez, sem lembrete
		}
		lembrete, vezes := p.Lembrete()
		if lembrete == "" {
			continue
		}
		if vezes >= pedidos.AtendenteAposVezes {
			if transbordo == "" {
				transbordo = "pendência sem resposta depois de " + fmt.Sprint(vezes) + " perguntas"
			}
			passo("pedido", p.Protocolo+" sem resposta "+fmt.Sprint(vezes)+" vezes: vai ao atendente")
			continue // o bloco do atendente, abaixo, leva o pedido com o resumo
		}
		texto := "Ainda sobre " + strings.ToLower(p.CasoNome) + " (" + p.Protocolo + "): " + lembrete
		if vezes >= pedidos.AvisoAposVezes {
			texto += " Se preferir, responda \"cancelar " + p.Protocolo + "\" ou peça um atendente."
		}
		perguntas = append(perguntas, texto)
		precisaRedigir = true
		anota(p, "lembrete")
		passo("pedido", p.Protocolo+" lembrete ("+fmt.Sprint(vezes)+"x): "+corta(lembrete, 100))
	}

	// atendente: pendências viram resumo para o humano
	if extracao.Atendente || transbordo != "" {
		if sess.Destino.Conector != "atende-emulado" && sess.Destino.Conector != "atende" {
			seguras := partes[:0]
			for _, parte := range partes {
				if !harness.AnunciaTransferencia(parte) {
					seguras = append(seguras, parte)
				}
			}
			partes = seguras
			partes = append(partes, "Não consigo transferir esta conversa para um atendente por este canal. Seus pedidos permanecem registrados e posso continuar ajudando por aqui.")
			precisaRedigir = false
			perguntas = nil
			transbordo = ""
		} else {
			var pend []string
			for _, p := range cart.Abertos() {
				if p.Estado == pedidos.ComAtendente {
					continue
				}
				cc, ok := contratos[p.Caso]
				if !ok {
					continue
				}
				resumo := pedidos.ResumoParaAtendente(cc.ct, p, ultimasFalas(sess, 6))
				turnos = append(turnos, PedidoTurno{Protocolo: p.Protocolo, Caso: p.CasoNome, Estado: p.Estado, Evento: "transbordo_pendente", Resumo: resumo})
				faltam := p.Faltando(cc.ct)
				var nomes []string
				for _, f := range faltam {
					nomes = append(nomes, f.Titulo)
				}
				if len(nomes) > 0 {
					pend = append(pend, p.Protocolo+" ("+p.CasoNome+": falta "+strings.Join(nomes, ", ")+")")
				} else {
					pend = append(pend, p.Protocolo+" ("+p.CasoNome+")")
				}
			}
			if transbordo == "" {
				transbordo = "o morador pediu um atendente"
			}
			if len(pend) > 0 {
				transbordo += "; pedidos pendentes: " + strings.Join(pend, "; ")
				passo("pedido", "ao atendente: "+strings.Join(pend, "; "))
			}
			if len(partes) == 0 && sess.Destino.Conector != "atende" {
				if extracao.Atendente {
					partes = append(partes, "Vou solicitar o atendimento de uma pessoa da administração. Ainda não tenho confirmação de que ela assumiu a conversa.")
				} else {
					partes = append(partes, "Vou solicitar que uma pessoa da administração continue com você. Ainda não tenho confirmação de que ela assumiu a conversa.")
				}
				precisaRedigir = true
			}
			perguntas = nil // quem pergunta agora é o atendente
		}
	}

	if abriuOuMudouDeEstado(estadosAntes, cart) {
		// a resposta cita o pedido aberto ou o estado novo: ele vai ao banco antes dela, e uma
		// falha aqui fecha o turno sem prometer o que não foi gravado. Se o protocolo de um
		// pedido novo era de outro contato, a gravação o trocou, e a resposta já sai com o novo.
		antes := protocolosDe(cart)
		if err := s.salvarCarteira(ctx, contato, cart); err != nil {
			return nil, nil, err
		}
		if r := renomeador(antes, protocolosDe(cart)); r != nil {
			partes, perguntas, transbordo = renomearTodos(r, partes), renomearTodos(r, perguntas), r.Replace(transbordo)
			for i := range turnos {
				turnos[i].Protocolo, turnos[i].confirmacaoTexto = r.Replace(turnos[i].Protocolo), r.Replace(turnos[i].confirmacaoTexto)
			}
		}
	} else {
		// o resto (valores preenchidos, eventos) vai depois da resposta, junto com a sessão
		// (gravarDepoisDaResposta)
		s.adiarCarteira(contato, cart)
	}

	conteudo := strings.TrimSpace(strings.Join(append(partes, perguntas...), "\n\n"))
	if conteudo == "" {
		if transbordo == "" || sess.Destino.Conector != "atende" {
			conteudo = "Não entendi o que você precisa. Pode me dizer de outra forma, ou dizer que quer falar com um atendente?"
			precisaRedigir = true
		}
	}
	resposta := conteudo
	if transbordo != "" && sess.Destino.Conector == "atende" {
		resposta = conteudo
		aviso := "Não consegui solicitar um atendente por este canal agora. Posso continuar ajudando por aqui."
		if s.transferidorDisponivel(sess.Destino) {
			aviso = "Vou solicitar que uma pessoa da administração assuma este protocolo. Ainda não tenho confirmação de que ela assumiu a conversa."
		}
		if strings.TrimSpace(resposta) != "" {
			resposta += "\n\n"
		}
		resposta += aviso
		precisaRedigir = false
	}
	if precisaRedigir {
		texto, u, err := ag.Redigir(ctx, "Entregue ao morador o conteúdo abaixo (resultados concluídos, pedidos de confirmação e perguntas), preservando EXATAMENTE protocolos, datas, valores, as opções numeradas e as perguntas de confirmação (sim/não); perguntas e confirmações ficam por último, uma por parágrafo.", mascarado(cofre, conteudo), len(historico) > 0)
		switch {
		case err != nil:
			passo("aviso", "redação falhou, conteúdo vai cru: "+err.Error())
		case strings.TrimSpace(texto) != "":
			resposta = texto
			uso.Soma(u)
		}
	}
	resposta = pii.Devolver(resposta, cofre.Ler())
	for _, turno := range turnos {
		if turno.confirmacaoTexto != "" && strings.Contains(conteudo, turno.confirmacaoTexto) && !strings.Contains(resposta, turno.confirmacaoTexto) {
			resposta = strings.TrimSpace(resposta + "\n\n" + turno.confirmacaoTexto)
		}
	}
	// o que volta vai ao turno, ao histórico e ao modelo: mascarado; antes_de_responder
	// devolve os valores no envio
	resposta = mascarado(cofre, resposta)
	passo("entrega", corta(resposta, 160))

	msgs := make([]*schema.Message, 0, len(historico)+2)
	msgs = append(msgs, historico...)
	msgs = append(msgs, schema.UserMessage(texto), schema.AssistantMessage(resposta, nil))
	return &harness.Resultado{Resposta: resposta, Agente: rel, Rota: rota, Uso: uso, Mensagens: msgs, Transbordo: transbordo, Registros: registros}, turnos, nil
}

func (s *Servico) marcarConfirmacoesEntregues(ctx context.Context, contato escopo.Contato, turnos []PedidoTurno, resposta string) error {
	confirmacoes := map[string]PedidoTurno{}
	for _, turno := range turnos {
		if turno.confirmacaoTexto != "" && strings.Contains(resposta, turno.confirmacaoTexto) {
			confirmacoes[turno.Protocolo] = turno
		}
	}
	if len(confirmacoes) == 0 {
		return nil
	}
	desbloquear := s.bloquearPedidos(contato)
	defer desbloquear()
	cart, err := s.carteiraDoTurno(ctx, contato)
	if err != nil {
		return err
	}
	alterada := false
	for protocolo, turno := range confirmacoes {
		p := cart.Por(protocolo)
		if p == nil || p.Estado != pedidos.Confirmando || p.Confirmacao == nil || p.Confirmacao.Resposta != "" || p.Confirmacao.Texto != turno.confirmacaoTexto || p.Confirmacao.Entregue == nil || *p.Confirmacao.Entregue {
			continue
		}
		var atuais, enviados bytes.Buffer
		if json.Compact(&atuais, p.Confirmacao.Valores) != nil || json.Compact(&enviados, turno.confirmacaoValores) != nil || !bytes.Equal(atuais.Bytes(), enviados.Bytes()) {
			continue
		}
		entregue := true
		p.Confirmacao.Entregue = &entregue
		alterada = true
	}
	if alterada {
		s.adiarCarteira(contato, cart)
	}
	return nil
}

// marcarPedidosAoAtendente passa ao atendente os pedidos que o turno deixou pendentes,
// depois que a transferência foi confirmada. É mudança de estado: grava já, sem esperar o
// lote depois da resposta.
func (s *Servico) marcarPedidosAoAtendente(ctx context.Context, contato escopo.Contato, turnos []PedidoTurno) error {
	pendentes := map[string]string{}
	for _, turno := range turnos {
		if turno.Evento == "transbordo_pendente" {
			pendentes[turno.Protocolo] = turno.Resumo
		}
	}
	if len(pendentes) == 0 || s.Pedidos == nil {
		return nil
	}
	desbloquear := s.bloquearPedidos(contato)
	defer desbloquear()
	cart, err := s.carteiraDoTurno(ctx, contato)
	if err != nil {
		return err
	}
	for protocolo, resumo := range pendentes {
		if p := cart.Por(protocolo); p != nil {
			switch p.Estado {
			case pedidos.Coletando, pedidos.Pronto, pedidos.Confirmando:
				pedidos.AoAtendente(p, resumo)
			}
		}
	}
	return s.salvarCarteira(ctx, contato, cart)
}

// ---- ações do atendente ------------------------------------------------------------------

// Carteira devolve os pedidos de um contato com os contratos (slots) de cada caso.
func (s *Servico) Carteira(ctx context.Context, contato escopo.Contato) (pedidos.Carteira, map[string][]*pedidos.Slot, error) {
	if s.Pedidos == nil {
		return pedidos.Carteira{}, nil, errors.New("pedidos desligados")
	}
	desbloquear := s.bloquearPedidos(contato)
	defer desbloquear()
	cart, err := s.carregarCarteira(ctx, contato)
	if err != nil {
		return cart, nil, err
	}
	slots := map[string][]*pedidos.Slot{}
	dir, err := s.Cat.ResolveAgente(s.Agente)
	if err != nil {
		return cart, slots, nil
	}
	raiz, err := s.Cat.Load(dir)
	if err != nil {
		return cart, slots, nil
	}
	for _, sub := range raiz.Subagentes {
		if sub.Caso == nil {
			continue
		}
		if ct, err := pedidos.ContratoDe(sub.Caso); err == nil {
			for _, n := range ct.Ordem {
				slots[ct.Caso.ID] = append(slots[ct.Caso.ID], ct.Slots[n])
			}
		}
	}
	return cart, slots, nil
}

// PreencherPedido: o atendente informa os parâmetros que faltavam (e, se for o caso,
// confirma em nome do morador); o motor avança e, concluído, o resultado vai ao morador.
// A sessão é persistida no fim, mesmo quando o preenchimento falha.
func (s *Servico) PreencherPedido(ctx context.Context, protocoloAtende, protocoloPedido, atendente string, valores map[string]any, confirmar bool) (*Sessao, *pedidos.Pedido, pedidos.Saida, error) {
	if s.Pedidos == nil {
		return nil, nil, pedidos.Saida{}, errors.New("pedidos desligados")
	}
	sess, ok, err := s.Sessao(ctx, protocoloAtende)
	if err != nil {
		return nil, nil, pedidos.Saida{}, fmt.Errorf("sessão do protocolo %s: %w", protocoloAtende, err)
	}
	if !ok {
		return nil, nil, pedidos.Saida{}, fmt.Errorf("%w: %s não está aberto neste servidor", ErrProtocoloDesconhecido, protocoloAtende)
	}
	res, p, saida, err := s.preencherPedido(ctx, sess, protocoloPedido, atendente, valores, confirmar)
	return res, p, saida, errors.Join(err, s.Sessoes.Persistir(context.WithoutCancel(ctx), sess))
}

func (s *Servico) preencherPedido(ctx context.Context, sess *Sessao, protocoloPedido, atendente string, valores map[string]any, confirmar bool) (*Sessao, *pedidos.Pedido, pedidos.Saida, error) {
	sess.workMu.Lock()
	defer sess.workMu.Unlock()
	sess.mu.Lock()
	encerrada := sess.Encerrada
	ctxTurno, cancelarTurno := context.WithCancel(ctx)
	if !encerrada {
		sess.cancelarTurno = cancelarTurno
	}
	sess.mu.Unlock()
	defer func() {
		sess.mu.Lock()
		sess.cancelarTurno = nil
		sess.mu.Unlock()
		cancelarTurno()
	}()
	if encerrada {
		return sess, nil, pedidos.Saida{}, errors.New("o protocolo está encerrado")
	}
	ctx = ctxTurno
	contato := sess.contato()
	desbloquear := s.bloquearPedidos(contato)
	defer desbloquear()
	ag, err := s.agenteDaSessao(ctx, sess)
	if err != nil {
		return sess, nil, pedidos.Saida{}, err
	}
	_, contratos := casosDisponiveis(ag)
	cart, err := s.carregarCarteira(ctx, contato)
	if err != nil {
		return sess, nil, pedidos.Saida{}, err
	}
	p := cart.Por(protocoloPedido)
	if p == nil {
		return sess, nil, pedidos.Saida{}, fmt.Errorf("pedido %s não encontrado", protocoloPedido)
	}
	cc, ok := contratos[p.Caso]
	if !ok {
		return sess, p, pedidos.Saida{}, fmt.Errorf("caso %s indisponível", p.Caso)
	}
	if atendente == "" {
		atendente = "Atendente"
	}
	if len(valores) > 0 {
		if rej := pedidos.Aplicar(cc.ct, p, valores, pedidos.OrigemAtendente); len(rej) > 0 {
			var itens []string
			for k, v := range rej {
				itens = append(itens, k+": "+v)
			}
			return sess, p, pedidos.Saida{}, errors.New("valores rejeitados — " + strings.Join(itens, "; "))
		}
	}
	if p.Estado == pedidos.ComAtendente {
		p.Estado = pedidos.Coletando
	}
	if confirmar {
		pedidos.Confirmar(p, "sim", atendente) // fica registrado quem confirmou em nome do morador
	}
	sess.mu.Lock()
	encerrada = sess.Encerrada
	sess.mu.Unlock()
	if encerrada {
		return sess, p, pedidos.Saida{}, errors.New("o protocolo está encerrado")
	}
	amb := &ambienteHarness{ag: ag}
	saida := pedidos.Avancar(ctx, cc.ct, p, &cart, amb)
	if amb.escritaIniciada {
		ctxSalvar, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := s.salvarCarteira(ctxSalvar, contato, cart)
		cancelar()
		if err != nil {
			return sess, p, saida, err
		}
	}
	if err := ctx.Err(); err != nil {
		return sess, p, saida, err
	}
	if saida.Tipo == "confirmar" && !confirmar {
		// volta ao atendente: ele confirma com o morador e marca "confirmar"
		pedidos.AoAtendente(p, pedidos.ResumoParaAtendente(cc.ct, p, ultimasFalas(sess, 6)))
	} else if saida.Tipo == "pergunta" {
		pedidos.AoAtendente(p, pedidos.ResumoParaAtendente(cc.ct, p, ultimasFalas(sess, 6)))
	}
	if err := s.salvarCarteira(ctx, contato, cart); err != nil {
		return sess, p, saida, err
	}
	if saida.Tipo == "concluido" || saida.Tipo == "erro" {
		texto := saida.Texto
		if saida.Tipo == "erro" {
			texto = "Não consegui concluir " + strings.ToLower(p.CasoNome) + ": " + saida.Texto
		}
		t, err := s.abrirTurno(sess)
		if err != nil {
			return sess, p, saida, err
		}
		defer sess.fecharTurno(t)
		sess.comecarRegistros()
		enviar, guardar := s.respostaPronta(ctx, sess, t, texto)
		var errEnvio error
		if s.Sender != nil {
			ctxEnvio, finalizar := sess.contextoEntregaManual(ctx)
			errEnvio = s.Sender.Enviar(ctxEnvio, sess.Destino, enviar)
			finalizar()
		}
		if entregue := textoEntregue(enviar, errEnvio); entregue != "" {
			if entregue != enviar {
				guardar = mascarado(t.Cofre(), entregue)
			}
			sess.registrar(Turno{Quem: "concierge", Texto: guardar, Agente: ag.Def.Rel, Pedidos: []PedidoTurno{{Protocolo: p.Protocolo, Caso: p.CasoNome, Estado: p.Estado, Evento: saida.Tipo}}})
			sess.historicoAdicionar(schema.AssistantMessage(guardar, nil))
		}
		if errEnvio != nil {
			sess.registrar(Turno{Quem: "erro", Texto: errEnvio.Error()})
			return sess, p, saida, errEnvio
		}
	}
	return sess, p, saida, nil
}

// CancelarPedido encerra um pedido a pedido do atendente.
func (s *Servico) CancelarPedido(ctx context.Context, contato escopo.Contato, protocoloPedido, atendente, motivo string) (*pedidos.Pedido, error) {
	if s.Pedidos == nil {
		return nil, errors.New("pedidos desligados")
	}
	desbloquear := s.bloquearPedidos(contato)
	defer desbloquear()
	cart, err := s.carregarCarteira(ctx, contato)
	if err != nil {
		return nil, err
	}
	p := cart.Por(protocoloPedido)
	if p == nil {
		return nil, fmt.Errorf("pedido %s não encontrado", protocoloPedido)
	}
	if atendente == "" {
		atendente = "Atendente"
	}
	pedidos.Cancelar(p, atendente, motivo)
	return p, s.salvarCarteira(ctx, contato, cart)
}

// RemoverPedido apaga do registro um pedido encerrado (concluído, cancelado ou que
// falhou). É limpeza do Console: o morador não é avisado.
func (s *Servico) RemoverPedido(ctx context.Context, contato escopo.Contato, protocoloPedido string) error {
	if s.Pedidos == nil {
		return errors.New("pedidos desligados")
	}
	// mesmo caminho do CancelarPedido: o lock do contato e a carteira adiada por um turno
	// (gravada antes de ler), senão a gravação adiada traria de volta o pedido removido
	desbloquear := s.bloquearPedidos(contato)
	defer desbloquear()
	cart, err := s.carregarCarteira(ctx, contato)
	if err != nil {
		return err
	}
	if err := cart.Remover(protocoloPedido); err != nil {
		return err
	}
	return s.salvarCarteira(ctx, contato, cart)
}

// estadosDe guarda o estado de cada pedido da carteira, pelo protocolo.
func estadosDe(cart pedidos.Carteira) map[string]string {
	out := make(map[string]string, len(cart.Pedidos))
	for _, p := range cart.Pedidos {
		out[p.Protocolo] = p.Estado
	}
	return out
}

// abriuOuMudouDeEstado: algum pedido é novo na carteira ou mudou de estado (confirmado,
// cancelado, com o atendente…) desde estadosAntes.
func abriuOuMudouDeEstado(estadosAntes map[string]string, cart pedidos.Carteira) bool {
	for _, p := range cart.Pedidos {
		if estado, existia := estadosAntes[p.Protocolo]; !existia || estado != p.Estado {
			return true
		}
	}
	return false
}

// renomeador troca cada protocolo de antes pelo de depois, na mesma posição; nil se nenhum
// mudou. Os que não mudaram entram trocando por si mesmos e os mais longos vêm primeiro: o
// Replacer compara na ordem dos pares, e um protocolo que é prefixo de outro não pode
// pegar um pedaço do maior.
func renomeador(antes, depois []string) *strings.Replacer {
	mudou := false
	pares := make([][2]string, 0, len(antes))
	for i := range antes {
		mudou = mudou || antes[i] != depois[i]
		pares = append(pares, [2]string{antes[i], depois[i]})
	}
	if !mudou {
		return nil
	}
	slices.SortStableFunc(pares, func(a, b [2]string) int { return len(b[0]) - len(a[0]) })
	args := make([]string, 0, 2*len(pares))
	for _, p := range pares {
		args = append(args, p[0], p[1])
	}
	return strings.NewReplacer(args...)
}

func renomearTodos(r *strings.Replacer, textos []string) []string {
	out := make([]string, len(textos))
	for i, t := range textos {
		out[i] = r.Replace(t)
	}
	return out
}
