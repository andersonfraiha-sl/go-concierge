// Package atende é o canal do ATENDE (Superlógica): o ATENDE manda um evento por
// webhook a cada mensagem do morador (CloudEvents, `com.echo.atende.user.protocol.*`),
// o concierge identifica o contato pelo Gruvi, roda o agente de primeiro nível e
// responde ao protocolo. O emulador (Console) forja esses eventos e lê a caixa de
// saída, para testar sem o ATENDE no ar.
package atende

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"go-concierge/internal/entrada"
)

// Eventos conhecidos.
const (
	EventoProtocoloCriado = "com.echo.atende.user.protocol.created"
	EventoMensagem        = "com.echo.atende.user.message.received" // premissa: nome a confirmar com o ATENDE
)

// Evento é o envelope que o ATENDE envia ao webhook.
type Evento struct {
	Event       string    `json:"event"`
	ID          string    `json:"id"`
	Created     time.Time `json:"created"`
	SpecVersion string    `json:"specversion"`
	Tenant      string    `json:"tenant"`
	Data        Dados     `json:"data"`
}

type Dados struct {
	Protocol Protocolo `json:"protocol"`
	Contact  Contato   `json:"contact"`
	Channel  Canal     `json:"channel"`
	Creation Criacao   `json:"creation"`
	Message  Mensagem  `json:"message"`
}

type Protocolo struct {
	Number              string     `json:"number"`
	Status              string     `json:"status"`
	IsHumanAttendance   bool       `json:"isHumanAttendance"`
	IsReceptive         bool       `json:"isReceptive"`
	Attendant           any        `json:"attendant"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	ClosedAt            *time.Time `json:"closedAt"`
	AttendanceStartedAt *time.Time `json:"attendanceStartedAt"`
	Tag                 any        `json:"tag"`
	Tags                []any      `json:"tags"`
}

type Contato struct {
	Phone string  `json:"phone"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

type Canal struct {
	Type          string `json:"type"`
	WabaID        string `json:"wabaId"`
	PhoneNumberID string `json:"phoneNumberId"`
	DisplayNumber string `json:"displayNumber"`
}

type Criacao struct {
	CreatedBy         string `json:"createdBy"`
	Trigger           string `json:"trigger"`
	CreatedByUser     any    `json:"createdByUser"`
	ExternalReference any    `json:"externalReference"`
}

type Mensagem struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Direction string    `json:"direction"`
	Status    string    `json:"status"`
	SentAt    time.Time `json:"sentAt"`
	Metadata  any       `json:"metadata"`
	Error     any       `json:"error"`
	Content   string    `json:"content"`
}

// Parse lê e valida o mínimo do evento.
func Parse(raw []byte) (*Evento, error) {
	var ev Evento
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil, errors.New("evento inválido: " + err.Error())
	}
	if ev.Data.Protocol.Number == "" {
		return nil, errors.New("evento sem data.protocol.number")
	}
	if strings.TrimSpace(ev.Data.Contact.Phone) == "" {
		return nil, errors.New("evento sem data.contact.phone")
	}
	return &ev, nil
}

// Texto é o conteúdo que o concierge processa (só mensagens de texto por enquanto).
func (e *Evento) Texto() string {
	return strings.TrimSpace(e.Data.Message.Content)
}

func (e *Evento) Normalizar() entrada.Mensagem {
	return entrada.Mensagem{
		ID:         e.ID,
		ConversaID: e.Data.Protocol.Number,
		Tenant:     e.Tenant,
		Contato: entrada.Contato{
			Phone: e.Data.Contact.Phone,
			Name:  e.Data.Contact.Name,
			Email: e.Data.Contact.Email,
		},
		Canal: map[string]string{
			"type":          e.Data.Channel.Type,
			"wabaId":        e.Data.Channel.WabaID,
			"phoneNumberId": e.Data.Channel.PhoneNumberID,
			"displayNumber": e.Data.Channel.DisplayNumber,
		},
		Tipo:  e.Data.Message.Type,
		Texto: e.Data.Message.Content,
	}
}
