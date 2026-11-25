package atendimento

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	comfalha "go-concierge/internal/falha"
	harness "go-concierge/internal/harness/agentes"
	"go-concierge/internal/harness/camadas"
	"go-concierge/internal/harness/canais"
	admin "go-concierge/internal/harness/configuracao"
	okf "go-concierge/internal/harness/conteudo"
	"go-concierge/internal/harness/ganchos"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/ganchos/fixos"
	regrasruntime "go-concierge/internal/harness/ganchos/regras"
	gruvi "go-concierge/internal/harness/identificacao"
	"go-concierge/internal/harness/memoria"
	"go-concierge/internal/harness/pedidos"
	"go-concierge/internal/harness/persistencia"
	"go-concierge/internal/identidade"
)

// Sender entrega a resposta do concierge ao protocolo do ATENDE.
type Sender interface {
	Enviar(ctx context.Context, destino entrada.Destino, texto string) error
}

type chaveControleEntrega = entrada.ChaveControleEntrega

var errEntregaSuprimida = entrada.ErrEntregaSuprimida

type entregaParcial = entrada.EntregaParcial

func textoEntregue(texto string, err error) string {
	if err == nil {
		return texto
	}
	var parcial *entregaParcial
	if errors.As(err, &parcial) {
		return strings.Join(parcial.Partes, "\n")
	}
	return ""
}

var autorizarEntrega = entrada.AutorizarEntrega

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
				s.Texto = strings.Join(parcial.Partes, "\n")
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

// Servico liga o webhook ao concierge: identidade B2C, harness no agente de primeiro
// nível, resposta pelo Sender.
// extracoes acompanha as extrações de memória em andamento (Aguardar: testes e desligamento).
var extracoes sync.WaitGroup

