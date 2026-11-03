package atende

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/admin"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
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
	Cat    *okf.Catalogo
	Agente string // caminho relativo do agente de primeiro nível (agentes/concierge)
	Config harness.Config
	// Implantacao restringe o processo a uma administradora (vazia = todas).
	Implantacao escopo.Implantacao
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
	// Mapeamento é o Admin das administradoras: resolve o condomínio de cada mensagem
	// (sem ele nenhuma é atendida), filtra os casos de uso e injeta os payloads.
	Mapeamento admin.Repositorio
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

func (s *Servico) bloquearPedidos(c escopo.Contato) func() {
	return s.lockPorChave(&s.pedidoMu, &s.pedidoLocks, c.Chave())
}

func (s *Servico) carregarCarteira(ctx context.Context, c escopo.Contato) (pedidos.Carteira, error) {
	chave := c.Chave()
	s.pedidoMu.Lock()
	pendente, existe := s.pedidosPendentes[chave]
	s.pedidoMu.Unlock()
	if existe {
		if err := s.Pedidos.Gravar(ctx, c, pendente); err != nil {
			return pedidos.Carteira{}, fmt.Errorf("pedidos: estado pendente ainda não pôde ser salvo: %w", err)
		}
		s.pedidoMu.Lock()
		delete(s.pedidosPendentes, chave)
		s.pedidoMu.Unlock()
	}
	return s.Pedidos.Ler(ctx, c)
}

