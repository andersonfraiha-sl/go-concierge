package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
	"go-concierge/internal/atende"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
)

// Canal ATENDE: o webhook que recebe os eventos, e a API do Console (emulador):
// sessões por protocolo, caixa de saída e consulta de identidade no Gruvi.

func (s *servidor) rotasAtende(mux *http.ServeMux) {
	mux.HandleFunc("POST /webhook/atende", s.webhookAtende) // público: confere o HMAC
	mux.Handle("POST /emulador/atende/webhook", s.protegida(acesso.StudioAvancado, alvoSuperlogica, s.webhookEmulado))
	mux.HandleFunc("GET /console", func(w http.ResponseWriter, r *http.Request) {
		raw, err := fs.ReadFile(s.web, "console.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(raw)
	})
	// a caixa de saída emulada recebe do próprio servidor (o HTTPSender passa pelo usuário
	// padrão do modo dev)
	mux.Handle("POST /emulador/atende/messages", s.protegida(acesso.StudioAvancado, alvoSuperlogica, s.emulador.Handler))
	// conversas: a administradora pedida ou, quando há conversa, a gravada nela
	// configuração da implantação e consulta de identidade no Gruvi (company global, qualquer
	// telefone): só a Superlógica, como o emulador
	mux.Handle("GET /api/atende/config", s.protegida(acesso.StudioAvancado, alvoSuperlogica, s.atendeConfig))
	mux.Handle("GET /api/atende/identidade", s.protegida(acesso.StudioAvancado, alvoSuperlogica, s.atendeIdentidade))
	mux.Handle("GET /api/atende/sessoes", s.protegida(acesso.ConversasVer, s.alvoAdministradoraOuSuperlogica, s.atendeSessoes))
	mux.Handle("GET /api/atende/sessao", s.protegida(acesso.ConversasVer, s.alvoSessaoDaConsulta, s.atendeSessao))
	mux.Handle("GET /api/atende/saidas", s.saidasPorEscopo())
	mux.Handle("DELETE /api/atende/sessao", s.protegida(acesso.ConversasAtender, s.alvoSessaoDaConsulta, s.atendeReiniciar))
	mux.Handle("POST /api/atende/transbordo", s.protegida(acesso.ConversasAtender, s.alvoSessaoDoCorpo, s.atendeTransbordo))
	mux.Handle("GET /api/pedidos", s.protegida(acesso.PedidosVer, s.alvoAdministradora, s.pedidosLer))
	mux.Handle("GET /api/pedidos/exportar", s.protegida(acesso.PedidosVer, s.alvoAdministradora, s.pedidosExportar))
	mux.Handle("POST /api/pedidos/acao", s.protegida(acesso.PedidosEditar, s.alvoAcaoDePedido, s.pedidosAcao))
	mux.Handle("GET /api/memoria", s.protegida(acesso.MemoriaVer, s.alvoAdministradora, s.memoriaLer))
	mux.Handle("GET /api/memorias", s.protegida(acesso.MemoriaVer, s.alvoAdministradora, s.memoriasListar))
	mux.Handle("DELETE /api/memoria", s.protegida(acesso.MemoriaEsquecer, s.alvoAdministradora, s.memoriaEsquecer))
}

// ---- escopo das rotas de memória e pedidos -----------------------------

// administradoraDaRota: a administradora pedida (?administradora=), ou a fixa da
// implantação (regras em administradoraPedida).
func (s *servidor) administradoraDaRota(w http.ResponseWriter, r *http.Request) (string, bool) {
	adm, err := administradoraPedida(r, s.implantacao)
	if err != nil {
		responderErroDeAlvo(w, r, err)
		return "", false
	}
	return adm, true
}

// administradoraOpcional: como administradoraDaRota, mas sem nenhuma (nem pedida nem
// fixa) devolve "" (todas as administradoras).
func (s *servidor) administradoraOpcional(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !s.temAdministradoraPedida(r) {
		return "", true
	}
	return s.administradoraDaRota(w, r)
}

// contatoDaRota: o contato (administradora + telefone) que a rota consulta. Memória e
// pedidos são guardados por administradora e telefone; o condomínio não entra na
// chave, e o Console não o conhece aqui: vale o padrão.
func (s *servidor) contatoDaRota(w http.ResponseWriter, r *http.Request, phone string) (escopo.Contato, bool) {
	adm, ok := s.administradoraDaRota(w, r)
	if !ok {
		return escopo.Contato{}, false
	}
	c, err := escopo.NovoContato(adm, admin.Padrao, phone)
	if err != nil {
		writeErr(w, http.StatusBadRequest, errTelefone)
		return escopo.Contato{}, false
	}
	return c, true
}

// ---- memória relacional por contato ------------------------------------

func (s *servidor) memoriaLer(w http.ResponseWriter, r *http.Request) {
	contato, ok := s.contatoDaRota(w, r, r.URL.Query().Get("phone"))
	if !ok {
		return
	}
	m, err := s.memorias.Ler(r.Context(), contato)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"memoria": m})
}

