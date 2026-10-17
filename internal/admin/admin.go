// Package admin é o mapeamento das administradoras: cada administradora tem seus
// condomínios e, para cada condomínio, quais casos de uso o concierge pode usar (e para
// quais papéis) e qual payload de definições vale para cada recurso do Nexus. É a
// "matriz" de um banco com RLS por administradora e condomínio; hoje em JSON
// (okf/var/admin/mapeamento.json), com a mesma forma que o Postgres terá.
//
// Quem mexe em quê: a Superlógica (Backdoor) cadastra as administradoras e distribui
// os condomínios entre elas; a administradora (Admin) só enxerga os seus condomínios
// (Visao) e neles habilita casos de uso. O padrão "*" é global.
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

// Administradora é quem administra um conjunto de condomínios (a cliente da
// Superlógica). O Admin é dela: ela só vê o que é seu.
type Administradora struct {
	ID   string `json:"id"`
	Nome string `json:"nome"`
}

// Condominio é um condomínio de uma administradora, com os ids que o ligam aos sistemas.
type Condominio struct {
	ID             string `json:"id"`
	Nome           string `json:"nome"`
	Administradora string `json:"administradora_id"` // id da administradora (vazio = sem dona, só o Backdoor vê)
	Account        string `json:"account"`           // account dos recursos no Nexus
	CompanyID      string `json:"company_id"`        // company no Gruvi (identidade por telefone)
	Tenant         string `json:"tenant"`            // tenant do ATENDE
}

// Orientacao é o que a administradora diz ao concierge sobre um condomínio, em texto
// livre: o geral entra no prompt da família inteira ("Contexto do condomínio"); o
// texto por caso de uso entra só na instrução daquele subagente.
type Orientacao struct {
	Geral   string            `json:"geral"`
	PorCaso map[string]string `json:"por_caso"`
}

// Habilitacao de um caso de uso num condomínio.
type Habilitacao struct {
	Habilitado bool     `json:"habilitado"`
	Papeis     []string `json:"papeis"` // vazio = todos os papéis
}

// Mapeamento é o conteúdo inteiro (todas as administradoras) ou, depois de Visao, o
// recorte de uma administradora.
type Mapeamento struct {
	Administradoras []Administradora `json:"administradoras"`
	Condominios     []Condominio     `json:"condominios"`
	// Legado: o nome da única administradora, de antes de haver a lista. normalizar migra.
	AdministradoraLegado string `json:"administradora,omitempty"`
	// Habilitacoes: condomínio (ou "*") → caso de uso → habilitação.
	Habilitacoes map[string]map[string]Habilitacao `json:"habilitacoes"`
	// Payloads: condomínio (ou "*") → recurso do Nexus → definições (os Ajustes do
	// Admin gravam aqui, pelas definições que o recurso declara; o Backdoor vê o JSON).
	Payloads map[string]map[string]map[string]any `json:"payloads"`
	// Orientacoes: condomínio (ou "*") → texto livre para o concierge.
	Orientacoes  map[string]Orientacao `json:"orientacoes"`
	AtualizadoEm time.Time             `json:"atualizado_em"`
}

