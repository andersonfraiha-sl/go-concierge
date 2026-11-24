package sessoes

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/escopo"
)

type sessaoSnapshot struct {
	SchemaVersion int             `json:"schema_version"`
	ID            string          `json:"id"`
	CompanyID     string          `json:"company_id"`
	ChannelID     uuid.UUID       `json:"channel_id"`
	Connector     string          `json:"connector"`
	Protocol      string          `json:"protocol"`
	PhoneNumber   string          `json:"phone_number"`
	CondoID       *int            `json:"condo_id"`
	UpdatedAt     time.Time       `json:"updated_at"`
	Scope         contatoSnapshot `json:"scope"`
	State         json.RawMessage `json:"state"`
}

type contatoSnapshot struct {
	CompanyID   string `json:"company_id"`
	PhoneNumber string `json:"phone_number"`
	CondoID     *int   `json:"condo_id"`
}

func serializarSessaoSnapshot(r RegistroSessao) ([]byte, error) {
	if err := ValidarRegistroSessao(r); err != nil {
		return nil, err
	}
	var state struct {
		Escopo escopo.Contato `json:"escopo"`
	}
	if err := json.Unmarshal(r.Dados, &state); err != nil {
		return nil, err
	}
	return json.Marshal(sessaoSnapshot{SchemaVersion: 2, ID: r.ID, CompanyID: r.CompanyID, ChannelID: r.ChannelID, Connector: r.Connector, Protocol: r.Protocolo, PhoneNumber: r.UserPhone, CondoID: r.CondoID, UpdatedAt: r.AtualizadaEm, Scope: contatoSnapshot{CompanyID: state.Escopo.CompanyID, PhoneNumber: state.Escopo.Telefone, CondoID: state.Escopo.CondoID}, State: r.Dados})
}

func restaurarSessaoSnapshot(r RegistroSessao) (RegistroSessao, error) {
	var snapshot sessaoSnapshot
	if err := json.Unmarshal(r.Dados, &snapshot); err != nil {
		return RegistroSessao{}, errors.Join(ErrRegistroSessao, err)
	}
	if snapshot.SchemaVersion != 2 || snapshot.ID != r.ID || snapshot.CompanyID != r.CompanyID || snapshot.ChannelID != r.ChannelID || snapshot.Connector != r.Connector || snapshot.Protocol != r.Protocolo || snapshot.PhoneNumber != r.UserPhone || !escopo.MesmoCondominio(snapshot.CondoID, r.CondoID) || !snapshot.UpdatedAt.Truncate(time.Microsecond).Equal(r.AtualizadaEm) {
		return RegistroSessao{}, ErrRegistroSessao
	}
	var state struct {
		Escopo escopo.Contato `json:"escopo"`
	}
	if err := json.Unmarshal(snapshot.State, &state); err != nil {
		return RegistroSessao{}, errors.Join(ErrRegistroSessao, err)
	}
	if snapshot.Scope.CompanyID != state.Escopo.CompanyID || snapshot.Scope.PhoneNumber != state.Escopo.Telefone || !escopo.MesmoCondominio(snapshot.Scope.CondoID, state.Escopo.CondoID) {
		return RegistroSessao{}, ErrRegistroSessao
	}
	r.Dados = snapshot.State
	if err := ValidarRegistroSessao(r); err != nil {
		return RegistroSessao{}, err
	}
	return r, nil
}
