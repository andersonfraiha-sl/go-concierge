package canais

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"slices"
	"strings"
)

var ErrDesconhecido = errors.New("canal não identificado")

var ErrInvalido = errors.New("configuração do canal inválida")

var ErrDesabilitado = errors.New("canal desabilitado")

type Identidade struct {
	Provider string
	Tipo     string
	Tenant   string
	Endpoint string
}

type Integracoes struct {
	GruviCondominiumID string `json:"gruvi_condominium_id"`
	GruviCompanyID     string `json:"gruvi_company_id"`
	NexusAccountID     string `json:"nexus_account_id"`
}

type Canal struct {
	ID            uuid.UUID
	CompanyID     string
	CondoID       *int
	Nome          string
	Provider      string
	Tipo          string
	Tenant        string
	Endpoint      string
	Modo          string
	Habilitado    bool
	Agente        string
	APIBaseURL    string
	APIKey        string `json:"-"`
	WebhookSecret string `json:"-"`
	Integracoes   Integracoes
}

type Caso struct {
	ID         string
	Habilitado bool
	Padrao     bool
	Papeis     []string
	Override   *bool
}

func (c Canal) Disponivel(caso Caso, papel string) bool {
	if !c.Habilitado || !caso.Habilitado {
		return false
	}
	if len(caso.Papeis) > 0 && !slices.ContainsFunc(caso.Papeis, func(p string) bool { return strings.EqualFold(p, papel) }) {
		return false
	}
	switch c.Modo {
	case "INHERIT":
		return caso.Padrao
	case "CUSTOM":
		if caso.Override != nil {
			return *caso.Override
		}
		return caso.Padrao
	case "EXPLICIT":
		return caso.Override != nil && *caso.Override
	default:
		return false
	}
}

type Repositorio interface {
	Resolver(context.Context, Identidade) (Canal, error)
	PorID(context.Context, string, uuid.UUID) (Canal, error)
	Casos(context.Context, Canal) ([]Caso, error)
}

type Resumo struct {
	ID         uuid.UUID `json:"id"`
	CompanyID  string    `json:"company_id"`
	CondoID    *int      `json:"condo_id"`
	Nome       string    `json:"nome"`
	Provider   string    `json:"provider"`
	Tipo       string    `json:"tipo"`
	Tenant     string    `json:"tenant"`
	Endpoint   string    `json:"endpoint"`
	Habilitado bool      `json:"habilitado"`
}