func (s *Servico) salvarCarteira(ctx context.Context, c escopo.Contato, cart pedidos.Carteira) error {
	chave := c.Chave()
	if err := s.Pedidos.Gravar(ctx, c, cart); err != nil {
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

func (s *Servico) bloquearMemoria(c escopo.Contato) func() {
	return s.lockPorChave(&s.memoriaMu, &s.memoriaLocks, c.Chave())
}

func (s *Servico) revisaoMemoria(c escopo.Contato) uint64 {
	s.memoriaMu.Lock()
	defer s.memoriaMu.Unlock()
	return s.memoriaRevisoes[c.Chave()]
}

var ErrFatoNaoEncontrado = errors.New("fato não encontrado")

func (s *Servico) EsquecerMemoria(ctx context.Context, c escopo.Contato, chave string) (memoria.Memoria, error) {
	desbloquear := s.bloquearMemoria(c)
	defer desbloquear()
	m, err := s.Memorias.Ler(ctx, c)
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
	if err := s.Memorias.Gravar(ctx, c, m); err != nil {
		return m, err
	}
	s.memoriaMu.Lock()
	if s.memoriaRevisoes == nil {
		s.memoriaRevisoes = map[string]uint64{}
	}
	s.memoriaRevisoes[c.Chave()]++
	s.memoriaMu.Unlock()
	return m, nil
}

func (s *Servico) PrepararMensagem(ctx context.Context, msg entrada.Mensagem) (entrada.Mensagem, *Sessao, error) {
	sess, err := s.Sessoes.Abrir(ctx, msg)
	if err != nil {
		return msg, nil, fmt.Errorf("abrir sessão do protocolo %s: %w", msg.ConversaID, err)
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if !sess.Encerrada && sess.AtendimentoHumano {
		sess.AtendimentoHumano = false
		sess.revisaoEstado++
	}
	msg.Revisao = sess.revisaoEstado
	return msg, sess, nil
}

// Processar trata um evento: abre a sessão, identifica o contato (uma vez por
// sessão), roda o concierge com o histórico, envia a resposta e persiste a sessão.
func (s *Servico) Processar(ctx context.Context, msg entrada.Mensagem) (*Sessao, error) {
	sess, err := s.processar(ctx, msg)
	if sess == nil {
		return nil, err
	}
	// a sessão é gravada mesmo com o turno cancelado: o que já aconteceu nele fica
	return sess, errors.Join(err, s.Sessoes.Persistir(context.WithoutCancel(ctx), sess))
}

func (s *Servico) processar(ctx context.Context, msg entrada.Mensagem) (*Sessao, error) {
	if !s.PermiteTelefone(msg.Contato.Phone) {
		return nil, fmt.Errorf("telefone diferente do morador configurado para o token B2C")
	}
	var sess *Sessao
	var err error
	if msg.Revisao == 0 {
		msg, sess, err = s.PrepararMensagem(ctx, msg)
	} else {
		sess, err = s.Sessoes.Abrir(ctx, msg)
	}
	if err != nil {
		return nil, err
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
	m, condo, err := s.resolverCondominio(ctx, sess, msg)
	if err != nil {
		if !condominioNaoAtendido(err) {
			s.responderFalha(ctx, sess, texto, historico, baseHistorico, err)
			historicoRegistrado = true
			return sess, err
		}
		s.avisar(ctx, "resolver_condominio", sess, err)
		sess.historicoAdicionar(schema.UserMessage(texto))
		s.transbordar(sess, "condomínio não identificado", "concierge", "")
		return sess, nil
	}
	contato := sess.contato()

	if identidadeVazia {
		id, resultadoVinculo, err := s.identidade(ctx, msg.Contato.Phone, sess.companyDoCondominio())
		if err != nil {
			if errors.Is(err, context.Canceled) && interrompido() {
				return sess, nil
			}
			sess.mu.Lock()
			sess.IdentErro = err.Error()
			sess.mu.Unlock()
			// o morador não fica sem resposta: a falha de vínculo (MCP, B2C) é dita
			// de forma amigável, e o erro técnico fica no turno
			s.responderFalha(ctx, sess, texto, historico, baseHistorico, err)
			historicoRegistrado = true
			return sess, err
		}
		sess.mu.Lock()
		sess.Identidade = id
		sess.mu.Unlock()
		if id.Authenticated {
			sess.guardarMCPResultado("gruvi.bff.building-operation.get-company-by-document-controller-handle", resultadoVinculo)
		}
	}

	revisaoMemoria := s.revisaoMemoria(contato)
	cfg, mem, def, err := s.prepararTurno(ctx, sess, m, condo)
	if err != nil {
		slog.ErrorContext(ctx, "atende: turno não montado", "operacao", "montar_turno", "sessao", sess.ID, "erro", err)
		s.responderFalha(ctx, sess, texto, historico, baseHistorico, err)
		historicoRegistrado = true
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
		err := s.marcarConfirmacoesEntregues(ctxSalvar, contato, pedidosTurno, res.Resposta)
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
			if len(fatos) > 0 {
				// a falha já sai logada como ERROR por Persistir; não há a quem devolver
				_ = s.Sessoes.Persistir(ctxMem, sess)
			}
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

// RegistrarHumano: um humano falou no protocolo; o concierge para de responder.
// Devolve false quando a sessão não existe ou já foi encerrada.
func (s *Servico) RegistrarHumano(ctx context.Context, msg entrada.Mensagem) (bool, error) {
	return s.mudarExistente(ctx, msg, func(sess *Sessao) bool { return sess.registrarHumano(msg) })
}

// Encerrar: o canal fechou o protocolo. Devolve false quando a sessão não existe.
func (s *Servico) Encerrar(ctx context.Context, msg entrada.Mensagem) (bool, error) {
	return s.mudarExistente(ctx, msg, func(sess *Sessao) bool { sess.encerrar(msg); return true })
}

// Sessao devolve a sessão pelo ID interno ou protocolo, só se ela for de uma
// administradora desta implantação: o store é compartilhado, e uma sessão alheia é
// tratada como inexistente (não se revela que existe).
func (s *Servico) Sessao(ctx context.Context, id string) (*Sessao, bool, error) {
	sess, ok, err := s.Sessoes.Get(ctx, id)
	if err != nil || !ok {
		return nil, false, err
	}
	if !s.Implantacao.Permite(sess.administradora()) {
		return nil, false, nil
	}
	return sess, true, nil
}

// mudarExistente aplica mudar à sessão do evento, se existir, e persiste o que mudou.
func (s *Servico) mudarExistente(ctx context.Context, msg entrada.Mensagem, mudar func(*Sessao) bool) (bool, error) {
	sess, ok, err := s.Sessoes.Existente(ctx, msg)
	if err != nil {
		return false, fmt.Errorf("sessão do protocolo %s: %w", msg.ConversaID, err)
	}
	if !ok || !mudar(sess) {
		return false, nil
	}
	return true, s.Sessoes.Persistir(ctx, sess)
}

// jaRegistrado: o evento já virou um turno de quem (o canal reenviou o webhook). Chame
// com sess.mu.
func (sess *Sessao) jaRegistrado(eventoID, quem string) bool {
	return eventoID != "" && slices.ContainsFunc(sess.Turnos, func(t Turno) bool { return t.EventoID == eventoID && t.Quem == quem })
}

// autorHumano: quem fala no turno de um evento humano.
func autorHumano(msg entrada.Mensagem) string {
	if msg.Autor == "atendente" {
		return "atendente"
	}
	return "morador"
}

// registrarHumano devolve true quando há o que persistir: o evento novo aplicado, ou um
// reenvio (nada muda, só a gravação é refeita).
func (sess *Sessao) registrarHumano(msg entrada.Mensagem) bool {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.jaRegistrado(msg.ID, autorHumano(msg)) {
		return true
	}
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
		quem := autorHumano(msg)
		fala := schema.UserMessage(texto)
		if quem == "atendente" {
			fala = schema.AssistantMessage("[atendente humano] "+texto, nil)
		}
		turno := Turno{Quem: quem, Texto: texto, EventoID: msg.ID, Quando: time.Now()}
		sess.Turnos = append(sess.Turnos, turno)
		sess.AtualizadaEm = turno.Quando
		sess.historico = append(sess.historico, fala)
	}
	return true
}

func (sess *Sessao) encerrar(msg entrada.Mensagem) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.jaRegistrado(msg.ID, "sistema") {
		return
	}
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

// prepararTurno monta a Config e carrega o agente de primeiro nível.
func (s *Servico) prepararTurno(ctx context.Context, sess *Sessao, m admin.Mapeamento, condo admin.Condominio) (harness.Config, memoria.Memoria, *okf.Agent, error) {
	cfg, mem, dir, err := s.montar(ctx, sess, m, condo)
	if err != nil {
		return cfg, mem, nil, err
	}
	def, err := s.Cat.Load(dir)
	if err != nil {
		return cfg, mem, nil, fmt.Errorf("carregar agente %s: %w", dir, err)
	}
	return cfg, mem, def, nil
}

// montar prepara a Config do harness para a sessão (identidade, Admin do condomínio,
// data e hora, unidades, memória) e devolve a memória lida e a pasta do agente. m e
// condo vêm de quem resolveu o condomínio: o turno não relê o mapeamento.
func (s *Servico) montar(ctx context.Context, sess *Sessao, m admin.Mapeamento, condo admin.Condominio) (cfg harness.Config, mem memoria.Memoria, dir string, err error) {
	cfg = s.Config
	cfg.Sessao = sess.ID
	sess.mu.Lock()
	identidade := sess.Identidade
	sess.mu.Unlock()
	cfg.Identidade = harness.Identidade{Authenticated: identidade.Authenticated, Role: identidade.Role}
	contato := sess.contato()
	cfg.MCPResultados = sess.resultadosMCP()

	// Admin da administradora: quais casos o condomínio pode usar e quais definições
	// (payloads) valem para os recursos
	cfg.Identidade.AccountID = condo.Account
	papel := identidade.Role
	cfg.Casos = func(nome, casoID string) bool {
		return m.Habilitado(condo.ID, nome, papel) && (casoID == "" || m.Habilitado(condo.ID, casoID, papel))
	}
	cfg.Payloads = m.PayloadsDe(condo.ID)
	orient := m.OrientacaoDe(condo.ID)
	cfg.Orientacoes = orient.PorCaso
	var contexto []string
	if c := admin.ContextoDoCondominio(condo, cfg.Payloads, orient.Geral); c != "" {
		contexto = append(contexto, c)
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
		desbloquear := s.bloquearMemoria(contato)
		m, err := s.Memorias.Ler(ctx, contato)
		desbloquear()
		if err != nil && s.Log != nil {
			s.Log.Printf("atende: memória de %s: %v", contato.Mascarado(), err)
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

// condominioDaSessao lê o mapeamento e o condomínio resolvido para a sessão. Falha
// fechada: sem mapeamento ou sem o condomínio, nada é montado.
func (s *Servico) condominioDaSessao(ctx context.Context, sess *Sessao, contato escopo.Contato) (admin.Mapeamento, admin.Condominio, error) {
	if s.Mapeamento == nil {
		return admin.Mapeamento{}, admin.Condominio{}, fmt.Errorf("montar sessão %s sem mapeamento: %w", sess.ID, admin.ErrCondominioDesconhecido)
	}
	m, err := s.Mapeamento.Ler(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "atende: mapeamento indisponível; sessão não montada",
			"operacao", "ler_mapeamento", "sessao", sess.ID, "erro", err)
		return admin.Mapeamento{}, admin.Condominio{}, fmt.Errorf("ler mapeamento: %w", err)
	}
	condo, ok := m.Condominio(contato.CondominioID)
	if !ok {
		return admin.Mapeamento{}, admin.Condominio{}, fmt.Errorf("condomínio %q da sessão %s: %w", contato.CondominioID, sess.ID, admin.ErrCondominioDesconhecido)
	}
	return m, condo, nil
}

// agenteDaSessao constrói a família de agentes para a sessão (ações do atendente).
func (s *Servico) agenteDaSessao(ctx context.Context, sess *Sessao) (*harness.Agente, error) {
	m, condo, err := s.condominioDaSessao(ctx, sess, sess.contato())
	if err != nil {
		return nil, err
	}
	cfg, _, dir, err := s.montar(ctx, sess, m, condo)
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
// resolverCondominio fixa na sessão o condomínio da mensagem e o contato na
// administradora dele. Sem mapeamento, nenhum condomínio é conhecido.
// Devolve o mapeamento lido e o condomínio, que o turno usa sem reler.
func (s *Servico) resolverCondominio(ctx context.Context, sess *Sessao, msg entrada.Mensagem) (admin.Mapeamento, admin.Condominio, error) {
	if s.Mapeamento == nil {
		return admin.Mapeamento{}, admin.Condominio{}, fmt.Errorf("resolver condomínio sem mapeamento: %w", admin.ErrCondominioDesconhecido)
	}
	m, err := s.Mapeamento.Ler(ctx)
	if err != nil {
		return admin.Mapeamento{}, admin.Condominio{}, fmt.Errorf("resolver condomínio: %w", err)
	}
	condo, err := admin.ResolverCondominio(m, s.Implantacao, msg.CompanyID, msg.Tenant)
	if err != nil {
		return admin.Mapeamento{}, admin.Condominio{}, err
	}
	if atual := sess.contato(); atual.CondominioID != "" && atual.CondominioID != condo.ID {
		return admin.Mapeamento{}, admin.Condominio{}, fmt.Errorf("conversa de %s recebeu mensagem de %s: %w", atual.CondominioID, condo.ID, errCondominioMudou)
	}
	contato, err := escopo.NovoContato(condo.Administradora, condo.ID, msg.Contato.Phone)
	if err != nil {
		return admin.Mapeamento{}, admin.Condominio{}, fmt.Errorf("contato no condomínio %s: %w", condo.ID, err)
	}
	sess.fixarContato(contato, condo.CompanyID)
	return m, condo, nil
}

var errCondominioMudou = errors.New("condomínio da conversa mudou")

// naoAtendidos: a mensagem é de um condomínio que este processo não atende; qualquer
// outro erro da resolução é falha de infraestrutura.
var naoAtendidos = []error{admin.ErrCondominioDesconhecido, admin.ErrForaDaImplantacao, admin.ErrCondominioAmbiguo, admin.ErrCondominioIncompleto, errCondominioMudou}

func condominioNaoAtendido(err error) bool {
	return slices.ContainsFunc(naoAtendidos, func(alvo error) bool { return errors.Is(err, alvo) })
}

func (s *Servico) avisar(ctx context.Context, operacao string, sess *Sessao, err error) {
	slog.WarnContext(ctx, "atende: condomínio não atendido; conversa transbordada",
		"operacao", operacao, "sessao", sess.ID, "erro", err)
}

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
func (s *Servico) Assumir(ctx context.Context, protocolo, atendente, motivo string) (*Sessao, error) {
	return s.noProtocolo(ctx, protocolo, func(sess *Sessao) (*Sessao, error) { return s.assumir(sess, atendente, motivo) })
}

// ErrProtocoloDesconhecido: não há sessão do protocolo visível nesta implantação.
var ErrProtocoloDesconhecido = errors.New("protocolo desconhecido")

// noProtocolo roda op na sessão do protocolo, com o trabalho dela travado, e persiste
// a sessão em seguida (mesmo que op falhe: o que já mudou fica gravado).
func (s *Servico) noProtocolo(ctx context.Context, protocolo string, op func(*Sessao) (*Sessao, error)) (*Sessao, error) {
	sess, ok, err := s.Sessao(ctx, protocolo)
	if err != nil {
		return nil, fmt.Errorf("sessão do protocolo %s: %w", protocolo, err)
	}
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProtocoloDesconhecido, protocolo)
	}
	sess.workMu.Lock()
	defer sess.workMu.Unlock()
	res, err := op(sess)
	return res, errors.Join(err, s.Sessoes.Persistir(context.WithoutCancel(ctx), sess))
}

func (s *Servico) assumir(sess *Sessao, atendente, motivo string) (*Sessao, error) {
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
	return s.noProtocolo(ctx, protocolo, func(sess *Sessao) (*Sessao, error) { return s.responder(ctx, sess, atendente, texto) })
}

func (s *Servico) responder(ctx context.Context, sess *Sessao, atendente, texto string) (*Sessao, error) {
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
func (s *Servico) Devolver(ctx context.Context, protocolo string) (*Sessao, error) {
	return s.noProtocolo(ctx, protocolo, func(sess *Sessao) (*Sessao, error) { return devolver(sess) })
}

func devolver(sess *Sessao) (*Sessao, error) {
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
	contato := sess.contato()
	desbloquear := s.bloquearMemoria(contato)
	defer desbloquear()
	if s.revisaoMemoria(contato) != revisao {
		return nil
	}
	memAtual, err := s.Memorias.Ler(ctx, contato)
	if err != nil {
		if s.Log != nil {
			s.Log.Printf("atende: leitura da memória para mesclagem: %v", err)
		}
		return nil
	}
	*mem = memAtual
	if mem.Nome == "" {
		mem.Nome = sess.Identidade.Name
	}
	var novos []memoria.Fato
	for _, f := range fatos {
		f.Fonte = sess.Protocolo
		f.CondominioID = contato.CondominioID
		if mem.Upsert(f) {
			novos = append(novos, f)
		}
	}
	if len(novos) > 0 {
		if err := s.Memorias.Gravar(ctx, contato, *mem); err != nil && s.Log != nil {
			s.Log.Printf("atende: gravar memória: %v", err)
		}
	}
	return novos
}

func (s *Servico) identidade(ctx context.Context, phone, companyID string) (gruvi.Identidade, any, error) {
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
	id.CompanyID = companyID
	if id.CompanyID == "" {
		return id, nil, errors.New("atende: condomínio sem company do Gruvi")
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
	companyID := sess.companyDoCondominio()
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