func (s *servidor) memoriaEsquecer(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	contato, ok := s.contatoDaRota(w, r, q.Get("phone"))
	if !ok {
		return
	}
	m, err := s.atende.EsquecerMemoria(r.Context(), contato, strings.TrimSpace(q.Get("chave")))
	if err != nil {
		if errors.Is(err, atende.ErrFatoNaoEncontrado) {
			writeErr(w, 404, err)
			return
		}
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"memoria": m})
}

func (s *servidor) memoriasListar(w http.ResponseWriter, r *http.Request) {
	adm, ok := s.administradoraDaRota(w, r)
	if !ok {
		return
	}
	l, err := s.memorias.Listar(r.Context(), adm)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"administradora": adm, "memorias": l})
}

// webhookAtende aceita o evento (202) e processa em segundo plano; a resposta ao
// morador sai pelo Sender (caixa de saída do emulador e, se configurado, o ATENDE).
func (s *servidor) webhookAtende(w http.ResponseWriter, r *http.Request) {
	s.receberEventoAtende(w, r, false)
}

func (s *servidor) webhookEmulado(w http.ResponseWriter, r *http.Request) {
	s.receberEventoAtende(w, r, true)
}

func (s *servidor) receberEventoAtende(w http.ResponseWriter, r *http.Request, emulado bool) {
	if emulado && s.webhookToken != "" {
		bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if bearer != s.webhookToken && r.URL.Query().Get("token") != s.webhookToken {
			writeErr(w, 401, errUnauthorized)
			return
		}
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if !emulado {
		if err := atende.VerificarAssinatura(s.atendeAPISecret, r.Header.Get("X-Atende-Signature"), raw); err != nil {
			writeErr(w, http.StatusUnauthorized, errUnauthorized)
			return
		}
	}
	ev, err := atende.Parse(raw)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	tipo := ev.Classificar()
	if tipo == atende.EventoIgnorado {
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "ignored": true, "event": ev.ID})
		return
	}
	if err := ev.Validar(tipo); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if !emulado && (s.atendeTenant == "" || s.atendeTelefoneCanal == "" || s.atendeBaseURL == "" || s.atendeAPIKey == "") {
		writeErr(w, http.StatusServiceUnavailable, errors.New("configuração real do ATENDE incompleta"))
		return
	}
	if !emulado {
		if err := ev.ValidarConexao(s.atendeTenant, s.atendeTelefoneCanal); err != nil {
			writeErr(w, http.StatusForbidden, err)
			return
		}
	}
	if tipo == atende.EventoElegivel && !strings.EqualFold(ev.Data.Message.Type, "text") {
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "ignored": true, "event": ev.ID})
		return
	}
	msg := ev.Normalizar()
	if !emulado && s.atende.TelefoneB2C != "" {
		// Mock temporário: o token B2C está vinculado a um único telefone,
		// mesmo quando a mensagem de teste chega de outro número pelo ATENDE.
		// msg.Contato.Phone = ev.Data.Contact.Phone
		msg.Contato.Phone = s.atende.TelefoneB2C
		msg.Destino.Telefone = s.atende.TelefoneB2C
	}
	if emulado {
		if !s.condominioEmulado(w, r, &msg) {
			return
		}
		msg.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
		msg.Destino = entrada.Destino{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console", Protocolo: ev.Data.Protocol.Number, Telefone: ev.Data.Contact.Phone, Canal: msg.Canal}
	}
	if tipo == atende.EventoHumano {
		if _, err := s.atende.RegistrarHumano(r.Context(), msg); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "event": ev.ID, "protocol": ev.Data.Protocol.Number})
		return
	}
	if tipo == atende.EventoFechado {
		if _, err := s.atende.Encerrar(r.Context(), msg); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "event": ev.ID, "protocol": ev.Data.Protocol.Number})
		return
	}
	if !s.atende.PermiteTelefone(msg.Contato.Phone) {
		writeErr(w, http.StatusForbidden, errors.New("telefone diferente do morador configurado para o token B2C"))
		return
	}
	msg, sess, err := s.atende.PrepararMensagem(r.Context(), msg)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
		defer cancel()
		if _, err := s.atende.Processar(ctx, msg); err != nil {
			s.log.Printf("atende: protocolo %s: %v", ev.Data.Protocol.Number, err)
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "event": ev.ID, "protocol": sess.Protocolo, "session_id": sess.ID})
}

