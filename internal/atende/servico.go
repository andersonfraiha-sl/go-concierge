package atende

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"go-concierge/internal/admin"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
	"go-concierge/internal/okf"
)

// Sender entrega a resposta do concierge ao protocolo do ATENDE.
type Sender interface {
	Enviar(ctx context.Context, protocolo string, texto string) error
}

// Saida é uma resposta enviada (ou a enviar) ao ATENDE.
type Saida struct {
	Protocolo string    `json:"protocolo"`
	Texto     string    `json:"texto"`
	Quando    time.Time `json:"quando"`
	Enviada   bool      `json:"enviada"`
	Erro      string    `json:"erro,omitempty"`
}

// Outbox é o Sender do emulador: guarda as respostas para o Console ler. Também
// embrulha um Sender real, quando houver, registrando o resultado.
type Outbox struct {
	mu    sync.Mutex
	itens []Saida
	Real  Sender
}

func (o *Outbox) Enviar(ctx context.Context, protocolo, texto string) error {
	s := Saida{Protocolo: protocolo, Texto: texto, Quando: time.Now()}
	if o.Real != nil {
		if err := o.Real.Enviar(ctx, protocolo, texto); err != nil {
			s.Erro = err.Error()
		} else {
			s.Enviada = true
		}
	}
	o.mu.Lock()
	o.itens = append(o.itens, s)
	o.mu.Unlock()
	if s.Erro != "" {
		return fmt.Errorf("atende: envio falhou: %s", s.Erro)
	}
	return nil
}

// Itens devolve as saídas de um protocolo ("" = todas).
func (o *Outbox) Itens(protocolo string) []Saida {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := []Saida{}
	for _, s := range o.itens {
		if protocolo == "" || s.Protocolo == protocolo {
			out = append(out, s)
		}
	}
	return out
}

// HTTPSender manda a resposta ao ATENDE por HTTP: POST {URL} com
// {"protocol": ..., "message": ...} e Bearer {Token}. O contrato real de resposta
// do ATENDE ainda não foi passado; este é o encaixe para quando vier.
type HTTPSender struct {
	URL   string
	Token string
	HTTP  *http.Client
}

func (h *HTTPSender) Enviar(ctx context.Context, protocolo, texto string) error {
	raw, _ := json.Marshal(map[string]string{"protocol": protocolo, "message": texto})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
	cli := h.HTTP
	if cli == nil {
		cli = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	return nil
}

// Servico liga o webhook ao concierge: identidade pelo Gruvi, harness no agente de
// primeiro nível, resposta pelo Sender.
type Servico struct {
	Cat         *okf.Catalogo
	Agente      string // caminho relativo do agente de primeiro nível (agentes/concierge)
	Config      harness.Config
	Account     string // account do Nexus para os recursos (DEFAULT_ACCOUNT_ID)
	Gruvi       *gruvi.Client
	Sender      Sender
	Sessoes     *Sessoes
	Timeout     time.Duration
	Log         *log.Logger
	Identificar func(ctx context.Context, phone string) (gruvi.Identidade, error) // nil = Gruvi (ou público, se não configurado)
	// Memorias é a memória relacional por contato (nil = desligada); Extrator captura
	// fatos novos a cada troca (nil = memoria.Regras).
	Memorias memoria.Store
	Extrator memoria.Extrator
	// Mapeamento é o Admin da administradora (nil = tudo habilitado, sem payloads):
	// resolve o condomínio da conversa, filtra os casos de uso e injeta os payloads.
	Mapeamento *admin.Store
}

// Processar trata um evento: abre a sessão, identifica o contato (uma vez por
// sessão), roda o concierge com o histórico e envia a resposta.
func (s *Servico) Processar(ctx context.Context, ev *Evento) (*Sessao, error) {
	sess := s.Sessoes.Abrir(ev)
	texto := ev.Texto()
	if texto == "" {
		return sess, fmt.Errorf("evento %s sem texto (tipo %s)", ev.ID, ev.Data.Message.Type)
	}
	sess.Processando = true
	defer func() { sess.Processando = false }()
	sess.registrar(Turno{Quem: "morador", Texto: texto, EventoID: ev.ID})

	if sess.Identidade.Phone == "" {
		sess.Identidade, sess.IdentErro = s.identificar(ctx, ev.Data.Contact.Phone)
		if sess.Identidade.Name == "" {
			sess.Identidade.Name = ev.Data.Contact.Name
		}
	}

	cfg := s.Config
	cfg.Sessao = ev.Data.Protocol.Number
	cfg.Identidade = harness.Identidade{Authenticated: sess.Identidade.Authenticated, AccountID: s.Account, Role: sess.Identidade.Role}

	// Admin da administradora: qual condomínio é este, quais casos ele pode usar e
	// quais definições (payloads) valem para os recursos
	var contexto []string
	if s.Mapeamento != nil {
		if m, err := s.Mapeamento.Ler(); err == nil {
			condo, ok := m.Resolver(sess.Identidade.CompanyID, ev.Tenant, s.Account)
			condoID := admin.Padrao
			if ok {
				condoID = condo.ID
				if condo.Account != "" {
					cfg.Identidade.AccountID = condo.Account
				}
			}
			sess.Condominio = condoID
			papel := sess.Identidade.Role
			cfg.Casos = func(nome, casoID string) bool {
				return m.Habilitado(condoID, nome, papel) && (casoID == "" || m.Habilitado(condoID, casoID, papel))
			}
			cfg.Payloads = m.PayloadsDe(condoID)
			orient := m.OrientacaoDe(condoID)
			cfg.Orientacoes = orient.PorCaso
			if c := admin.ContextoDoCondominio(condo, cfg.Payloads, orient.Geral); c != "" && (ok || len(cfg.Payloads) > 0 || orient.Geral != "") {
				contexto = append(contexto, c)
			}
		} else if s.Log != nil {
			s.Log.Printf("atende: mapeamento: %v", err)
		}
	}

	// memória relacional: o que já se sabe deste telefone entra no prompt da família
	var mem memoria.Memoria
	if s.Memorias != nil {
		m, err := s.Memorias.Ler(ctx, s.Account, ev.Data.Contact.Phone)
		if err != nil && s.Log != nil {
			s.Log.Printf("atende: memória de %s: %v", ev.Data.Contact.Phone, err)
		}
		mem = m
		if c := harness.ContextoDeMemoria(mem); c != "" {
			contexto = append(contexto, c)
		}
	}
	cfg.Contexto = strings.Join(contexto, "\n\n")
	dir, err := s.Cat.ResolveAgente(s.Agente)
	if err != nil {
		return sess, err
	}
	def, err := s.Cat.Load(dir)
	if err != nil {
		return sess, err
	}
	if s.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.Timeout)
		defer cancel()
	}
	ag, err := harness.Build(ctx, s.Cat, def, cfg)
	if err != nil {
		sess.registrar(Turno{Quem: "erro", Texto: err.Error()})
		return sess, err
	}
	entrada := texto
	if sess.Identidade.Authenticated && sess.Identidade.Name != "" && len(sess.historico) == 0 {
		entrada = fmt.Sprintf("[morador identificado: %s, unidades: %s]\n%s", sess.Identidade.Name, rotulos(sess.Identidade.Units), texto)
	}
	res, err := harness.Run(ctx, ag, entrada, sess.historico)
	if err != nil {
		sess.registrar(Turno{Quem: "erro", Texto: err.Error()})
		return sess, err
	}
	sess.historico = res.Mensagens
	uso := res.Uso
	turno := Turno{Quem: "concierge", Texto: res.Resposta, Agente: res.Agente, Rota: res.Rota, Uso: &uso, Modelo: ag.NomeModelo}
	if s.Memorias != nil {
		turno.Memorias = s.lembrar(ctx, &mem, sess, texto, res.Resposta)
	}
	sess.registrar(turno)
	if s.Sender != nil && res.Resposta != "" {
		if err := s.Sender.Enviar(ctx, sess.Protocolo, res.Resposta); err != nil {
			sess.registrar(Turno{Quem: "erro", Texto: err.Error()})
			return sess, err
		}
	}
	return sess, nil
}

