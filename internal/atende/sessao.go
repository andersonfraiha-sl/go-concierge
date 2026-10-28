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
	// Transbordo: o protocolo passou a um atendente humano (o concierge para de
	// responder até ser devolvido).
	Transbordo *Transbordo `json:"transbordo,omitempty"`
	// Plano: estado da orquestração (planning/react) enquanto um item espera o morador;
	// a próxima mensagem retoma por harness.Continuar em vez de planejar de novo.
	Plano *harness.Plano `json:"plano,omitempty"`

	historico []*schema.Message
	mu        sync.Mutex
}

// Transbordo é a passagem do protocolo a um humano, como o ATENDE faria ao sair do
// bot: quem pediu (o concierge, por um motivo, ou o atendente assumindo), quando, e
// quem está atendendo.
type Transbordo struct {
	Ativo     bool       `json:"ativo"`
	Motivo    string     `json:"motivo"`
	PedidoPor string     `json:"pedido_por"` // concierge | atendente
	Quando    time.Time  `json:"quando"`
	Atendente string     `json:"atendente,omitempty"`
	Devolvido *time.Time `json:"devolvido,omitempty"`
}

// Turno é uma fala: do morador (incoming), do concierge (reply), do atendente humano
// (durante o transbordo) ou do sistema (notas: transferido, devolvido).
type Turno struct {
	Quem     string          `json:"quem"` // morador | concierge | atendente | sistema | erro
	Texto    string          `json:"texto"`
	Quando   time.Time       `json:"quando"`
	EventoID string          `json:"evento_id,omitempty"`
	Agente   string          `json:"agente,omitempty"`
	Rota     []harness.Passo `json:"rota,omitempty"`
	Uso      *harness.Uso    `json:"uso,omitempty"`
	Modelo   string          `json:"modelo,omitempty"`
	// Plano: o TODO/checklist da orquestração depois deste turno (planning/react).
	Plano *harness.Plano `json:"plano,omitempty"`
	// Pedidos: o que este turno fez no registro de pedidos (abriu, perguntou, concluiu…).
	Pedidos []PedidoTurno `json:"pedidos,omitempty"`
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

func (sess *Sessao) registrar(t Turno) int {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	t.Quando = time.Now()
	sess.Turnos = append(sess.Turnos, t)
	sess.AtualizadaEm = t.Quando
	return len(sess.Turnos) - 1
}

// anotarMemorias põe no turno idx os fatos que a extração (assíncrona) capturou.
func (sess *Sessao) anotarMemorias(idx int, fatos []memoria.Fato) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if idx >= 0 && idx < len(sess.Turnos) && len(fatos) > 0 {
		sess.Turnos[idx].Memorias = fatos
	}
}
