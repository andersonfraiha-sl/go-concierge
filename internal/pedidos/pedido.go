// Package pedidos é o registro de trabalho do concierge por contato: cada pedido do
// morador (um caso de uso com os parâmetros do caso.yaml) ganha um protocolo e um
// ciclo de vida que atravessa sessões. O estado é decidido em código, pelos
// parâmetros capturados e validados, nunca inferido do texto de uma resposta:
//
//	coletando → pronto → aguardando_confirmacao → executando → concluido
//	                 ↘ com_atendente (pendência levada ao humano) ↗
//	                 ↘ cancelado | falhou
//
// O modelo só extrai (que caso é, que valores vieram); o motor (motor.go) percorre os
// passos do caso.yaml: lista recursos, escolhe, coleta, confirma, executa.
package pedidos

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"go-concierge/internal/escopo"
)

const (
	Coletando    = "coletando"
	Pronto       = "pronto"
	Confirmando  = "aguardando_confirmacao"
	Executando   = "executando"
	Concluido    = "concluido"
	ComAtendente = "com_atendente"
	Cancelado    = "cancelado"
	Falhou       = "falhou"

	OrigemMorador   = "morador"
	OrigemAtendente = "atendente"
	OrigemSistema   = "sistema"
	OrigemMemoria   = "memoria"
)

// Pedido é um caso de uso em andamento para um contato.
type Pedido struct {
	Protocolo        string `json:"protocolo"`
	AdministradoraID string `json:"administradora_id"`
	CondominioID     string `json:"condominio_id"` // o condomínio da conversa que abriu o pedido
	Telefone         string `json:"telefone"`
	Caso             string `json:"caso"`      // id do caso.yaml
	CasoNome         string `json:"caso_nome"` // name do caso.yaml
	Agente           string `json:"agente"`    // pasta do subagente (nome), para as permissões
	Estado           string `json:"estado"`
	// Valores são os parâmetros normalizados (slot → valor); Rotulos, o texto humano de
	// cada um; Origens, quem informou (morador | atendente | sistema | memoria).
	Valores map[string]any    `json:"valores"`
	Rotulos map[string]string `json:"rotulos,omitempty"`
	Origens map[string]string `json:"origens,omitempty"`
	// Dados são os retornos dos recursos (save_as): listas usadas nas escolhas e o
	// resultado da execução.
	Dados map[string]any `json:"dados,omitempty"`
	// Pendencia é o que falta agora (uma pergunta ao morador, ou ao atendente).
	Pendencia   *Pendencia   `json:"pendencia,omitempty"`
	Confirmacao *Confirmacao `json:"confirmacao,omitempty"`
	Resultado   *Resultado   `json:"resultado,omitempty"`
	// Resumo é o contexto para o atendente quando o pedido vai ao humano.
	Resumo string `json:"resumo,omitempty"`
	Erro   string `json:"erro,omitempty"`
	Origem Origem `json:"origem"`
	// Formulario: o caso tem um passo `form` que o motor não executa; o pedido fica
	// registrado e a conversa segue pelo subagente do caso.
	Formulario   bool      `json:"formulario,omitempty"`
	CriadoEm     time.Time `json:"criado_em"`
	AtualizadoEm time.Time `json:"atualizado_em"`
	Historico    []Evento  `json:"historico"`
}

// Pendencia é a pergunta em aberto: o slot, o que perguntar, as opções (quando há) e
// quantas vezes já foi perguntado. Contexto é o resumo de como surgiu, para o humano.
type Pendencia struct {
	Slot     string    `json:"slot"`
	Pergunta string    `json:"pergunta"`
	Opcoes   []Opcao   `json:"opcoes,omitempty"`
	Vezes    int       `json:"vezes"`
	Motivo   string    `json:"motivo,omitempty"` // por que a última resposta não serviu
	Desde    time.Time `json:"desde"`
	Contexto string    `json:"contexto,omitempty"`
}

