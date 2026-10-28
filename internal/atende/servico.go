package atende

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/schema"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go-concierge/internal/admin"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
	"go-concierge/internal/okf"
	"go-concierge/internal/pedidos"
)

// Sender entrega a resposta do concierge ao protocolo do ATENDE.
type Sender interface {
	Enviar(ctx context.Context, protocolo string, texto string) error
}

// Saida é uma resposta enviada (ou a enviar) ao ATENDE.
type Saida struct {
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
// Transbordar registra o pedido de atendente humano ao ATENDE. No ATENDE real isso vira
// a chamada que tira o protocolo do bot (isHumanAttendance); aqui fica na caixa.
func (o *Outbox) Transbordar(protocolo, motivo string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.itens = append(o.itens, Saida{Protocolo: protocolo, Tipo: "transbordo", Texto: motivo, Quando: time.Now(), Enviada: o.Real == nil})
}

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
// extracoes acompanha as extrações de memória em andamento (Aguardar: testes e desligamento).
var extracoes sync.WaitGroup

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
	// Pedidos é o registro de pedidos por contato (nil = desligado: a conversa segue só
	// pelo orquestrador). ExtratorPedidos detecta os pedidos na mensagem (nil = Regras).
	Pedidos         pedidos.Store
	ExtratorPedidos pedidos.Extrator
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
	// em transbordo o concierge não responde: a mensagem fica para o atendente humano
	if sess.Transbordo != nil && sess.Transbordo.Ativo {
		sess.historico = append(sess.historico, schema.UserMessage(texto))
		return sess, nil
	}

	if sess.Identidade.Phone == "" {
		sess.Identidade, sess.IdentErro = s.identificar(ctx, ev.Data.Contact.Phone)
		if sess.Identidade.Name == "" {
			sess.Identidade.Name = ev.Data.Contact.Name
		}
	}

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
		sess.registrar(Turno{Quem: "erro", Texto: err.Error()})
		return sess, err
	}
	var res *harness.Resultado
	var pedidosTurno []PedidoTurno
	if s.Pedidos != nil {
		res, pedidosTurno, err = s.turnoComPedidos(ctx, sess, ag, texto)
	} else if sess.Plano != nil && sess.Plano.Pendente != "" {
		// um item do plano perguntou algo ao morador: a resposta vai direto a ele
		res, err = harness.Continuar(ctx, ag, sess.Plano, texto, sess.historico)
	} else {
		res, err = harness.Run(ctx, ag, texto, sess.historico)
	}
	if err != nil {
		sess.registrar(Turno{Quem: "erro", Texto: err.Error()})
		return sess, err
	}
	sess.historico = res.Mensagens
	sess.Plano = nil
	if res.Plano != nil && res.Plano.Pendente != "" {
		sess.Plano = res.Plano
	}
	uso := res.Uso
	turno := Turno{Quem: "concierge", Texto: res.Resposta, Agente: res.Agente, Rota: res.Rota, Uso: &uso, Modelo: ag.NomeModelo, Plano: res.Plano, Pedidos: pedidosTurno}
	idx := sess.registrar(turno)
	if s.Sender != nil && res.Resposta != "" {
		if err := s.Sender.Enviar(ctx, sess.Protocolo, res.Resposta); err != nil {
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
			fatos := s.lembrar(ctxMem, &memCopia, sess, texto, res.Resposta)
			sess.anotarMemorias(idx, fatos)
		}()
	}
	if res.Transbordo != "" {
		s.transbordar(sess, res.Transbordo, "concierge", "")
	}
	return sess, nil
}

