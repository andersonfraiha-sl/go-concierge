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
	"github.com/jackc/pgx/v5/pgxpool"

	"go-concierge/internal/admin"
	"go-concierge/internal/banco"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	comfalha "go-concierge/internal/falha" // falha é o nome local do erro de entrega
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/auditoria"
	"go-concierge/internal/ganchos/fixos"
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

func (o *Outbox) PodeEntregarTransbordo() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.ATENDE != nil
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
// Transbordar registra o pedido de atendente humano no emulador.
func (o *Outbox) Transbordar(destino entrada.Destino, motivo string) {
	o.RegistrarTransbordo(destino, motivo, destino.Conector == "atende-emulado", "")
}

func (o *Outbox) RegistrarTransbordo(destino entrada.Destino, motivo string, enviada bool, erro string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.itens = append(o.itens, Saida{SessaoID: destino.SessaoID, Protocolo: destino.Protocolo, Tipo: "transbordo", Texto: motivo, Quando: time.Now(), Enviada: enviada, Erro: erro})
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
		return comfalha.ComPilha(err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return comfalha.ComPilha(fmt.Errorf("HTTP %d", res.StatusCode))
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
			"contact": map[string]string{"phone": destino.Telefone},
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
			return falha(comfalha.ComPilha(fmt.Errorf("ATENDE: parte %d de %d falhou: %w", i+1, len(partes), err)))
		}
		_ = res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return falha(comfalha.ComPilha(fmt.Errorf("ATENDE: parte %d de %d respondeu HTTP %d", i+1, len(partes), res.StatusCode)))
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
	// Agora é o relógio dos eventos dos ganchos; nil = Config.Agora, senão time.Now.
	Agora func() time.Time
	// Implantacao restringe o processo a uma administradora (vazia = todas).
	Implantacao  escopo.Implantacao
	Gruvi        *gruvi.Client
	TelefoneB2C  string
	Sender       Sender
	Transferidor Transferidor
	Sessoes      *Sessoes
	Timeout      time.Duration
	Log          *log.Logger
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
	// Pool envia os lotes do turno: a leitura a frio antes da resposta e as escritas depois
	// dela, uma ida ao banco cada. nil (stores sem Postgres) = cada store lê e grava sozinho.
	Pool *pgxpool.Pool
	// Ganchos avaliam o turno inteiro (entrada, ferramentas, resposta, transbordo); nil =
	// Config.Ganchos, senão só os fixos. Validado no primeiro uso: sem algum fixo, recusado.
	Ganchos *ganchos.Conjunto
	// Auditoria liga a auditoria dos ganchos no lote pós resposta: o mesmo auditor que
	// observa o Conjunto. nil = o turno não coleta; o que o observador vir grava na hora.
	Auditoria     *auditoria.Auditor
	ganchosUmaVez sync.Once
	conjunto      *ganchos.Conjunto
	errGanchos    error
}

// aquecivelNoLote é o store com cache por contato que lê o contato frio dentro de um lote.
type aquecivelNoLote interface {
	AquecerNoLote(ctx context.Context, lote *banco.Lote, c escopo.Contato)
}

// pedidosEmLote é o store de pedidos que grava dentro de um lote.
type pedidosEmLote interface {
	GravarNoLote(ctx context.Context, lote *banco.Lote, c escopo.Contato, cart pedidos.Carteira) (confirmar func(), err error)
}

// aquecer carrega numa ida ao banco o que o turno vai ler e este processo ainda não tem: a
// sessão do protocolo e a memória e os pedidos do contato. A falha aqui não para o turno:
// cada store lê sozinho depois, e o erro aparece lá.
func (s *Servico) aquecer(ctx context.Context, msg entrada.Mensagem) {
	if s.Pool == nil {
		return
	}
	lote := banco.NovoLote()
	roteada := normalizarRoteamento(msg)
	s.Sessoes.aquecerNoLote(ctx, lote, idSessao(roteada.Origem, roteada.ConversaID))
	if contato, ok := s.contatoDaMensagem(ctx, msg); ok {
		for _, st := range []any{s.Memorias, s.Pedidos} {
			if a, ok := st.(aquecivelNoLote); ok {
				a.AquecerNoLote(ctx, lote, contato)
			}
		}
	}
	if lote.Vazio() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	if err := lote.Enviar(ctx, s.Pool); err != nil {
		slog.WarnContext(ctx, "atende: leitura em lote falhou; cada store lê sozinho", "operacao", "aquecer_turno", "erro", err)
	}
}

// contatoDaMensagem resolve o contato pelo mapeamento, sem mexer na sessão; o turno resolve
// de novo (resolverCondominio) e é lá que um erro vira resposta.
func (s *Servico) contatoDaMensagem(ctx context.Context, msg entrada.Mensagem) (escopo.Contato, bool) {
	if s.Mapeamento == nil {
		return escopo.Contato{}, false
	}
	m, err := s.Mapeamento.Ler(ctx)
	if err != nil {
		return escopo.Contato{}, false
	}
	condo, err := admin.ResolverCondominio(m, s.Implantacao, msg.CompanyID, msg.Tenant)
	if err != nil {
		return escopo.Contato{}, false
	}
	contato, err := escopo.NovoContato(condo.Administradora, condo.ID, msg.Contato.Phone)
	return contato, err == nil
}

