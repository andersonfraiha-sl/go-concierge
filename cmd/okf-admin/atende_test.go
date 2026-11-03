package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"go-concierge/internal/entrada"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/admin"
	"go-concierge/internal/atende"
	"go-concierge/internal/escopo"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/mcp"
	"go-concierge/internal/memoria"
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
	repo := mapeamentoTeste(t)
	s := &servidor{
		atende: &atende.Servico{
			Cat: cat, Agente: "agentes/concierge", Mapeamento: repo,
			Sessoes: atende.NewSessoes(atende.NovaSessaoEmMemoria()), Sender: out,
			Config: harness.Config{Mock: true, Env: func(string) string { return "" }},
		},
		admin: repo,
		log:   log.New(io.Discard, "", 0),
	}
	corpo := `{"event":"com.echo.atende.user.protocol.created","id":"evt-1","tenant":"tenant-1","data":{"protocol":{"number":"protocolo-1"},"contact":{"phone":"5519981621916","name":"Morador"},"channel":{"type":"whatsapp","wabaId":"waba-1","displayNumber":"551940402710"},"message":{"id":"msg-1","type":"text","content":"posso fazer barulho depois das 22h?"}}}`
	w := httptest.NewRecorder()
	// o evento vem com um tenant que nenhum condomínio tem: o seletor do Console decide
	s.webhookEmulado(w, httptest.NewRequest("POST", "/emulador/atende/webhook?condominio=cond-x", strings.NewReader(corpo)))
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
			if sess := listarSessoes(t, s.atende.Sessoes)[0]; sess.Condominio != "cond-x" || sess.AdministradoraID != "adm-x" {
				t.Fatalf("condomínio da conversa: %s/%s", sess.AdministradoraID, sess.Condominio)
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
		atende:       &atende.Servico{Sessoes: atende.NewSessoes(atende.NovaSessaoEmMemoria())},
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
			if w.Code != c.status || len(listarSessoes(t, s.atende.Sessoes)) != 0 {
				t.Fatalf("status=%d (esperado %d) sessoes=%d body=%s", w.Code, c.status, len(listarSessoes(t, s.atende.Sessoes)), w.Body.String())
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
	s := &servidor{atende: &atende.Servico{Sessoes: atende.NewSessoes(atende.NovaSessaoEmMemoria())}, atendeTenant: "tenant-1", atendeTelefoneCanal: "551940402710", atendeBaseURL: "https://atende.invalid/v2", atendeAPIKey: "key", atendeAPISecret: segredoTeste}
	w := httptest.NewRecorder()
	s.webhookAtende(w, requisicaoAssinadaTeste(corpoEventoTeste("com.echo.atende.evento.desconhecido", "outro", "1")))
	if w.Code != 202 || len(listarSessoes(t, s.atende.Sessoes)) != 0 {
		t.Fatalf("evento desconhecido deveria ser ignorado: status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.webhookAtende(w, requisicaoAssinadaTeste(corpoEventoTeste(atende.EventoProtocoloFechadoSistema, "outro", "551940402710")))
	if w.Code != 403 || len(listarSessoes(t, s.atende.Sessoes)) != 0 {
		t.Fatalf("conexão divergente deveria ser recusada sem criar sessão: status=%d", w.Code)
	}
}

// O token B2C é de um telefone só. Sem o mock (fora de dev) o morador de outro número é
// recusado; com o mock (dev) o evento passa como se viesse do telefone de teste.
func TestWebhookMockTelefoneB2C(t *testing.T) {
	tests := []struct {
		name     string
		mock     string
		wantCode int
	}{
		{"desligado fora de dev", "", http.StatusForbidden},
		{"ligado em dev", "5511900000000", http.StatusAccepted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &atende.Servico{
				Sessoes: atende.NewSessoes(atende.NovaSessaoEmMemoria()), Sender: &atende.Outbox{}, Mapeamento: mapeamentoTeste(t),
				TelefoneB2C: "5511900000000", Log: log.New(io.Discard, "", 0),
			}
			s := &servidor{atende: svc, log: log.New(io.Discard, "", 0), telefoneMockB2C: tt.mock,
				atendeTenant: "tenant-1", atendeTelefoneCanal: "551940402710", atendeBaseURL: "https://atende.invalid/v2", atendeAPIKey: "key", atendeAPISecret: segredoTeste}
			corpo := strings.Replace(corpoEventoTeste(atende.EventoProtocoloCriado, "tenant-1", "551940402710"), `"phone":""`, `"phone":"5519981621916"`, 1)
			w := httptest.NewRecorder()
			s.webhookAtende(w, requisicaoAssinadaTeste(corpo))
			if w.Code != tt.wantCode {
				t.Fatalf("status %d, want %d: %s", w.Code, tt.wantCode, w.Body.String())
			}
		})
	}
}

func TestWebhookEmuladoIgnoraEventoHumanoDesconhecido(t *testing.T) {
	s := &servidor{atende: &atende.Servico{Sessoes: atende.NewSessoes(atende.NovaSessaoEmMemoria())}}
	w := httptest.NewRecorder()
	s.webhookEmulado(w, httptest.NewRequest("POST", "/emulador/atende/webhook", strings.NewReader(corpoEventoTeste(atende.EventoMensagemAtendente, "tenant-1", "551940402710"))))
	if w.Code != 202 || len(listarSessoes(t, s.atende.Sessoes)) != 0 {
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

// mapeamentoTeste: a administradora adm-x, dona do condomínio cond-x (tenant-x, company 123).
func mapeamentoTeste(t *testing.T) *admin.EmMemoria {
	t.Helper()
	repo := admin.NovaEmMemoria()
	if _, err := repo.SalvarAdministradora(t.Context(), admin.Administradora{ID: "adm-x", Nome: "X"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SalvarCondominio(t.Context(), admin.Condominio{ID: "cond-x", Nome: "X", Administradora: "adm-x", Account: "cond-001", CompanyID: "123", Tenant: "tenant-x"}); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestWebhookEmuladoRecusaCondominioDesconhecido(t *testing.T) {
	s := &servidor{atende: &atende.Servico{Sessoes: atende.NewSessoes(atende.NovaSessaoEmMemoria())}, admin: mapeamentoTeste(t)}
	corpo := `{"event":"com.echo.atende.user.protocol.created","id":"evt-1","tenant":"tenant-1","data":{"protocol":{"number":"protocolo-1"},"contact":{"phone":"5519981621916","name":"Morador"},"channel":{"type":"whatsapp","wabaId":"waba-1","displayNumber":"551940402710"},"message":{"id":"msg-1","type":"text","content":"oi"}}}`
	w := httptest.NewRecorder()
	s.webhookEmulado(w, httptest.NewRequest("POST", "/emulador/atende/webhook?condominio=nao-existe", strings.NewReader(corpo)))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "CONDOMINIO_DESCONHECIDO") {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
}

func TestRotasDeMemoriaExigemAdministradora(t *testing.T) {
	st := memoria.NovoFake()
	c, err := escopo.NovoContato("adm-x", admin.Padrao, "5519981621916")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Gravar(t.Context(), c, memoria.Memoria{Fatos: []memoria.Fato{{Chave: "pets", Texto: "tem um cachorro"}}}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		implantacao escopo.Implantacao
		url         string
		status      int
		trecho      string
	}{
		{"sem administradora", escopo.Implantacao{}, "/api/memoria?phone=5519981621916", 400, "ADMINISTRADORA_OBRIGATORIA"},
		{"administradora informada", escopo.Implantacao{}, "/api/memoria?phone=5519981621916&administradora=adm-x", 200, "tem um cachorro"},
		{"outra administradora nao ve", escopo.Implantacao{}, "/api/memoria?phone=5519981621916&administradora=adm-y", 200, `"fatos":[]`},
		{"fixa vale por padrao", escopo.Implantacao{AdministradoraFixa: "adm-x"}, "/api/memoria?phone=5519981621916", 200, "tem um cachorro"},
		{"fora da implantacao", escopo.Implantacao{AdministradoraFixa: "adm-x"}, "/api/memoria?phone=5519981621916&administradora=adm-y", 403, "FORA_DA_IMPLANTACAO"},
		{"administradora com formato invalido", escopo.Implantacao{}, "/api/memoria?phone=5519981621916&administradora=..%2Fx", 400, "ADMINISTRADORA_INVALIDA"},
		{"listagem sem administradora", escopo.Implantacao{}, "/api/memorias", 400, "ADMINISTRADORA_OBRIGATORIA"},
		{"pedidos sem administradora", escopo.Implantacao{}, "/api/pedidos?phone=5519981621916", 400, "ADMINISTRADORA_OBRIGATORIA"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &servidor{atende: &atende.Servico{Sessoes: atende.NewSessoes(atende.NovaSessaoEmMemoria()), Memorias: st}, memorias: st, implantacao: tt.implantacao,
				acessos: acessosFake{atribuicoes: usuariosTeste(), mapeamento: mapeamentoTeste(t)}, autenticador: autenticadorDev{padrao: "admin"}}
			mux := http.NewServeMux()
			s.rotasAtende(mux)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", tt.url, nil))
			if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.trecho) {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func listarSessoes(t *testing.T, s *atende.Sessoes) []*atende.Sessao {
	t.Helper()
	sessoes, err := s.Lista(context.Background(), "")
	if err != nil {
		t.Fatalf("listar sessões: %v", err)
	}
	return sessoes
}

// storeFalho falha toda gravação; leituras vão ao store em memória.
type storeFalho struct{ *atende.SessaoEmMemoria }

func (storeFalho) Salvar(context.Context, atende.RegistroSessao) error {
	return errors.New("postgres fora")
}

// servidorComSessoes: sessões de adm-a (s-a) e de adm-b (s-b) num store compartilhado.
func servidorComSessoes(t *testing.T, implantacao escopo.Implantacao) *servidor {
	t.Helper()
	st := atende.NovaSessaoEmMemoria()
	for id, adm := range map[string]string{"s-a": "adm-a", "s-b": "adm-b"} {
		sess := &atende.Sessao{ID: id, AdministradoraID: adm, Protocolo: "P-" + id, Destino: entrada.Destino{Conector: "atende-emulado"}, AtualizadaEm: time.Now()}
		r, err := sess.Registro()
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Salvar(t.Context(), r); err != nil {
			t.Fatal(err)
		}
	}
	return &servidor{
		atende:      &atende.Servico{Sessoes: atende.NewSessoes(st), Implantacao: implantacao},
		implantacao: implantacao, outbox: &atende.Outbox{}, emulador: atende.NewEmulador(),
		acessos: acessosFake{atribuicoes: usuariosTeste(), mapeamento: mapeamentoTeste(t)}, autenticador: autenticadorDev{padrao: "admin"},
	}
}

func TestSessoesFicamNaImplantacao(t *testing.T) {
	fixa := escopo.Implantacao{AdministradoraFixa: "adm-a"}
	casos := []struct {
		nome        string
		implantacao escopo.Implantacao
		metodo, url string
		corpo       string
		status      int
		trecho      string
	}{
		{"lista da fixa", fixa, "GET", "/api/atende/sessoes", "", 200, `"id":"s-a"`},
		{"lista de outra administradora", fixa, "GET", "/api/atende/sessoes?administradora=adm-b", "", 403, "FORA_DA_IMPLANTACAO"},
		{"lista com administradora invalida", fixa, "GET", "/api/atende/sessoes?administradora=..%2Fx", "", 400, "ADMINISTRADORA_INVALIDA"},
		{"lista sem fixa traz todas", escopo.Implantacao{}, "GET", "/api/atende/sessoes", "", 200, `"id":"s-b"`},
		{"lista filtrada sem fixa", escopo.Implantacao{}, "GET", "/api/atende/sessoes?administradora=adm-b", "", 200, `"id":"s-b"`},
		{"le a propria", fixa, "GET", "/api/atende/sessao?id=s-a", "", 200, `"id":"s-a"`},
		{"le alheia", fixa, "GET", "/api/atende/sessao?id=s-b", "", 403, "SEM_PERMISSAO"},
		{"le alheia por protocolo", fixa, "GET", "/api/atende/sessao?protocolo=P-s-b", "", 403, "SEM_PERMISSAO"},
		{"le de outra administradora pedida", escopo.Implantacao{}, "GET", "/api/atende/sessao?id=s-a&administradora=adm-b", "", 404, ""},
		{"saidas de alheia", fixa, "GET", "/api/atende/saidas?id=s-b", "", 403, "SEM_PERMISSAO"},
		{"assume a propria", fixa, "POST", "/api/atende/transbordo", `{"sessao_id":"s-a","acao":"assumir"}`, 200, `"ativo":true`},
		{"assume alheia", fixa, "POST", "/api/atende/transbordo", `{"sessao_id":"s-b","acao":"assumir"}`, 403, "SEM_PERMISSAO"},
		{"devolve alheia", fixa, "POST", "/api/atende/transbordo", `{"sessao_id":"s-b","acao":"devolver"}`, 403, "SEM_PERMISSAO"},
		{"preenche pedido de alheia", fixa, "POST", "/api/pedidos/acao", `{"sessao_id":"s-b","acao":"preencher"}`, 403, "SEM_PERMISSAO"},
		{"apaga alheia", fixa, "DELETE", "/api/atende/sessao?id=s-b", "", 403, "SEM_PERMISSAO"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			s := servidorComSessoes(t, c.implantacao)
			mux := http.NewServeMux()
			s.rotasAtende(mux)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(c.metodo, c.url, strings.NewReader(c.corpo)))
			if w.Code != c.status || !strings.Contains(w.Body.String(), c.trecho) {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if c.status >= 400 && strings.Contains(w.Body.String(), "s-b") {
				t.Fatalf("resposta revela a sessão alheia: %s", w.Body.String())
			}
			if _, ok, err := s.atende.Sessoes.Get(t.Context(), "s-b"); err != nil || !ok {
				t.Fatalf("sessão alheia sumiu do store: ok=%v err=%v", ok, err)
			}
		})
	}
	t.Run("lista da fixa nao traz alheia", func(t *testing.T) {
		s := servidorComSessoes(t, fixa)
		w := httptest.NewRecorder()
		s.atendeSessoes(w, httptest.NewRequest("GET", "/api/atende/sessoes", nil))
		if strings.Contains(w.Body.String(), `"id":"s-b"`) {
			t.Fatalf("lista trouxe sessão de outra administradora: %s", w.Body.String())
		}
	})
}

func TestFalhaAoPersistirResponde500(t *testing.T) {
	novo := func(t *testing.T) *servidor {
		s := &servidor{atende: &atende.Servico{Sessoes: atende.NewSessoes(storeFalho{atende.NovaSessaoEmMemoria()})}, outbox: &atende.Outbox{}, emulador: atende.NewEmulador()}
		msg := entrada.Mensagem{ConversaID: "protocolo-teste", Origem: entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}}
		if _, err := s.atende.Sessoes.Abrir(t.Context(), msg); err != nil {
			t.Fatal(err)
		}
		return s
	}
	t.Run("webhook humano", func(t *testing.T) {
		s := novo(t)
		w := httptest.NewRecorder()
		s.webhookEmulado(w, httptest.NewRequest("POST", "/emulador/atende/webhook", strings.NewReader(corpoEventoTeste(atende.EventoMensagemAtendente, "tenant-1", "551940402710"))))
		if w.Code != 500 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})
	t.Run("transbordo", func(t *testing.T) {
		s := novo(t)
		sessoes := listarSessoes(t, s.atende.Sessoes)
		if len(sessoes) != 1 {
			t.Fatalf("sessões: %d", len(sessoes))
		}
		w := httptest.NewRecorder()
		s.atendeTransbordo(w, httptest.NewRequest("POST", "/api/atende/transbordo", strings.NewReader(`{"sessao_id":"`+sessoes[0].ID+`","acao":"assumir"}`)))
		if w.Code != 500 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
	})
}
