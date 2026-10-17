// Package admin é o mapeamento da administradora: para cada condomínio, quais casos
// de uso o concierge pode usar (e para quais papéis) e qual payload de definições vale
// para cada recurso do Nexus naquele condomínio. É a "matriz" de um banco com RLS por
// administradora e condomínio; hoje em JSON (okf/var/admin/mapeamento.json), com a
// mesma forma que o Postgres terá.
//
// Regras:
//   - sem mapeamento para um condomínio, o atributo é comum a todos ("*", o padrão);
//   - um caso de uso sem entrada está HABILITADO (o padrão é oferecer);
//   - o payload de um recurso num condomínio é "*" sobreposto pelo do condomínio, chave a chave;
//   - em tempo de execução o payload do recurso é remontado na chamada (preenche os
//     params que o agente não passou) e entra no prompt como contexto do condomínio.
package admin

import (
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

// Padrao é a chave do mapeamento comum a todos os condomínios.
const Padrao = "*"

// Condominio é um condomínio da administradora, com os ids que o ligam aos sistemas.
type Condominio struct {
	ID        string `json:"id"`
	Nome      string `json:"nome"`
	Account   string `json:"account"`    // account dos recursos no Nexus
	CompanyID string `json:"company_id"` // company no Gruvi (identidade por telefone)
	Tenant    string `json:"tenant"`     // tenant do ATENDE
}

// Habilitacao de um caso de uso num condomínio.
type Habilitacao struct {
	Habilitado bool     `json:"habilitado"`
	Papeis     []string `json:"papeis"` // vazio = todos os papéis
}

// Mapeamento é o conteúdo inteiro da administradora.
type Mapeamento struct {
	Administradora string       `json:"administradora"`
	Condominios    []Condominio `json:"condominios"`
	// Habilitacoes: condomínio (ou "*") → caso de uso → habilitação.
	Habilitacoes map[string]map[string]Habilitacao `json:"habilitacoes"`
	// Payloads: condomínio (ou "*") → recurso do Nexus → definições.
	Payloads     map[string]map[string]map[string]any `json:"payloads"`
	AtualizadoEm time.Time                            `json:"atualizado_em"`
}

func (m *Mapeamento) normalizar() {
	if m.Condominios == nil {
		m.Condominios = []Condominio{}
	}
	if m.Habilitacoes == nil {
		m.Habilitacoes = map[string]map[string]Habilitacao{}
	}
	if m.Payloads == nil {
		m.Payloads = map[string]map[string]map[string]any{}
	}
	for k, v := range m.Habilitacoes {
		if v == nil {
			m.Habilitacoes[k] = map[string]Habilitacao{}
		}
		for c, h := range v {
			if h.Papeis == nil {
				h.Papeis = []string{}
				m.Habilitacoes[k][c] = h
			}
		}
	}
	for k, v := range m.Payloads {
		if v == nil {
			m.Payloads[k] = map[string]map[string]any{}
		}
	}
	sort.Slice(m.Condominios, func(i, j int) bool { return m.Condominios[i].Nome < m.Condominios[j].Nome })
}

// Condominio procura pelo id.
func (m Mapeamento) Condominio(id string) (Condominio, bool) {
	for _, c := range m.Condominios {
		if c.ID == id {
			return c, true
		}
	}
	return Condominio{}, false
}

// Resolver acha o condomínio pelos ids que chegam em tempo de execução: company do
// Gruvi, tenant do ATENDE ou account do Nexus. Sem casar, devolve ok=false (vale o padrão).
func (m Mapeamento) Resolver(companyID, tenant, account string) (Condominio, bool) {
	for _, c := range m.Condominios {
		if companyID != "" && c.CompanyID == companyID {
			return c, true
		}
	}
	for _, c := range m.Condominios {
		if tenant != "" && c.Tenant == tenant {
			return c, true
		}
	}
	for _, c := range m.Condominios {
		if account != "" && c.Account == account {
			return c, true
		}
	}
	return Condominio{}, false
}

// Habilitado diz se o caso de uso vale no condomínio para o papel: a entrada do
// condomínio vence a do padrão; sem entrada, está habilitado.
func (m Mapeamento) Habilitado(condoID, caso, papel string) bool {
	h, ok := m.Habilitacoes[condoID][caso]
	if !ok {
		h, ok = m.Habilitacoes[Padrao][caso]
	}
	if !ok {
		return true
	}
	if !h.Habilitado {
		return false
	}
	if len(h.Papeis) == 0 || papel == "" {
		return true
	}
	for _, p := range h.Papeis {
		if strings.EqualFold(p, papel) {
			return true
		}
	}
	return false
}

// HabilitacaoEfetiva devolve a entrada que vale (condomínio, senão padrão, senão o
// implícito "habilitado para todos") e de onde veio.
func (m Mapeamento) HabilitacaoEfetiva(condoID, caso string) (Habilitacao, string) {
	if h, ok := m.Habilitacoes[condoID][caso]; ok {
		return h, condoID
	}
	if h, ok := m.Habilitacoes[Padrao][caso]; ok {
		return h, Padrao
	}
	return Habilitacao{Habilitado: true, Papeis: []string{}}, ""
}

// PayloadsDe remonta os payloads do condomínio: o padrão sobreposto, chave a chave,
// pelo do condomínio.
func (m Mapeamento) PayloadsDe(condoID string) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, origem := range []string{Padrao, condoID} {
		for recurso, p := range m.Payloads[origem] {
			if out[recurso] == nil {
				out[recurso] = map[string]any{}
			}
			for k, v := range p {
				out[recurso][k] = v
			}
		}
	}
	return out
}

