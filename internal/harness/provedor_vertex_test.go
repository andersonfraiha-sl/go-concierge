package harness

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func credencialVertexDeTeste(t *testing.T, tokenURL string) string {
	t.Helper()
	chave, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]string{
		"type":         "service_account",
		"project_id":   "projeto-teste",
		"client_email": "concierge@projeto-teste.iam.gserviceaccount.com",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(chave)})),
		"token_uri":    tokenURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	caminho := filepath.Join(t.TempDir(), "service-account.json")
	if err := os.WriteFile(caminho, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return caminho
}

func TestResolverVertexPadrao(t *testing.T) {
	valores := map[string]string{"GEMINI_API_KEY": "chave-legada", "OPENROUTER_API_KEY": "outra-chave"}
	env := func(k string) string { return valores[k] }
	p := ResolverProvedor(env)
	if p.Nome != "vertex" || p.Modelo != ModeloVertex || p.Localizacao != "global" || p.Chave != "" {
		t.Fatalf("padrão Vertex incorreto: %+v", p)
	}
	valores["LLM_MODE"] = "vertex"
	valores["GOOGLE_CLOUD_PROJECT"] = " projeto-teste "
	valores["GOOGLE_CLOUD_LOCATION"] = " eu "
	valores["GOOGLE_APPLICATION_CREDENTIALS"] = " credencial.json "
	valores["MODEL_PLANNER"] = "outro-modelo"
	p = ResolverProvedor(env)
	if p.Projeto != "projeto-teste" || p.Localizacao != "eu" || p.ArquivoCredenciais != "credencial.json" || p.Modelo != "outro-modelo" {
		t.Fatalf("configuração Vertex ignorada: %+v", p)
	}
	valores["LLM_MODE"] = "gemini"
	if p := ResolverProvedor(env); p.Nome != "gemini" || p.Chave != "chave-legada" {
		t.Fatal("o provedor explícito deve ser preservado")
	}
}

func TestVertexAutenticaEContinuaFerramentaAposPersistirHistorico(t *testing.T) {
	var tokens, chamadas atomic.Int32
	requisicoes := make(chan []byte, 2)
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			tokens.Add(1)
			if err := r.ParseForm(); err != nil || r.Form.Get("assertion") == "" {
				t.Error("a service account deve enviar uma asserção assinada")
			}
			_, _ = io.WriteString(w, `{"access_token":"token-teste","token_type":"Bearer","expires_in":3600}`)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/projects/projeto-teste/locations/global/publishers/google/models/gemini-3.5-flash:generateContent" || r.Header.Get("Authorization") != "Bearer token-teste" || r.Header.Get("x-goog-api-key") != "" {
			t.Errorf("requisição Vertex inesperada: %s %s", r.Method, r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requisicoes <- raw
		if chamadas.Add(1) == 1 {
			_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"consultar","args":{"id":"1"}},"thoughtSignature":"YXNzaW5hdHVyYQ=="}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":3,"thoughtsTokenCount":2,"totalTokenCount":15}}`)
			return
		}
		_, _ = io.WriteString(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"Concluído"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":2,"totalTokenCount":22}}`)
	}))
	defer servidor.Close()
	p := Provedor{Nome: "vertex", Projeto: "projeto-teste", Localizacao: "global", ArquivoCredenciais: credencialVertexDeTeste(t, servidor.URL+"/token")}
	m, err := novoModelo(context.Background(), p, "", servidor.URL, ModeloVertex, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	m, err = m.WithTools([]*schema.ToolInfo{{Name: "consultar", Desc: "Consulta", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{"id": {Type: schema.String, Required: true}})}})
	if err != nil {
		t.Fatal(err)
	}
	historico := []*schema.Message{schema.SystemMessage("Instruções"), schema.UserMessage("Consulte")}
	primeira, err := m.Generate(context.Background(), historico)
	if err != nil {
		t.Fatal(err)
	}
	if len(primeira.ToolCalls) != 1 || primeira.ResponseMeta.Usage.CompletionTokens != 5 || primeira.ResponseMeta.Usage.TotalTokens != 15 {
		t.Fatalf("ferramenta ou uso incorretos: %+v", primeira)
	}
	historico = append(historico, primeira, schema.ToolMessage(`{"resultado":"ok"}`, primeira.ToolCalls[0].ID))
	persistido, err := json.Marshal(historico)
	if err != nil {
		t.Fatal(err)
	}
	var restaurado []*schema.Message
	if err := json.Unmarshal(persistido, &restaurado); err != nil {
		t.Fatal(err)
	}
	segunda, err := m.Generate(context.Background(), restaurado)
	if err != nil || segunda.Content != "Concluído" {
		t.Fatalf("continuação falhou: %+v, %v", segunda, err)
	}
	primeiroRequest := string(<-requisicoes)
	segundoRequest := string(<-requisicoes)
	for _, campo := range []string{`"functionDeclarations"`, `"systemInstruction"`, `"consultar"`} {
		if !strings.Contains(primeiroRequest, campo) {
			t.Errorf("faltou %s no request", campo)
		}
	}
	if !strings.Contains(segundoRequest, `"thoughtSignature":"YXNzaW5hdHVyYQ=="`) || !strings.Contains(segundoRequest, `"functionResponse"`) {
		t.Fatalf("histórico perdeu a assinatura ou o resultado da ferramenta: %s", segundoRequest)
	}
	if tokens.Load() != 1 {
		t.Fatalf("o token deve ser reutilizado enquanto válido: %d emissões", tokens.Load())
	}
}

func TestConsumidoresVertexSemAPIKey(t *testing.T) {
	valores := map[string]string{"LLM_MODE": "vertex", "GOOGLE_CLOUD_PROJECT": "projeto-teste", "GOOGLE_APPLICATION_CREDENTIALS": credencialVertexDeTeste(t, "http://127.0.0.1/token")}
	cfg := Config{Env: func(k string) string { return valores[k] }}
	if _, ok := NovoExtrator(context.Background(), cfg).(*ExtratorLLM); !ok {
		t.Error("Vertex deve habilitar o extrator de memória")
	}
	if _, ok := NovoExtratorPedidos(context.Background(), cfg).(*ExtratorPedidosLLM); !ok {
		t.Error("Vertex deve habilitar o extrator de pedidos")
	}
	if _, ok := modeloBarato(context.Background(), cfg); !ok {
		t.Error("Vertex deve habilitar sugestões")
	}
	if _, ok := NovoCopiloto(cfg); !ok {
		t.Error("Vertex deve habilitar o copiloto")
	}
	cat, a := catalogoDeSkills(t)
	ag, err := Build(context.Background(), cat, a, cfg)
	if err != nil || ag.Mock || ag.Provedor != "vertex" {
		t.Fatalf("agente Vertex não deve cair em mock: %v", err)
	}
}

func TestVertexRecusaConfiguracaoInvalidaSemMock(t *testing.T) {
	for _, valores := range []map[string]string{
		{},
		{"GOOGLE_CLOUD_PROJECT": "projeto-teste", "GOOGLE_APPLICATION_CREDENTIALS": filepath.Join(t.TempDir(), "ausente.json")},
	} {
		cat, a := catalogoDeSkills(t)
		_, err := Build(context.Background(), cat, a, Config{Env: func(k string) string { return valores[k] }})
		if err == nil || !strings.Contains(err.Error(), "vertex:") {
			t.Fatalf("configuração Vertex inválida deve produzir erro: %v", err)
		}
	}
}
