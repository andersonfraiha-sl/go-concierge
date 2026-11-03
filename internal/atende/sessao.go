package atende

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
)

// Sessao é uma conversa do ATENDE: um protocolo, um contato, a identidade resolvida
// e os turnos (mensagens do morador e respostas do concierge). Sessoes a grava no
// SessaoStore ao fim de cada operação.
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
	Condominio        string            `json:"condominio,omitempty"` // id no Admin, resolvido pela mensagem
	AdministradoraID  string            `json:"administradora_id,omitempty"`
	CompanyID         string            `json:"company_id,omitempty"` // company do Gruvi do condomínio resolvido
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

	escopo          escopo.Contato // o contato na administradora resolvida (vazio até resolver)
	historico       []*schema.Message
	mcpResultados   map[string]any
	revisaoEstado   uint64
	cancelarEntrega context.CancelFunc
	cancelarTurno   context.CancelFunc
	mu              sync.Mutex
	workMu          sync.Mutex
	persistMu       sync.Mutex // ordena as gravações: a última a gravar leva a foto mais nova
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
	Quem  string `json:"quem"` // morador | concierge | atendente | sistema | erro
	Texto string `json:"texto"`
	// Erro: o erro técnico por trás de uma resposta de falha do concierge (o morador
	// recebe só o Texto; o Console mostra o Erro em letra miúda).
	Erro     string          `json:"erro,omitempty"`
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

// Sessoes guarda as conversas por protocolo. O store é a fonte durável; o mapa é o
// cache das sessões vivas neste processo (mutexes e cancelamentos não se serializam).
type Sessoes struct {
	store SessaoStore
	mu    sync.Mutex
	m     map[string]*Sessao
}

// limiteLista: quantas sessões a listagem do Console devolve.
const limiteLista = 200

func NewSessoes(store SessaoStore) *Sessoes {
	return &Sessoes{store: store, m: map[string]*Sessao{}}
}

// Abrir devolve a sessão do protocolo (do cache, do store ou nova, com os dados do evento).
func (s *Sessoes) Abrir(ctx context.Context, msg entrada.Mensagem) (*Sessao, error) {
	msg = normalizarRoteamento(msg)
	id := idSessao(msg.Origem, msg.ConversaID)
	sess, ok, err := s.carregar(ctx, id)
	if err != nil {
		return nil, err
	}
	if !ok {
		sess = s.adotar(novaSessao(id, msg))
	}
	sess.mu.Lock()
	if sess.Contato.Name == "" {
		sess.Contato.Name = msg.Contato.Name
	}
	sess.mu.Unlock()
	return sess, nil
}

func novaSessao(id string, msg entrada.Mensagem) *Sessao {
	sess := &Sessao{ID: id, Origem: msg.Origem, Destino: msg.Destino, Protocolo: msg.ConversaID, Tenant: msg.Tenant, Contato: msg.Contato, Canal: msg.Canal, CriadaEm: time.Now(), Turnos: []Turno{}, revisaoEstado: 1}
	sess.Destino.SessaoID = id
	return sess
}

// Existente devolve a sessão do protocolo do evento, sem criar.
func (s *Sessoes) Existente(ctx context.Context, msg entrada.Mensagem) (*Sessao, bool, error) {
	msg = normalizarRoteamento(msg)
	return s.carregar(ctx, idSessao(msg.Origem, msg.ConversaID))
}

// Get devolve a sessão pelo ID interno ou, entre as do cache, pelo protocolo (quando só
// uma o usa).
func (s *Sessoes) Get(ctx context.Context, id string) (*Sessao, bool, error) {
	if sess, ok := s.porProtocolo(id); ok {
		return sess, true, nil
	}
	return s.carregar(ctx, id)
}

func (s *Sessoes) porProtocolo(protocolo string) (*Sessao, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[protocolo]; ok {
		return nil, false
	}
	var encontrado *Sessao
	for _, sess := range s.m {
		if sess.Protocolo != protocolo {
			continue
		}
		if encontrado != nil {
			return nil, false
		}
		encontrado = sess
	}
	return encontrado, encontrado != nil
}

// carregar busca a sessão no cache e, se não estiver, no store.
func (s *Sessoes) carregar(ctx context.Context, id string) (*Sessao, bool, error) {
	s.mu.Lock()
	sess, ok := s.m[id]
	s.mu.Unlock()
	if ok {
		return sess, true, nil
	}
	r, ok, err := s.store.Carregar(ctx, id)
	if err != nil || !ok {
		return nil, false, err
	}
	sess, err = RestaurarSessao(r)
	if err != nil {
		return nil, false, err
	}
	return s.adotar(sess), true, nil
}

// adotar põe a sessão no cache, a menos que outra chamada já tenha posto uma com o
// mesmo ID: vale a que chegou primeiro, para todos partilharem a mesma instância.
func (s *Sessoes) adotar(sess *Sessao) *Sessao {
	s.mu.Lock()
	defer s.mu.Unlock()
	if viva, ok := s.m[sess.ID]; ok {
		return viva
	}
	s.m[sess.ID] = sess
	return sess
}

