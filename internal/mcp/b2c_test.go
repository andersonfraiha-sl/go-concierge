package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// O fluxo de dev (SMS → token), o cache até perto do vencimento, a renovação pelo
// refresh_token e a gravação do token corrente no arquivo que o servidor MCP lê.
func TestB2CSMSRefreshEArquivo(t *testing.T) {
	var sms, password, refresh int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Basic abc" || r.Method != http.MethodPost {
			w.WriteHeader(401)
			return
		}
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/auth/sms":
			sms++
			if r.Form.Get("validation") != "dev" || r.Form.Get("mobile_number") != "5511999" {
				w.WriteHeader(400)
				return
			}
			w.Write([]byte(`{"session_id":"sess-1"}`))
		case "/auth/token":
			switch r.Form.Get("grant_type") {
			case "password":
				password++
				if r.Form.Get("session_id") != "sess-1" || r.Form.Get("sms_code") != "999999" || r.Form.Get("client_id") != "app" || r.Form.Get("fingerprint") != "fp" {
					w.WriteHeader(400)
					return
				}
				w.Write([]byte(`{"access_token":"tok-1","refresh_token":"ref-1","expires_in":300}`))
			case "refresh_token":
				refresh++
				if r.Form.Get("refresh_token") != "ref-1" {
					w.WriteHeader(400)
					return
				}
				w.Write([]byte(`{"access_token":"tok-2","refresh_token":"ref-2","expires_in":300}`))
			default:
				w.WriteHeader(400)
			}
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	agora := time.Now()
	arquivo := filepath.Join(t.TempDir(), "var", "b2c.token")
	b := &B2C{BaseURL: srv.URL, Basic: "Basic abc", Mobile: "5511999", Fingerprint: "fp", File: arquivo, Now: func() time.Time { return agora }}
	ctx := context.Background()
	tok, err := b.Token(ctx)
	if err != nil || tok != "tok-1" || sms != 1 || password != 1 {
		t.Fatalf("primeiro token pelo SMS: %q %v (sms=%d password=%d)", tok, err, sms, password)
	}
	if raw, err := os.ReadFile(arquivo); err != nil || strings.TrimSpace(string(raw)) != "tok-1" {
		t.Fatalf("token devia estar gravado em %s: %q %v", arquivo, raw, err)
	}
	if tok, _ := b.Token(ctx); tok != "tok-1" || sms != 1 {
		t.Fatalf("dentro da validade usa o cache: %q sms=%d", tok, sms)
	}
	agora = agora.Add(4*time.Minute + 30*time.Second) // faltam < 45 s
	tok, err = b.Token(ctx)
	if err != nil || tok != "tok-2" || refresh != 1 || sms != 1 {
		t.Fatalf("perto do vencimento renova pelo refresh_token: %q %v (refresh=%d sms=%d)", tok, err, refresh, sms)
	}
	if raw, _ := os.ReadFile(arquivo); strings.TrimSpace(string(raw)) != "tok-2" {
		t.Fatalf("arquivo devia ter o token novo: %q", raw)
	}
	if tok, _ := Arquivo(arquivo).Token(ctx); tok != "tok-2" {
		t.Fatalf("modo file lê o mesmo arquivo: %q", tok)
	}

	b.refresh = "ref-x" // refresh inválido → refaz o SMS
	agora = agora.Add(10 * time.Minute)
	if tok, err := b.Token(ctx); err != nil || tok != "tok-1" || sms != 2 {
		t.Fatalf("refresh inválido devia cair no SMS: %q %v sms=%d", tok, err, sms)
	}
}

func TestFromEnvB2C(t *testing.T) {
	env := map[string]string{"MCP_URL": "http://127.0.0.1:3337/mcp/gruvi/bff", "MCP_AUTH": "b2c", "MCP_B2C_MOBILE": "5511999", "MCP_TOKEN_FILE": "/tmp/x.token"}
	get := func(k string) string { return env[k] }
	c, modo, err := FromEnv(get)
	if err != nil || modo != "b2c" {
		t.Fatalf("modo b2c: %v %q", err, modo)
	}
	b, ok := c.Token.(*B2C)
	if !ok || b.BaseURL != "https://api.superlogica.com/spaces/v1" || b.ClientID != "app" || b.SMSCode != "999999" || b.File != "/tmp/x.token" || b.Basic == "" {
		t.Fatalf("padrões do b2c: %+v", b)
	}
	delete(env, "MCP_B2C_MOBILE")
	if _, _, err := FromEnv(get); err == nil {
		t.Fatal("sem MCP_B2C_MOBILE devia falhar")
	}
	env["MCP_AUTH"], env["MCP_TOKEN_FILE"] = "file", "/tmp/y.token"
	c, modo, err = FromEnv(get)
	if err != nil || modo != "file" || c.Token != Arquivo("/tmp/y.token") {
		t.Fatalf("modo file: %v %q %v", err, modo, c.Token)
	}
}
