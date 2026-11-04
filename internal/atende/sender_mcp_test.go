package atende

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go-concierge/internal/entrada"
	"go-concierge/internal/mcp"
)

type chamadaGateway struct {
	tool string
	args map[string]any
}

// gatewayFalso imita o servidor MCP `atende` do gateway: initialize, notifications e
// tools/call com resultado definido pelo teste (texto, isError).
type gatewayFalso struct {
	srv       *httptest.Server
	mu        sync.Mutex
	chamadas  []chamadaGateway
	responder func(tool string, args map[string]any) (texto string, isError bool)
}

func novoGatewayFalso(t *testing.T, responder func(string, map[string]any) (string, bool)) *gatewayFalso {
	t.Helper()
	g := &gatewayFalso{responder: responder}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		var req struct {
			ID     *int64 `json:"id"`
			Method string `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(corpo, &req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "initialize":
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}}}}`, *req.ID)
		case "tools/call":
			g.mu.Lock()
			g.chamadas = append(g.chamadas, chamadaGateway{req.Params.Name, req.Params.Arguments})
			g.mu.Unlock()
			texto, isError := g.responder(req.Params.Name, req.Params.Arguments)
			resultado, _ := json.Marshal(map[string]any{"content": []map[string]string{{"type": "text", "text": texto}}, "isError": isError})
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":%s}`, *req.ID, resultado)
		default:
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *gatewayFalso) sender() *MCPSender { return &MCPSender{Client: mcp.New(g.srv.URL, nil)} }

func (g *gatewayFalso) chamadasDe(tool string) []chamadaGateway {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []chamadaGateway
	for _, c := range g.chamadas {
		if c.tool == tool {
			out = append(out, c)
		}
	}
	return out
}

func gatewayOK(t *testing.T) *gatewayFalso {
	return novoGatewayFalso(t, func(string, map[string]any) (string, bool) { return `{"ok":true}`, false })
}

func corpoDe(t *testing.T, c chamadaGateway) map[string]any {
	t.Helper()
	corpo, ok := c.args["body"].(map[string]any)
	if !ok {
		t.Fatalf("%s sem body: %+v", c.tool, c.args)
	}
	return corpo
}

func TestMCPSenderEnviaEmPartes(t *testing.T) {
	gw := gatewayOK(t)
	texto := "**Olá**" + strings.Repeat("x", 8993) // 9000 caracteres
	err := gw.sender().Enviar(context.Background(), entrada.Destino{Telefone: "5511999999999"}, texto)
	if err != nil {
		t.Fatal(err)
	}
	envios := gw.chamadasDe("messages_send")
	if len(envios) != 3 {
		t.Fatalf("esperava 3 partes, vieram %d", len(envios))
	}
	for i, c := range envios {
		corpo := corpoDe(t, c)
		contato, _ := corpo["contact"].(map[string]any)
		mensagem, _ := corpo["message"].(map[string]any)
		if contato["phone"] != "5511999999999" || mensagem["type"] != "text" {
			t.Fatalf("parte %d com destino/tipo inesperado: %+v", i+1, corpo)
		}
		if len([]rune(fmt.Sprint(mensagem["text"]))) > 4000 {
			t.Fatalf("parte %d acima do limite", i+1)
		}
	}
	if primeira := fmt.Sprint(corpoDe(t, envios[0])["message"].(map[string]any)["text"]); !strings.HasPrefix(primeira, "*Olá*x") {
		t.Fatalf("FormatarMensagem não aplicada: %.20q", primeira)
	}
}

func TestMCPSenderFalhaParcialNaoRepete(t *testing.T) {
	chamadas := 0
	gw := novoGatewayFalso(t, func(string, map[string]any) (string, bool) {
		chamadas++
		return `{"error":"upstream_error","details":"502"}`, chamadas == 2
	})
	err := gw.sender().Enviar(context.Background(), entrada.Destino{Telefone: "1"}, strings.Repeat("x", 8500))
	var parcial *entregaParcial
	if !errors.As(err, &parcial) || len(parcial.partes) != 1 || !strings.Contains(err.Error(), "parte 2 de 3") || chamadas != 2 {
		t.Fatalf("falha parcial incorreta: err=%v chamadas=%d", err, chamadas)
	}
}

