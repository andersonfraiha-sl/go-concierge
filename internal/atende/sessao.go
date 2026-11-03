package atende

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/entrada"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
)

// Sessao é uma conversa do ATENDE: um protocolo, um contato, a identidade resolvida
// e os turnos (mensagens do morador e respostas do concierge). Fica em memória; a
// persistência vem com o Postgres (decisão 4.6).
type Sessao struct {
	ID                string            `json:"id"`
	Origem            entrada.Origem    `json:"origem"`
	Destino           entrada.Destino   `json:"destino"`
	Protocolo         string            `json:"protocolo"`
	Tenant            string            `json:"tenant"`
	Contato           entrada.Contato   `json:"contato"`
	Canal             map[string]string `json:"canal"`
	Identidade        gruvi.Identidade  `json:"identidade"`
	IdentErro         string            `json:"identidade_erro,omitempty"`
	Condominio        string            `json:"condominio,omitempty"` // id no Admin ("*" = padrão)
	Turnos            []Turno           `json:"turnos"`
	CriadaEm          time.Time         `json:"criada_em"`
	AtualizadaEm      time.Time         `json:"atualizada_em"`
	Processando       bool              `json:"processando"`
	Encerrada         bool              `json:"encerrada"`
	AtendimentoHumano bool              `json:"atendimento_humano"`
	// Transbordo: o protocolo passou a um atendente humano (o concierge para de
	// responder até ser devolvido).
	Transbordo *Transbordo `json:"transbordo,omitempty"`
	// Plano: estado da orquestração (planning/react) enquanto um item espera o morador;
	// a próxima mensagem retoma por harness.Continuar em vez de planejar de novo.
	Plano *harness.Plano `json:"plano,omitempty"`

	historico       []*schema.Message
	mcpResultados   map[string]any
	revisaoEstado   uint64
	cancelarEntrega context.CancelFunc
	cancelarTurno   context.CancelFunc
	mu              sync.Mutex
	workMu          sync.Mutex
}

// Transbordo é a passagem do protocolo a um humano, como o ATENDE faria ao sair do
// bot: quem pediu (o concierge, por um motivo, ou o atendente assumindo), quando, e
// quem está atendendo.
type Transbordo struct {
	Ativo     bool       `json:"ativo"`
	Estado    string     `json:"estado"`
	Motivo    string     `json:"motivo"`
	Erro      string     `json:"erro,omitempty"`
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
func (s *Sessoes) Abrir(msg entrada.Mensagem) *Sessao {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg = normalizarRoteamento(msg)
	id := idSessao(msg.Origem, msg.ConversaID)
	sess, ok := s.m[id]
	if !ok {
		sess = &Sessao{ID: id, Origem: msg.Origem, Destino: msg.Destino, Protocolo: msg.ConversaID, Tenant: msg.Tenant, Contato: msg.Contato, Canal: msg.Canal, CriadaEm: time.Now(), Turnos: []Turno{}, revisaoEstado: 1}
		sess.Destino.SessaoID = id
		s.m[id] = sess
	}
	sess.mu.Lock()
	if sess.Contato.Name == "" {
		sess.Contato.Name = msg.Contato.Name
	}
	sess.mu.Unlock()
	return sess
}

func (s *Sessoes) Existente(msg entrada.Mensagem) (*Sessao, bool) {
	msg = normalizarRoteamento(msg)
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.m[idSessao(msg.Origem, msg.ConversaID)]
	return sess, ok
}

// Get devolve a sessão pelo protocolo.
func (s *Sessoes) Get(protocolo string) (*Sessao, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.m[protocolo]; ok {
		return sess, true
	}
	var encontrado *Sessao
	for _, sess := range s.m {
		if sess.Protocolo == protocolo {
			if encontrado != nil {
				return nil, false
			}
			encontrado = sess
		}
	}
	return encontrado, encontrado != nil
}

// Lista devolve as sessões, mais recentes primeiro.
func (s *Sessoes) Lista() []*Sessao {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Sessao, 0, len(s.m))
	for _, v := range s.m {
		out = append(out, v)
	}
	instante := func(sess *Sessao) time.Time {
		sess.mu.Lock()
		defer sess.mu.Unlock()
		return sess.AtualizadaEm
	}
	sort.Slice(out, func(i, j int) bool { return instante(out[i]).After(instante(out[j])) })
	return out
}

// Remover apaga a sessão (o emulador reinicia a conversa).
func (s *Sessoes) Remover(protocolo string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[protocolo]; ok {
		delete(s.m, protocolo)
		return
	}
	var id string
	for key, sess := range s.m {
		if sess.Protocolo == protocolo {
			if id != "" {
				return
			}
			id = key
		}
	}
	if id != "" {
		delete(s.m, id)
	}
}

func idSessao(origem entrada.Origem, protocolo string) string {
	return fmt.Sprintf("%d:%s:%d:%s:%d:%s", len(origem.Conector), origem.Conector, len(origem.ConexaoID), origem.ConexaoID, len(protocolo), protocolo)
}

func normalizarRoteamento(msg entrada.Mensagem) entrada.Mensagem {
	if msg.Origem.Conector == "" {
		msg.Origem = entrada.Origem{Conector: "atende", Meio: "whatsapp", ConexaoID: msg.Tenant + ":" + msg.Canal["displayNumber"]}
	}
	if msg.Destino.Conector == "" {
		msg.Destino = entrada.Destino{Conector: msg.Origem.Conector, Meio: msg.Origem.Meio, ConexaoID: msg.Origem.ConexaoID, Protocolo: msg.ConversaID, Telefone: msg.Contato.Phone, Canal: msg.Canal}
	}
	return msg
}

func (sess *Sessao) registrar(t Turno) int {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	t.Quando = time.Now()
	sess.Turnos = append(sess.Turnos, t)
	sess.AtualizadaEm = t.Quando
	return len(sess.Turnos) - 1
}

func (sess *Sessao) MarshalJSON() ([]byte, error) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	type sessaoSerializavel Sessao
	return json.Marshal((*sessaoSerializavel)(sess))
}

func (sess *Sessao) historicoSnapshot() []*schema.Message {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return append([]*schema.Message(nil), sess.historico...)
}

func (sess *Sessao) historicoAtualizar(base int, mensagens []*schema.Message) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if base < 0 || base > len(sess.historico) {
		base = len(sess.historico)
	}
	novas := append([]*schema.Message(nil), sess.historico[base:]...)
	sess.historico = append(append([]*schema.Message(nil), mensagens...), novas...)
}

func (sess *Sessao) historicoAdicionar(mensagem *schema.Message) {
	sess.mu.Lock()
	sess.historico = append(sess.historico, mensagem)
	sess.mu.Unlock()
}

// anotarMemorias põe no turno idx os fatos que a extração (assíncrona) capturou.
func (sess *Sessao) anotarMemorias(idx int, fatos []memoria.Fato) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if idx >= 0 && idx < len(sess.Turnos) && len(fatos) > 0 {
		sess.Turnos[idx].Memorias = fatos
	}
}

func (sess *Sessao) guardarMCPResultado(nome string, resultado any) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.mcpResultados == nil {
		sess.mcpResultados = map[string]any{}
	}
	sess.mcpResultados[nome] = resultado
}

func (sess *Sessao) resultadosMCP() map[string]any {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	resultados := make(map[string]any, len(sess.mcpResultados))
	for nome, resultado := range sess.mcpResultados {
		resultados[nome] = resultado
	}
	return resultados
}
