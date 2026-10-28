package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"go-concierge/internal/atende"
	"go-concierge/internal/memoria"
)

// Canal ATENDE: o webhook que recebe os eventos, e a API do Console (emulador):
// sessões por protocolo, caixa de saída e consulta de identidade no Gruvi.

func (s *servidor) rotasAtende(mux *http.ServeMux) {
	mux.HandleFunc("POST /webhook/atende", s.webhookAtende)
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
	mux.HandleFunc("POST /emulador/atende/messages", s.emulador.Handler)
	mux.HandleFunc("GET /api/atende/config", s.atendeConfig)
	mux.HandleFunc("GET /api/atende/sessoes", s.atendeSessoes)
	mux.HandleFunc("GET /api/atende/sessao", s.atendeSessao)
	mux.HandleFunc("DELETE /api/atende/sessao", s.atendeReiniciar)
	mux.HandleFunc("GET /api/atende/saidas", s.atendeSaidas)
	mux.HandleFunc("POST /api/atende/transbordo", s.atendeTransbordo)
	mux.HandleFunc("GET /api/atende/identidade", s.atendeIdentidade)
	mux.HandleFunc("GET /api/pedidos", s.pedidosLer)
	mux.HandleFunc("POST /api/pedidos/acao", s.pedidosAcao)
	mux.HandleFunc("GET /api/memoria", s.memoriaLer)
	mux.HandleFunc("DELETE /api/memoria", s.memoriaEsquecer)
	mux.HandleFunc("GET /api/memorias", s.memoriasListar)
}

// ---- memória relacional por contato ------------------------------------

func (s *servidor) memoriaLer(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		writeErr(w, 400, errTelefone)
		return
	}
	m, err := s.memorias.Ler(r.Context(), s.account, phone)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"memoria": m})
}

func (s *servidor) memoriaEsquecer(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	phone, chave := strings.TrimSpace(q.Get("phone")), strings.TrimSpace(q.Get("chave"))
	if phone == "" {
		writeErr(w, 400, errTelefone)
		return
	}
	m, err := s.memorias.Ler(r.Context(), s.account, phone)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	if chave == "" {
		m.Fatos = nil
	} else if !m.Esquecer(chave) {
		writeErr(w, 404, errors.New("fato não encontrado: "+chave))
		return
	}
	if m.Fatos == nil {
		m.Fatos = []memoria.Fato{}
	}
	if err := s.memorias.Gravar(r.Context(), m); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"memoria": m})
}

func (s *servidor) memoriasListar(w http.ResponseWriter, r *http.Request) {
	l, err := s.memorias.Listar(r.Context(), s.account)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"account": s.account, "memorias": l})
}

// webhookAtende aceita o evento (202) e processa em segundo plano; a resposta ao
// morador sai pelo Sender (caixa de saída do emulador e, se configurado, o ATENDE).
func (s *servidor) webhookAtende(w http.ResponseWriter, r *http.Request) {
	if s.webhookToken != "" {
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
	ev, err := atende.Parse(raw)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	sess := s.atende.Sessoes.Abrir(ev)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
		defer cancel()
		if _, err := s.atende.Processar(ctx, ev); err != nil {
			s.log.Printf("atende: protocolo %s: %v", ev.Data.Protocol.Number, err)
		}
	}()
	writeJSON(w, 202, map[string]any{"accepted": true, "event": ev.ID, "protocol": sess.Protocolo})
}

func (s *servidor) atendeConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"agente":          s.atende.Agente,
		"account":         s.account,
		"gruvi":           map[string]any{"configurado": s.gruviOK, "token_url": s.gruviTokenURL, "bff_url": s.gruviBFF, "company_id": s.gruviCompany},
		"webhook_token":   s.webhookToken != "",
		"reply_url":       s.replyURL,
		"reply_emulada":   s.replyEmulada,
		"nexus_origem":    s.nexus,
		"modelo_provedor": s.provedor,
	})
}