func TestMCPSenderSemClienteNaoPanica(t *testing.T) {
	if err := (&MCPSender{}).Enviar(context.Background(), entrada.Destino{}, "texto"); err == nil || strings.Contains(err.Error(), "texto") {
		t.Fatalf("sem cliente deve falhar sem incluir conteúdo: %v", err)
	}
}

func TestTransferirDistingueIncertaDeRecusa(t *testing.T) {
	destino := entrada.Destino{Protocolo: "P-1"}
	recusa := novoGatewayFalso(t, func(string, map[string]any) (string, bool) {
		return `{"error":"upstream_credential_missing","details":"sem chave"}`, true
	})
	err := recusa.sender().Transferir(context.Background(), destino, "motivo")
	var incerta *TransferenciaIncerta
	if err == nil || errors.As(err, &incerta) {
		t.Fatalf("recusa do gateway é definitiva: %v", err)
	}
	fora := novoGatewayFalso(t, nil)
	sender := fora.sender()
	fora.srv.Close()
	if err := sender.Transferir(context.Background(), destino, "motivo"); !errors.As(err, &incerta) {
		t.Fatalf("falha de transporte deve ser incerta: %v", err)
	}
}

// servicoNoCanalReal: o serviço de teste com o canal "atende" entregue pelo sender MCP.
func servicoNoCanalReal(t *testing.T, sender *MCPSender, texto string) (*Servico, *Outbox, entrada.Mensagem) {
	t.Helper()
	svc, out, _ := servicoComPedidos(t)
	out.ATENDE = sender
	svc.Transferidor = TransferidoresPorConector{ATENDE: sender}
	msg := mensagemDeTexto(t, texto)
	msg.Origem = entrada.Origem{Conector: "atende", Meio: "whatsapp", ConexaoID: "tenant:canal"}
	msg.Destino.Conector, msg.Destino.ConexaoID = "atende", "tenant:canal"
	return svc, out, msg
}

func TestTransbordoRealChamaProtocolsTransfer(t *testing.T) {
	gw := gatewayOK(t)
	svc, _, msg := servicoNoCanalReal(t, gw.sender(), "quero falar com um atendente")
	sess, err := svc.Processar(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	transferencias := gw.chamadasDe("protocols_transfer")
	if len(transferencias) != 1 || transferencias[0].args["protocolNumber"] != sess.Destino.Protocolo || sess.Destino.Protocolo == "" {
		t.Fatalf("transferência inesperada (protocolo %q): %+v", sess.Destino.Protocolo, transferencias)
	}
	for _, e := range gw.chamadasDe("messages_send") {
		if texto := fmt.Sprint(corpoDe(t, e)["message"]); strings.Contains(texto, "Não consigo transferir") {
			t.Fatalf("canal real não pode negar a transferência: %s", texto)
		}
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.Transbordo == nil || sess.Transbordo.Estado != "transferido" {
		t.Fatalf("transbordo não registrado como transferido: %+v", sess.Transbordo)
	}
}

func TestErroDoGatewayNaoVazaParaMorador(t *testing.T) {
	gw := novoGatewayFalso(t, func(string, map[string]any) (string, bool) {
		return `{"error":"upstream_credential_missing","details":"sem chave da company"}`, true
	})
	svc, out, msg := servicoNoCanalReal(t, gw.sender(), "quero falar com um atendente")
	sess, _ := svc.Processar(context.Background(), msg)
	for _, e := range gw.chamadasDe("messages_send") {
		if strings.Contains(fmt.Sprint(e.args), "credential") {
			t.Fatalf("o erro do gateway foi mandado ao morador: %+v", e.args)
		}
	}
	for _, s := range out.Itens("") {
		if s.Enviada {
			t.Fatalf("nada foi entregue, mas a saída consta enviada: %+v", s)
		}
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	var detalheNoTurno bool
	for _, tr := range sess.Turnos {
		if tr.Quem == "erro" && strings.Contains(tr.Texto, "upstream_credential_missing") {
			detalheNoTurno = true
		}
		if tr.Quem == "concierge" && strings.Contains(tr.Texto, "credential") {
			t.Fatalf("erro técnico na fala do concierge: %q", tr.Texto)
		}
	}
	if !detalheNoTurno {
		t.Fatal("o erro técnico deveria ficar no turno de erro para o Console")
	}
}

func TestNovoMCPSenderUsaTokenDeServicoEHostDaURL(t *testing.T) {
	var tokenForm string
	tokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		tokenForm = r.PostForm.Get("grant_type") + "|" + r.PostForm.Get("client_id") + "|" + r.PostForm.Get("client_secret")
		fmt.Fprint(w, `{"access_token":"tok-servico","token_type":"Bearer","expires_in":900}`)
	}))
	defer tokens.Close()
	var bearer string
	gw := novoGatewayFalso(t, func(string, map[string]any) (string, bool) { return `{}`, false })
	original := gw.srv.Config.Handler
	gw.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer = r.Header.Get("Authorization")
		original.ServeHTTP(w, r)
	})
	env := map[string]string{"ATENDE_MCP_URL": gw.srv.URL, "ATENDE_MCP_TOKEN_URL": tokens.URL, "ATENDE_MCP_CLIENT_ID": "concierge", "ATENDE_MCP_CLIENT_SECRET": "segredo"}
	sender, err := NovoMCPSender(func(k string) string { return env[k] })
	if err != nil || sender == nil {
		t.Fatalf("sender: %v %v", sender, err)
	}
	if err := sender.Enviar(context.Background(), entrada.Destino{Telefone: "1"}, "oi"); err != nil {
		t.Fatal(err)
	}
	if tokenForm != "client_credentials|concierge|segredo" || bearer != "Bearer tok-servico" {
		t.Fatalf("token de serviço: form=%q authorization=%q", tokenForm, bearer)
	}
}