// montar prepara a Config do harness para a sessão (identidade, Admin do condomínio,
// data e hora, unidades, memória) e devolve a memória lida e a pasta do agente.
func (s *Servico) montar(ctx context.Context, sess *Sessao) (cfg harness.Config, mem memoria.Memoria, dir string, err error) {
	cfg = s.Config
	cfg.Sessao = sess.Protocolo
	cfg.Identidade = harness.Identidade{Authenticated: sess.Identidade.Authenticated, AccountID: s.Account, Role: sess.Identidade.Role}

	// Admin da administradora: qual condomínio é este, quais casos ele pode usar e
	// quais definições (payloads) valem para os recursos
	var contexto []string
	if s.Mapeamento != nil {
		if m, err := s.Mapeamento.Ler(); err == nil {
			condo, ok := m.Resolver(sess.Identidade.CompanyID, sess.Tenant, s.Account)
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

	// data e hora de agora: sem isso o modelo chuta o ano ao converter "sábado" em data
	contexto = append(contexto, contextoDeAgora(time.Now()))

	// identidade do Gruvi: entra no prompt em toda troca e resolve unidades.listar sem Nexus
	if c := contextoDeIdentidade(sess.Identidade); c != "" {
		contexto = append(contexto, c)
	}
	if sess.Identidade.Authenticated && len(sess.Identidade.Units) > 0 {
		unidades := make([]map[string]any, 0, len(sess.Identidade.Units))
		for _, u := range sess.Identidade.Units {
			item := map[string]any{"id": u.ID, "label": u.Label}
			if u.Bloco != "" {
				item["bloco"] = u.Bloco
			}
			if u.Condominio != "" {
				item["condominio"] = u.Condominio
			}
			unidades = append(unidades, item)
		}
		cfg.Locais = map[string]func(context.Context, map[string]any) (any, error){
			"unidades.listar": func(context.Context, map[string]any) (any, error) { return unidades, nil },
		}
		// uma só unidade: o unidade_id de todo recurso que o pede já vai preenchido, para o
		// agente nunca ter de perguntar (o payload do condomínio, se existir, tem precedência)
		if len(sess.Identidade.Units) == 1 {
			if cfg.Payloads == nil {
				cfg.Payloads = map[string]map[string]any{}
			}
			uid := sess.Identidade.Units[0].ID
			for _, r := range s.Cat.RecursosNexus() {
				for _, p := range r.Params {
					if p.Name != "unidade_id" {
						continue
					}
					if cfg.Payloads[r.Name] == nil {
						cfg.Payloads[r.Name] = map[string]any{}
					}
					if _, tem := cfg.Payloads[r.Name]["unidade_id"]; !tem {
						cfg.Payloads[r.Name]["unidade_id"] = uid
					}
				}
			}
		}
	}

	// memória relacional: o que já se sabe deste telefone entra no prompt da família
	if s.Memorias != nil {
		m, err := s.Memorias.Ler(ctx, s.Account, sess.Contato.Phone)
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
	sess.Transbordo = &Transbordo{Ativo: true, Motivo: motivo, PedidoPor: pedidoPor, Quando: time.Now(), Atendente: atendente}
	sess.registrar(Turno{Quem: "sistema", Texto: "Transferido ao atendente humano: " + motivo})
	if ob, ok := s.Sender.(*Outbox); ok && ob != nil {
		ob.Transbordar(sess.Protocolo, motivo)
	}
}

// Assumir: o atendente toma o protocolo (mesmo sem o concierge ter pedido).
func (s *Servico) Assumir(protocolo, atendente, motivo string) (*Sessao, error) {
	sess, ok := s.Sessoes.Get(protocolo)
	if !ok {
		return nil, fmt.Errorf("protocolo desconhecido: %s", protocolo)
	}
	if atendente == "" {
		atendente = "atendente"
	}
	if sess.Transbordo != nil && sess.Transbordo.Ativo {
		sess.Transbordo.Atendente = atendente
		sess.registrar(Turno{Quem: "sistema", Texto: atendente + " assumiu o atendimento"})
		return sess, nil
	}
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
	if sess.Transbordo == nil || !sess.Transbordo.Ativo {
		return nil, fmt.Errorf("o protocolo não está em transbordo: assuma o atendimento antes")
	}
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return nil, fmt.Errorf("mensagem vazia")
	}
	if atendente != "" {
		sess.Transbordo.Atendente = atendente
	}
	sess.registrar(Turno{Quem: "atendente", Texto: texto, Agente: sess.Transbordo.Atendente})
	sess.historico = append(sess.historico, schema.AssistantMessage("[atendente humano] "+texto, nil))
	if s.Sender != nil {
		if err := s.Sender.Enviar(ctx, sess.Protocolo, texto); err != nil {
			sess.registrar(Turno{Quem: "erro", Texto: err.Error()})
			return sess, err
		}
	}
	return sess, nil
}

// Devolver: o atendente devolve o protocolo ao concierge, que volta a responder.
func (s *Servico) Devolver(protocolo string) (*Sessao, error) {
	sess, ok := s.Sessoes.Get(protocolo)
	if !ok {
		return nil, fmt.Errorf("protocolo desconhecido: %s", protocolo)
	}
	if sess.Transbordo == nil || !sess.Transbordo.Ativo {
		return sess, nil
	}
	agora := time.Now()
	sess.Transbordo.Ativo = false
	sess.Transbordo.Devolvido = &agora
	sess.registrar(Turno{Quem: "sistema", Texto: "Devolvido ao concierge"})
	sess.historico = append(sess.historico, schema.AssistantMessage("[sistema] o atendente humano devolveu a conversa ao concierge; continue de onde parou.", nil))
	return sess, nil
}

// Aguardar espera as extrações de memória em andamento (testes e desligamento).
func (s *Servico) Aguardar() { extracoes.Wait() }

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

var diasSemana = []string{"domingo", "segunda-feira", "terça-feira", "quarta-feira", "quinta-feira", "sexta-feira", "sábado"}
var meses = []string{"", "janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

// contextoDeAgora: a data e a hora atuais, no fuso local, para datas relativas
// ("sábado", "amanhã", "dia 12") virarem AAAA-MM-DD certos.
func contextoDeAgora(t time.Time) string {
	t = t.Local()
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
	s := fmt.Sprintf("# Morador identificado\n\n- Nome: %s\n- Papel: %s", nome, papel)
	condos := id.Condominios
	if len(condos) == 0 && len(id.Units) > 0 {
		condos = []gruvi.Condominio{{Nome: id.CompanyName, Unidades: id.Units}}
	}
	for _, c := range condos {
		nomeC := c.Nome
		if nomeC == "" {
			nomeC = "condomínio"
		}
		var us []string
		for _, u := range c.Unidades {
			us = append(us, fmt.Sprintf("%s (unidade_id %s)", u.Label, u.ID))
		}
		s += fmt.Sprintf("\n- Condomínio %s: %s", nomeC, strings.Join(us, "; "))
	}
	switch {
	case len(condos) == 1 && len(id.Units) == 1:
		s += fmt.Sprintf("\n\nHá um só condomínio e uma só unidade: use unidade_id %s sem perguntar e registre o pedido nela.", id.Units[0].ID)
	case len(id.Units) > 1:
		s += "\n\nO morador tem mais de um condomínio ou mais de uma unidade: antes de registrar qualquer pedido, pergunte de qual se trata (ofereça as opções pelo nome) e registre o pedido naquele condomínio e unidade (unidade_id). Se ele já disse qual é, não pergunte de novo."
	}
	return s
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
