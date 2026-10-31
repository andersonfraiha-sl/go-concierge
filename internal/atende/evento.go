// Package atende é o canal do ATENDE (Superlógica): o ATENDE manda um evento por
// webhook a cada mensagem do morador (CloudEvents, `com.echo.atende.user.protocol.*`),
// o concierge identifica o contato pelo Gruvi, roda o agente de primeiro nível e
// responde ao protocolo. O emulador (Console) forja esses eventos e lê a caixa de
// saída, para testar sem o ATENDE no ar.
package atende

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go-concierge/internal/entrada"
)

// Eventos conhecidos.
const (
	EventoProtocoloCriado           = "com.echo.atende.user.protocol.created"
	EventoMensagem                  = "com.echo.atende.user.message"
	EventoMensagemLegada            = "com.echo.atende.user.message.received"
	EventoMensagemAtendente         = "com.echo.atende.attendant.message"
	EventoProtocoloFechadoAtendente = "com.echo.atende.attendant.protocol.closed"
	EventoProtocoloFechadoSistema   = "com.echo.atende.system.protocol.closed"
	EventoProtocoloFechadoBot       = "com.echo.atende.bot.protocol.closed"
)

type TipoEvento string

const (
	EventoIgnorado TipoEvento = "ignorado"
	EventoElegivel TipoEvento = "elegivel"
	EventoHumano   TipoEvento = "humano"
	EventoFechado  TipoEvento = "fechado"
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
	if ev.Event == "" {
		return nil, errors.New("evento sem event")
	}
	return &ev, nil
}

func (e *Evento) Classificar() TipoEvento {
	if e == nil {
		return EventoIgnorado
	}
	switch e.Event {
	case EventoProtocoloCriado, EventoMensagem, EventoMensagemLegada:
		if e.Data.Protocol.IsHumanAttendance {
			return EventoHumano
		}
		return EventoElegivel
	case EventoMensagemAtendente:
		return EventoHumano
	case EventoProtocoloFechadoAtendente, EventoProtocoloFechadoSistema, EventoProtocoloFechadoBot:
		return EventoFechado
	default:
		return EventoIgnorado
	}
}

func (e *Evento) Validar(tipo TipoEvento) error {
	if tipo == EventoIgnorado {
		return nil
	}
	if strings.TrimSpace(e.Data.Protocol.Number) == "" {
		return errors.New("evento sem data.protocol.number")
	}
	if strings.TrimSpace(e.Tenant) == "" {
		return errors.New("evento sem tenant")
	}
	if strings.TrimSpace(e.Data.Channel.DisplayNumber) == "" {
		return errors.New("evento sem data.channel.displayNumber")
	}
	if tipo == EventoElegivel {
		if strings.TrimSpace(e.ID) == "" {
			return errors.New("evento sem id")
		}
		if strings.TrimSpace(e.Data.Contact.Phone) == "" {
			return errors.New("evento sem data.contact.phone")
		}
		if strings.EqualFold(e.Data.Message.Type, "text") && strings.TrimSpace(e.Data.Message.Content) == "" {
			return errors.New("mensagem de texto sem conteúdo")
		}
	}
	return nil
}

func (e *Evento) ValidarConexao(tenant, telefone string) error {
	if tenant != "" && e.Tenant != tenant {
		return errors.New("tenant do evento não corresponde à conexão ATENDE")
	}
	if telefone != "" && normalizarCanal(e.Data.Channel.DisplayNumber) != normalizarCanal(telefone) {
		return errors.New("número do canal do evento não corresponde à conexão ATENDE")
	}
	return nil
}

func normalizarCanal(valor string) string {
	var b strings.Builder
	for _, r := range valor {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Texto é o conteúdo que o concierge processa (só mensagens de texto por enquanto).
func (e *Evento) Texto() string {
	return strings.TrimSpace(e.Data.Message.Content)
}

func (e *Evento) Normalizar() entrada.Mensagem {
	conexao := e.Tenant + ":" + normalizarCanal(e.Data.Channel.DisplayNumber)
	autor := "morador"
	if e.Event == EventoMensagemAtendente {
		autor = "atendente"
	}
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
		Origem:  entrada.Origem{Conector: "atende", Meio: e.Data.Channel.Type, ConexaoID: conexao},
		Destino: entrada.Destino{Conector: "atende", Meio: e.Data.Channel.Type, ConexaoID: conexao, Protocolo: e.Data.Protocol.Number, Telefone: e.Data.Contact.Phone, Canal: map[string]string{"displayNumber": e.Data.Channel.DisplayNumber, "phoneNumberId": e.Data.Channel.PhoneNumberID, "wabaId": e.Data.Channel.WabaID}},
		Tipo:    e.Data.Message.Type,
		Texto:   e.Data.Message.Content,
		Autor:   autor,
	}
}

func (e *Evento) String() string {
	return fmt.Sprintf("%s/%s", e.Event, e.Data.Protocol.Number)
}