// Contexto formata os payloads para o prompt ("# Contexto do condomínio").
func Contexto(c Condominio, payloads map[string]map[string]any) string {
	if len(payloads) == 0 && c.Nome == "" {
		return ""
	}
	var b strings.Builder
	nome := c.Nome
	if nome == "" {
		nome = "padrão"
	}
	fmt.Fprintf(&b, "# Contexto do condomínio (%s)\n\nDefinições deste condomínio para os recursos; valem como parâmetros padrão e como regras locais. Use-as sem perguntar ao morador.\n", nome)
	recursos := make([]string, 0, len(payloads))
	for r := range payloads {
		recursos = append(recursos, r)
	}
	sort.Strings(recursos)
	for _, r := range recursos {
		raw, _ := json.Marshal(payloads[r])
		fmt.Fprintf(&b, "- `%s`: %s\n", r, raw)
	}
	return strings.TrimRight(b.String(), "\n")
}

// Store guarda o mapeamento em JSON.
type Store struct {
	Path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{Path: path} }

// Ler devolve o mapeamento (vazio e normalizado se o arquivo não existe).
func (s *Store) Ler() (Mapeamento, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ler()
}

func (s *Store) ler() (Mapeamento, error) {
	var m Mapeamento
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			m.normalizar()
			return m, nil
		}
		return m, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("admin: %s: %w", s.Path, err)
	}
	m.normalizar()
	return m, nil
}

// Gravar persiste o mapeamento inteiro.
func (s *Store) Gravar(m Mapeamento) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gravar(m)
}

func (s *Store) gravar(m Mapeamento) error {
	m.normalizar()
	m.AtualizadoEm = time.Now()
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(s.Path, raw, 0o644)
}

// Alterar lê, aplica a função e grava, sob o mesmo lock.
func (s *Store) Alterar(fn func(m *Mapeamento) error) (Mapeamento, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.ler()
	if err != nil {
		return m, err
	}
	if err := fn(&m); err != nil {
		return m, err
	}
	return m, s.gravar(m)
}

var reID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// SalvarCondominio cria ou atualiza (pelo id).
func (s *Store) SalvarCondominio(c Condominio) (Mapeamento, error) {
	c.ID = strings.TrimSpace(strings.ToLower(c.ID))
	c.Nome = strings.TrimSpace(c.Nome)
	if c.ID == "" || !reID.MatchString(c.ID) {
		return Mapeamento{}, errors.New("id do condomínio inválido (minúsculas, dígitos, hífen)")
	}
	if c.Nome == "" {
		return Mapeamento{}, errors.New("nome do condomínio vazio")
	}
	return s.Alterar(func(m *Mapeamento) error {
		for i, atual := range m.Condominios {
			if atual.ID == c.ID {
				m.Condominios[i] = c
				return nil
			}
		}
		m.Condominios = append(m.Condominios, c)
		return nil
	})
}

// RemoverCondominio apaga o condomínio e tudo que era dele.
func (s *Store) RemoverCondominio(id string) (Mapeamento, error) {
	return s.Alterar(func(m *Mapeamento) error {
		var resto []Condominio
		for _, c := range m.Condominios {
			if c.ID != id {
				resto = append(resto, c)
			}
		}
		m.Condominios = resto
		delete(m.Habilitacoes, id)
		delete(m.Payloads, id)
		return nil
	})
}

// SalvarHabilitacao define (ou limpa, com limpar=true) a entrada condomínio × caso.
func (s *Store) SalvarHabilitacao(condoID, caso string, h Habilitacao, limpar bool) (Mapeamento, error) {
	if caso == "" {
		return Mapeamento{}, errors.New("caso de uso vazio")
	}
	return s.Alterar(func(m *Mapeamento) error {
		if condoID != Padrao {
			if _, ok := m.Condominio(condoID); !ok {
				return fmt.Errorf("condomínio desconhecido: %s", condoID)
			}
		}
		if limpar {
			delete(m.Habilitacoes[condoID], caso)
			return nil
		}
		if m.Habilitacoes[condoID] == nil {
			m.Habilitacoes[condoID] = map[string]Habilitacao{}
		}
		if h.Papeis == nil {
			h.Papeis = []string{}
		}
		m.Habilitacoes[condoID][caso] = h
		return nil
	})
}

// SalvarPayload define (ou apaga, payload nil) o payload condomínio × recurso.
func (s *Store) SalvarPayload(condoID, recurso string, payload map[string]any) (Mapeamento, error) {
	if recurso == "" {
		return Mapeamento{}, errors.New("recurso vazio")
	}
	return s.Alterar(func(m *Mapeamento) error {
		if condoID != Padrao {
			if _, ok := m.Condominio(condoID); !ok {
				return fmt.Errorf("condomínio desconhecido: %s", condoID)
			}
		}
		if payload == nil {
			delete(m.Payloads[condoID], recurso)
			return nil
		}
		if m.Payloads[condoID] == nil {
			m.Payloads[condoID] = map[string]map[string]any{}
		}
		m.Payloads[condoID][recurso] = payload
		return nil
	})
}
