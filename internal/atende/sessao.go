package atende

import (
	"sort"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
)

// Sessao é uma conversa do ATENDE: um protocolo, um contato, a identidade resolvida
// e os turnos (mensagens do morador e respostas do concierge). Fica em memória; a
// persistência vem com o Postgres (decisão 4.6).
type Sessao struct {
	Protocolo    string           `json:"protocolo"`
	Tenant       string           `json:"tenant"`
	Contato      Contato          `json:"contato"`
	Canal        Canal            `json:"canal"`
	Identidade   gruvi.Identidade `json:"identidade"`
	IdentErro    string           `json:"identidade_erro,omitempty"`
	Condominio   string           `json:"condominio,omitempty"` // id no Admin ("*" = padrão)
	Turnos       []Turno          `json:"turnos"`
	CriadaEm     time.Time        `json:"criada_em"`
	AtualizadaEm time.Time        `json:"atualizada_em"`
	Processando  bool             `json:"processando"`

	historico []*schema.Message
}

// Turno é uma fala: do morador (incoming) ou do concierge (reply).
type Turno struct {
	Quem     string          `json:"quem"` // morador | concierge | erro
	Texto    string          `json:"texto"`
	Quando   time.Time       `json:"quando"`
	EventoID string          `json:"evento_id,omitempty"`
	Agente   string          `json:"agente,omitempty"`
	Rota     []harness.Passo `json:"rota,omitempty"`
	Uso      *harness.Uso    `json:"uso,omitempty"`
	Modelo   string          `json:"modelo,omitempty"`
	// Memorias: fatos novos ou alterados que esta troca gravou na memória do contato.
	Memorias []memoria.Fato `json:"memorias,omitempty"`
}

// Sessoes guarda as conversas por protocolo.
type Sessoes struct {
	mu sync.Mutex
	m  map[string]*Sessao
}

func NewSessoes() *Sessoes { return &Sessoes{m: map[string]*Sessao{}} }

// Abrir devolve a sessão do protocolo, criando-a com os dados do evento.
func (s *Sessoes) Abrir(ev *Evento) *Sessao {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[ev.Data.Protocol.Number]
	if !ok {
		sess = &Sessao{Protocolo: ev.Data.Protocol.Number, Tenant: ev.Tenant, Contato: ev.Data.Contact, Canal: ev.Data.Channel, CriadaEm: time.Now(), Turnos: []Turno{}}
		s.m[sess.Protocolo] = sess
	}
	if sess.Contato.Name == "" {
		sess.Contato.Name = ev.Data.Contact.Name
	}
	return sess
}

// Get devolve a sessão pelo protocolo.
func (s *Sessoes) Get(protocolo string) (*Sessao, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[protocolo]
	return sess, ok
}

// Lista devolve as sessões, mais recentes primeiro.
func (s *Sessoes) Lista() []*Sessao {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Sessao, 0, len(s.m))
	for _, v := range s.m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AtualizadaEm.After(out[j].AtualizadaEm) })
	return out
}

// Remover apaga a sessão (o emulador reinicia a conversa).
func (s *Sessoes) Remover(protocolo string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, protocolo)
}

func (sess *Sessao) registrar(t Turno) {
	t.Quando = time.Now()
	sess.Turnos = append(sess.Turnos, t)
	sess.AtualizadaEm = t.Quando
}
