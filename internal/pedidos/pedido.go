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
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
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
	Protocolo string `json:"protocolo"`
	Account   string `json:"account"`
	Telefone  string `json:"telefone"`
	Caso      string `json:"caso"`      // id do caso.yaml
	CasoNome  string `json:"caso_nome"` // name do caso.yaml
	Agente    string `json:"agente"`    // pasta do subagente (nome), para as permissões
	Estado    string `json:"estado"`
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
	Texto    string    `json:"texto"`
	Pedida   time.Time `json:"pedida"`
	Vezes    int       `json:"vezes,omitempty"`    // quantas vezes foi pedida (lembretes contam)
	Resposta string    `json:"resposta,omitempty"` // sim | nao
	Por      string    `json:"por,omitempty"`      // morador | atendente
	Quando   time.Time `json:"quando,omitempty"`
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
	Valor  string `json:"valor"`
	Rotulo string `json:"rotulo"`
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
	Account      string    `json:"account"`
	Telefone     string    `json:"telefone"`
	Pedidos      []*Pedido `json:"pedidos"`
	AtualizadaEm time.Time `json:"atualizada_em"`
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

// Novo abre um pedido para o caso, com protocolo aleatório único na carteira.
func (c *Carteira) Novo(ct *Contrato, agente string, origem Origem) *Pedido {
	p := &Pedido{Account: c.Account, Telefone: c.Telefone, Caso: ct.Caso.ID, CasoNome: ct.Caso.Name, Agente: agente, Estado: Coletando,
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
	Ler(ctx context.Context, account, telefone string) (Carteira, error)
	Gravar(ctx context.Context, c Carteira) error
	Listar(ctx context.Context, account string) ([]Carteira, error)
}

// Arquivos guarda uma carteira por telefone em <dir>/<account>/<telefone>.json.
type Arquivos struct {
	Dir string
	mu  sync.Mutex
}

func NewArquivos(dir string) *Arquivos { return &Arquivos{Dir: dir} }

var naoDigitos = regexp.MustCompile(`\D+`)

// NormalizarTelefone deixa só os dígitos.
func NormalizarTelefone(t string) string { return naoDigitos.ReplaceAllString(t, "") }

var naoChave = regexp.MustCompile(`[^a-z0-9_-]+`)

func (a *Arquivos) caminho(account, telefone string) (string, error) {
	tel := naoDigitos.ReplaceAllString(telefone, "")
	if tel == "" {
		return "", errors.New("pedidos: telefone vazio")
	}
	acc := naoChave.ReplaceAllString(strings.ToLower(strings.TrimSpace(account)), "")
	if acc == "" {
		acc = "default"
	}
	return filepath.Join(a.Dir, acc, tel+".json"), nil
}

func (a *Arquivos) Ler(_ context.Context, account, telefone string) (Carteira, error) {
	c := Carteira{Account: account, Telefone: naoDigitos.ReplaceAllString(telefone, "")}
	p, err := a.caminho(account, telefone)
	if err != nil {
		return c, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	raw, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("pedidos: %s: %w", p, err)
	}
	return c, nil
}

func (a *Arquivos) Gravar(_ context.Context, c Carteira) error {
	p, err := a.caminho(c.Account, c.Telefone)
	if err != nil {
		return err
	}
	c.AtualizadaEm = time.Now()
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (a *Arquivos) Listar(ctx context.Context, account string) ([]Carteira, error) {
	acc := naoChave.ReplaceAllString(strings.ToLower(strings.TrimSpace(account)), "")
	if acc == "" {
		acc = "default"
	}
	entradas, err := os.ReadDir(filepath.Join(a.Dir, acc))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Carteira
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		c, err := a.Ler(ctx, account, strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AtualizadaEm.After(out[j].AtualizadaEm) })
	return out, nil
}