// lembrar extrai os fatos duráveis da troca e grava na memória do contato. Devolve
// os fatos novos ou alterados (para a bancada do Console).
func (s *Servico) lembrar(ctx context.Context, mem *memoria.Memoria, sess *Sessao, mensagem, resposta string) []memoria.Fato {
	ex := s.Extrator
	if ex == nil {
		ex = memoria.Regras{}
	}
	fatos, err := ex.Extrair(ctx, mensagem, resposta, *mem)
	if err != nil {
		if s.Log != nil {
			s.Log.Printf("atende: extrator de memória: %v", err)
		}
		return nil
	}
	if mem.Account == "" {
		mem.Account = s.Account
	}
	if mem.Telefone == "" {
		mem.Telefone = sess.Contato.Phone
	}
	if mem.Nome == "" {
		mem.Nome = sess.Identidade.Name
	}
	var novos []memoria.Fato
	for _, f := range fatos {
		f.Fonte = sess.Protocolo
		if mem.Upsert(f) {
			novos = append(novos, f)
		}
	}
	if len(novos) > 0 {
		if err := s.Memorias.Gravar(ctx, *mem); err != nil && s.Log != nil {
			s.Log.Printf("atende: gravar memória: %v", err)
		}
	}
	return novos
}

func (s *Servico) identificar(ctx context.Context, phone string) (gruvi.Identidade, string) {
	if s.Identificar != nil {
		id, err := s.Identificar(ctx, phone)
		if err != nil {
			return gruvi.Identidade{Phone: phone, Units: []gruvi.Unidade{}}, err.Error()
		}
		return id, ""
	}
	if s.Gruvi == nil || !s.Gruvi.Configurado() {
		return gruvi.Identidade{Phone: phone, Units: []gruvi.Unidade{}}, "Gruvi não configurado (GRUVI_CLIENT_BASIC, GRUVI_COMPANY_ID): contato tratado como público"
	}
	id, err := s.Gruvi.Identidade(ctx, phone)
	if err != nil {
		if s.Log != nil {
			s.Log.Printf("atende: identidade de %s falhou: %v", phone, err)
		}
		return gruvi.Identidade{Phone: phone, Units: []gruvi.Unidade{}}, err.Error()
	}
	return id, ""
}

func rotulos(us []gruvi.Unidade) string {
	if len(us) == 0 {
		return "nenhuma"
	}
	s := ""
	for i, u := range us {
		if i > 0 {
			s += ", "
		}
		s += u.Label
	}
	return s
}
