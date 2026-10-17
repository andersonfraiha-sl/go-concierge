package atende

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/okf"
)

const eventoExemplo = `{
  "event": "com.echo.atende.user.protocol.created",
  "id": "evt_68fb1427-896b-44b3-99d5-a2f39565184d",
  "created": "2026-03-06T20:15:13.677Z",
  "specversion": "1.0",
  "tenant": "69a5a131e67bd8e2f54e5981",
  "data": {
    "protocol": {"number": "344200011995", "status": "open", "isHumanAttendance": false, "isReceptive": true, "attendant": null, "createdAt": "2026-04-09T12:49:10.534Z", "updatedAt": "2026-04-09T12:49:10.785Z", "closedAt": null, "attendanceStartedAt": null, "tag": null, "tags": []},
    "contact": {"phone": "5519981621916", "name": "Vanderlei Martins", "email": null},
    "channel": {"type": "whatsapp", "wabaId": "806679752450694", "phoneNumberId": "1049048231621427", "displayNumber": "551940402710"},
    "creation": {"createdBy": "user", "trigger": "incoming_message", "createdByUser": null, "externalReference": null},
    "message": {"id": "wamid.X", "type": "text", "direction": "incoming", "status": "received", "sentAt": "2026-03-06T20:15:12.862Z", "metadata": null, "error": null, "content": "posso fazer barulho depois das 22h?"}
  }
}`

func raizDoRepo(t *testing.T) string {
	_, arq, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(arq), "..", "..")
}

func TestParse(t *testing.T) {
	ev, err := Parse([]byte(eventoExemplo))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Event != EventoProtocoloCriado || ev.Data.Protocol.Number != "344200011995" || ev.Data.Contact.Phone != "5519981621916" || !strings.Contains(ev.Texto(), "barulho") {
		t.Fatalf("%+v", ev)
	}
	if _, err := Parse([]byte(`{"event":"x","data":{}}`)); err == nil {
		t.Fatal("sem protocolo deveria falhar")
	}
}

func TestProcessarComMockEOutbox(t *testing.T) {
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	fix, _ := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	cat.Recursos = func() []okf.Recurso { return fix }
	out := &Outbox{}
	svc := &Servico{
		Cat: cat, Agente: "agentes/concierge", Account: "cond-001", Sessoes: NewSessoes(), Sender: out,
		Config: harness.Config{Mock: true, Env: func(string) string { return "" }},
		Identificar: func(ctx context.Context, phone string) (gruvi.Identidade, error) {
			return gruvi.Identidade{Authenticated: true, Phone: phone, Name: "Vanderlei", Role: "morador", Units: []gruvi.Unidade{{ID: "fp-1", Label: "A - 101"}}}, nil
		},
	}
	ev, _ := Parse([]byte(eventoExemplo))
	sess, err := svc.Processar(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if !sess.Identidade.Authenticated || sess.Identidade.Name != "Vanderlei" {
		t.Fatalf("identidade: %+v", sess.Identidade)
	}
	if len(sess.Turnos) != 2 || sess.Turnos[0].Quem != "morador" || sess.Turnos[1].Quem != "concierge" || sess.Turnos[1].Texto == "" {
		t.Fatalf("turnos: %+v", sess.Turnos)
	}
	if sess.Turnos[1].Agente != "agentes/concierge/agentes/regimento" {
		t.Fatalf("deveria ter roteado para o regimento: %s", sess.Turnos[1].Agente)
	}
	itens := out.Itens("344200011995")
	if len(itens) != 1 || itens[0].Texto != sess.Turnos[1].Texto {
		t.Fatalf("caixa de saída: %+v", itens)
	}
	// segunda mensagem no mesmo protocolo mantém a sessão e o histórico
	ev2, _ := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "até que horas a piscina abre?", 1)))
	ev2.ID = "evt_2"
	sess2, err := svc.Processar(context.Background(), ev2)
	if err != nil {
		t.Fatal(err)
	}
	if sess2 != sess || len(sess.Turnos) != 4 || len(svc.Sessoes.Lista()) != 1 {
		t.Fatalf("sessão deveria continuar: %d turnos, %d sessões", len(sess.Turnos), len(svc.Sessoes.Lista()))
	}
	// identidade falha: segue como público, com o erro registrado
	svc2 := *svc
	svc2.Sessoes = NewSessoes()
	svc2.Identificar = func(ctx context.Context, phone string) (gruvi.Identidade, error) {
		return gruvi.Identidade{}, errors.New("bff fora")
	}
	s3, err := svc2.Processar(context.Background(), ev)
	if err != nil || s3.Identidade.Authenticated || s3.IdentErro != "bff fora" {
		t.Fatalf("%v %+v", err, s3.Identidade)
	}
}