// adiarCarteira deixa a carteira do turno para gravar depois da resposta
// (gravarDepoisDaResposta). Até lá ela é o estado pendente do contato: quem carregar a
// carteira antes grava a pendente primeiro. Quem chama segura o lock de pedidos do contato.
func (s *Servico) adiarCarteira(c escopo.Contato, cart pedidos.Carteira) {
	s.pedidoMu.Lock()
	defer s.pedidoMu.Unlock()
	if s.pedidosPendentes == nil {
		s.pedidosPendentes = map[string]pedidos.Carteira{}
	}
	s.pedidosPendentes[c.Chave()] = cart
}

// carteiraDoTurno é a carteira adiada pelo turno, se houver, ou a carregada. Quem chama
// segura o lock de pedidos do contato.
func (s *Servico) carteiraDoTurno(ctx context.Context, c escopo.Contato) (pedidos.Carteira, error) {
	if cart, adiada := s.carteiraAdiada(c); adiada {
		return cart, nil
	}
	return s.carregarCarteira(ctx, c)
}

func (s *Servico) carteiraAdiada(c escopo.Contato) (pedidos.Carteira, bool) {
	s.pedidoMu.Lock()
	defer s.pedidoMu.Unlock()
	cart, adiada := s.pedidosPendentes[c.Chave()]
	return cart, adiada
}

// gravarDepoisDaResposta grava a sessão, a carteira que o turno adiou e a auditoria dos
// ganchos numa ida ao banco. Sem lote, ou com o lote recusado (protocolo de outro contato,
// banco fora), grava uma por uma pelo caminho de sempre, que redesenha o protocolo e guarda
// a carteira como pendente se falhar; a auditoria vai depois da sessão, linha a linha se o
// lote dela falhar. Os erros saem logados com a operação e voltam juntos.
func (s *Servico) gravarDepoisDaResposta(ctx context.Context, sess *Sessao, linhas []auditoria.Linha) error {
	contato := sess.contato()
	if s.Pedidos == nil || contato.Telefone == "" {
		return s.persistirComAuditoria(ctx, sess, linhas)
	}
	desbloquear := s.bloquearPedidos(contato)
	defer desbloquear()
	cart, adiada := s.carteiraAdiada(contato)
	if !adiada {
		return s.persistirComAuditoria(ctx, sess, linhas)
	}
	errLote := s.gravarEmLote(ctx, sess, contato, cart, linhas)
	if errLote == nil {
		s.pedidoMu.Lock()
		delete(s.pedidosPendentes, contato.Chave())
		s.pedidoMu.Unlock()
		return nil
	}
	if !errors.Is(errLote, errSemLote) {
		slog.WarnContext(ctx, "atende: gravação em lote recusada; gravando uma por uma", "operacao", "gravar_pos_resposta", "sessao", sess.ID, "erro", errLote)
	}
	return errors.Join(s.gravarCarteiraDepoisDaResposta(ctx, sess, contato, cart), s.persistirComAuditoria(ctx, sess, linhas))
}

// persistirComAuditoria grava a sessão e, em seguida, a auditoria do turno. Linha de
// auditoria perdida não é erro do turno: sai em ERROR e na métrica.
func (s *Servico) persistirComAuditoria(ctx context.Context, sess *Sessao, linhas []auditoria.Linha) error {
	err := s.Sessoes.Persistir(ctx, sess)
	s.gravarAuditoria(ctx, linhas)
	return err
}

// gravarAuditoria grava as linhas que o turno coletou; só há linhas com auditor.
func (s *Servico) gravarAuditoria(ctx context.Context, linhas []auditoria.Linha) {
	if len(linhas) == 0 || s.Auditoria == nil {
		return
	}
	s.Auditoria.Gravar(ctx, linhas)
}

func (s *Servico) gravarCarteiraDepoisDaResposta(ctx context.Context, sess *Sessao, contato escopo.Contato, cart pedidos.Carteira) error {
	antes := protocolosDe(cart)
	if err := s.salvarCarteira(ctx, contato, cart); err != nil {
		slog.ErrorContext(ctx, "atende: pedidos não gravados depois da resposta", "operacao", "gravar_pedidos_pos_resposta", "sessao", sess.ID, "erro", err)
		return err
	}
	if depois := protocolosDe(cart); !slices.Equal(antes, depois) {
		// a resposta saiu com um protocolo que já era de outro contato; o pedido foi gravado
		// com outro, e o morador só o verá na próxima vez que a carteira aparecer
		slog.ErrorContext(ctx, "atende: protocolo de pedido mudou depois da resposta", "operacao", "gravar_pedidos_pos_resposta", "sessao", sess.ID, "antes", antes, "depois", depois)
	}
	return nil
}