// ErrSessaoNaoPersistida: a sessão mudou em memória, mas não foi gravada no store.
var ErrSessaoNaoPersistida = errors.New("sessão não persistida")

// Persistir grava a sessão no store. A falha é logada (ERROR) e devolvida.
func (s *Sessoes) Persistir(ctx context.Context, sess *Sessao) error {
	sess.persistMu.Lock()
	defer sess.persistMu.Unlock()
	r, err := sess.Registro()
	if err == nil {
		err = s.store.Salvar(ctx, r)
	}
	if err != nil {
		slog.ErrorContext(ctx, "atende: sessão não persistida", "operacao", "persistir_sessao", "sessao", sess.ID, "erro", err)
		return fmt.Errorf("%w %s: %w", ErrSessaoNaoPersistida, sess.ID, err)
	}
	return nil
}

// Lista devolve as sessões da administradora (vazia = todas), mais recentes primeiro:
// as do store e as que ainda só existem neste processo, na versão viva do cache.
func (s *Sessoes) Lista(ctx context.Context, administradoraID string) ([]*Sessao, error) {
	regs, err := s.store.Listar(ctx, administradoraID, limiteLista)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	vivas := maps.Clone(s.m)
	s.mu.Unlock()
	out := make([]*Sessao, 0, len(regs)+len(vivas))
	for _, r := range regs {
		if viva, ok := vivas[r.ID]; ok {
			out = append(out, viva)
			delete(vivas, r.ID)
			continue
		}
		sess, err := RestaurarSessao(r)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	for _, viva := range vivas {
		if administradoraID == "" || viva.administradora() == administradoraID {
			out = append(out, viva)
		}
	}
	slices.SortFunc(out, maisRecentesPrimeiro)
	return out[:min(limiteLista, len(out))], nil
}

// Remover apaga a sessão do cache e do store (o emulador reinicia a conversa).
func (s *Sessoes) Remover(ctx context.Context, id string) error {
	s.mu.Lock()
	delete(s.m, id)
	s.mu.Unlock()
	return s.store.Remover(ctx, id)
}

func maisRecentesPrimeiro(a, b *Sessao) int { return b.atualizadaEm().Compare(a.atualizadaEm()) }

func (sess *Sessao) atualizadaEm() time.Time {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return sess.AtualizadaEm
}

func (sess *Sessao) administradora() string {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return sess.AdministradoraID
}

// sessaoSerializavel é a Sessao sem os métodos (o MarshalJSON dela trava sess.mu).
type sessaoSerializavel Sessao

// sessaoGravada é o que vai em RegistroSessao.Dados: os campos públicos e o estado
// privado que a conversa precisa para continuar depois de um restart.
type sessaoGravada struct {
	*sessaoSerializavel
	Escopo        escopo.Contato    `json:"escopo"`
	Historico     []*schema.Message `json:"historico"`
	MCPResultados map[string]any    `json:"mcp_resultados,omitempty"`
	RevisaoEstado uint64            `json:"revisao_estado"`
}

// Registro serializa a sessão para o store.
func (sess *Sessao) Registro() (RegistroSessao, error) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	dados, err := json.Marshal(sessaoGravada{
		sessaoSerializavel: (*sessaoSerializavel)(sess),
		Escopo:             sess.escopo, Historico: sess.historico, MCPResultados: sess.mcpResultados, RevisaoEstado: sess.revisaoEstado,
	})
	if err != nil {
		return RegistroSessao{}, fmt.Errorf("serializar sessão %s: %w", sess.ID, err)
	}
	return RegistroSessao{
		ID: sess.ID, AdministradoraID: sess.AdministradoraID, CondominioID: sess.Condominio, Protocolo: sess.Protocolo,
		AtualizadaEm: sess.AtualizadaEm, Dados: dados,
	}, nil
}

// RestaurarSessao reconstrói a sessão gravada. Um turno não sobrevive ao processo que o
// rodava: a sessão volta destravada (Processando false).
func RestaurarSessao(r RegistroSessao) (*Sessao, error) {
	sess := &Sessao{}
	gravada := sessaoGravada{sessaoSerializavel: (*sessaoSerializavel)(sess)}
	if err := json.Unmarshal(r.Dados, &gravada); err != nil {
		return nil, fmt.Errorf("restaurar sessão %s: %w", r.ID, err)
	}
	sess.escopo, sess.historico, sess.mcpResultados, sess.revisaoEstado = gravada.Escopo, gravada.Historico, gravada.MCPResultados, gravada.RevisaoEstado
	sess.Processando = false
	return sess, nil
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

// contato é o morador na administradora do condomínio resolvido para a conversa.
func (sess *Sessao) contato() escopo.Contato {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return sess.escopo
}

// companyDoCondominio é a company do Gruvi do condomínio resolvido (vazia até resolver).
func (sess *Sessao) companyDoCondominio() string {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return sess.CompanyID
}

func (sess *Sessao) fixarContato(c escopo.Contato, companyID string) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.escopo, sess.AdministradoraID, sess.Condominio, sess.CompanyID = c, c.AdministradoraID, c.CondominioID, companyID
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