type Servico struct {
	Catalogos interface {
		Carregar(context.Context, string, *int) (okf.Catalogo, error)
	}
	Conteudo interface {
		LerRetratoDoEscopo(context.Context, string, *int) (camadas.Retrato, error)
	}
	Regras interface {
		AtivasDoEscopo(context.Context, string, *int) ([]regrasruntime.RegraGravada, error)
	}
	CamadasDe func(context.Context, string, *int, time.Time) (harness.SkillPublicadaFunc, error)
	Canais    CanaisRuntime
	Cat       okf.Catalogo
	Agente    string // caminho relativo do agente de primeiro nível (agentes/concierge)
	Config    harness.Config
	// Agora é o relógio dos eventos dos ganchos; nil = Config.Agora, senão time.Now.
	Agora func() time.Time
	// Implantacao restringe o processo a uma administradora (vazia = todas).
	Implantacao  escopo.Implantacao
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
	Mapeamento interface {
		Ler(context.Context) (admin.Mapeamento, error)
	}
	// SkillsDe devolve a consulta das skills compostas (base e camadas) de uma administradora
	// e condomínio, em memória e sem ida ao banco. nil = o turno lê os arquivos do catálogo.
	SkillsDe func(administradora, condominio string, agora time.Time) func(string) (okf.SkillEfetiva, bool)
	// Pedidos é o registro de pedidos por contato (nil = desligado: a conversa segue só
	// pelo orquestrador). ExtratorPedidos detecta os pedidos na mensagem (nil = Regras).
	Pedidos            pedidos.Store
	ExtratorPedidos    pedidos.Extrator
	pedidoMu           sync.Mutex
	pedidoLocks        map[string]*sync.Mutex
	pedidosPendentes   map[string]pedidos.Carteira
	pedidosBloqueados  map[string]map[string]pedidoBloqueado
	memoriaMu          sync.Mutex
	memoriaLocks       map[string]*sync.Mutex
	memoriaRevisoes    map[string]uint64
	memoriasBloqueadas map[string]map[string]memoriaPendente
	GravacaoTurno      GravacaoTurno
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

var ErrGravacaoIncerta = persistencia.ErrGravacaoIncerta
var ErrGravacaoNaoAplicada = persistencia.ErrGravacaoNaoAplicada
var ErrGravacaoRecusada = persistencia.ErrGravacaoRecusada

type GravacaoTurno interface {
	Gravar(context.Context, RegistroSessao, escopo.Contato, pedidos.Carteira, []auditoria.Linha) error
}

type pedidoBloqueado struct {
	contato escopo.Contato
	dados   json.RawMessage
	err     error
}

type memoriaPendente struct {
	contato escopo.Contato
	dados   json.RawMessage
	err     error
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
	if err := s.bloqueioCarteira(c); err != nil {
		return pedidos.Carteira{}, err
	}
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
	if bloqueado, ok := s.carteiraBloqueada(contato); ok {
		sess.persistMu.Lock()
		if sess.persistErr == nil {
			r, err := sess.Registro()
			if err != nil {
				bloqueado.err = errors.Join(bloqueado.err, err)
			}
			sess.reterPendente(r, bloqueado.contato, bloqueado.dados, linhas, bloqueado.err)
		}
		sess.persistMu.Unlock()
		return bloqueado.err
	}
	sess.persistMu.Lock()
	defer sess.persistMu.Unlock()
	if sess.persistErr != nil {
		return sess.persistErr
	}
	cart, adiada := s.carteiraAdiada(contato)
	if !adiada {
		return s.persistirComAuditoriaBloqueado(ctx, sess, linhas)
	}
	cart, err := pedidos.CarteiraParaTransporte(contato, cart)
	if err != nil {
		return err
	}
	carteira, err := json.Marshal(cart)
	if err != nil {
		return err
	}
	errLote := s.gravarEmLoteBloqueado(ctx, sess, contato, cart, carteira, linhas)
	if errLote == nil {
		s.pedidoMu.Lock()
		delete(s.pedidosPendentes, contato.Chave())
		s.pedidoMu.Unlock()
		return nil
	}
	if errors.Is(errLote, ErrGravacaoIncerta) {
		s.bloquearCarteira(contato, errLote, cart)
		return errLote
	}
	if errors.Is(errLote, ErrGravacaoRecusada) {
		s.pedidoMu.Lock()
		delete(s.pedidosPendentes, contato.Chave())
		s.pedidoMu.Unlock()
		return errLote
	}
	if !errors.Is(errLote, errSemLote) {
		slog.WarnContext(ctx, "atende: gravação em lote recusada; gravando uma por uma", "operacao", "gravar_pos_resposta", "sessao", sess.ID, "erro", errLote)
	}
	r, err := sess.Registro()
	if err != nil {
		return err
	}
	err = s.gravarCarteiraDepoisDaResposta(ctx, sess, contato, cart)
	if errors.Is(err, ErrGravacaoIncerta) {
		sess.reterPendente(r, contato, carteira, linhas, err)
		return err
	}
	if errors.Is(err, ErrGravacaoRecusada) {
		s.Sessoes.invalidarBloqueado(sess, err)
		s.pedidoMu.Lock()
		delete(s.pedidosPendentes, contato.Chave())
		s.pedidoMu.Unlock()
		return err
	}
	return errors.Join(err, s.persistirComAuditoriaBloqueado(ctx, sess, linhas))
}

// persistirComAuditoria grava a sessão e, em seguida, a auditoria do turno. Linha de
// auditoria perdida não é erro do turno: sai em ERROR e na métrica.
func (s *Servico) persistirComAuditoria(ctx context.Context, sess *Sessao, linhas []auditoria.Linha) error {
	sess.persistMu.Lock()
	defer sess.persistMu.Unlock()
	return s.persistirComAuditoriaBloqueado(ctx, sess, linhas)
}

func (s *Servico) persistirComAuditoriaBloqueado(ctx context.Context, sess *Sessao, linhas []auditoria.Linha) error {
	jaPendente := sess.pendente != nil
	err := s.Sessoes.persistirBloqueado(ctx, sess)
	if errors.Is(err, ErrGravacaoIncerta) {
		if sess.pendente != nil && (!jaPendente || sess.pendente.auditoria == nil) {
			sess.pendente.auditoria = slices.Clone(linhas)
		}
		return err
	}
	if errors.Is(err, ErrGravacaoRecusada) {
		return err
	}
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

func (s *Servico) gravarEmLoteBloqueado(ctx context.Context, sess *Sessao, contato escopo.Contato, cart pedidos.Carteira, carteira json.RawMessage, linhas []auditoria.Linha) error {
	if s.GravacaoTurno == nil {
		return errSemLote
	}
	r, err := sess.Registro()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutSessao)
	defer cancel()
	if s.GravacaoTurno != nil {
		err := s.GravacaoTurno.Gravar(ctx, r, contato, cart, linhas)
		if errors.Is(err, ErrGravacaoRecusada) {
			s.Sessoes.invalidarBloqueado(sess, err)
		} else if err != nil && !errors.Is(err, ErrGravacaoNaoAplicada) {
			err = errors.Join(ErrGravacaoIncerta, err)
			sess.reterPendente(r, contato, carteira, linhas, err)
		}
		return err
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
	return s.lockPorChave(&s.pedidoMu, &s.pedidoLocks, c.ChavePessoa())
}

func (s *Servico) carregarCarteira(ctx context.Context, c escopo.Contato) (pedidos.Carteira, error) {
	if err := s.bloqueioCarteira(c); err != nil {
		return pedidos.Carteira{}, err
	}
	chave := c.Chave()
	s.pedidoMu.Lock()
	pendente, existe := s.pedidosPendentes[chave]
	s.pedidoMu.Unlock()
	if existe {
		if err := s.Pedidos.Gravar(ctx, c, pendente); err != nil {
			if errors.Is(err, ErrGravacaoRecusada) {
				s.pedidoMu.Lock()
				delete(s.pedidosPendentes, chave)
				s.pedidoMu.Unlock()
				return pedidos.Carteira{}, err
			}
			if errors.Is(err, ErrGravacaoIncerta) {
				err = errors.Join(ErrGravacaoIncerta, err)
				s.bloquearCarteira(c, err, pendente)
			}
			return pedidos.Carteira{}, fmt.Errorf("pedidos: estado pendente ainda não pôde ser salvo: %w", err)
		}
		s.pedidoMu.Lock()
		delete(s.pedidosPendentes, chave)
		s.pedidoMu.Unlock()
	}
	return s.Pedidos.Ler(ctx, c)
}

func (s *Servico) bloqueioCarteira(c escopo.Contato) error {
	pendente, ok := s.carteiraBloqueada(c)
	if ok {
		return pendente.err
	}
	return nil
}

func (s *Servico) carteiraBloqueada(c escopo.Contato) (pedidoBloqueado, bool) {
	c = c.Canonico()
	s.pedidoMu.Lock()
	defer s.pedidoMu.Unlock()
	for _, pendente := range s.pedidosBloqueados[c.ChavePessoa()] {
		if c.CondoID == nil || pendente.contato.CondoID == nil || escopo.MesmoCondominio(c.CondoID, pendente.contato.CondoID) {
			return pendente, true
		}
	}
	return pedidoBloqueado{}, false
}

func (s *Servico) bloquearCarteira(c escopo.Contato, err error, carteiras ...pedidos.Carteira) {
	var dados json.RawMessage
	if len(carteiras) > 0 {
		cart, erro := pedidos.CarteiraParaTransporte(c, carteiras[0])
		if erro == nil {
			dados, erro = json.Marshal(cart)
		}
		if erro != nil {
			err = errors.Join(err, erro)
		}
	}
	s.pedidoMu.Lock()
	defer s.pedidoMu.Unlock()
	if s.pedidosBloqueados == nil {
		s.pedidosBloqueados = map[string]map[string]pedidoBloqueado{}
	}
	c = c.Canonico()
	pessoa := c.ChavePessoa()
	if s.pedidosBloqueados[pessoa] == nil {
		s.pedidosBloqueados[pessoa] = map[string]pedidoBloqueado{}
	}
	if c.CondoID != nil {
		condo := *c.CondoID
		c.CondoID = &condo
	}
	s.pedidosBloqueados[pessoa][c.Chave()] = pedidoBloqueado{contato: c, dados: slices.Clone(dados), err: err}
}

func (s *Servico) salvarCarteira(ctx context.Context, c escopo.Contato, cart pedidos.Carteira) error {
	if err := s.bloqueioCarteira(c); err != nil {
		return err
	}
	chave := c.Chave()
	if err := s.Pedidos.Gravar(ctx, c, cart); err != nil {
		if errors.Is(err, ErrGravacaoRecusada) {
			s.pedidoMu.Lock()
			delete(s.pedidosPendentes, chave)
			s.pedidoMu.Unlock()
			return err
		}
		s.pedidoMu.Lock()
		if s.pedidosPendentes == nil {
			s.pedidosPendentes = map[string]pedidos.Carteira{}
		}
		s.pedidosPendentes[chave] = cart
		s.pedidoMu.Unlock()
		if errors.Is(err, ErrGravacaoIncerta) {
			err = errors.Join(ErrGravacaoIncerta, err)
			s.bloquearCarteira(c, err, cart)
		}
		return fmt.Errorf("pedidos: estado não foi salvo e permanece pendente de recuperação: %w", err)
	}
	s.pedidoMu.Lock()
	delete(s.pedidosPendentes, chave)
	s.pedidoMu.Unlock()
	return nil
}

func (s *Servico) bloquearMemoria(c escopo.Contato) func() {
	return s.lockPorChave(&s.memoriaMu, &s.memoriaLocks, c.ChavePessoa())
}

func (s *Servico) bloqueioMemoria(c escopo.Contato) error {
	c = c.Canonico()
	s.memoriaMu.Lock()
	defer s.memoriaMu.Unlock()
	for _, pendente := range s.memoriasBloqueadas[c.ChavePessoa()] {
		if c.CondoID == nil || pendente.contato.CondoID == nil || escopo.MesmoCondominio(c.CondoID, pendente.contato.CondoID) {
			return pendente.err
		}
	}
	return nil
}

func (s *Servico) bloquearMemoriaIncerta(c escopo.Contato, dados json.RawMessage, err error) {
	c = c.Canonico()
	if c.CondoID != nil {
		condo := *c.CondoID
		c.CondoID = &condo
	}
	s.memoriaMu.Lock()
	defer s.memoriaMu.Unlock()
	if s.memoriasBloqueadas == nil {
		s.memoriasBloqueadas = map[string]map[string]memoriaPendente{}
	}
	if s.memoriasBloqueadas[c.ChavePessoa()] == nil {
		s.memoriasBloqueadas[c.ChavePessoa()] = map[string]memoriaPendente{}
	}
	s.memoriasBloqueadas[c.ChavePessoa()][c.Chave()] = memoriaPendente{contato: c, dados: slices.Clone(dados), err: err}
}

func (s *Servico) gravarMemoria(ctx context.Context, c escopo.Contato, m memoria.Memoria) error {
	if err := s.bloqueioMemoria(c); err != nil {
		return err
	}
	normalizada, err := memoria.ParaTransporte(c, m)
	if err != nil {
		return err
	}
	dados, err := json.Marshal(normalizada)
	if err != nil {
		return err
	}
	if err := s.Memorias.Gravar(ctx, c, normalizada); err != nil {
		if errors.Is(err, ErrGravacaoIncerta) {
			err = errors.Join(ErrGravacaoIncerta, err)
			s.bloquearMemoriaIncerta(c, dados, err)
		}
		return err
	}
	return nil
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
	if err := s.bloqueioMemoria(c); err != nil {
		return memoria.Memoria{}, err
	}
	m, err := s.Memorias.Ler(ctx, c)
	if err != nil {
		return m, err
	}
	m, err = esquecerFatos(m, chave, c.CondoID)
	if err != nil {
		return m, err
	}
	if err := s.gravarMemoria(ctx, c, m); err != nil {
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
func esquecerFatos(m memoria.Memoria, chave string, condoID ...*int) (memoria.Memoria, error) {
	if chave == "" {
		m.Fatos = []memoria.Fato{}
		return m, nil
	}
	semFato, esqueceu := m.Esquecer(chave, condoID...)
	if !esqueceu {
		return m, fmt.Errorf("%w: %s", ErrFatoNaoEncontrado, chave)
	}
	return semFato, nil
}

func (s *Servico) PrepararMensagem(ctx context.Context, msg entrada.Mensagem) (entrada.Mensagem, *Sessao, error) {
	var err error
	ctx, msg, err = s.autorizarMensagem(ctx, msg)
	if err != nil {
		return msg, nil, err
	}
	sess, err := s.Sessoes.Abrir(ctx, msg)
	if err != nil {
		return msg, nil, fmt.Errorf("abrir sessão do protocolo %s: %w", msg.ConversaID, err)
	}
	sess.persistMu.Lock()
	defer sess.persistMu.Unlock()
	if sess.persistErr != nil {
		return msg, nil, sess.persistErr
	}
	if c, ok := canalDoContexto(ctx); ok {
		if err := sess.fixarCanal(c, msg.Contato.Phone); err != nil {
			return msg, nil, err
		}
	}
	if s.Memorias != nil {
		if err := s.bloqueioMemoria(sess.contato()); err != nil {
			return msg, nil, err
		}
	}
	if s.Pedidos != nil {
		if err := s.bloqueioCarteira(sess.contato()); err != nil {
			return msg, nil, err
		}
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
	ctx = identidade.ComoRuntime(ctx)
	telefoneOriginal := msg.TelefoneOriginal
	if telefoneOriginal == "" {
		telefoneOriginal = msg.Contato.Phone
	}
	ctx = entrada.ComTelefoneOriginal(ctx, telefoneOriginal)
	var err error
	ctx, msg, err = s.autorizarMensagem(ctx, msg)
	if err != nil {
		return nil, err
	}
	ctx, coletor := s.coletarAuditoria(ctx)
	controle := &controleExtracao{feito: make(chan struct{})}
	ctx = context.WithValue(ctx, chaveExtracaoTurno{}, controle)
	return s.processar(ctx, msg, func(sess *Sessao) error {
		err := s.gravarDepoisDaResposta(context.WithoutCancel(ctx), sess, coletor.Fechar())
		controle.err = err
		close(controle.feito)
		return err
	})
}

type chaveExtracaoTurno struct{}

type controleExtracao struct {
	feito chan struct{}
	err   error
}

// coletarAuditoria abre o coletor do turno: a auditoria dos ganchos espera a resposta sair
// e vai ao banco junto com a sessão. Sem auditor, nada a coletar.
func (s *Servico) coletarAuditoria(ctx context.Context) (context.Context, *auditoria.Coletor) {
	if s.Auditoria == nil {
		return ctx, nil
	}
	return auditoria.ComColetor(ctx)
}

func (s *Servico) processar(ctx context.Context, msg entrada.Mensagem, finalizar func(*Sessao) error) (sess *Sessao, err error) {
	if !s.PermiteTelefone(msg.Contato.Phone) {
		return nil, fmt.Errorf("telefone diferente do morador configurado para o token B2C")
	}
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
	if err := sess.verificarPersistencia(); err != nil {
		return sess, err
	}
	if s.Memorias != nil {
		if err := s.bloqueioMemoria(sess.contato()); err != nil {
			return sess, err
		}
	}
	if s.Pedidos != nil {
		if err := s.bloqueioCarteira(sess.contato()); err != nil {
			return sess, err
		}
	}
	defer func() { err = errors.Join(err, finalizar(sess)) }()
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
	if c, ok := canalDoContexto(ctx); ok && c.CondoID != nil {
		identidadeVazia = true
		sess.Identidade = gruvi.Identidade{}
		sess.mcpResultados = nil
	}
	plano := sess.Plano
	sess.mu.Unlock()
	turnoGanchos, err := s.abrirTurnoDaExecucao(ctx, sess)
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
	var m admin.Mapeamento
	var errMapeamento error
	if c, ok := canalDoContexto(ctx); ok {
		m, errMapeamento = s.conteudoDoCanal(ctx, c)
	} else {
		m, errMapeamento = s.lerMapeamento(ctx)
	}
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
	var errFixar error
	if c, ok := canalDoContexto(ctx); ok {
		condo = condominioDoCanal(c)
		errFixar = sess.fixarCanal(c, msg.Contato.Phone)
	} else {
		errFixar = fixarCondominio(sess, condo, msg.Contato.Phone)
	}
	if err := errFixar; err != nil {
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

	if s.Catalogos != nil {
		cat, err := s.Catalogos.Carregar(ctx, contato.CompanyID, contato.CondoID)
		if err != nil {
			s.responderFalha(ctx, sess, turnoGanchos, troca, err)
			historicoRegistrado = true
			return sess, err
		}
		ctx = context.WithValue(ctx, chaveCatalogoExecucao{}, cat)
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
	ag, err := harness.Build(ctx, s.catalogoDoContexto(ctx), def, cfg)
	if err != nil {
		if errors.Is(err, context.Canceled) && interrompido() {
			return sess, nil
		}
		if errors.Is(err, ErrGravacaoRecusada) {
			sess.persistMu.Lock()
			s.Sessoes.invalidarBloqueado(sess, err)
			sess.persistMu.Unlock()
			return sess, err
		}
		if errors.Is(err, ErrGravacaoIncerta) {
			return sess, err
		}
		s.responderFalha(ctx, sess, turnoGanchos, troca, err)
		historicoRegistrado = true
		return sess, err
	}
	var res *harness.Resultado
	var pedidosTurno []PedidoTurno
	if s.Pedidos != nil {
		if s.Canais != nil {
			if err := s.Sessoes.Persistir(ctx, sess); err != nil {
				return sess, err
			}
		}
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
		suprimida := errors.Is(erroEntrega, errEntregaSuprimida) || errors.Is(erroEntrega, context.Canceled) && ctxCancelavel.Err() != nil && !verificar()
		var parcial *entregaParcial
		if errors.As(erroEntrega, &parcial) {
			entregue = strings.Join(parcial.Partes, "\n")
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
		sess.guardarPerguntasDeRegra(turnoGanchos, msg.Revisao)
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
			ctxMem, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
			defer cancel()
			defer comfalha.Recuperar(ctxMem, "extrair_memoria")
			if controle, ok := ctxMem.Value(chaveExtracaoTurno{}).(*controleExtracao); ok {
				select {
				case <-controle.feito:
					if controle.err != nil {
						return
					}
				case <-ctxMem.Done():
					return
				}
			}
			fatos := s.lembrar(ctxMem, &memCopia, sess, texto, res.Resposta, revisaoMemoria)
			sess.persistMu.Lock()
			if sess.persistErr == nil {
				sess.anotarMemorias(idx, fatos)
				if len(fatos) > 0 {
					_ = s.Sessoes.persistirBloqueado(ctxMem, sess)
				}
			}
			sess.persistMu.Unlock()
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
	ok, err := s.mudarExistente(ctx, msg, func(_ context.Context, sess *Sessao) bool {
		msg.Texto, errMascara = s.mascararForaDoTurno(sess, strings.TrimSpace(msg.Texto))
		return errMascara == nil && sess.registrarHumano(msg)
	})
	return ok, errors.Join(errMascara, err)
}

// Encerrar: o canal fechou o protocolo. Devolve false quando a sessão não existe.
func (s *Servico) Encerrar(ctx context.Context, msg entrada.Mensagem) (bool, error) {
	return s.mudarExistente(ctx, msg, func(ctx context.Context, sess *Sessao) bool {
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
func (s *Servico) mudarExistente(ctx context.Context, msg entrada.Mensagem, mudar func(context.Context, *Sessao) bool) (bool, error) {
	ctx = identidade.ComoRuntime(ctx)
	var err error
	ctx, msg, err = s.autorizarMensagem(ctx, msg)
	if err != nil {
		return false, err
	}
	sess, ok, err := s.Sessoes.Existente(ctx, msg)
	if err != nil {
		return false, fmt.Errorf("sessão do protocolo %s: %w", msg.ConversaID, err)
	}
	if !ok {
		return false, nil
	}
	sess.persistMu.Lock()
	defer sess.persistMu.Unlock()
	if sess.persistErr != nil {
		return false, sess.persistErr
	}
	if err := s.bloqueioCarteira(sess.contato()); err != nil {
		return false, err
	}
	if c, presente := canalDoContexto(ctx); presente {
		phone := msg.Contato.Phone
		if phone == "" {
			phone = sess.Contato.Phone
		}
		if err := sess.fixarCanal(c, phone); err != nil {
			return false, err
		}
	}
	if !mudar(ctx, sess) {
		return false, nil
	}
	return true, s.Sessoes.persistirBloqueado(ctx, sess)
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
type chaveCatalogoExecucao struct{}

func (s *Servico) catalogoDoContexto(ctx context.Context) okf.Catalogo {
	if cat, ok := ctx.Value(chaveCatalogoExecucao{}).(okf.Catalogo); ok {
		return cat
	}
	return s.Cat
}

func (s *Servico) prepararTurno(ctx context.Context, sess *Sessao, m admin.Mapeamento, condo admin.Condominio) (harness.Config, memoria.Memoria, *okf.Agent, error) {
	cfg, mem, dir, err := s.montar(ctx, sess, m, condo)
	if err != nil {
		return cfg, mem, nil, err
	}
	def, err := s.catalogoDoContexto(ctx).Load(dir)
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
	if c, ok := canalDoContexto(ctx); ok {
		casos, errCasos := s.Canais.Casos(ctx, c)
		if errCasos != nil {
			return cfg, mem, "", errCasos
		}
		porID := map[string]canais.Caso{}
		for _, caso := range casos {
			porID[caso.ID] = caso
		}
		cfg.Casos = func(_ string, id string) bool {
			if id == "" {
				return c.Habilitado
			}
			caso, ok := porID[id]
			return ok && c.Disponivel(caso, papel)
		}
		cfg.CompanyID, cfg.CondoID, cfg.ChannelID = c.CompanyID, c.CondoID, c.ID
		cfg.ValidarFerramenta = validarFerramentaDoCanal(c, identidade, cfg.MCPResultados)
	}
	if s.SkillsDe != nil {
		cfg.Skills = s.SkillsDe(condo.Administradora, condo.ID, s.agora())
	}
	if s.Conteudo != nil {
		retrato, errConteudo := s.Conteudo.LerRetratoDoEscopo(ctx, contato.CompanyID, contato.CondoID)
		if errConteudo != nil {
			return cfg, mem, "", errConteudo
		}
		if len(retrato.Bases) > 0 {
			efetivas := camadas.NovasEfetivas(slog.Default())
			efetivas.Trocar(retrato)
			id := ""
			if contato.CondoID != nil {
				id = fmt.Sprint(*contato.CondoID)
			}
			consulta := efetivas.DoCondominio(contato.CompanyID, id, s.agora())
			cfg.SkillPublicada = func(skill string) (okf.SkillEfetiva, bool, bool) {
				if _, ok := retrato.Bases[skill]; !ok {
					return okf.SkillEfetiva{}, false, false
				}
				v, ok := consulta(skill)
				return v, ok, true
			}
		}
	} else if s.CamadasDe != nil {
		cfg.SkillPublicada, err = s.CamadasDe(ctx, contato.CompanyID, contato.CondoID, s.agora())
		if err != nil {
			return cfg, mem, "", err
		}
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
		if err := s.bloqueioMemoria(contato); err != nil {
			desbloquear()
			return cfg, mem, "", err
		}
		m, err := s.Memorias.Ler(ctx, contato)
		desbloquear()
		if err != nil {
			return cfg, mem, "", fmt.Errorf("ler memória de %s: %w", contato.Mascarado(), err)
		}
		mem = m
		if c := harness.ContextoDeMemoria(mem); c != "" {
			contexto = append(contexto, c)
		}
	}
	cfg.Contexto = strings.Join(contexto, "\n\n")
	agenteRaiz := s.Agente
	if c, ok := canalDoContexto(ctx); ok {
		agenteRaiz = c.Agente
	}
	dir, err = s.catalogoDoContexto(ctx).ResolveAgente(agenteRaiz)
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
	if s.Canais != nil {
		c, err := s.Canais.PorID(ctx, sess.CompanyID, sess.ChannelID)
		if err != nil {
			return admin.Mapeamento{}, admin.Condominio{}, err
		}
		if err := sess.fixarCanal(c, sess.Contato.Phone); err != nil {
			return admin.Mapeamento{}, admin.Condominio{}, err
		}
		m, err := s.conteudoDoCanal(ctx, c)
		return m, condominioDoCanal(c), err
	}
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
	var err error
	ctx, err = s.contextoDaSessao(ctx, sess)
	if err != nil {
		return nil, err
	}
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
	if err := sess.verificarPersistencia(); err != nil {
		return sess, err
	}
	if err := s.bloqueioCarteira(sess.contato()); err != nil {
		return sess, err
	}
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
	if err := sess.verificarPersistencia(); err != nil {
		return nil
	}
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
	sess.persistMu.Lock()
	defer sess.persistMu.Unlock()
	if sess.persistErr != nil {
		return nil
	}
	if err := s.bloqueioMemoria(contato); err != nil {
		return nil
	}
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
		f.CondoID = contato.CondoID
		if f.Quando.IsZero() {
			f.Quando = agora
		}
		if atualizada, mudou := mem.Upsert(f); mudou {
			*mem = atualizada
			novos = append(novos, f)
		}
	}
	if len(novos) > 0 {
		if err := s.gravarMemoria(ctx, contato, *mem); err != nil {
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
	resultado, err := s.Config.MCP.ExecutarRecurso(ctx, ferramenta, map[string]any{
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
	dados, err = restringirIdentidadeGruvi(ctx, dados)
	if err != nil {
		return id, nil, err
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
	nomes := []string{company, packages, code, "gruvi.bff.building-operation.fetch-visit-reasons-controller-handle", visit,
		"gruvi.bff.mobile.fetch-fees-controller-handle-1", "gruvi.bff.mobile.get-fee-by-id-controller-handle-2",
		"gruvi.bff.mobile.fetch-documents-controller-handle-1", "gruvi.bff.mobile.get-document-by-id-controller-handle-1",
		"gruvi.bff.mobile.get-parameters-controller-handle-get-ticket-parameters-c4fee62606", "gruvi.bff.mobile.create-ticket-controller-handle"}
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
