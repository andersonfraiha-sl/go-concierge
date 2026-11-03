package atende

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/admin"
	"go-concierge/internal/entrada"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
	"go-concierge/internal/okf"
	"go-concierge/internal/pedidos"
)

// Sender entrega a resposta do concierge ao protocolo do ATENDE.
type Sender interface {
	Enviar(ctx context.Context, destino entrada.Destino, texto string) error
}

type chaveControleEntrega struct{}

var errEntregaSuprimida = errors.New("entrega suprimida após mudança de atendimento")

type entregaParcial struct {
	partes []string
	erro   error
}

func (e *entregaParcial) Error() string { return e.erro.Error() }
func (e *entregaParcial) Unwrap() error { return e.erro }

func textoEntregue(texto string, err error) string {
	if err == nil {
		return texto
	}
	var parcial *entregaParcial
	if errors.As(err, &parcial) {
		return strings.Join(parcial.partes, "\n")
	}
	return ""
}

func autorizarEntrega(ctx context.Context) bool {
	verificar, ok := ctx.Value(chaveControleEntrega{}).(func() bool)
	return !ok || verificar()
}

// Saida é uma resposta enviada (ou a enviar) ao ATENDE.
type Saida struct {
	SessaoID  string    `json:"sessao_id,omitempty"`
	Protocolo string    `json:"protocolo"`
	Tipo      string    `json:"tipo,omitempty"` // "" = mensagem ao morador; transbordo = pedido de humano ao ATENDE
	Texto     string    `json:"texto"`
	Quando    time.Time `json:"quando"`
	Enviada   bool      `json:"enviada"`
	Erro      string    `json:"erro,omitempty"`
}

// Outbox é o Sender do emulador: guarda as respostas para o Console ler. Também
// embrulha um Sender real, quando houver, registrando o resultado.
type Outbox struct {
	mu      sync.Mutex
	itens   []Saida
	Real    Sender
	ATENDE  Sender
	Emulado Sender
}

func (o *Outbox) Enviar(ctx context.Context, destino entrada.Destino, texto string) error {
	if !autorizarEntrega(ctx) {
		return errEntregaSuprimida
	}
	s := Saida{SessaoID: destino.SessaoID, Protocolo: destino.Protocolo, Texto: texto, Quando: time.Now()}
	sender := o.Real
	if destino.Conector == "atende-emulado" {
		if o.Emulado != nil {
			sender = o.Emulado
		}
	} else if destino.Conector == "atende" {
		if o.ATENDE != nil {
			sender = o.ATENDE
		}
	}
	var erro error
	if sender != nil {
		if err := sender.Enviar(ctx, destino, texto); err != nil {
			erro = err
			s.Erro = err.Error()
			var parcial *entregaParcial
			if errors.As(err, &parcial) {
				s.Texto = strings.Join(parcial.partes, "\n")
			}
		} else {
			s.Enviada = true
		}
	}
	o.mu.Lock()
	o.itens = append(o.itens, s)
	o.mu.Unlock()
	if s.Erro != "" {
		return fmt.Errorf("atende: envio falhou: %w", erro)
	}
	return nil
}

// Itens devolve as saídas de um protocolo ("" = todas).
// Transbordar registra o pedido de atendente humano ao ATENDE. No ATENDE real isso vira
// a chamada que tira o protocolo do bot (isHumanAttendance); aqui fica na caixa.
func (o *Outbox) Transbordar(destino entrada.Destino, motivo string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.itens = append(o.itens, Saida{SessaoID: destino.SessaoID, Protocolo: destino.Protocolo, Tipo: "transbordo", Texto: motivo, Quando: time.Now(), Enviada: destino.Conector == "atende-emulado"})
}