type Confirmacao struct {
	Texto    string          `json:"texto"`
	Pedida   time.Time       `json:"pedida"`
	Entregue *bool           `json:"entregue,omitempty"`
	Valores  json.RawMessage `json:"valores,omitempty"`
	Vezes    int             `json:"vezes,omitempty"`    // quantas vezes foi pedida (lembretes contam)
	Resposta string          `json:"resposta,omitempty"` // sim | nao
	Por      string          `json:"por,omitempty"`      // morador | atendente
	Quando   time.Time       `json:"quando,omitempty"`
}

type Resultado struct {
	Texto     string    `json:"texto"`
	Protocolo string    `json:"protocolo,omitempty"` // devolvido pelo recurso
	Quando    time.Time `json:"quando"`
}

type Origem struct {
	ProtocoloAtende string `json:"protocolo_atende,omitempty"`
	Turno           int    `json:"turno,omitempty"`
}

type Evento struct {
	Quando time.Time `json:"quando"`
	Tipo   string    `json:"tipo"` // criado | valor | pergunta | confirmacao | executado | estado | atendente | erro
	Texto  string    `json:"texto"`
	Por    string    `json:"por,omitempty"`
}

type Opcao struct {
	Valor  string         `json:"valor"`
	Rotulo string         `json:"rotulo"`
	Dados  any            `json:"-"`
	Campos map[string]any `json:"-"`
}

// Aberto diz se o pedido ainda está em andamento.
func (p *Pedido) Aberto() bool {
	switch p.Estado {
	case Concluido, Cancelado, Falhou:
		return false
	}
	return true
}

func (p *Pedido) registrar(tipo, texto, por string) {
	p.Historico = append(p.Historico, Evento{Quando: time.Now(), Tipo: tipo, Texto: texto, Por: por})
	p.AtualizadoEm = time.Now()
}

func (p *Pedido) mudar(estado, motivo string) {
	if p.Estado == estado {
		return
	}
	p.Estado = estado
	p.registrar("estado", estado+orMotivo(motivo), "")
}

func orMotivo(m string) string {
	if m == "" {
		return ""
	}
	return ": " + m
}

// Valor registra um parâmetro com o rótulo e a origem.
func (p *Pedido) Valor(slot string, valor any, rotulo, origem string) {
	if p.Valores == nil {
		p.Valores = map[string]any{}
	}
	if p.Rotulos == nil {
		p.Rotulos = map[string]string{}
	}
	if p.Origens == nil {
		p.Origens = map[string]string{}
	}
	p.Valores[slot] = valor
	if rotulo == "" {
		rotulo = fmt.Sprint(valor)
	}
	p.Rotulos[slot] = rotulo
	p.Origens[slot] = origem
	p.registrar("valor", slot+" = "+rotulo, origem)
	if p.Pendencia != nil && p.Pendencia.Slot == slot {
		p.Pendencia = nil
	}
}