var errSemLote = errors.New("stores sem gravação em lote")

func (s *Servico) gravarEmLote(ctx context.Context, sess *Sessao, contato escopo.Contato, cart pedidos.Carteira, linhas []auditoria.Linha) error {
	pg, pedidosOK := s.Pedidos.(pedidosEmLote)
	st, sessoesOK := s.Sessoes.store.(sessaoEmLote)
	if s.Pool == nil || !pedidosOK || !sessoesOK {
		return errSemLote
	}
	sess.persistMu.Lock()
	defer sess.persistMu.Unlock()
	r, err := sess.Registro()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	lote := banco.NovoLote()
	confirmar, err := pg.GravarNoLote(ctx, lote, contato, cart)
	if err != nil {
		return err
	}
	st.SalvarNoLote(ctx, lote, r)
	auditoria.NoLote(lote, linhas)
	if err := lote.Enviar(ctx, s.Pool); err != nil {
		return err
	}
	if confirmar != nil {
		confirmar()
	}
	return nil
}

func protocolosDe(cart pedidos.Carteira) []string {
	out := make([]string, 0, len(cart.Pedidos))
	for _, p := range cart.Pedidos {
		out = append(out, p.Protocolo)
	}
	return out
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
	m, err = esquecerFatos(m, chave)
	if err != nil {
		return m, err
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

// esquecerFatos tira da memória o fato da chave; a chave vazia esquece todos.
func esquecerFatos(m memoria.Memoria, chave string) (memoria.Memoria, error) {
	if chave == "" {
		m.Fatos = []memoria.Fato{}
		return m, nil
	}
	semFato, esqueceu := m.Esquecer(chave)
	if !esqueceu {
		return m, fmt.Errorf("%w: %s", ErrFatoNaoEncontrado, chave)
	}
	return semFato, nil
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
	ctx, coletor := s.coletarAuditoria(ctx)
	sess, err := s.processar(ctx, msg)
	if sess == nil {
		return nil, err
	}
	// a sessão é gravada mesmo com o turno cancelado: o que já aconteceu nele fica
	return sess, errors.Join(err, s.gravarDepoisDaResposta(context.WithoutCancel(ctx), sess, coletor.Fechar()))
}

// coletarAuditoria abre o coletor do turno: a auditoria dos ganchos espera a resposta sair
// e vai ao banco junto com a sessão. Sem auditor, nada a coletar.
func (s *Servico) coletarAuditoria(ctx context.Context) (context.Context, *auditoria.Coletor) {
	if s.Auditoria == nil {
		return ctx, nil
	}
	return auditoria.ComColetor(ctx)
}

func (s *Servico) processar(ctx context.Context, msg entrada.Mensagem) (*Sessao, error) {
	if !s.PermiteTelefone(msg.Contato.Phone) {
		return nil, fmt.Errorf("telefone diferente do morador configurado para o token B2C")
	}
	s.aquecer(ctx, msg)
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
	ctx = comAuditoria(ctx, sess, "")
	sess.mu.Lock()
	// o canal reenvia o webhook (timeout na confirmação, retry de rede); o mesmo evento não
	// pode virar um segundo turno nem uma segunda resposta ao morador.
	if sess.jaRegistrado(msg.ID, "morador") {
		sess.mu.Unlock()
		return sess, nil
	}
	if sess.Encerrada {
		sess.mu.Unlock()
		return sess, nil
	}
	if sess.AtendimentoHumano || sess.revisaoEstado != msg.Revisao {
		sess.mu.Unlock()
		return sess, s.guardarFalaSemTurno(sess, msg)
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
	sess.mu.Lock()
	transbordado := sess.Transbordo != nil && (sess.Transbordo.Ativo || sess.Transbordo.Estado == "solicitado" || sess.Transbordo.Estado == "incerto")
	identidadeVazia := sess.Identidade.Phone == ""
	plano := sess.Plano
	sess.mu.Unlock()
	turnoGanchos, err := s.abrirTurno(sess)
	if err != nil {
		return sess, err
	}
	defer sess.fecharTurno(turnoGanchos)
	// as perguntas da resposta anterior valem só para esta mensagem: turno que falha,
	// é cancelado ou não envia nada não deixa pergunta para o próximo sim
	turnoGanchos = turnoGanchos.ComPerguntasConfirmadas(sess.tomarPerguntasConfirmadas(texto))
	if transbordado {
		// em transbordo o concierge não responde: a mensagem fica, mascarada, para o atendente
		texto = mascararEntrada(turnoGanchos, texto)
		sess.registrar(Turno{Quem: "morador", Texto: texto, EventoID: msg.ID})
		sess.historicoAdicionar(schema.UserMessage(texto))
		return sess, nil
	}
	m, errMapeamento := s.lerMapeamento(ctx)
	entradaGanchos := s.mensagemRecebida(ctx, sess, turnoGanchos, msg, texto, m)
	if bloqueioPorFalha(entradaGanchos.Registros) {
		// sem a checagem e a máscara a mensagem não segue, nem para o histórico
		s.transbordar(ctx, sess, turnoGanchos, MotivoFalhaDeGancho, "concierge", "")
		return sess, nil
	}
	texto = entradaGanchos.Evento.Texto
	sess.registrar(Turno{Quem: "morador", Texto: texto, EventoID: msg.ID})
	historico := sess.historicoSnapshot()
	baseHistorico := len(historico)
	troca := trocaDoTurno{texto: texto, historico: historico, base: baseHistorico}
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
	if errMapeamento != nil {
		err := fmt.Errorf("resolver condomínio: %w", errMapeamento)
		s.responderFalha(ctx, sess, turnoGanchos, troca, err)
		historicoRegistrado = true
		return sess, err
	}
	decisao := entradaGanchos.Decisao
	if decisao.Tipo == ganchos.Bloquear && decisao.Origem.Gancho == fixos.EscopoDaConversa {
		// condomínio que este processo não atende: transborda, como sempre
		s.avisar(ctx, "resolver_condominio", sess, fixos.CausaDoEscopo(entradaGanchos.Evento))
		sess.historicoAdicionar(schema.UserMessage(texto))
		s.transbordar(ctx, sess, turnoGanchos, decisao.Motivo, "concierge", "")
		return sess, nil
	}
	condo := entradaGanchos.Evento.Condominio
	if err := fixarCondominio(sess, condo, msg.Contato.Phone); err != nil {
		s.responderFalha(ctx, sess, turnoGanchos, troca, err)
		historicoRegistrado = true
		return sess, err
	}
	if decisao.Tipo == ganchos.Bloquear {
		s.responderAoMorador(ctx, sess, turnoGanchos, troca, decisao.Motivo, "")
		historicoRegistrado = true
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
			s.responderFalha(ctx, sess, turnoGanchos, troca, err)
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
	cfg.Turno = &turnoGanchos
	if err != nil {
		slog.ErrorContext(ctx, "atende: turno não montado", "operacao", "montar_turno", "sessao", sess.ID, "erro", err)
		s.responderFalha(ctx, sess, turnoGanchos, troca, err)
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
		s.responderFalha(ctx, sess, turnoGanchos, troca, err)
		historicoRegistrado = true
		return sess, err
	}
	var res *harness.Resultado
	var pedidosTurno []PedidoTurno
	if s.Pedidos != nil {
		res, pedidosTurno, err = s.turnoComPedidos(ctx, sess, ag, texto, historico, turnoGanchos.Cofre())
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
		s.responderFalha(ctx, sess, turnoGanchos, troca, err)
		historicoRegistrado = true
		return sess, err
	}
	if res.Transbordo != "" {
		// o motivo vai ao ATENDE, à sessão e ao log: sai daqui já sem PII
		res.Transbordo = s.motivoDoTransbordo(ctx, sess, turnoGanchos, res.Transbordo)
	}
	if res.Transbordo != "" && sess.Destino.Conector != "atende-emulado" && sess.Destino.Conector != "atende" {
		res.Resposta = "Não consigo transferir esta conversa para um atendente por este canal. Posso continuar ajudando por aqui."
		mensagens := append([]*schema.Message(nil), res.Mensagens...)
		if len(mensagens) > 0 && mensagens[len(mensagens)-1].Role == schema.Assistant {
			mensagens[len(mensagens)-1] = schema.AssistantMessage(res.Resposta, nil)
		}
		res.Mensagens = mensagens
	}
	transferivel := s.transferidorDisponivel(sess.Destino)
	bloqueioAvisoTransbordo := ""
	if res.Transbordo != "" && sess.Destino.Conector == "atende" {
		if s.Sender == nil {
			bloqueioAvisoTransbordo = "aviso de transbordo não enviado: sender não configurado"
		} else if ob, ok := s.Sender.(*Outbox); ok && !ob.PodeEntregarTransbordo() {
			bloqueioAvisoTransbordo = "aviso de transbordo não enviado: sender ATENDE não configurado"
		}
		if bloqueioAvisoTransbordo != "" {
			mensagens := append([]*schema.Message(nil), res.Mensagens...)
			if len(mensagens) > 0 && mensagens[len(mensagens)-1].Role == schema.Assistant {
				mensagens = mensagens[:len(mensagens)-1]
			}
			res.Resposta = ""
			res.Mensagens = mensagens
		} else if s.Pedidos == nil {
			aviso := "Não consegui solicitar um atendente por este canal agora. Posso continuar ajudando por aqui."
			if transferivel {
				aviso = "Vou solicitar que uma pessoa da administração assuma este protocolo. Ainda não tenho confirmação de que ela assumiu a conversa."
			}
			res.Resposta = anotarAvisoTransbordo(res.Resposta, aviso)
			res.Mensagens = substituirUltimaResposta(res.Mensagens, res.Resposta)
		}
	}
	if bloqueioAvisoTransbordo != "" {
		sess.historicoAtualizar(baseHistorico, res.Mensagens)
		historicoRegistrado = true
		s.registrarAvisoTransbordoFalho(sess, res.Transbordo, errors.New(bloqueioAvisoTransbordo))
		return sess, nil
	}
	var erroEntrega error
	entregue := res.Resposta
	if s.Sender != nil && res.Resposta != "" {
		saida := s.antesDeResponder(ctx, sess, turnoGanchos, res.Resposta)
		if saida.trocada {
			res.Resposta = saida.texto
			res.Mensagens = substituirUltimaResposta(res.Mensagens, saida.texto)
		}
		mascarada := res.Resposta
		entregue = saida.texto
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
		erroEntrega = s.Sender.Enviar(ctxEntrega, sess.Destino, entregue)
		suprimida := errors.Is(erroEntrega, errEntregaSuprimida) || erroEntrega != nil && ctxCancelavel.Err() != nil && !verificar()
		var parcial *entregaParcial
		if errors.As(erroEntrega, &parcial) {
			entregue = strings.Join(parcial.partes, "\n")
			res.Resposta = entregue
			if saida.texto != mascarada {
				// as partes saíram com os valores do cofre: o turno guarda a máscara
				res.Resposta, _ = turnoGanchos.Cofre().Mascarar(entregue)
			}
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
			if res.Transbordo != "" {
				s.registrarAvisoTransbordoFalho(sess, res.Transbordo, errEntregaSuprimida)
			}
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
			if res.Transbordo != "" {
				s.registrarAvisoTransbordoFalho(sess, res.Transbordo, erroEntrega)
			}
			return sess, erroEntrega
		}
	}
	if respostaEnviada(s.Sender, res, erroEntrega) {
		sess.guardarPerguntasDeRegra(turnoGanchos)
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
		if res.Transbordo != "" {
			s.registrarAvisoTransbordoFalho(sess, res.Transbordo, erroEntrega)
		}
		return sess, erroEntrega
	}
	if s.Pedidos != nil {
		ctxSalvar, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := s.marcarConfirmacoesEntregues(ctxSalvar, contato, pedidosTurno, entregue)
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
			defer comfalha.Recuperar(ctxMem, "extrair_memoria")
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
			if s.Transferidor == nil {
				s.marcarTransbordo(sess, res.Transbordo, "concierge", "")
			} else if err := s.Transferidor.Transferir(ctx, sess.Destino, res.Transbordo); err != nil {
				s.registrarFalhaTransbordo(sess, res.Transbordo, err)
			} else {
				s.registrarSucessoTransbordo(sess, res.Transbordo)
			}
			sess.mu.Lock()
			transferido := sess.Transbordo != nil && sess.Transbordo.Estado == "transferido"
			sess.mu.Unlock()
			if transferido {
				s.marcarPedidosTransbordados(sess, pedidosTurno)
			}
		} else if sess.Destino.Conector == "atende" {
			if !transferivel {
				err := errors.New("ATENDE: transferência não configurada para este canal")
				s.registrarFalhaTransbordo(sess, res.Transbordo, err)
				if ob, ok := s.Sender.(*Outbox); ok {
					ob.RegistrarTransbordo(sess.Destino, res.Transbordo, false, err.Error())
				}
				return sess, nil
			}
			if !s.iniciarTransbordo(sess, msg.Revisao, res.Transbordo) {
				sess.registrar(Turno{Quem: "sistema", Texto: "Transbordo não solicitado porque o protocolo mudou para atendimento humano ou foi encerrado durante o aviso."})
				return sess, nil
			}
			err := s.Transferidor.Transferir(ctx, sess.Destino, res.Transbordo)
			if err != nil {
				s.registrarFalhaTransbordo(sess, res.Transbordo, err)
				if ob, ok := s.Sender.(*Outbox); ok {
					ob.RegistrarTransbordo(sess.Destino, res.Transbordo, false, err.Error())
				}
				if s.Log != nil {
					s.Log.Printf("atende: transbordo do protocolo %s: %v", sess.Protocolo, err)
				}
				return sess, nil
			}
			s.registrarSucessoTransbordo(sess, res.Transbordo)
			if ob, ok := s.Sender.(*Outbox); ok {
				ob.RegistrarTransbordo(sess.Destino, res.Transbordo, true, "")
			}
			ctxSalvar, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			err = s.marcarPedidosAoAtendente(ctxSalvar, contato, pedidosTurno)
			cancelar()
			if err != nil {
				sess.registrar(Turno{Quem: "erro", Texto: "transbordo confirmado, mas não foi possível atualizar os pedidos: " + err.Error()})
				if s.Log != nil {
					s.Log.Printf("atende: atualizar pedidos após transbordo do protocolo %s: %v", sess.Protocolo, err)
				}
				return sess, nil
			}
		} else {
			sess.registrar(Turno{Quem: "sistema", Texto: "Solicitação de atendimento humano não encaminhada pelo canal ATENDE."})
		}
	}
	return sess, nil
}

// RegistrarHumano: um humano falou no protocolo; o concierge para de responder.
// Devolve false quando a sessão não existe ou já foi encerrada.
func (s *Servico) RegistrarHumano(ctx context.Context, msg entrada.Mensagem) (bool, error) {
	var errMascara error
	ok, err := s.mudarExistente(ctx, msg, func(sess *Sessao) bool {
		msg.Texto, errMascara = s.mascararForaDoTurno(sess, strings.TrimSpace(msg.Texto))
		return errMascara == nil && sess.registrarHumano(msg)
	})
	return ok, errors.Join(errMascara, err)
}

// Encerrar: o canal fechou o protocolo. Devolve false quando a sessão não existe.
func (s *Servico) Encerrar(ctx context.Context, msg entrada.Mensagem) (bool, error) {
	return s.mudarExistente(ctx, msg, func(sess *Sessao) bool {
		if sess.encerrar(msg) {
			s.encerramento(ctx, sess)
		}
		return true
	})
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
	sess.PerguntasDeRegra = nil // o sim que vier depois responde o atendente, não a regra
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

// encerrar fecha o protocolo; devolve false quando o evento já tinha sido aplicado.
func (sess *Sessao) encerrar(msg entrada.Mensagem) bool {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.jaRegistrado(msg.ID, "sistema") {
		return false
	}
	sess.Encerrada = true
	sess.PerguntasDeRegra = nil
	if sess.Transbordo != nil {
		sess.Transbordo.Ativo = false
	}
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
func (s *Servico) responderFalha(ctx context.Context, sess *Sessao, t ganchos.Turno, troca trocaDoTurno, err error) {
	frase := FraseFalha
	if FalhaTemporaria(err) {
		frase = FraseInstabilidade
	}
	s.responderAoMorador(ctx, sess, t, troca, frase, err.Error())
}

// guardarFalaSemTurno guarda, mascarada, a fala que chega durante o atendimento humano ou
// com a revisão trocada: o concierge não responde, mas o histórico volta ao modelo depois.
func (s *Servico) guardarFalaSemTurno(sess *Sessao, msg entrada.Mensagem) error {
	texto := strings.TrimSpace(msg.Texto)
	if texto == "" {
		return nil
	}
	texto, err := s.mascararForaDoTurno(sess, texto)
	if err != nil {
		return err
	}
	sess.registrar(Turno{Quem: "morador", Texto: texto, EventoID: msg.ID})
	sess.historicoAdicionar(schema.UserMessage(texto))
	return nil
}

// trocaDoTurno é a mensagem do morador (já mascarada) e o histórico em que ela entra.
type trocaDoTurno struct {
	texto     string
	historico []*schema.Message
	base      int
}

// responderAoMorador responde a troca com uma frase pronta (falha do concierge ou
// mensagem de uma regra), que passa por antes_de_responder. erro é o técnico, para o turno.
func (s *Servico) responderAoMorador(ctx context.Context, sess *Sessao, t ganchos.Turno, troca trocaDoTurno, frase, erro string) {
	frase, guardar := s.respostaPronta(ctx, sess, t, frase)
	sess.registrar(Turno{Quem: "concierge", Texto: guardar, Erro: erro})
	mensagens := append(append([]*schema.Message(nil), troca.historico...), schema.UserMessage(troca.texto), schema.AssistantMessage(guardar, nil))
	sess.historicoAtualizar(troca.base, mensagens)
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
	if cfg.Ganchos == nil {
		if cfg.Ganchos, err = s.ganchos(); err != nil {
			return cfg, mem, "", err
		}
	}
	cfg.AdministradoraID, cfg.CondominioID = condo.Administradora, condo.ID
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
		if err != nil {
			slog.WarnContext(ctx, "atende: memória não lida; turno segue sem ela", "operacao", "ler_memoria", "contato", contato.Mascarado(), "erro", err)
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
	if condo.Administradora != contato.AdministradoraID {
		return admin.Mapeamento{}, admin.Condominio{}, fmt.Errorf("condomínio %q da sessão %s mudou de dona: %w", contato.CondominioID, sess.ID, fixos.ErrCondominioMudou)
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

// fixarCondominio fixa na sessão o condomínio que o escopo_da_conversa resolveu e o
// contato na administradora dele.
func fixarCondominio(sess *Sessao, condo admin.Condominio, telefone string) error {
	contato, err := escopo.NovoContato(condo.Administradora, condo.ID, telefone)
	if err != nil {
		return fmt.Errorf("contato no condomínio %s: %w", condo.ID, err)
	}
	sess.fixarContato(contato, condo.CompanyID)
	return nil
}

func (s *Servico) avisar(ctx context.Context, operacao string, sess *Sessao, err error) {
	slog.WarnContext(ctx, "atende: condomínio não atendido; conversa transbordada",
		"operacao", operacao, "sessao", sess.ID, "erro", err)
}

// transbordar passa o protocolo ao atendente humano, com o motivo saneado pelos ganchos.
func (s *Servico) transbordar(ctx context.Context, sess *Sessao, t ganchos.Turno, motivo, pedidoPor, atendente string) {
	s.marcarTransbordo(sess, s.motivoDoTransbordo(ctx, sess, t, motivo), pedidoPor, atendente)
}

// marcarTransbordo registra o transbordo com um motivo que já passou pelos ganchos.
func (s *Servico) marcarTransbordo(sess *Sessao, motivo, pedidoPor, atendente string) {
	sess.mu.Lock()
	if sess.Encerrada {
		sess.mu.Unlock()
		sess.registrar(Turno{Quem: "sistema", Texto: "Transbordo não registrado porque o protocolo está encerrado."})
		return
	}
	sess.Transbordo = &Transbordo{Ativo: true, Estado: "transferido", Motivo: motivo, PedidoPor: pedidoPor, Quando: time.Now(), Atendente: atendente}
	sess.PerguntasDeRegra = nil
	sess.mu.Unlock()
	sess.registrar(Turno{Quem: "sistema", Texto: "Transferido ao atendente humano: " + motivo})
	if ob, ok := s.Sender.(*Outbox); ok && ob != nil {
		ob.Transbordar(sess.Destino, motivo)
	}
}

func substituirUltimaResposta(mensagens []*schema.Message, resposta string) []*schema.Message {
	mensagens = append([]*schema.Message(nil), mensagens...)
	if len(mensagens) > 0 && mensagens[len(mensagens)-1].Role == schema.Assistant {
		mensagens[len(mensagens)-1] = schema.AssistantMessage(resposta, nil)
	}
	return mensagens
}

func prometeTransbordo(texto string) bool {
	t := strings.ToLower(texto)
	if harness.AnunciaTransferencia(t) {
		return true
	}
	if !strings.Contains(t, "atendente") && !strings.Contains(t, "humano") && !strings.Contains(t, "administra") && !strings.Contains(t, "equipe") && !strings.Contains(t, "pessoa") {
		return false
	}
	for _, termo := range []string{"assum", "encaminh", "transfer", "passar", "repass", "continu", "entrar", "receber", "vai assumir", "irá assumir", "já foi"} {
		if strings.Contains(t, termo) {
			return true
		}
	}
	return false
}

func removerPromessasTransbordo(texto string) string {
	var partes []string
	inicio := 0
	for i, r := range texto {
		if r == '.' || r == '!' || r == '?' || r == ';' || r == '\n' {
			fim := i + 1
			if !prometeTransbordo(strings.TrimSpace(texto[inicio:fim])) {
				partes = append(partes, texto[inicio:fim])
			}
			inicio = fim
		}
	}
	if inicio < len(texto) && !prometeTransbordo(strings.TrimSpace(texto[inicio:])) {
		partes = append(partes, texto[inicio:])
	}
	return strings.Join(partes, "")
}

func anotarAvisoTransbordo(texto, aviso string) string {
	conteudo := removerPromessasTransbordo(texto)
	if strings.TrimSpace(conteudo) == "" {
		return aviso
	}
	return conteudo + "\n\n" + aviso
}

func (s *Servico) transferidorDisponivel(destino entrada.Destino) bool {
	if s.Transferidor == nil {
		return destino.Conector == "atende-emulado"
	}
	if disponibilidade, ok := s.Transferidor.(interface{ Disponivel(entrada.Destino) bool }); ok {
		return disponibilidade.Disponivel(destino)
	}
	return true
}

func (s *Servico) iniciarTransbordo(sess *Sessao, revisao uint64, motivo string) bool {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.Encerrada || sess.AtendimentoHumano || sess.revisaoEstado != revisao || sess.Transbordo != nil && sess.Transbordo.Ativo {
		return false
	}
	sess.Transbordo = &Transbordo{Estado: "solicitado", Motivo: motivo, PedidoPor: "concierge", Quando: time.Now()}
	return true
}

func (s *Servico) registrarSucessoTransbordo(sess *Sessao, motivo string) {
	sess.mu.Lock()
	if sess.Transbordo == nil {
		sess.Transbordo = &Transbordo{Motivo: motivo, PedidoPor: "concierge", Quando: time.Now()}
	}
	sess.Transbordo.Estado = "transferido"
	sess.Transbordo.Ativo = !sess.Encerrada
	if sess.Transbordo.Motivo == "" {
		sess.Transbordo.Motivo = motivo
	}
	sess.mu.Unlock()
	texto := "O ATENDE confirmou a transferência deste protocolo ao atendimento humano."
	if sess.Destino.Conector == "atende-emulado" {
		texto = "O emulador registrou o transbordo deste protocolo."
	}
	sess.registrar(Turno{Quem: "sistema", Texto: texto})
}

func (s *Servico) registrarFalhaTransbordo(sess *Sessao, motivo string, err error) {
	var incerta *TransferenciaIncerta
	estado := "falhou"
	if errors.As(err, &incerta) {
		estado = "incerto"
	}
	sess.mu.Lock()
	if sess.Transbordo == nil {
		sess.Transbordo = &Transbordo{Motivo: motivo, PedidoPor: "concierge", Quando: time.Now()}
	}
	sess.Transbordo.Estado = estado
	sess.Transbordo.Erro = err.Error()
	if !sess.AtendimentoHumano && !sess.Encerrada {
		sess.Transbordo.Ativo = false
	}
	if sess.Encerrada {
		sess.Transbordo.Ativo = false
	}
	sess.mu.Unlock()
	texto := "A transferência ao atendimento humano falhou: " + err.Error()
	if estado == "incerto" {
		texto = "O resultado da transferência ao atendimento humano é incerto. O protocolo ficará bloqueado para evitar respostas duplicadas. Erro: " + err.Error()
	}
	sess.registrar(Turno{Quem: "erro", Texto: texto})
}

func (s *Servico) registrarAvisoTransbordoFalho(sess *Sessao, motivo string, err error) {
	sess.mu.Lock()
	if sess.Transbordo == nil {
		sess.Transbordo = &Transbordo{Motivo: motivo, PedidoPor: "concierge", Quando: time.Now()}
	}
	sess.Transbordo.Estado = "falhou"
	sess.Transbordo.Erro = err.Error()
	if !sess.AtendimentoHumano && !sess.Encerrada {
		sess.Transbordo.Ativo = false
	}
	if sess.Encerrada {
		sess.Transbordo.Ativo = false
	}
	sess.mu.Unlock()
	sess.registrar(Turno{Quem: "sistema", Texto: "Transbordo não solicitado porque o aviso ao morador não foi entregue integralmente: " + err.Error()})
	if ob, ok := s.Sender.(*Outbox); ok {
		ob.RegistrarTransbordo(sess.Destino, motivo, false, err.Error())
	}
}

func (s *Servico) marcarPedidosTransbordados(sess *Sessao, turnos []PedidoTurno) {
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	if err := s.marcarPedidosAoAtendente(ctx, sess.contato(), turnos); err != nil {
		sess.registrar(Turno{Quem: "erro", Texto: "transbordo confirmado, mas não foi possível atualizar os pedidos: " + err.Error()})
		if s.Log != nil {
			s.Log.Printf("atende: atualizar pedidos após transbordo emulado do protocolo %s: %v", sess.Protocolo, err)
		}
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
	return s.noProtocolo(ctx, protocolo, func(sess *Sessao) (*Sessao, error) { return s.assumir(ctx, sess, atendente, motivo) })
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

func (s *Servico) assumir(ctx context.Context, sess *Sessao, atendente, motivo string) (*Sessao, error) {
	if sess.Destino.Conector != "atende-emulado" {
		return sess, errors.New("transferência de atendimento indisponível neste canal")
	}
	sess.mu.Lock()
	encerrada := sess.Encerrada
	sess.mu.Unlock()
	if encerrada {
		return sess, errors.New("o protocolo está encerrado")
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
	t, err := s.abrirTurno(sess)
	if err != nil {
		return sess, err
	}
	defer sess.fecharTurno(t)
	s.transbordar(comAuditoria(ctx, sess, atendente), sess, t, motivo, "atendente", atendente)
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
		// a fala humana sai como foi escrita; o turno e o histórico (que volta ao modelo) a
		// guardam mascarada
		guardado, err := s.mascararForaDoTurno(sess, entregue)
		if err != nil {
			return sess, errors.Join(err, errEnvio)
		}
		sess.registrar(Turno{Quem: "atendente", Texto: guardado, Agente: quem})
		sess.historicoAdicionar(schema.AssistantMessage("[atendente humano] "+guardado, nil))
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
		slog.WarnContext(ctx, "atende: extrator de memória falhou", "operacao", "extrair_memoria", "sessao", sess.ID, "erro", err)
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
		slog.WarnContext(ctx, "atende: memória não lida para mesclagem", "operacao", "mesclar_memoria", "contato", contato.Mascarado(), "erro", err)
		return nil
	}
	*mem = memAtual
	if mem.Nome == "" {
		mem.Nome = sess.Identidade.Name
	}
	var novos []memoria.Fato
	agora := time.Now()
	for _, f := range fatosSemPII(fatos) {
		f.Fonte = sess.Protocolo
		f.CondominioID = contato.CondominioID
		if f.Quando.IsZero() {
			f.Quando = agora
		}
		if atualizada, mudou := mem.Upsert(f); mudou {
			*mem = atualizada
			novos = append(novos, f)
		}
	}
	if len(novos) > 0 {
		if err := s.Memorias.Gravar(ctx, contato, *mem); err != nil {
			slog.ErrorContext(ctx, "atende: memória não gravada", "operacao", "gravar_memoria", "contato", contato.Mascarado(), "erro", err)
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
