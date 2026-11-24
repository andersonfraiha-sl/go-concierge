package canaiscore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go-concierge/internal/entrada"
)

func TestHTTPSenderPropagaEscopoAoConsole(t *testing.T) {
	channel := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("company_id") != "empresa-a" || q.Get("channel_id") != channel.String() || q.Get("origem") != "teste" {
			t.Errorf("escopo de destino perdido: %s", r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer exemplo" {
			t.Error("autenticação perdida")
		}
		var corpo struct {
			Protocolo string `json:"protocol"`
			Sessao    string `json:"session_id"`
			Texto     string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
			t.Error(err)
			return
		}
		if corpo.Protocolo != "123" || corpo.Sessao != "sessao-a" || corpo.Texto != "resposta" {
			t.Errorf("corpo alterado: %+v", corpo)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	sender := &HTTPSender{URL: server.URL + "/emulador/atende/messages?origem=teste", Token: "exemplo"}
	err := sender.Enviar(t.Context(), entrada.Destino{CompanyID: "empresa-a", ChannelID: channel, SessaoID: "sessao-a", Protocolo: "123"}, "resposta")
	if err != nil {
		t.Fatal(err)
	}
}