// condominioEmulado: com ?condominio=, a mensagem forjada pelo Console sai com o tenant
// e a company daquele condomínio (é por eles que o concierge resolve o condomínio).
func (s *servidor) condominioEmulado(w http.ResponseWriter, r *http.Request, msg *entrada.Mensagem) bool {
	id := strings.TrimSpace(r.URL.Query().Get("condominio"))
	if id == "" {
		return true
	}
	m, err := s.admin.Ler(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("condomínio do emulador: %w", err))
		return false
	}
	c, ok := m.Condominio(id)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"codigo": "CONDOMINIO_DESCONHECIDO", "error": "condomínio desconhecido: " + id})
		return false
	}
	msg.Tenant, msg.CompanyID = c.Tenant, c.CompanyID
	return true
}

func (s *servidor) atendeConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"agente":                  s.atende.Agente,
		"account":                 s.account,
		"gruvi":                   map[string]any{"configurado": s.gruviOK, "bff_url": s.gruviBFF, "company_id": s.gruviCompany},
		"webhook_token":           s.webhookToken != "",
		"hmac_verificacao_ativa":  true,
		"atende_real_configurado": s.atendeTenant != "" && s.atendeTelefoneCanal != "" && s.atendeBaseURL != "" && s.atendeAPIKey != "",
		"atende_api_base_url":     s.atendeBaseURL,
		"webhook_real":            "/webhook/atende",
		"webhook_emulado":         "/emulador/atende/webhook",
		"reply_url":               s.replyURL,
		"reply_emulada":           s.replyEmulada,
		"nexus_origem":            s.nexus,
		"modelo_provedor":         s.provedor,
		"telefone_b2c":            s.atende.TelefoneB2C,
	})
}

// atendeSessoes: as sessões da administradora pedida (?administradora=) ou da fixa da
// implantação; sem nenhuma, todas.
func (s *servidor) atendeSessoes(w http.ResponseWriter, r *http.Request) {
	adm, ok := s.administradoraOpcional(w, r)
	if !ok {
		return
	}
	sessoes, err := s.atende.Sessoes.Lista(r.Context(), adm)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, 200, map[string]any{"sessoes": sessoes})
}

// sessaoDaRota: a sessão pelo identificador (ID interno ou protocolo), visível nesta
// implantação e, com ?administradora=, dessa administradora. Sessão alheia responde 404,
// como inexistente. Devolve false quando a resposta já foi escrita.
func (s *servidor) sessaoDaRota(w http.ResponseWriter, r *http.Request, identificador string) (*atende.Sessao, bool) {
	sess, ok, err := s.atende.Sessao(r.Context(), identificador)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return nil, false
	}
	if adm := strings.TrimSpace(r.URL.Query().Get("administradora")); ok && adm != "" && sess.AdministradoraID != adm {
		ok = false
	}
	if !ok {
		writeErr(w, 404, errNaoEncontrado)
	}
	return sess, ok
}