// Faltando lista os slots obrigatórios ainda sem valor, na ordem do contrato.
func (p *Pedido) Faltando(c *Contrato) []*Slot {
	var out []*Slot
	for _, nome := range c.Ordem {
		s := c.Slots[nome]
		if s.Obrigatorio {
			if _, ok := p.Valores[nome]; !ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// ---- carteira: os pedidos de um contato ----------------------------------------------

type Carteira struct {
	AdministradoraID string    `json:"administradora_id"`
	Telefone         string    `json:"telefone"`
	Pedidos          []*Pedido `json:"pedidos"`
	AtualizadaEm     time.Time `json:"atualizada_em"`
}

func (c *Carteira) Abertos() []*Pedido {
	var out []*Pedido
	for _, p := range c.Pedidos {
		if p.Aberto() {
			out = append(out, p)
		}
	}
	return out
}

func (c *Carteira) Por(protocolo string) *Pedido {
	for _, p := range c.Pedidos {
		if strings.EqualFold(p.Protocolo, protocolo) {
			return p
		}
	}
	return nil
}

// Remover tira da carteira um pedido encerrado (concluído, cancelado ou falhou). Um
// pedido aberto não sai: cancela-se antes, para o morador ficar sabendo.
func (c *Carteira) Remover(protocolo string) error {
	for i, p := range c.Pedidos {
		if !strings.EqualFold(p.Protocolo, protocolo) {
			continue
		}
		if p.Aberto() {
			return fmt.Errorf("pedido %s ainda está %s: cancele antes de remover", p.Protocolo, p.Estado)
		}
		c.Pedidos = append(c.Pedidos[:i], c.Pedidos[i+1:]...)
		c.AtualizadaEm = time.Now()
		return nil
	}
	return fmt.Errorf("pedido %s não encontrado", protocolo)
}

// AbertoDoCaso devolve o pedido aberto deste caso, se houver (um por vez por caso).
func (c *Carteira) AbertoDoCaso(caso string) *Pedido {
	for _, p := range c.Pedidos {
		if p.Aberto() && p.Caso == caso {
			return p
		}
	}
	return nil
}

// Novo abre um pedido para o caso, no condomínio da conversa, com protocolo aleatório
// único na carteira.
func (c *Carteira) Novo(ct *Contrato, agente, condominioID string, origem Origem) *Pedido {
	p := &Pedido{AdministradoraID: c.AdministradoraID, CondominioID: condominioID, Telefone: c.Telefone, Caso: ct.Caso.ID, CasoNome: ct.Caso.Name, Agente: agente, Estado: Coletando,
		Valores: map[string]any{}, Rotulos: map[string]string{}, Origens: map[string]string{}, Dados: map[string]any{}, Origem: origem, CriadoEm: time.Now()}
	for {
		p.Protocolo = NovoProtocolo()
		if c.Por(p.Protocolo) == nil {
			break
		}
	}
	p.registrar("criado", ct.Caso.Name, OrigemMorador)
	c.Pedidos = append(c.Pedidos, p)
	return p
}

// ValorReutilizavel: o valor que outro pedido do contato já tem para um slot com
// reuse: always (a unidade vale para a conversa toda).
func (c *Carteira) ValorReutilizavel(slot string) (any, string, bool) {
	for i := len(c.Pedidos) - 1; i >= 0; i-- {
		p := c.Pedidos[i]
		if v, ok := p.Valores[slot]; ok && p.Estado != Cancelado {
			return v, p.Rotulos[slot], true
		}
	}
	return nil, "", false
}

// NovoProtocolo gera "P-" + 5 caracteres sem ambiguidade (sem 0/O, 1/I).
func NovoProtocolo() string {
	const alfabeto = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("P-%05d", time.Now().UnixNano()%100000)
	}
	for i := range b {
		b[i] = alfabeto[int(b[i])%len(alfabeto)]
	}
	return "P-" + string(b)
}

// ---- armazenamento ----------------------------------------------------------------------

type Store interface {
	Ler(ctx context.Context, c escopo.Contato) (Carteira, error)
	Gravar(ctx context.Context, c escopo.Contato, cart Carteira) error
	Listar(ctx context.Context, administradoraID string) ([]Carteira, error)
}

// Arquivos guarda uma carteira por contato em <dir>/<administradora>/<telefone>.json.
type Arquivos struct {
	Dir string
	mu  sync.Mutex
}

func NewArquivos(dir string) *Arquivos { return &Arquivos{Dir: dir} }

var naoDigitos = regexp.MustCompile(`\D+`)

var errContatoInvalido = errors.New("contato sem administradora válida ou sem telefone")

// errProtocoloDeOutroContato: o protocolo é único no banco inteiro, mas só é sorteado único
// dentro da carteira; gravar um que já é de outro contato tomaria o pedido dele.
var errProtocoloDeOutroContato = errors.New("protocolo já pertence a outro contato")

func validarAdministradora(administradoraID string) error {
	if !escopo.AdministradoraValida(administradoraID) {
		return fmt.Errorf("pedidos da administradora %q: %w", administradoraID, errContatoInvalido)
	}
	return nil
}

func validarContato(c escopo.Contato) error {
	if err := validarAdministradora(c.AdministradoraID); err != nil {
		return err
	}
	if c.Telefone == "" {
		return errContatoInvalido
	}
	return nil
}

// copiasDoContato copia os pedidos com a administradora e o telefone do contato: quem grava é o
// contato, não o que veio escrito no pedido. Os pedidos do chamador ficam intactos.
func copiasDoContato(c escopo.Contato, pedidos []*Pedido) []Pedido {
	out := make([]Pedido, 0, len(pedidos))
	for _, p := range pedidos {
		copia := *p
		copia.AdministradoraID, copia.Telefone = c.AdministradoraID, c.Telefone
		out = append(out, copia)
	}
	return out
}

// montar é a carteira do contato com os pedidos na ordem em que foram abertos; ela foi
// atualizada quando o seu pedido mais recente foi.
func montar(c escopo.Contato, pedidos []*Pedido) Carteira {
	cart := Carteira{AdministradoraID: c.AdministradoraID, Telefone: c.Telefone, Pedidos: slices.Clone(pedidos)}
	slices.SortStableFunc(cart.Pedidos, func(a, b *Pedido) int {
		return cmp.Or(a.CriadoEm.Compare(b.CriadoEm), strings.Compare(a.Protocolo, b.Protocolo))
	})
	for _, p := range cart.Pedidos {
		if p.AtualizadoEm.After(cart.AtualizadaEm) {
			cart.AtualizadaEm = p.AtualizadoEm
		}
	}
	return cart
}

func maisRecentesPrimeiro(a, b Carteira) int { return b.AtualizadaEm.Compare(a.AtualizadaEm) }

func (a *Arquivos) pasta(administradoraID string) (string, error) {
	if err := validarAdministradora(administradoraID); err != nil {
		return "", err
	}
	return filepath.Join(a.Dir, administradoraID), nil
}

func (a *Arquivos) caminho(c escopo.Contato) (string, error) {
	if err := validarContato(c); err != nil {
		return "", err
	}
	return filepath.Join(a.Dir, c.AdministradoraID, c.Telefone+".json"), nil
}

func (a *Arquivos) Ler(_ context.Context, c escopo.Contato) (Carteira, error) {
	p, err := a.caminho(c)
	if err != nil {
		return Carteira{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return lerArquivo(p, c)
}

func lerArquivo(p string, c escopo.Contato) (Carteira, error) {
	cart := Carteira{AdministradoraID: c.AdministradoraID, Telefone: c.Telefone}
	raw, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return cart, nil
	}
	if err != nil {
		return cart, fmt.Errorf("ler pedidos de %s: %w", c.Mascarado(), semCaminho(err))
	}
	if err := json.Unmarshal(raw, &cart); err != nil {
		return cart, fmt.Errorf("ler pedidos de %s: %w", c.Mascarado(), err)
	}
	return cart, nil
}

// Gravar guarda a carteira sob o contato: a administradora e o telefone são os dele.
func (a *Arquivos) Gravar(_ context.Context, c escopo.Contato, cart Carteira) error {
	p, err := a.caminho(c)
	if err != nil {
		return err
	}
	cart.AdministradoraID, cart.Telefone = c.AdministradoraID, c.Telefone
	cart.AtualizadaEm = time.Now()
	raw, err := json.MarshalIndent(cart, "", "  ")
	if err != nil {
		return fmt.Errorf("codificar pedidos de %s: %w", c.Mascarado(), err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("gravar pedidos de %s: %w", c.Mascarado(), semCaminho(err))
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("gravar pedidos de %s: %w", c.Mascarado(), semCaminho(err))
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("gravar pedidos de %s: %w", c.Mascarado(), semCaminho(err))
	}
	return nil
}

// semCaminho tira do erro de arquivo o caminho, que tem o telefone; quem embrulha
// identifica o contato mascarado.
func semCaminho(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s: %w", pe.Op, pe.Err)
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return fmt.Errorf("%s: %w", le.Op, le.Err)
	}
	return err
}

func (a *Arquivos) Listar(_ context.Context, administradoraID string) ([]Carteira, error) {
	pasta, err := a.pasta(administradoraID)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	entradas, err := os.ReadDir(pasta)
	if errors.Is(err, os.ErrNotExist) {
		return []Carteira{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listar pedidos %s: %w", pasta, err)
	}
	out := []Carteira{}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		c := escopo.Contato{AdministradoraID: administradoraID, Telefone: strings.TrimSuffix(e.Name(), ".json")}
		cart, err := lerArquivo(filepath.Join(pasta, e.Name()), c)
		if err != nil {
			return nil, err
		}
		out = append(out, cart)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AtualizadaEm.After(out[j].AtualizadaEm) })
	return out, nil
}
