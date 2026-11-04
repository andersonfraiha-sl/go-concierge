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
	"maps"
	"regexp"
	"slices"
	"strings"
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
	Formulario bool `json:"formulario,omitempty"`
	// Ordem é a posição de abertura na carteira (1, 2, ...): a fila de pedidos (um por vez)
	// segue essa ordem. CriadoEm não basta: dois pedidos da mesma mensagem podem ter o mesmo
	// instante. Pedidos anteriores à fila ficam com 0 e vêm antes, pelo CriadoEm.
	Ordem        int       `json:"ordem,omitempty"`
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
		Valores: map[string]any{}, Rotulos: map[string]string{}, Origens: map[string]string{}, Dados: map[string]any{}, Origem: origem, CriadoEm: time.Now(),
		Ordem: c.ultimaOrdem() + 1}
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

func (c *Carteira) ultimaOrdem() int {
	ultima := 0
	for _, p := range c.Pedidos {
		ultima = max(ultima, p.Ordem)
	}
	return ultima
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

// NovoProtocolo gera "P-" + 8 caracteres sem ambiguidade (sem 0/O, 1/I). Os pedidos antigos
// têm 5 e continuam valendo.
func NovoProtocolo() string {
	const alfabeto = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("P-%08d", time.Now().UnixNano()%100000000)
	}
	for i := range b {
		b[i] = alfabeto[int(b[i])%len(alfabeto)]
	}
	return "P-" + string(b)
}

// ---- armazenamento ----------------------------------------------------------------------

// Store guarda as carteiras. Gravar pode trocar o protocolo de um pedido novo que colide com
// o de outro contato (veja redesenharColisoes); o pedido da carteira passa a ter o novo.
type Store interface {
	Ler(ctx context.Context, c escopo.Contato) (Carteira, error)
	Gravar(ctx context.Context, c escopo.Contato, cart Carteira) error
	Listar(ctx context.Context, administradoraID string) ([]Carteira, error)
}

var naoDigitos = regexp.MustCompile(`\D+`)

var errContatoInvalido = errors.New("contato sem administradora válida ou sem telefone")

// ErrProtocoloEmUso: o protocolo de um pedido novo já é de outro contato e nenhum dos
// tentativasProtocolo sorteios saiu livre.
var ErrProtocoloEmUso = errors.New("protocolo já em uso por outro contato")

// tentativasProtocolo: quantos sorteios um pedido novo com protocolo de outro contato ganha.
const tentativasProtocolo = 3

// deOutros devolve, dentre os protocolos, os que já são de outro contato.
type deOutros func(protocolos []string) (map[string]bool, error)

// redesenharColisoes devolve os pedidos com novo protocolo para os que colidem com o de outro
// contato. O protocolo é único no banco inteiro, mas Carteira.Novo só o sorteia único na
// carteira. Só pedido novo colide: o que já está gravado para o contato é dele e nunca muda
// de protocolo.
func redesenharColisoes(pedidos []Pedido, alheios deOutros, sortear func() string) ([]Pedido, error) {
	usados := map[string]bool{}
	for _, p := range pedidos {
		usados[p.Protocolo] = true
	}
	colidem, err := alheios(slices.Collect(maps.Keys(usados)))
	if err != nil || len(colidem) == 0 {
		return pedidos, err
	}
	out := slices.Clone(pedidos)
	for i := range out {
		if !colidem[out[i].Protocolo] {
			continue
		}
		novo, err := sortearLivre(usados, alheios, sortear)
		if err != nil {
			return nil, fmt.Errorf("pedido %s: %w", out[i].Protocolo, err)
		}
		usados[novo] = true
		out[i].Protocolo = novo
	}
	return out, nil
}

func sortearLivre(usados map[string]bool, alheios deOutros, sortear func() string) (string, error) {
	for range tentativasProtocolo {
		candidato := sortear()
		if usados[candidato] {
			continue
		}
		colide, err := alheios([]string{candidato})
		if err != nil {
			return "", err
		}
		if !colide[candidato] {
			return candidato, nil
		}
	}
	return "", ErrProtocoloEmUso
}

// adotarProtocolos leva aos pedidos do chamador os protocolos redesenhados, na mesma ordem.
func adotarProtocolos(carteira []*Pedido, gravados []Pedido) {
	for i, p := range carteira {
		p.Protocolo = gravados[i].Protocolo
	}
}

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
		return cmp.Or(cmp.Compare(a.Ordem, b.Ordem), a.CriadoEm.Compare(b.CriadoEm), strings.Compare(a.Protocolo, b.Protocolo))
	})
	for _, p := range cart.Pedidos {
		if p.AtualizadoEm.After(cart.AtualizadaEm) {
			cart.AtualizadaEm = p.AtualizadoEm
		}
	}
	return cart
}

func maisRecentesPrimeiro(a, b Carteira) int { return b.AtualizadaEm.Compare(a.AtualizadaEm) }