// statusDaFalha: persistência que falhou é do servidor (500); o resto é do pedido (400).
func statusDaFalha(err error) int {
	if errors.Is(err, atende.ErrSessaoNaoPersistida) {
		return http.StatusInternalServerError
	}
	return http.StatusBadRequest
}

// responderFalhaDoTransbordo: persistência que falhou é do servidor (500, sem detalhe); a
// ação recusada é do pedido (400), sem ecoar o erro interno.
func (s *servidor) responderFalhaDoTransbordo(w http.ResponseWriter, r *http.Request, acao string, err error) {
	if statusDaFalha(err) == http.StatusInternalServerError {
		s.logger().ErrorContext(r.Context(), "transbordo", "operacao", operacaoDe(r), "acao", acao, "erro", err.Error())
		writeJSON(w, http.StatusInternalServerError, map[string]string{"codigo": "ERRO_INTERNO", "error": "erro interno; tente de novo"})
		return
	}
	s.logger().WarnContext(r.Context(), "transbordo", "operacao", operacaoDe(r), "acao", acao, "erro", err.Error())
	writeJSON(w, http.StatusBadRequest, map[string]string{"codigo": "TRANSBORDO_RECUSADO", "error": "não foi possível " + acao + " o protocolo"})
}

func (s *servidor) atendeSessao(w http.ResponseWriter, r *http.Request) {
	identificador := r.URL.Query().Get("id")
	if identificador == "" {
		identificador = r.URL.Query().Get("protocolo")
	}
	sess, ok := s.sessaoDaRota(w, r, identificador)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"sessao": sess, "saidas": s.outbox.Itens(sess.ID), "recebidas": s.emulador.Recebidas(sess.ID), "reply_emulada": sess.Origem.Conector == "atende-emulado"})
}

