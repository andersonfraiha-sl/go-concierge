package atende

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
)

type RegistroSessao struct {
	ID, CompanyID, Connector, Protocolo, UserPhone string
	ChannelID                                      uuid.UUID
	CondoID                                        *int
	AdministradoraID, CondominioID                 string
	AtualizadaEm                                   time.Time
	Dados                                          []byte
}

type SessaoStore interface {
	Salvar(context.Context, RegistroSessao) error
	Carregar(context.Context, string) (RegistroSessao, bool, error)
	Listar(context.Context, string, int) ([]RegistroSessao, error)
	Remover(context.Context, string) error
}

type chaveEscopoSessao struct{}
type escopoSessao struct {
	companyID string
	channelID uuid.UUID
}

var ErrEscopoSessao = errors.New("sessão fora do escopo autorizado")
var ErrRegistroSessao = errors.New("colunas e JSON da sessão incompatíveis")

func ComEscopoSessao(ctx context.Context, companyID string, channelID uuid.UUID) context.Context {
	return context.WithValue(ctx, chaveEscopoSessao{}, escopoSessao{strings.TrimSpace(companyID), channelID})
}

func ValidarEscopoSessao(ctx context.Context, r RegistroSessao) error {
	e, ok := ctx.Value(chaveEscopoSessao{}).(escopoSessao)
	if !ok || e.companyID == "" || r.CompanyID != e.companyID || e.channelID != uuid.Nil && e.channelID != r.ChannelID {
		return ErrEscopoSessao
	}
	return nil
}

func ValidarRegistroSessao(r RegistroSessao) error {
	if !escopo.CondominioValido(r.CondoID) || r.ID == "" || strings.TrimSpace(r.CompanyID) == "" || r.ChannelID == uuid.Nil || r.Connector == "" || r.Protocolo == "" || r.UserPhone == "" || r.UserPhone != escopo.NormalizarTelefone(r.UserPhone) {
		return ErrRegistroSessao
	}
	var j struct {
		ID           string          `json:"id"`
		CompanyID    string          `json:"company_id"`
		ChannelID    uuid.UUID       `json:"channel_id"`
		CondoID      *int            `json:"condo_id"`
		Protocolo    string          `json:"protocolo"`
		Contato      entrada.Contato `json:"contato"`
		Origem       entrada.Origem  `json:"origem"`
		Escopo       escopo.Contato  `json:"escopo"`
		AtualizadaEm time.Time       `json:"atualizada_em"`
	}
	if err := json.Unmarshal(r.Dados, &j); err != nil {
		return errors.Join(ErrRegistroSessao, err)
	}
	if !j.AtualizadaEm.Truncate(time.Microsecond).Equal(r.AtualizadaEm.Truncate(time.Microsecond)) || j.ID != r.ID || j.CompanyID != r.CompanyID || j.ChannelID != r.ChannelID || !escopo.MesmoCondominio(j.CondoID, r.CondoID) || j.Protocolo != r.Protocolo || j.Contato.Phone != r.UserPhone || j.Origem.Conector != r.Connector {
		return ErrRegistroSessao
	}
	if j.Escopo.CompanyID != r.CompanyID || j.Escopo.Telefone != r.UserPhone || !escopo.MesmoCondominio(j.Escopo.CondoID, r.CondoID) {
		return ErrRegistroSessao
	}
	return nil
}

type SessaoEmMemoria struct {
	mu        sync.Mutex
	registros map[string]RegistroSessao
}

func NovaSessaoEmMemoria() *SessaoEmMemoria {
	return &SessaoEmMemoria{registros: map[string]RegistroSessao{}}
}

func clonarRegistro(r RegistroSessao) RegistroSessao {
	r.Dados = slices.Clone(r.Dados)
	if r.CondoID != nil {
		n := *r.CondoID
		r.CondoID = &n
	}
	return r
}

func (m *SessaoEmMemoria) Salvar(ctx context.Context, r RegistroSessao) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := ctx.Value(chaveEscopoSessao{}).(escopoSessao); ok {
		if err := ValidarEscopoSessao(ctx, r); err != nil {
			return err
		}
	}
	if atual, existe := m.registros[r.ID]; existe && atual.CompanyID != "" && (atual.CompanyID != r.CompanyID || atual.ChannelID != r.ChannelID || atual.Connector != r.Connector || atual.Protocolo != r.Protocolo || atual.UserPhone != r.UserPhone) {
		return ErrEscopoSessao
	}
	m.registros[r.ID] = clonarRegistro(r)
	return nil
}

func (m *SessaoEmMemoria) Carregar(ctx context.Context, id string) (RegistroSessao, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.registros[id]
	if _, autorizado := ctx.Value(chaveEscopoSessao{}).(escopoSessao); autorizado && ok {
		if err := ValidarEscopoSessao(ctx, r); err != nil {
			return RegistroSessao{}, false, err
		}
	}
	return clonarRegistro(r), ok, nil
}

func (m *SessaoEmMemoria) Listar(ctx context.Context, companyID string, limite int) ([]RegistroSessao, error) {
	if e, ok := ctx.Value(chaveEscopoSessao{}).(escopoSessao); ok && (companyID == "" || e.companyID != companyID) {
		return nil, ErrEscopoSessao
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var regs []RegistroSessao
	for _, r := range m.registros {
		company := r.CompanyID
		if company == "" {
			company = r.AdministradoraID
		}
		if companyID != "" && company != companyID {
			continue
		}
		if _, ok := ctx.Value(chaveEscopoSessao{}).(escopoSessao); ok && ValidarEscopoSessao(ctx, r) != nil {
			continue
		}
		regs = append(regs, clonarRegistro(r))
	}
	slices.SortFunc(regs, func(a, b RegistroSessao) int { return b.AtualizadaEm.Compare(a.AtualizadaEm) })
	if limite < 0 {
		limite = 0
	}
	return regs[:min(limite, len(regs))], nil
}

func (m *SessaoEmMemoria) Remover(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.registros[id]; ok {
		if _, autorizado := ctx.Value(chaveEscopoSessao{}).(escopoSessao); autorizado {
			if err := ValidarEscopoSessao(ctx, r); err != nil {
				return err
			}
		}
	}
	delete(m.registros, id)
	return nil
}

func EscopoSessaoDoContexto(ctx context.Context) (string, uuid.UUID, bool) {
	e, ok := ctx.Value(chaveEscopoSessao{}).(escopoSessao)
	return e.companyID, e.channelID, ok && e.companyID != ""
}