func (o *Outbox) Itens(protocolo string) []Saida {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := []Saida{}
	porID := false
	for _, s := range o.itens {
		if protocolo != "" && s.SessaoID == protocolo {
			porID = true
			break
		}
	}
	var sessao string
	for _, s := range o.itens {
		if protocolo == "" || porID && s.SessaoID == protocolo || !porID && s.Protocolo == protocolo {
			if protocolo != "" && !porID && s.SessaoID != "" {
				if sessao != "" && sessao != s.SessaoID {
					return nil
				}
				sessao = s.SessaoID
			}
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

func (h *HTTPSender) Enviar(ctx context.Context, destino entrada.Destino, texto string) error {
	raw, _ := json.Marshal(map[string]string{"protocol": destino.Protocolo, "session_id": destino.SessaoID, "message": texto})
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

type ATENDESender struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func (a *ATENDESender) Enviar(ctx context.Context, destino entrada.Destino, texto string) error {
	if strings.TrimSpace(a.BaseURL) == "" || strings.TrimSpace(a.APIKey) == "" {
		return errors.New("ATENDE: configuração de envio incompleta")
	}
	client := a.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	} else {
		copia := *client
		copia.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		if copia.Timeout <= 0 {
			copia.Timeout = 20 * time.Second
		}
		client = &copia
	}
	partes := DividirMensagem(texto, 4000)
	var enviadas []string
	falha := func(err error) error {
		if len(enviadas) == 0 {
			return err
		}
		return &entregaParcial{partes: append([]string(nil), enviadas...), erro: err}
	}
	for i, parte := range partes {
		formatada := FormatarMensagem(parte)
		corpo, err := json.Marshal(map[string]any{
			"contact": map[string]string{"phone": "5519981621916"},
			// "contact": map[string]string{"phone": destino.Telefone},
			"message": map[string]string{"type": "text", "text": formatada},
		})
		if err != nil {
			return falha(fmt.Errorf("ATENDE: preparar parte %d: %w", i+1, err))
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.BaseURL, "/")+"/integrations/messages", bytes.NewReader(corpo))
		if err != nil {
			return falha(fmt.Errorf("ATENDE: preparar parte %d: %w", i+1, err))
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", a.APIKey)
		if !autorizarEntrega(ctx) {
			return falha(fmt.Errorf("ATENDE: parte %d de %d: %w", i+1, len(partes), errEntregaSuprimida))
		}
		res, err := client.Do(req)
		if err != nil {
			return falha(fmt.Errorf("ATENDE: parte %d de %d falhou: %w", i+1, len(partes), err))
		}
		_ = res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return falha(fmt.Errorf("ATENDE: parte %d de %d respondeu HTTP %d", i+1, len(partes), res.StatusCode))
		}
		enviadas = append(enviadas, formatada)
	}
	return nil
}

var negritoWhatsApp = regexp.MustCompile(`(?s)\*\*(.+?)\*\*`)
var marcadorWhatsApp = regexp.MustCompile(`(?m)^\* +`)
var tituloWhatsApp = regexp.MustCompile(`(?m)^#{1,6} (.+)$`)

func FormatarMensagem(texto string) string {
	texto = negritoWhatsApp.ReplaceAllString(texto, "*$1*")
	texto = marcadorWhatsApp.ReplaceAllString(texto, "- ")
	return tituloWhatsApp.ReplaceAllString(texto, "*$1*")
}

func DividirMensagem(texto string, maxUTF16 int) []string {
	if maxUTF16 < 1 {
		return nil
	}
	runes := []rune(texto)
	if len(runes) == 0 {
		return []string{""}
	}
	var partes []string
	for inicio := 0; inicio < len(runes); {
		fim, unidades := inicio, 0
		for fim < len(runes) {
			n := 1
			if runes[fim] > 0xffff {
				n = 2
			}
			if unidades+n > maxUTF16 {
				break
			}
			unidades += n
			fim++
		}
		if fim == len(runes) {
			partes = append(partes, string(runes[inicio:fim]))
			break
		}
		if fim == inicio {
			fim++
		}
		corte := fim
		for _, separador := range []string{"\n\n", "\n", " "} {
			for i := fim - 1; i > inicio; i-- {
				tamanho := len([]rune(separador))
				if i+tamanho <= fim && string(runes[i:i+tamanho]) == separador {
					corte = i + tamanho
					break
				}
			}
			if corte != fim {
				break
			}
		}
		parte := strings.TrimSpace(string(runes[inicio:corte]))
		if parte == "" {
			corte = fim
			parte = string(runes[inicio:corte])
		}
		partes = append(partes, parte)
		inicio = corte
		for inicio < len(runes) && (runes[inicio] == ' ' || runes[inicio] == '\n') {
			inicio++
		}
	}
	return partes
}

// Servico liga o webhook ao concierge: identidade B2C, harness no agente de primeiro
// nível, resposta pelo Sender.
// extracoes acompanha as extrações de memória em andamento (Aguardar: testes e desligamento).
var extracoes sync.WaitGroup

type Servico struct {
	Cat         *okf.Catalogo
	Agente      string // caminho relativo do agente de primeiro nível (agentes/concierge)
	Config      harness.Config
	Account     string // account do Nexus para os recursos (DEFAULT_ACCOUNT_ID)
	Gruvi       *gruvi.Client
	TelefoneB2C string
	Sender      Sender
	Sessoes     *Sessoes
	Timeout     time.Duration
	Log         *log.Logger
	// Memorias é a memória relacional por contato (nil = desligada); Extrator captura
	// fatos novos a cada troca (nil = memoria.Regras).
	Memorias memoria.Store
	Extrator memoria.Extrator
	// Mapeamento é o Admin da administradora (nil = tudo habilitado, sem payloads):
	// resolve o condomínio da conversa, filtra os casos de uso e injeta os payloads.
	Mapeamento *admin.Store
	// Pedidos é o registro de pedidos por contato (nil = desligado: a conversa segue só
	// pelo orquestrador). ExtratorPedidos detecta os pedidos na mensagem (nil = Regras).
	Pedidos          pedidos.Store
	ExtratorPedidos  pedidos.Extrator
	pedidoMu         sync.Mutex
	pedidoLocks      map[string]*sync.Mutex
	pedidosPendentes map[string]pedidos.Carteira
	memoriaMu        sync.Mutex
	memoriaLocks     map[string]*sync.Mutex
	memoriaRevisoes  map[string]uint64
}

func (s *Servico) lockPorChave(raiz *sync.Mutex, locks *map[string]*sync.Mutex, chave string) func() {
	raiz.Lock()
	if *locks == nil {
		*locks = map[string]*sync.Mutex{}
	}
	lock := (*locks)[chave]
	if lock == nil {
		lock = &sync.Mutex{}
		(*locks)[chave] = lock
	}
	raiz.Unlock()
	lock.Lock()
	return lock.Unlock
}

func (s *Servico) bloquearPedidos(phone string) func() {
	return s.lockPorChave(&s.pedidoMu, &s.pedidoLocks, s.Account+":"+pedidos.NormalizarTelefone(phone))
}

func (s *Servico) carregarCarteira(ctx context.Context, phone string) (pedidos.Carteira, error) {
	chave := s.Account + ":" + pedidos.NormalizarTelefone(phone)
	s.pedidoMu.Lock()
	pendente, existe := s.pedidosPendentes[chave]
	s.pedidoMu.Unlock()
	if existe {
		if err := s.Pedidos.Gravar(ctx, pendente); err != nil {
			return pedidos.Carteira{}, fmt.Errorf("pedidos: estado pendente ainda não pôde ser salvo: %w", err)
		}
		s.pedidoMu.Lock()
		delete(s.pedidosPendentes, chave)
		s.pedidoMu.Unlock()
	}
	return s.Pedidos.Ler(ctx, s.Account, phone)
}

func (s *Servico) salvarCarteira(ctx context.Context, cart pedidos.Carteira) error {
	chave := s.Account + ":" + pedidos.NormalizarTelefone(cart.Telefone)
	if err := s.Pedidos.Gravar(ctx, cart); err != nil {
		s.pedidoMu.Lock()
		if s.pedidosPendentes == nil {
			s.pedidosPendentes = map[string]pedidos.Carteira{}
		}
		s.pedidosPendentes[chave] = cart
		s.pedidoMu.Unlock()
		return fmt.Errorf("pedidos: estado não foi salvo e permanece pendente de recuperação: %w", err)
	}
	s.pedidoMu.Lock()
	delete(s.pedidosPendentes, chave)
	s.pedidoMu.Unlock()
	return nil
}

func (s *Servico) bloquearMemoria(phone string) func() {
	return s.lockPorChave(&s.memoriaMu, &s.memoriaLocks, s.Account+":"+memoria.NormalizarTelefone(phone))
}

func (s *Servico) revisaoMemoria(phone string) uint64 {
	s.memoriaMu.Lock()
	defer s.memoriaMu.Unlock()
	return s.memoriaRevisoes[s.Account+":"+memoria.NormalizarTelefone(phone)]
}

var ErrFatoNaoEncontrado = errors.New("fato não encontrado")

func (s *Servico) EsquecerMemoria(ctx context.Context, phone, chave string) (memoria.Memoria, error) {
	desbloquear := s.bloquearMemoria(phone)
	defer desbloquear()
	m, err := s.Memorias.Ler(ctx, s.Account, phone)
	if err != nil {
		return m, err
	}
	if chave == "" {
		m.Fatos = []memoria.Fato{}
	} else if !m.Esquecer(chave) {
		return m, fmt.Errorf("%w: %s", ErrFatoNaoEncontrado, chave)
	}
	if m.Fatos == nil {
		m.Fatos = []memoria.Fato{}
	}
	if err := s.Memorias.Gravar(ctx, m); err != nil {
		return m, err
	}
	s.memoriaMu.Lock()
	if s.memoriaRevisoes == nil {
		s.memoriaRevisoes = map[string]uint64{}
	}
	s.memoriaRevisoes[s.Account+":"+memoria.NormalizarTelefone(phone)]++
	s.memoriaMu.Unlock()
	return m, nil
}

func (s *Servico) PrepararMensagem(msg entrada.Mensagem) (entrada.Mensagem, *Sessao) {
	sess := s.Sessoes.Abrir(msg)
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if !sess.Encerrada && sess.AtendimentoHumano {
		sess.AtendimentoHumano = false
		sess.revisaoEstado++
	}
	msg.Revisao = sess.revisaoEstado
	return msg, sess
}

// Processar trata um evento: abre a sessão, identifica o contato (uma vez por
// sessão), roda o concierge com o histórico e envia a resposta.
func (s *Servico) Processar(ctx context.Context, msg entrada.Mensagem) (*Sessao, error) {
	if !s.PermiteTelefone(msg.Contato.Phone) {
		return nil, fmt.Errorf("telefone diferente do morador configurado para o token B2C")
	}
	var sess *Sessao
	if msg.Revisao == 0 {
		msg, sess = s.PrepararMensagem(msg)
	} else {
		sess = s.Sessoes.Abrir(msg)
	}
	sess.workMu.Lock()
	defer sess.workMu.Unlock()
	sess.mu.Lock()
	if sess.Encerrada {
		sess.mu.Unlock()
		return sess, nil
	}
	if sess.AtendimentoHumano || sess.revisaoEstado != msg.Revisao {
		sess.mu.Unlock()
		texto := strings.TrimSpace(msg.Texto)
		if texto != "" {
			sess.registrar(Turno{Quem: "morador", Texto: texto, EventoID: msg.ID})
			sess.historicoAdicionar(schema.UserMessage(texto))
		}
		return sess, nil
	}
	ctxTurno, cancelarTurno := context.WithCancel(ctx)
	sess.cancelarTurno = cancelarTurno
	sess.Processando = true
	sess.mu.Unlock()
	defer func() {
		sess.mu.Lock()
		sess.cancelarTurno = nil
		sess.Processando = false
		sess.mu.Unlock()
		cancelarTurno()
	}()
	ctx = ctxTurno
	texto := strings.TrimSpace(msg.Texto)
	if texto == "" {
		return sess, fmt.Errorf("evento %s sem texto (tipo %s)", msg.ID, msg.Tipo)
	}
	sess.registrar(Turno{Quem: "morador", Texto: texto, EventoID: msg.ID})
	historico := sess.historicoSnapshot()
	baseHistorico := len(historico)
	historicoRegistrado := false
	interrompido := func() bool {
		sess.mu.Lock()
		defer sess.mu.Unlock()
		return sess.Encerrada || sess.AtendimentoHumano || sess.revisaoEstado != msg.Revisao
	}
	defer func() {
		if !historicoRegistrado && interrompido() {
			mensagens := append(append([]*schema.Message(nil), historico...), schema.UserMessage(texto))
			sess.historicoAtualizar(baseHistorico, mensagens)
		}
	}()
	// em transbordo o concierge não responde: a mensagem fica para o atendente humano
	sess.mu.Lock()
	transbordado := sess.Transbordo != nil && sess.Transbordo.Ativo
	identidadeVazia := sess.Identidade.Phone == ""
	plano := sess.Plano
	sess.mu.Unlock()
	if transbordado {
		sess.historicoAdicionar(schema.UserMessage(texto))
		return sess, nil
	}

	if identidadeVazia {
		id, resultadoVinculo, err := s.identidade(ctx, msg.Contato.Phone)
		if err != nil {
			if errors.Is(err, context.Canceled) && interrompido() {
				return sess, nil
			}
			sess.mu.Lock()
			sess.IdentErro = err.Error()
			sess.mu.Unlock()
			return sess, err
		}
		sess.mu.Lock()
		sess.Identidade = id
		sess.mu.Unlock()
		if id.Authenticated {
			sess.guardarMCPResultado("gruvi.bff.building-operation.get-company-by-document-controller-handle", resultadoVinculo)
		}
	}

	revisaoMemoria := s.revisaoMemoria(sess.Contato.Phone)
	cfg, mem, dir, err := s.montar(ctx, sess)
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
		if errors.Is(err, context.Canceled) && interrompido() {
			return sess, nil
		}
		s.responderFalha(ctx, sess, texto, historico, baseHistorico, err)
		historicoRegistrado = true
		return sess, err
	}
	var res *harness.Resultado
	var pedidosTurno []PedidoTurno
	if s.Pedidos != nil {
		res, pedidosTurno, err = s.turnoComPedidos(ctx, sess, ag, texto, historico)
	} else if plano != nil && plano.Pendente != "" {
		// um item do plano perguntou algo ao morador: a resposta vai direto a ele
		res, err = harness.Continuar(ctx, ag, plano, texto, historico)
	} else {
		res, err = harness.Run(ctx, ag, texto, historico)
	}
	if err != nil {
		if errors.Is(err, context.Canceled) && interrompido() {
			return sess, nil
		}
		s.responderFalha(ctx, sess, texto, historico, baseHistorico, err)
		historicoRegistrado = true
		return sess, err
	}
	if res.Transbordo != "" && sess.Destino.Conector != "atende-emulado" {
		res.Resposta = "Não consigo transferir esta conversa para um atendente por este canal. Posso continuar ajudando por aqui."
		mensagens := append([]*schema.Message(nil), res.Mensagens...)
		if len(mensagens) > 0 && mensagens[len(mensagens)-1].Role == schema.Assistant {
			mensagens[len(mensagens)-1] = schema.AssistantMessage(res.Resposta, nil)
		}
		res.Mensagens = mensagens
	}
	var erroEntrega error
	if s.Sender != nil && res.Resposta != "" {
		ctxCancelavel, cancelar := context.WithCancel(ctx)
		sess.mu.Lock()
		if sess.Encerrada || sess.AtendimentoHumano || sess.revisaoEstado != msg.Revisao {
			cancelar()
		} else {
			sess.cancelarEntrega = cancelar
		}
		sess.mu.Unlock()
		verificar := func() bool {
			sess.mu.Lock()
			defer sess.mu.Unlock()
			return !sess.Encerrada && !sess.AtendimentoHumano && sess.revisaoEstado == msg.Revisao
		}
		ctxEntrega := context.WithValue(ctxCancelavel, chaveControleEntrega{}, verificar)
		erroEntrega = s.Sender.Enviar(ctxEntrega, sess.Destino, res.Resposta)
		suprimida := errors.Is(erroEntrega, errEntregaSuprimida) || erroEntrega != nil && ctxCancelavel.Err() != nil && !verificar()
		var parcial *entregaParcial
		if errors.As(erroEntrega, &parcial) {
			res.Resposta = strings.Join(parcial.partes, "\n")
			mensagens := append([]*schema.Message(nil), res.Mensagens...)
			if len(mensagens) > 0 && mensagens[len(mensagens)-1].Role == schema.Assistant {
				mensagens[len(mensagens)-1] = schema.AssistantMessage(res.Resposta, nil)
			}
			res.Mensagens = mensagens
		}
		sess.mu.Lock()
		sess.cancelarEntrega = nil
		sess.mu.Unlock()
		cancelar()
		if suprimida {
			mensagens := res.Mensagens
			if parcial == nil && len(mensagens) > 0 && mensagens[len(mensagens)-1].Role == schema.Assistant {
				mensagens = mensagens[:len(mensagens)-1]
			}
			sess.historicoAtualizar(baseHistorico, mensagens)
			historicoRegistrado = true
			if parcial != nil {
				uso := res.Uso
				sess.registrar(Turno{Quem: "concierge", Texto: res.Resposta, Agente: res.Agente, Rota: res.Rota, Uso: &uso, Modelo: ag.NomeModelo, Plano: res.Plano, Pedidos: pedidosTurno})
			}
			sess.registrar(Turno{Quem: "sistema", Texto: "Resposta automática não entregue integralmente após mudança de atendimento."})
			return sess, nil
		}
	}
	if erroEntrega != nil {
		var parcial *entregaParcial
		if !errors.As(erroEntrega, &parcial) {
			mensagens := res.Mensagens
			if len(mensagens) > 0 && mensagens[len(mensagens)-1].Role == schema.Assistant {
				mensagens = mensagens[:len(mensagens)-1]
			}
			sess.historicoAtualizar(baseHistorico, mensagens)
			historicoRegistrado = true
			sess.registrar(Turno{Quem: "erro", Texto: erroEntrega.Error()})
			return sess, erroEntrega
		}
	}
	sess.historicoAtualizar(baseHistorico, res.Mensagens)
	historicoRegistrado = true
	sess.mu.Lock()
	sess.Plano = nil
	if res.Plano != nil && res.Plano.Pendente != "" {
		sess.Plano = res.Plano
	}
	sess.mu.Unlock()
	uso := res.Uso
	turno := Turno{Quem: "concierge", Texto: res.Resposta, Agente: res.Agente, Rota: res.Rota, Uso: &uso, Modelo: ag.NomeModelo, Plano: res.Plano, Pedidos: pedidosTurno}
	idx := sess.registrar(turno)
	if erroEntrega != nil {
		sess.registrar(Turno{Quem: "erro", Texto: erroEntrega.Error()})
		return sess, erroEntrega
	}
	if s.Pedidos != nil {
		ctxSalvar, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := s.marcarConfirmacoesEntregues(ctxSalvar, sess.Contato.Phone, pedidosTurno, res.Resposta)
		cancelar()
		if err != nil {
			sess.registrar(Turno{Quem: "erro", Texto: err.Error()})
			return sess, err
		}
	}
	// a memória relacional é extraída depois de responder (fora do caminho crítico): o
	// morador não espera a chamada extra ao modelo; o turno recebe os fatos quando saírem
	if s.Memorias != nil {
		extracoes.Add(1)
		memCopia := mem
		go func() {
			defer extracoes.Done()
			ctxMem, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			fatos := s.lembrar(ctxMem, &memCopia, sess, texto, res.Resposta, revisaoMemoria)
			sess.anotarMemorias(idx, fatos)
		}()
	}
	if res.Transbordo != "" {
		if sess.Destino.Conector == "atende-emulado" {
			s.transbordar(sess, res.Transbordo, "concierge", "")
		} else {
			sess.registrar(Turno{Quem: "sistema", Texto: "Solicitação de atendimento humano não encaminhada pelo canal ATENDE."})
		}
	}
	return sess, nil
}

func (s *Servico) RegistrarHumano(msg entrada.Mensagem) bool {
	sess, ok := s.Sessoes.Existente(msg)
	if !ok {
		return false
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.Encerrada {
		return false
	}
	sess.AtendimentoHumano = true
	sess.revisaoEstado++
	if sess.cancelarTurno != nil {
		sess.cancelarTurno()
	}
	if sess.cancelarEntrega != nil {
		sess.cancelarEntrega()
	}
	texto := strings.TrimSpace(msg.Texto)
	if texto != "" {
		quem := "morador"
		fala := schema.UserMessage(texto)
		if msg.Autor == "atendente" {
			quem = "atendente"
			fala = schema.AssistantMessage("[atendente humano] "+texto, nil)
		}
		turno := Turno{Quem: quem, Texto: texto, EventoID: msg.ID, Quando: time.Now()}
		sess.Turnos = append(sess.Turnos, turno)
		sess.AtualizadaEm = turno.Quando
		sess.historico = append(sess.historico, fala)
	}
	return true
}

func (s *Servico) Encerrar(msg entrada.Mensagem) bool {
	sess, ok := s.Sessoes.Existente(msg)
	if !ok {
		return false
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.Encerrada = true
	sess.revisaoEstado++
	if sess.cancelarTurno != nil {
		sess.cancelarTurno()
	}
	if sess.cancelarEntrega != nil {
		sess.cancelarEntrega()
	}
	turno := Turno{Quem: "sistema", Texto: "Protocolo encerrado pelo canal ATENDE.", EventoID: msg.ID, Quando: time.Now()}
	sess.Turnos = append(sess.Turnos, turno)
	sess.AtualizadaEm = turno.Quando
	return true
}

func (s *Servico) PermiteTelefone(phone string) bool {
	return s.TelefoneB2C == "" || strings.TrimSpace(phone) != "" && gruvi.NormalizarTelefone(phone) == gruvi.NormalizarTelefone(s.TelefoneB2C)
}

// Frases que o morador recebe quando o próprio concierge falha (modelo fora do ar, tempo
// esgotado, erro interno). O erro técnico nunca vai ao morador: fica em Turno.Erro,
// para o Console e o log.
var (
	FraseInstabilidade = "Estou com uma instabilidade momentânea por aqui. Pode repetir sua mensagem em instantes? Se preferir, me diga que quer falar com um atendente."
	FraseFalha         = "Não consegui processar sua mensagem agora. Pode tentar de novo em instantes? Se preferir, me diga que quer falar com um atendente."
)

var reFalhaTemporaria = regexp.MustCompile(`(?i)status code: (429|5\d\d)|\b(429|500|502|503|504)\b|high demand|overloaded|unavailable|resource.?exhausted|rate.?limit|deadline exceeded|timeout|timed out|connection (reset|refused)|unexpected EOF`)

// FalhaTemporaria: o erro é do tipo que passa sozinho (modelo sobrecarregado, limite de
// taxa, tempo esgotado, rede), e vale pedir ao morador que repita em instantes.
func FalhaTemporaria(err error) bool {
	return err != nil && (errors.Is(err, context.DeadlineExceeded) || reFalhaTemporaria.MatchString(err.Error()))
}

// responderFalha responde ao morador com uma frase amigável no lugar do erro: o turno
// guarda o erro técnico, a troca entra no histórico (o próximo turno sabe o que houve)
// e a frase sai pelo canal. O envio usa um contexto novo (o da requisição pode ter
// sido justamente o que estourou), mas respeita a sessão: encerrada, não entrega.
func (s *Servico) responderFalha(ctx context.Context, sess *Sessao, texto string, historico []*schema.Message, baseHistorico int, err error) {
	frase := FraseFalha
	if FalhaTemporaria(err) {
		frase = FraseInstabilidade
	}
	sess.registrar(Turno{Quem: "concierge", Texto: frase, Erro: err.Error()})
	mensagens := append(append([]*schema.Message(nil), historico...), schema.UserMessage(texto), schema.AssistantMessage(frase, nil))
	sess.historicoAtualizar(baseHistorico, mensagens)
	if s.Sender == nil {
		return
	}
	ctxEnvio, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancelar()
	ctxEnvio = context.WithValue(ctxEnvio, chaveControleEntrega{}, func() bool {
		sess.mu.Lock()
		defer sess.mu.Unlock()
		return !sess.Encerrada && !sess.AtendimentoHumano
	})
	if e := s.Sender.Enviar(ctxEnvio, sess.Destino, frase); e != nil && !errors.Is(e, errEntregaSuprimida) {
		sess.registrar(Turno{Quem: "erro", Texto: e.Error()})
	}
}

// montar prepara a Config do harness para a sessão (identidade, Admin do condomínio,
// data e hora, unidades, memória) e devolve a memória lida e a pasta do agente.
func (s *Servico) montar(ctx context.Context, sess *Sessao) (cfg harness.Config, mem memoria.Memoria, dir string, err error) {
	cfg = s.Config
	cfg.Sessao = sess.ID
	sess.mu.Lock()
	identidade := sess.Identidade
	sess.mu.Unlock()
	cfg.Identidade = harness.Identidade{Authenticated: identidade.Authenticated, AccountID: s.Account, Role: identidade.Role}
	cfg.MCPResultados = sess.resultadosMCP()

	// Admin da administradora: qual condomínio é este, quais casos ele pode usar e
	// quais definições (payloads) valem para os recursos
	var contexto []string
	if s.Mapeamento != nil {
		if m, err := s.Mapeamento.Ler(); err == nil {
			condo, ok := m.Resolver(identidade.CompanyID, sess.Tenant, s.Account)
			condoID := admin.Padrao
			if ok {
				condoID = condo.ID
				if condo.Account != "" {
					cfg.Identidade.AccountID = condo.Account
				}
			}
			sess.mu.Lock()
			sess.Condominio = condoID
			sess.mu.Unlock()
			papel := identidade.Role
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
	s.injetarParametrosMCP(&cfg, sess)

	// data e hora de agora: sem isso o modelo chuta o ano ao converter "sábado" em data
	contexto = append(contexto, contextoDeAgora(time.Now()))

	// identidade autenticada: entra no prompt em toda troca
	if c := contextoDeIdentidade(identidade); c != "" {
		contexto = append(contexto, c)
	}

	// memória relacional: o que já se sabe deste telefone entra no prompt da família
	if s.Memorias != nil {
		desbloquear := s.bloquearMemoria(sess.Contato.Phone)
		m, err := s.Memorias.Ler(ctx, s.Account, sess.Contato.Phone)
		desbloquear()
		if err != nil && s.Log != nil {
			s.Log.Printf("atende: memória de %s: %v", sess.Contato.Phone, err)
		}
		mem = m
		if c := harness.ContextoDeMemoria(mem); c != "" {
			contexto = append(contexto, c)
		}
	}
	cfg.Contexto = strings.Join(contexto, "\n\n")
	dir, err = s.Cat.ResolveAgente(s.Agente)
	if err != nil {
		return cfg, mem, "", err
	}
	if s.Pedidos != nil {
		// com o registro de pedidos ligado, o orquestrador só atende o que não é pedido:
		// sempre em modo router (planning/react viram o motor de pedidos)
		base := cfg.Env
		cfg.Env = func(k string) string {
			if k == "ORQUESTRACAO_MODO" {
				return harness.ModoRouter
			}
			if base != nil {
				return base(k)
			}
			return os.Getenv(k)
		}
	}
	return cfg, mem, dir, nil
}

// agenteDaSessao constrói a família de agentes para a sessão (ações do atendente).
func (s *Servico) agenteDaSessao(ctx context.Context, sess *Sessao) (*harness.Agente, error) {
	cfg, _, dir, err := s.montar(ctx, sess)
	if err != nil {
		return nil, err
	}
	def, err := s.Cat.Load(dir)
	if err != nil {
		return nil, err
	}
	return harness.Build(ctx, s.Cat, def, cfg)
}

// transbordar passa o protocolo ao atendente humano.
func (s *Servico) transbordar(sess *Sessao, motivo, pedidoPor, atendente string) {
	sess.mu.Lock()
	sess.Transbordo = &Transbordo{Ativo: true, Motivo: motivo, PedidoPor: pedidoPor, Quando: time.Now(), Atendente: atendente}
	sess.mu.Unlock()
	sess.registrar(Turno{Quem: "sistema", Texto: "Transferido ao atendente humano: " + motivo})
	if ob, ok := s.Sender.(*Outbox); ok && ob != nil {
		ob.Transbordar(sess.Destino, motivo)
	}
}

func (sess *Sessao) contextoEntregaManual(ctx context.Context) (context.Context, func()) {
	ctxEnvio, cancelar := context.WithCancel(ctx)
	sess.mu.Lock()
	if sess.Encerrada {
		cancelar()
	} else {
		sess.cancelarEntrega = cancelar
	}
	sess.mu.Unlock()
	ctxEnvio = context.WithValue(ctxEnvio, chaveControleEntrega{}, func() bool {
		sess.mu.Lock()
		defer sess.mu.Unlock()
		return !sess.Encerrada
	})
	return ctxEnvio, func() {
		sess.mu.Lock()
		sess.cancelarEntrega = nil
		sess.mu.Unlock()
		cancelar()
	}
}

// Assumir: o atendente toma o protocolo (mesmo sem o concierge ter pedido).
func (s *Servico) Assumir(protocolo, atendente, motivo string) (*Sessao, error) {
	sess, ok := s.Sessoes.Get(protocolo)
	if !ok {
		return nil, fmt.Errorf("protocolo desconhecido: %s", protocolo)
	}
	sess.workMu.Lock()
	defer sess.workMu.Unlock()
	if sess.Destino.Conector != "atende-emulado" {
		return sess, errors.New("transferência de atendimento indisponível neste canal")
	}
	if atendente == "" {
		atendente = "atendente"
	}
	sess.mu.Lock()
	if sess.Transbordo != nil && sess.Transbordo.Ativo {
		sess.Transbordo.Atendente = atendente
		sess.mu.Unlock()
		sess.registrar(Turno{Quem: "sistema", Texto: atendente + " assumiu o atendimento"})
		return sess, nil
	}
	sess.mu.Unlock()
	if motivo == "" {
		motivo = "o atendente assumiu a conversa"
	}
	s.transbordar(sess, motivo, "atendente", atendente)
	return sess, nil
}

// Responder: o atendente humano fala com o morador pelo mesmo canal do concierge.
func (s *Servico) Responder(ctx context.Context, protocolo, atendente, texto string) (*Sessao, error) {
	sess, ok := s.Sessoes.Get(protocolo)
	if !ok {
		return nil, fmt.Errorf("protocolo desconhecido: %s", protocolo)
	}
	sess.workMu.Lock()
	defer sess.workMu.Unlock()
	sess.mu.Lock()
	if sess.Encerrada {
		sess.mu.Unlock()
		return sess, errors.New("o protocolo está encerrado")
	}
	if sess.Transbordo == nil || !sess.Transbordo.Ativo {
		sess.mu.Unlock()
		return nil, fmt.Errorf("o protocolo não está em transbordo: assuma o atendimento antes")
	}
	texto = strings.TrimSpace(texto)
	if texto == "" {
		sess.mu.Unlock()
		return nil, fmt.Errorf("mensagem vazia")
	}
	if atendente != "" {
		sess.Transbordo.Atendente = atendente
	}
	quem := sess.Transbordo.Atendente
	sess.mu.Unlock()
	var errEnvio error
	if s.Sender != nil {
		ctxEnvio, finalizar := sess.contextoEntregaManual(ctx)
		errEnvio = s.Sender.Enviar(ctxEnvio, sess.Destino, texto)
		finalizar()
	}
	if entregue := textoEntregue(texto, errEnvio); entregue != "" {
		sess.registrar(Turno{Quem: "atendente", Texto: entregue, Agente: quem})
		sess.historicoAdicionar(schema.AssistantMessage("[atendente humano] "+entregue, nil))
	}
	if errEnvio != nil {
		sess.registrar(Turno{Quem: "erro", Texto: errEnvio.Error()})
		return sess, errEnvio
	}
	return sess, nil
}

// Devolver: o atendente devolve o protocolo ao concierge, que volta a responder.
func (s *Servico) Devolver(protocolo string) (*Sessao, error) {
	sess, ok := s.Sessoes.Get(protocolo)
	if !ok {
		return nil, fmt.Errorf("protocolo desconhecido: %s", protocolo)
	}
	sess.workMu.Lock()
	defer sess.workMu.Unlock()
	sess.mu.Lock()
	if sess.Transbordo == nil || !sess.Transbordo.Ativo {
		sess.mu.Unlock()
		return sess, nil
	}
	agora := time.Now()
	sess.Transbordo.Ativo = false
	sess.Transbordo.Devolvido = &agora
	sess.mu.Unlock()
	sess.registrar(Turno{Quem: "sistema", Texto: "Devolvido ao concierge"})
	sess.historicoAdicionar(schema.AssistantMessage("[sistema] o atendente humano devolveu a conversa ao concierge; continue de onde parou.", nil))
	return sess, nil
}

// Aguardar espera as extrações de memória em andamento (testes e desligamento).
func (s *Servico) Aguardar() { extracoes.Wait() }

// lembrar extrai os fatos duráveis da troca e grava na memória do contato. Devolve
// os fatos novos ou alterados (para a bancada do Console).
func (s *Servico) lembrar(ctx context.Context, mem *memoria.Memoria, sess *Sessao, mensagem, resposta string, revisao uint64) []memoria.Fato {
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
	desbloquear := s.bloquearMemoria(sess.Contato.Phone)
	defer desbloquear()
	if s.revisaoMemoria(sess.Contato.Phone) != revisao {
		return nil
	}
	memAtual, err := s.Memorias.Ler(ctx, s.Account, sess.Contato.Phone)
	if err != nil {
		if s.Log != nil {
			s.Log.Printf("atende: leitura da memória para mesclagem: %v", err)
		}
		return nil
	}
	*mem = memAtual
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

func (s *Servico) identidade(ctx context.Context, phone string) (gruvi.Identidade, any, error) {
	id := gruvi.Identidade{Phone: phone, Units: []gruvi.Unidade{}}
	if s.TelefoneB2C == "" {
		return id, nil, nil
	}
	if !s.PermiteTelefone(phone) {
		return id, nil, errors.New("telefone diferente do morador configurado para o token B2C")
	}
	validar, ok := s.Config.MCP.(interface{ ValidarToken(context.Context) error })
	if !ok {
		return id, nil, errors.New("atende: cliente MCP não valida o token B2C")
	}
	if err := validar.ValidarToken(ctx); err != nil {
		return id, nil, fmt.Errorf("atende: autenticação B2C: %w", err)
	}
	if s.Gruvi != nil {
		id.CompanyID = s.Gruvi.CompanyID
	}
	if id.CompanyID == "" {
		return id, nil, errors.New("atende: GRUVI_COMPANY_ID não configurado")
	}
	const ferramenta = "gruvi.bff.building-operation.get-company-by-document-controller-handle"
	resultado, err := s.Config.MCP.CallTool(ctx, ferramenta, map[string]any{
		"companyId":            id.CompanyID,
		"filter_mobile_number": gruvi.NormalizarTelefone(phone),
	})
	if err != nil {
		return id, nil, fmt.Errorf("atende: consulta de vínculo B2C: %w", err)
	}
	dados, err := harness.NormalizarResultadoMCP(ferramenta, resultado)
	if err != nil {
		return id, nil, fmt.Errorf("atende: consulta de vínculo B2C: %w", err)
	}
	id, dadosVinculados := gruvi.IdentidadeComDadosVinculadosMCP(phone, id.CompanyID, dados)
	return id, dadosVinculados, nil
}

func (s *Servico) injetarParametrosMCP(cfg *harness.Config, sess *Sessao) {
	if cfg.Payloads == nil {
		cfg.Payloads = map[string]map[string]any{}
	}
	companyID := strings.TrimSpace(sess.Identidade.CompanyID)
	if companyID == "" && s.Gruvi != nil {
		companyID = strings.TrimSpace(s.Gruvi.CompanyID)
	}
	telefone := gruvi.NormalizarTelefone(sess.Contato.Phone)
	company := "gruvi.bff.building-operation.get-company-by-document-controller-handle"
	packages := "gruvi.bff.building-operation.fetch-unit-packages-controller-handle"
	code := "gruvi.bff.building-operation.get-package-delivery-code-controller-handle"
	visit := "gruvi.bff.building-operation.create-visit-controller-handle"
	nomes := []string{company, packages, code, "gruvi.bff.building-operation.fetch-visit-reasons-controller-handle", visit}
	for _, nome := range nomes {
		if cfg.Payloads[nome] == nil {
			cfg.Payloads[nome] = map[string]any{}
		}
		if companyID != "" {
			cfg.Payloads[nome]["companyId"] = companyID
		}
	}
	for _, nome := range []string{company, packages, code} {
		cfg.Payloads[nome]["filter_mobile_number"] = telefone
	}
	cfg.Payloads[visit]["resident"] = map[string]any{"mobile_number": telefone}
}

var diasSemana = []string{"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"}
var meses = []string{"", "janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

// contextoDeAgora: a data e a hora atuais, no fuso de São Paulo, para datas relativas
// ("sábado", "amanhã", "dia 12") virarem AAAA-MM-DD certos.
func contextoDeAgora(t time.Time) string {
	t = pedidos.NoFusoSaoPaulo(t)
	return fmt.Sprintf("# Agora\n\nHoje é %s, %d de %s de %d, %02d:%02d (%s). Datas relativas do morador (amanhã, sábado, dia 12) contam a partir de hoje e vão para os recursos como AAAA-MM-DD; confirme a data por extenso com ele.",
		diasSemana[t.Weekday()], t.Day(), meses[t.Month()], t.Year(), t.Hour(), t.Minute(), t.Format("-07:00"))
}

// contextoDeIdentidade: quem é o morador, para toda a família (o canal já sabe; o
// agente não deve perguntar a unidade quando só há uma).
func contextoDeIdentidade(id gruvi.Identidade) string {
	if !id.Authenticated {
		return "# Contato\n\nContato não identificado pelo telefone: trate como público; não acesse dados de unidade."
	}
	nome := id.Name
	if nome == "" {
		nome = "morador"
	}
	papel := id.Role
	if papel == "" {
		papel = "morador"
	}
	return fmt.Sprintf("# Morador identificado\n\n- Nome: %s\n- Papel: %s\n\nConsulte as unidades pela tool MCP antes de qualquer operação dependente delas.", nome, papel)
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