// atendeTransbordo: o atendente humano assume, responde ou devolve o protocolo.
func (s *servidor) atendeTransbordo(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Protocolo string `json:"protocolo"`
		SessaoID  string `json:"sessao_id"`
		Acao      string `json:"acao"` // assumir | responder | devolver
		Atendente string `json:"atendente"`
		Texto     string `json:"texto"`
		Motivo    string `json:"motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	if corpo.SessaoID != "" {
		corpo.Protocolo = corpo.SessaoID
	}
	alvo, ok := s.sessaoDaRota(w, r, corpo.Protocolo)
	if !ok {
		return
	}
	var (
		sess *atende.Sessao
		err  error
	)
	switch corpo.Acao {
	case "assumir":
		sess, err = s.atende.Assumir(r.Context(), alvo.ID, corpo.Atendente, corpo.Motivo)
	case "responder":
		sess, err = s.atende.Responder(r.Context(), alvo.ID, corpo.Atendente, corpo.Texto)
	case "devolver":
		sess, err = s.atende.Devolver(r.Context(), alvo.ID)
	default:
		writeErr(w, 400, errors.New("acao: assumir | responder | devolver"))
		return
	}
	if err != nil {
		s.responderFalhaDoTransbordo(w, r, corpo.Acao, err)
		return
	}
	writeJSON(w, 200, map[string]any{"sessao": sess, "saidas": s.outbox.Itens(sess.ID), "recebidas": s.emulador.Recebidas(sess.ID), "reply_emulada": sess.Origem.Conector == "atende-emulado"})
}

func (s *servidor) atendeReiniciar(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("id")
	if p == "" {
		p = r.URL.Query().Get("protocolo")
	}
	sess, ok := s.sessaoDaRota(w, r, p)
	if !ok {
		return
	}
	if err := s.atende.Sessoes.Remover(r.Context(), sess.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, 200, map[string]any{"removido": p})
}

// saidasPorEscopo: as saídas de uma conversa são de quem vê a conversa; a caixa de saída
// inteira (sem id nem protocolo) é do processo, só da Superlógica.
func (s *servidor) saidasPorEscopo() http.Handler {
	daSessao := s.protegida(acesso.ConversasVer, s.alvoSessaoDaConsulta, s.atendeSaidas)
	todas := s.protegida(acesso.StudioAvancado, alvoSuperlogica, s.atendeSaidas)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if identificadorDaSessao(r) != "" {
			daSessao.ServeHTTP(w, r)
			return
		}
		todas.ServeHTTP(w, r)
	})
}

func (s *servidor) atendeSaidas(w http.ResponseWriter, r *http.Request) {
	identificador := r.URL.Query().Get("id")
	if identificador == "" {
		identificador = r.URL.Query().Get("protocolo")
	}
	if identificador != "" {
		sess, ok := s.sessaoDaRota(w, r, identificador)
		if !ok {
			return
		}
		identificador = sess.ID
	}
	writeJSON(w, 200, map[string]any{"saidas": s.outbox.Itens(identificador)})
}

func (s *servidor) atendeIdentidade(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		writeErr(w, 400, errTelefone)
		return
	}
	if !s.gruviOK || s.gruvi == nil || !s.gruvi.PermiteTelefone(phone) {
		writeJSON(w, 200, map[string]any{"configurado": false, "erro": "Gruvi não configurado (MCP_AUTH=b2c, MCP_B2C_MOBILE, MCP_URL e GRUVI_COMPANY_ID no .env)"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	c, ok := s.atende.Config.MCP.(interface{ ValidarToken(context.Context) error })
	if !ok {
		writeJSON(w, 200, map[string]any{"configurado": false, "erro": "cliente MCP não valida o token B2C"})
		return
	}
	if err := c.ValidarToken(ctx); err != nil {
		writeJSON(w, 200, map[string]any{"configurado": true, "erro": err.Error()})
		return
	}
	name := "gruvi.bff.building-operation.get-company-by-document-controller-handle"
	res, err := s.atende.Config.MCP.CallTool(ctx, name, map[string]any{
		"companyId":            s.gruvi.CompanyID,
		"filter_mobile_number": gruvi.NormalizarTelefone(phone),
	})
	if err != nil {
		writeJSON(w, 200, map[string]any{"configurado": true, "erro": err.Error()})
		return
	}
	data, err := harness.NormalizarResultadoMCP(name, res)
	if err != nil {
		writeJSON(w, 200, map[string]any{"configurado": true, "erro": err.Error()})
		return
	}
	identidade := gruvi.IdentidadeDoResultadoMCP(phone, s.gruvi.CompanyID, data)
	writeJSON(w, 200, map[string]any{"configurado": true, "identidade": identidade})
}

// ---- pedidos: o registro por contato e as ações do atendente ----

func (s *servidor) pedidosLer(w http.ResponseWriter, r *http.Request) {
	contato, ok := s.contatoDaRota(w, r, r.URL.Query().Get("phone"))
	if !ok {
		return
	}
	if s.atende.Pedidos == nil {
		writeJSON(w, 200, map[string]any{"ligado": false})
		return
	}
	cart, slots, err := s.atende.Carteira(r.Context(), contato)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ligado": true, "carteira": cart, "slots": slots})
}

func (s *servidor) pedidosAcao(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Acao      string         `json:"acao"` // preencher | cancelar | remover
		Protocolo string         `json:"protocolo"`
		SessaoID  string         `json:"sessao_id"`
		Pedido    string         `json:"pedido"`
		Phone     string         `json:"phone"`
		Atendente string         `json:"atendente"`
		Valores   map[string]any `json:"valores"`
		Confirmar bool           `json:"confirmar"`
		Motivo    string         `json:"motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	if corpo.SessaoID != "" {
		corpo.Protocolo = corpo.SessaoID
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	switch corpo.Acao {
	case "preencher":
		alvo, ok := s.sessaoDaRota(w, r, corpo.Protocolo)
		if !ok {
			return
		}
		sess, p, saida, err := s.atende.PreencherPedido(ctx, alvo.ID, corpo.Pedido, corpo.Atendente, corpo.Valores, corpo.Confirmar)
		if err != nil {
			writeErr(w, statusDaFalha(err), err)
			return
		}
		writeJSON(w, 200, map[string]any{"pedido": p, "saida": saida, "sessao": sess, "saidas": s.outbox.Itens(sess.ID), "recebidas": s.emulador.Recebidas(sess.ID), "reply_emulada": sess.Origem.Conector == "atende-emulado"})
	case "cancelar":
		contato, ok := s.contatoDaRota(w, r, corpo.Phone)
		if !ok {
			return
		}
		p, err := s.atende.CancelarPedido(ctx, contato, corpo.Pedido, corpo.Atendente, corpo.Motivo)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]any{"pedido": p})
	case "remover":
		contato, ok := s.contatoDaRota(w, r, corpo.Phone)
		if !ok {
			return
		}
		if err := s.atende.RemoverPedido(ctx, contato, corpo.Pedido); err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]any{"removido": corpo.Pedido})
	default:
		writeErr(w, 400, errors.New("acao: preencher | cancelar | remover"))
	}
}