func (s *servidor) atendeSessoes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"sessoes": s.atende.Sessoes.Lista()})
}

func (s *servidor) atendeSessao(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.atende.Sessoes.Get(r.URL.Query().Get("protocolo"))
	if !ok {
		writeErr(w, 404, errNaoEncontrado)
		return
	}
	writeJSON(w, 200, map[string]any{"sessao": sess, "saidas": s.outbox.Itens(sess.Protocolo), "recebidas": s.emulador.Recebidas(sess.Protocolo), "reply_emulada": s.replyEmulada})
}

// atendeTransbordo: o atendente humano assume, responde ou devolve o protocolo.
func (s *servidor) atendeTransbordo(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Protocolo string `json:"protocolo"`
		Acao      string `json:"acao"` // assumir | responder | devolver
		Atendente string `json:"atendente"`
		Texto     string `json:"texto"`
		Motivo    string `json:"motivo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	var (
		sess *atende.Sessao
		err  error
	)
	switch corpo.Acao {
	case "assumir":
		sess, err = s.atende.Assumir(corpo.Protocolo, corpo.Atendente, corpo.Motivo)
	case "responder":
		sess, err = s.atende.Responder(r.Context(), corpo.Protocolo, corpo.Atendente, corpo.Texto)
	case "devolver":
		sess, err = s.atende.Devolver(corpo.Protocolo)
	default:
		err = errors.New("acao: assumir | responder | devolver")
	}
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"sessao": sess, "saidas": s.outbox.Itens(sess.Protocolo), "recebidas": s.emulador.Recebidas(sess.Protocolo), "reply_emulada": s.replyEmulada})
}

func (s *servidor) atendeReiniciar(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("protocolo")
	s.atende.Sessoes.Remover(p)
	writeJSON(w, 200, map[string]any{"removido": p})
}

func (s *servidor) atendeSaidas(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"saidas": s.outbox.Itens(r.URL.Query().Get("protocolo"))})
}

func (s *servidor) atendeIdentidade(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		writeErr(w, 400, errTelefone)
		return
	}
	if !s.gruviOK {
		writeJSON(w, 200, map[string]any{"configurado": false, "erro": "Gruvi não configurado (GRUVI_CLIENT_BASIC, GRUVI_COMPANY_ID no .env)"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	id, err := s.gruvi.Identidade(ctx, phone)
	if err != nil {
		writeJSON(w, 200, map[string]any{"configurado": true, "erro": err.Error(), "identidade": id})
		return
	}
	writeJSON(w, 200, map[string]any{"configurado": true, "identidade": id})
}

// ---- pedidos: o registro por contato e as ações do atendente ----

func (s *servidor) pedidosLer(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))
	if phone == "" {
		writeErr(w, 400, errTelefone)
		return
	}
	if s.atende.Pedidos == nil {
		writeJSON(w, 200, map[string]any{"ligado": false})
		return
	}
	cart, slots, err := s.atende.Carteira(r.Context(), phone)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ligado": true, "carteira": cart, "slots": slots})
}

func (s *servidor) pedidosAcao(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Acao      string         `json:"acao"` // preencher | cancelar
		Protocolo string         `json:"protocolo"`
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
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	switch corpo.Acao {
	case "preencher":
		sess, p, saida, err := s.atende.PreencherPedido(ctx, corpo.Protocolo, corpo.Pedido, corpo.Atendente, corpo.Valores, corpo.Confirmar)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]any{"pedido": p, "saida": saida, "sessao": sess, "saidas": s.outbox.Itens(sess.Protocolo), "recebidas": s.emulador.Recebidas(sess.Protocolo), "reply_emulada": s.replyEmulada})
	case "cancelar":
		p, err := s.atende.CancelarPedido(ctx, corpo.Phone, corpo.Pedido, corpo.Atendente, corpo.Motivo)
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		writeJSON(w, 200, map[string]any{"pedido": p})
	default:
		writeErr(w, 400, errors.New("acao: preencher | cancelar"))
	}
}
