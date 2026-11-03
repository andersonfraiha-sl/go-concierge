package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/atende"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/mcp"
	"go-concierge/internal/okf"
)

type tokenIdentidadeFake struct{}

func TestWebhookAtendeProcessaMensagemNormalizada(t *testing.T) {
	cat, err := okf.NewCatalogo(filepath.Join("..", "..", "okf"))
	if err != nil {
		t.Fatal(err)
	}
	fix, err := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cat.Recursos = func() []okf.Recurso { return fix }
	out := &atende.Outbox{}
	s := &servidor{
		atende: &atende.Servico{
			Cat: cat, Agente: "agentes/concierge", Account: "cond-001",
			Sessoes: atende.NewSessoes(), Sender: out,
			Config: harness.Config{Mock: true, Env: func(string) string { return "" }},
		},
		log: log.New(io.Discard, "", 0),
	}
	corpo := `{"event":"com.echo.atende.user.protocol.created","id":"evt-1","tenant":"tenant-1","data":{"protocol":{"number":"protocolo-1"},"contact":{"phone":"5519981621916","name":"Morador"},"channel":{"type":"whatsapp","wabaId":"waba-1","displayNumber":"551940402710"},"message":{"id":"msg-1","type":"text","content":"posso fazer barulho depois das 22h?"}}}`
	w := httptest.NewRecorder()
	s.webhookEmulado(w, httptest.NewRequest("POST", "/emulador/atende/webhook", strings.NewReader(corpo)))
	if w.Code != 202 {
		t.Fatalf("status HTTP %d: %s", w.Code, w.Body.String())
	}
	var resposta struct {
		Accepted bool   `json:"accepted"`
		Event    string `json:"event"`
		Protocol string `json:"protocol"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resposta); err != nil {
		t.Fatal(err)
	}
	if !resposta.Accepted || resposta.Event != "evt-1" || resposta.Protocol != "protocolo-1" {
		t.Fatalf("resposta do webhook: %+v", resposta)
	}
	deadline := time.After(5 * time.Second)
	for {
		if saidas := out.Itens("protocolo-1"); len(saidas) > 0 {
			if saidas[0].Texto == "" {
				t.Fatal("resposta vazia")
			}
			return
		}
		select {
		case <-deadline:
			t.Fatal("processamento não produziu resposta")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func corpoEventoTeste(evento, tenant, canal string) string {
	corpo, _ := json.Marshal(map[string]any{
		"event":  evento,
		"id":     "evt-teste",
		"tenant": tenant,
		"data": map[string]any{
			"protocol": map[string]any{"number": "protocolo-teste", "isHumanAttendance": false},
			"contact":  map[string]any{"phone": "", "name": "Morador"},
			"channel":  map[string]any{"type": "whatsapp", "displayNumber": canal},
			"message":  map[string]any{"id": "msg-teste", "type": "text", "content": "mensagem de teste"},
		},
	})
	return string(corpo)
}

const segredoTeste = "segredo-hmac-teste"

func assinaturaTeste(corpo string) string { return assinaturaComSegredo(segredoTeste, corpo) }

func assinaturaComSegredo(segredo, corpo string) string {
	mac := hmac.New(sha256.New, []byte(segredo))
	mac.Write([]byte("1." + corpo))
	return "t=1,v1=" + hex.EncodeToString(mac.Sum(nil))
}

func requisicaoAssinadaTeste(corpo string) *http.Request {
	r := httptest.NewRequest("POST", "/webhook/atende", strings.NewReader(corpo))
	r.Header.Set("X-Atende-Signature", assinaturaTeste(corpo))
	return r
}

func servidorWebhookRealTeste() *servidor {
	return &servidor{
		atende:       &atende.Servico{Sessoes: atende.NewSessoes()},
		atendeTenant: "tenant-1", atendeTelefoneCanal: "+55 (19) 4040-2710",
		atendeBaseURL: "https://atende.invalid/v2", atendeAPIKey: "nao-exibida",
		atendeAPISecret: segredoTeste,
	}
}

func TestWebhookRealExigeAssinaturaHMAC(t *testing.T) {
	corpo := corpoEventoTeste(atende.EventoMensagemAtendente, "tenant-1", "551940402710")
	casos := []struct {
		nome, assinatura string
		status           int
	}{
		{"sem cabecalho", "", 401},
		{"assinatura incorreta", "t=1,v1=assinatura-incorreta", 401},
		{"assinatura valida", assinaturaTeste(corpo), 202},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			s := servidorWebhookRealTeste()
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/webhook/atende", strings.NewReader(corpo))
			if c.assinatura != "" {
				r.Header.Set("X-Atende-Signature", c.assinatura)
			}
			s.webhookAtende(w, r)
			if w.Code != c.status || len(s.atende.Sessoes.Lista()) != 0 {
				t.Fatalf("status=%d (esperado %d) sessoes=%d body=%s", w.Code, c.status, len(s.atende.Sessoes.Lista()), w.Body.String())
			}
		})
	}
}

func TestWebhookRealSemSegredoConfiguradoRecusa(t *testing.T) {
	s := servidorWebhookRealTeste()
	s.atendeAPISecret = ""
	corpo := corpoEventoTeste(atende.EventoMensagemAtendente, "tenant-1", "551940402710")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/webhook/atende", strings.NewReader(corpo))
	r.Header.Set("X-Atende-Signature", assinaturaComSegredo("", corpo))
	s.webhookAtende(w, r)
	if w.Code != 401 {
		t.Fatalf("sem segredo deveria recusar: status=%d", w.Code)
	}
}

func TestWebhookEmuladoAceitaSomenteTokenDoWebhook(t *testing.T) {
	corpo := corpoEventoTeste(atende.EventoMensagemAtendente, "tenant-1", "551940402710")
	casos := []struct {
		nome, bearer, assinatura string
		status                   int
	}{
		{"token correto", "token-webhook", "", 202},
		{"token errado", "outro", "", 401},
		{"sem token", "", "", 401},
		{"hmac valido sem token", "", assinaturaTeste(corpo), 401},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			s := servidorWebhookRealTeste()
			s.webhookToken = "token-webhook"
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/emulador/atende/webhook", strings.NewReader(corpo))
			if c.bearer != "" {
				r.Header.Set("Authorization", "Bearer "+c.bearer)
			}
			if c.assinatura != "" {
				r.Header.Set("X-Atende-Signature", c.assinatura)
			}
			s.webhookEmulado(w, r)
			if w.Code != c.status {
				t.Fatalf("status=%d (esperado %d) body=%s", w.Code, c.status, w.Body.String())
			}
		})
	}
}

func TestWebhookIgnoraEventoDesconhecidoERecusaConexaoDivergente(t *testing.T) {
	s := &servidor{atende: &atende.Servico{Sessoes: atende.NewSessoes()}, atendeTenant: "tenant-1", atendeTelefoneCanal: "551940402710", atendeBaseURL: "https://atende.invalid/v2", atendeAPIKey: "key", atendeAPISecret: segredoTeste}
	w := httptest.NewRecorder()
	s.webhookAtende(w, requisicaoAssinadaTeste(corpoEventoTeste("com.echo.atende.evento.desconhecido", "outro", "1")))
	if w.Code != 202 || len(s.atende.Sessoes.Lista()) != 0 {
		t.Fatalf("evento desconhecido deveria ser ignorado: status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.webhookAtende(w, requisicaoAssinadaTeste(corpoEventoTeste(atende.EventoProtocoloFechadoSistema, "outro", "551940402710")))
	if w.Code != 403 || len(s.atende.Sessoes.Lista()) != 0 {
		t.Fatalf("conexão divergente deveria ser recusada sem criar sessão: status=%d", w.Code)
	}
}

func TestWebhookEmuladoIgnoraEventoHumanoDesconhecido(t *testing.T) {
	s := &servidor{atende: &atende.Servico{Sessoes: atende.NewSessoes()}}
	w := httptest.NewRecorder()
	s.webhookEmulado(w, httptest.NewRequest("POST", "/emulador/atende/webhook", strings.NewReader(corpoEventoTeste(atende.EventoMensagemAtendente, "tenant-1", "551940402710"))))
	if w.Code != 202 || len(s.atende.Sessoes.Lista()) != 0 {
		t.Fatalf("evento humano desconhecido criou sessão: status=%d", w.Code)
	}
}

func (tokenIdentidadeFake) Token(context.Context) (string, error) { return "token", nil }

type mcpIdentidadeFake struct {
	chamadas int
	args     map[string]any
}

func (f *mcpIdentidadeFake) ValidarToken(context.Context) error { return nil }

func (f *mcpIdentidadeFake) CallTool(_ context.Context, nome string, args map[string]any) (*mcp.CallResult, error) {
	f.chamadas++
	f.args = args
	if nome != "gruvi.bff.building-operation.get-company-by-document-controller-handle" {
		return nil, context.Canceled
	}
	return &mcp.CallResult{StructuredContent: json.RawMessage(`{"data":{"name":"Residencial Exemplo","condominium":{"name":"Cond A"},"units":[{"id":"uuid-109","id_fastpass":"fp-109","unit":"109","block":"Jataí","resident":{"name":"Vanderlei Martins","mobile_number":"5519981621916"}}]}}`)}, nil
}

func TestBotaoIdentidadeDevolveContratoDaConsole(t *testing.T) {
	fake := &mcpIdentidadeFake{}
	s := &servidor{
		gruvi:   gruvi.New("", "5982", "5519981621916", tokenIdentidadeFake{}),
		gruviOK: true,
		atende:  &atende.Servico{Config: harness.Config{MCP: fake}},
	}
	r := httptest.NewRequest("GET", "/api/atende/identidade?phone=19+98162-1916", nil)
	w := httptest.NewRecorder()
	s.atendeIdentidade(w, r)
	if w.Code != 200 {
		t.Fatalf("status HTTP %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Configurado bool             `json:"configurado"`
		Erro        string           `json:"erro"`
		Identidade  gruvi.Identidade `json:"identidade"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	id := body.Identidade
	if !body.Configurado || body.Erro != "" || !id.Authenticated || id.Phone != "5519981621916" || id.CompanyID != "5982" || len(id.Units) != 1 || len(id.Condominios) != 1 {
		t.Fatalf("resposta para a console: %+v", body)
	}
	if id.Units[0].ID != "fp-109" || id.Units[0].IDInterno != "uuid-109" || id.Units[0].Label != "Jataí - 109" {
		t.Fatalf("unidade deve expor o ID operacional separado do interno: %+v", id.Units[0])
	}
	if fake.chamadas != 1 || fake.args["companyId"] != "5982" || fake.args["filter_mobile_number"] != "5519981621916" {
		t.Fatalf("chamada MCP filtrada pelo telefone B2C: %+v", fake)
	}
}