// pedidosExportar baixa a carteira do telefone: JSON (completo, com histórico) ou CSV
// (uma linha por pedido, para planilha).
func (s *servidor) pedidosExportar(w http.ResponseWriter, r *http.Request) {
	contato, ok := s.contatoDaRota(w, r, r.URL.Query().Get("phone"))
	if !ok {
		return
	}
	if s.atende.Pedidos == nil {
		writeErr(w, 400, errors.New("registro de pedidos desligado"))
		return
	}
	cart, slots, err := s.atende.Carteira(r.Context(), contato)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	nome := "pedidos-" + contato.AdministradoraID + "-" + contato.Telefone + "-" + time.Now().Format("20060102-1504")
	if strings.EqualFold(r.URL.Query().Get("formato"), "csv") {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+nome+".csv\"")
		w.Write([]byte("\xEF\xBB\xBF")) // BOM: o Excel abre em UTF-8
		cw := csv.NewWriter(w)
		cw.Comma = ';'
		cw.Write([]string{"protocolo", "caso", "estado", "criado_em", "atualizado_em", "parametros", "faltando", "pergunta_em_aberto", "confirmacao", "resultado", "protocolo_recurso", "erro", "protocolo_atende"})
		for _, p := range cart.Pedidos {
			var params, faltam []string
			for _, sl := range slots[p.Caso] {
				if _, ok := p.Valores[sl.Nome]; ok {
					params = append(params, sl.Nome+"="+p.Rotulos[sl.Nome]+" ("+p.Origens[sl.Nome]+")")
				} else if sl.Obrigatorio && p.Aberto() {
					faltam = append(faltam, sl.Titulo)
				}
			}
			pergunta, confirmacao, resultado, protRec := "", "", "", ""
			if p.Pendencia != nil {
				pergunta = fmt.Sprintf("%s (%dx)", p.Pendencia.Pergunta, p.Pendencia.Vezes)
			}
			if p.Confirmacao != nil {
				confirmacao = p.Confirmacao.Texto
				if p.Confirmacao.Resposta != "" {
					confirmacao += " -> " + p.Confirmacao.Resposta + " (" + p.Confirmacao.Por + ")"
				}
			}
			if p.Resultado != nil {
				resultado, protRec = p.Resultado.Texto, p.Resultado.Protocolo
			}
			cw.Write([]string{p.Protocolo, p.CasoNome, p.Estado, p.CriadoEm.Format(time.RFC3339), p.AtualizadoEm.Format(time.RFC3339),
				strings.Join(params, " | "), strings.Join(faltam, " | "), pergunta, confirmacao, resultado, protRec, p.Erro, p.Origem.ProtocoloAtende})
		}
		cw.Flush()
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+nome+".json\"")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(map[string]any{"exportado_em": time.Now(), "carteira": cart, "slots": slots})
}