func (m *Mapeamento) normalizar() {
	if m.Administradoras == nil {
		m.Administradoras = []Administradora{}
	}
	if m.Condominios == nil {
		m.Condominios = []Condominio{}
	}
	if m.AdministradoraLegado != "" && len(m.Administradoras) == 0 {
		// migração: a administradora única vira a primeira da lista, dona de todos
		a := Administradora{ID: "adm-1", Nome: m.AdministradoraLegado}
		m.Administradoras = append(m.Administradoras, a)
		for i := range m.Condominios {
			if m.Condominios[i].Administradora == "" {
				m.Condominios[i].Administradora = a.ID
			}
		}
		m.AdministradoraLegado = ""
	}
	sort.Slice(m.Administradoras, func(i, j int) bool { return m.Administradoras[i].Nome < m.Administradoras[j].Nome })
	if m.Habilitacoes == nil {
		m.Habilitacoes = map[string]map[string]Habilitacao{}
	}
	if m.Payloads == nil {
		m.Payloads = map[string]map[string]map[string]any{}
	}
	if m.Orientacoes == nil {
		m.Orientacoes = map[string]Orientacao{}
	}
	for k, o := range m.Orientacoes {
		if o.PorCaso == nil {
			o.PorCaso = map[string]string{}
			m.Orientacoes[k] = o
		}
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

// Administradora procura pelo id.
func (m Mapeamento) Administradora(id string) (Administradora, bool) {
	for _, a := range m.Administradoras {
		if a.ID == id {
			return a, true
		}
	}
	return Administradora{}, false
}

// Visao é o RLS da administradora: só ela, seus condomínios e o mapeamento deles
// (mais o padrão "*", que é global). Com id vazio devolve tudo (o Backdoor).
func (m Mapeamento) Visao(admID string) Mapeamento {
	if admID == "" {
		return m
	}
	v := Mapeamento{Administradoras: []Administradora{}, Condominios: []Condominio{}, Habilitacoes: map[string]map[string]Habilitacao{}, Payloads: map[string]map[string]map[string]any{}, Orientacoes: map[string]Orientacao{}, AtualizadoEm: m.AtualizadoEm}
	if a, ok := m.Administradora(admID); ok {
		v.Administradoras = append(v.Administradoras, a)
	}
	meus := map[string]bool{Padrao: true}
	for _, c := range m.Condominios {
		if c.Administradora == admID {
			v.Condominios = append(v.Condominios, c)
			meus[c.ID] = true
		}
	}
	for k, h := range m.Habilitacoes {
		if meus[k] {
			v.Habilitacoes[k] = h
		}
	}
	for k, p := range m.Payloads {
		if meus[k] {
			v.Payloads[k] = p
		}
	}
	for k, o := range m.Orientacoes {
		if meus[k] {
			v.Orientacoes[k] = o
		}
	}
	return v
}

// OrientacaoDe junta o padrão e o do condomínio: o geral concatena (padrão primeiro);
// por caso, os dois textos também, quando existem.
func (m Mapeamento) OrientacaoDe(condoID string) Orientacao {
	out := Orientacao{PorCaso: map[string]string{}}
	for _, origem := range []string{Padrao, condoID} {
		o, ok := m.Orientacoes[origem]
		if !ok {
			continue
		}
		if g := strings.TrimSpace(o.Geral); g != "" {
			if out.Geral != "" {
				out.Geral += "\n\n"
			}
			out.Geral += g
		}
		for caso, t := range o.PorCaso {
			if t = strings.TrimSpace(t); t == "" {
				continue
			}
			if out.PorCaso[caso] != "" {
				out.PorCaso[caso] += "\n\n"
			}
			out.PorCaso[caso] += t
		}
	}
	return out
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
	return ContextoDoCondominio(c, payloads, "")
}

// ContextoDoCondominio: as orientações gerais da administradora e as definições por
// recurso, no bloco que entra no prompt da família inteira.
func ContextoDoCondominio(c Condominio, payloads map[string]map[string]any, orientacao string) string {
	orientacao = strings.TrimSpace(orientacao)
	if len(payloads) == 0 && c.Nome == "" && orientacao == "" {
		return ""
	}
	var b strings.Builder
	nome := c.Nome
	if nome == "" {
		nome = "padrão"
	}
	fmt.Fprintf(&b, "# Contexto do condomínio (%s)\n\n", nome)
	if orientacao != "" {
		fmt.Fprintf(&b, "## Orientações da administradora\n\n%s\n\n", orientacao)
	}
	if len(payloads) == 0 {
		return strings.TrimRight(b.String(), "\n")
	}
	b.WriteString("## Definições\n\nDefinições deste condomínio para os recursos; valem como parâmetros padrão e como regras locais. Use-as sem perguntar ao morador.\n")
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

// SalvarAdministradora cria ou atualiza (pelo id).
func (s *Store) SalvarAdministradora(a Administradora) (Mapeamento, error) {
	a.ID = strings.TrimSpace(strings.ToLower(a.ID))
	a.Nome = strings.TrimSpace(a.Nome)
	if a.ID == "" || !reID.MatchString(a.ID) {
		return Mapeamento{}, errors.New("id da administradora inválido (minúsculas, dígitos, hífen)")
	}
	if a.Nome == "" {
		return Mapeamento{}, errors.New("nome da administradora vazio")
	}
	return s.Alterar(func(m *Mapeamento) error {
		for i, atual := range m.Administradoras {
			if atual.ID == a.ID {
				m.Administradoras[i] = a
				return nil
			}
		}
		m.Administradoras = append(m.Administradoras, a)
		return nil
	})
}

// RemoverAdministradora apaga a administradora; recusa enquanto ela tiver condomínios
// (mova-os antes, para não perder mapeamento sem querer).
func (s *Store) RemoverAdministradora(id string) (Mapeamento, error) {
	return s.Alterar(func(m *Mapeamento) error {
		for _, c := range m.Condominios {
			if c.Administradora == id {
				return fmt.Errorf("a administradora %s ainda tem o condomínio %s; mova-o antes", id, c.ID)
			}
		}
		var resto []Administradora
		for _, a := range m.Administradoras {
			if a.ID != id {
				resto = append(resto, a)
			}
		}
		m.Administradoras = resto
		return nil
	})
}

// SalvarCondominio cria ou atualiza (pelo id). A administradora, se informada, tem de existir.
func (s *Store) SalvarCondominio(c Condominio) (Mapeamento, error) {
	c.ID = strings.TrimSpace(strings.ToLower(c.ID))
	c.Nome = strings.TrimSpace(c.Nome)
	c.Administradora = strings.TrimSpace(c.Administradora)
	if c.ID == "" || !reID.MatchString(c.ID) {
		return Mapeamento{}, errors.New("id do condomínio inválido (minúsculas, dígitos, hífen)")
	}
	if c.Nome == "" {
		return Mapeamento{}, errors.New("nome do condomínio vazio")
	}
	return s.Alterar(func(m *Mapeamento) error {
		if c.Administradora != "" {
			if _, ok := m.Administradora(c.Administradora); !ok {
				return fmt.Errorf("administradora desconhecida: %s", c.Administradora)
			}
		}
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
		delete(m.Orientacoes, id)
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

// SalvarOrientacao grava o texto livre do condomínio (ou do padrão).
func (s *Store) SalvarOrientacao(condoID string, o Orientacao) (Mapeamento, error) {
	return s.Alterar(func(m *Mapeamento) error {
		if condoID != Padrao {
			if _, ok := m.Condominio(condoID); !ok {
				return fmt.Errorf("condomínio desconhecido: %s", condoID)
			}
		}
		o.Geral = strings.TrimSpace(o.Geral)
		porCaso := map[string]string{}
		for k, v := range o.PorCaso {
			if v = strings.TrimSpace(v); v != "" {
				porCaso[k] = v
			}
		}
		o.PorCaso = porCaso
		if o.Geral == "" && len(porCaso) == 0 {
			delete(m.Orientacoes, condoID)
			return nil
		}
		m.Orientacoes[condoID] = o
		return nil
	})
}

// SalvarAjustes grava, chave a chave, os ajustes de vários recursos de um condomínio:
// valor vazio ou nulo apaga a chave; um recurso que fica sem chaves sai do payload.
// Chaves que o formulário não conhece (postas pelo Backdoor) ficam como estão.
func (s *Store) SalvarAjustes(condoID string, ajustes map[string]map[string]any) (Mapeamento, error) {
	return s.Alterar(func(m *Mapeamento) error {
		if condoID != Padrao {
			if _, ok := m.Condominio(condoID); !ok {
				return fmt.Errorf("condomínio desconhecido: %s", condoID)
			}
		}
		if m.Payloads[condoID] == nil {
			m.Payloads[condoID] = map[string]map[string]any{}
		}
		for recurso, campos := range ajustes {
			p := m.Payloads[condoID][recurso]
			if p == nil {
				p = map[string]any{}
			}
			for k, v := range campos {
				if v == nil || fmt.Sprint(v) == "" {
					delete(p, k)
				} else {
					p[k] = v
				}
			}
			if len(p) == 0 {
				delete(m.Payloads[condoID], recurso)
			} else {
				m.Payloads[condoID][recurso] = p
			}
		}
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
