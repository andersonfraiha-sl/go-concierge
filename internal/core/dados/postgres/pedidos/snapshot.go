package pedidos

import (
	"time"
)

type pedidoSnapshot struct {
	SchemaVersion    int       `json:"schema_version"`
	Protocol         string    `json:"protocol"`
	CompanyID        string    `json:"company_id"`
	PhoneNumber      string    `json:"phone_number"`
	CondoID          *int      `json:"condo_id"`
	UseCaseID        string    `json:"use_case_id"`
	CompanyUseCaseID string    `json:"company_use_case_id,omitempty"`
	SourceSessionID  *string   `json:"source_session_id"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	State            Pedido    `json:"state"`
}

func snapshotDoPedido(p Pedido, source *string) pedidoSnapshot {
	return pedidoSnapshot{SchemaVersion: 2, Protocol: p.Protocolo, CompanyID: p.CompanyID, PhoneNumber: p.Telefone, CondoID: p.CondoID, UseCaseID: p.Caso, CompanyUseCaseID: p.CompanyUseCaseID, SourceSessionID: source, Status: statusSQL(p.Estado), CreatedAt: p.CriadoEm.Truncate(time.Microsecond), UpdatedAt: p.AtualizadoEm.Truncate(time.Microsecond), State: p}
}