func TestNovoMCPSenderConfiguracao(t *testing.T) {
	casos := []struct {
		nome string
		env  map[string]string
		erro bool
	}{
		{"sem URL desliga o canal", map[string]string{}, false},
		{"URL sem credencial", map[string]string{"ATENDE_MCP_URL": "https://nexus.example/mcp/atende"}, true},
		{"URL inválida", map[string]string{"ATENDE_MCP_URL": "nexus"}, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			sender, err := NovoMCPSender(func(k string) string { return c.env[k] })
			if sender != nil || (err != nil) != c.erro {
				t.Fatalf("sender=%v err=%v", sender, err)
			}
		})
	}
}

func TestTransferirClassificaFalhasDoGateway(t *testing.T) {
	casos := []struct {
		nome, texto string
		incerta     bool
	}{
		{"timeout", `{"error":"upstream_timeout","details":"x","upstream_status":null}`, true},
		{"inalcançável", `{"error":"upstream_unreachable","details":"x","upstream_status":null}`, true},
		{"erro interno do gateway", `{"error":"internal_error","details":"x","upstream_status":null}`, true},
		{"upstream 503", `{"error":"upstream_error","details":"x","upstream_status":503}`, true},
		{"upstream sem status", `{"error":"upstream_error","details":"x","upstream_status":null}`, true},
		{"upstream 404", `{"error":"upstream_error","details":"x","upstream_status":404}`, false},
		{"upstream 422", `{"error":"upstream_error","details":"x","upstream_status":422}`, false},
		{"argumentos", `{"error":"invalid_arguments","details":"x","upstream_status":null}`, false},
		{"sem credencial", `{"error":"upstream_credential_missing","details":"x","upstream_status":null}`, false},
		{"proibido", `{"error":"forbidden","details":"x","upstream_status":null}`, false},
		{"não encontrado", `{"error":"not_found","details":"x","upstream_status":null}`, false},
		{"texto ilegível", `Internal Server Error`, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			gw := novoGatewayFalso(t, func(string, map[string]any) (string, bool) { return c.texto, true })
			err := gw.sender().Transferir(context.Background(), entrada.Destino{Protocolo: "P-1"}, "m")
			var incerta *TransferenciaIncerta
			if err == nil || errors.As(err, &incerta) != c.incerta {
				t.Fatalf("incerta=%v, want %v (err=%v)", errors.As(err, &incerta), c.incerta, err)
			}
		})
	}
}
