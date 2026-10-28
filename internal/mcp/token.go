package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenProvider dá o Bearer de cada requisição ao servidor MCP.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// Static é um token fixo: o token B2C do Keycloak, como na entrega (o servidor em
// request_bearer confere o mesmo token e o repassa ao BFF). Expira: troque no .env.
type Static string

func (s Static) Token(context.Context) (string, error) { return string(s), nil }

// Keycloak obtém um token por client_credentials e o guarda até expirar. Serve
// para um servidor MCP que aceite o token de serviço (ou em modo client_credentials).
type Keycloak struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	HTTP         *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

func (k *Keycloak) Token(ctx context.Context) (string, error) {
	if k.TokenURL == "" || k.ClientID == "" {
		return "", errors.New("mcp keycloak: MCP_KEYCLOAK_URL e MCP_KEYCLOAK_CLIENT_ID são obrigatórios")
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.token != "" && time.Now().Before(k.expires) {
		return k.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {k.ClientID}}
	if k.ClientSecret != "" {
		form.Set("client_secret", k.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, k.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	hc := k.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return "", fmt.Errorf("mcp keycloak: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mcp keycloak: HTTP %d", res.StatusCode) // corpo omitido: pode ecoar credenciais
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if json.Unmarshal(raw, &tok) != nil || tok.AccessToken == "" {
		return "", errors.New("mcp keycloak: resposta sem access_token")
	}
	ttl := tok.ExpiresIn - 30
	if ttl < 30 {
		ttl = 30
	}
	k.token, k.expires = tok.AccessToken, time.Now().Add(time.Duration(ttl)*time.Second)
	return k.token, nil
}

// FromEnv monta o cliente a partir do ambiente. Sem MCP_URL devolve nil (sem MCP).
//
//	MCP_URL                  endpoint (ex.: http://127.0.0.1:3337/mcp/gruvi/bff)
//	MCP_AUTH                 bearer | b2c | file | keycloak | none (vazio: bearer se houver MCP_BEARER_TOKEN,
//	                         keycloak se houver MCP_KEYCLOAK_URL, senão none)
//	MCP_BEARER_TOKEN         o token B2C do Keycloak (modo bearer)
//	MCP_KEYCLOAK_URL, MCP_KEYCLOAK_CLIENT_ID, MCP_KEYCLOAK_CLIENT_SECRET   (modo keycloak)
//	MCP_B2C_MOBILE (+ MCP_B2C_URL, MCP_B2C_BASIC, MCP_B2C_SMS_CODE, MCP_B2C_VALIDATION,
//	                 MCP_B2C_FINGERPRINT, MCP_B2C_CLIENT_ID)                 (modo b2c: token de
//	                 morador pelo fluxo SMS de dev, renovado pelo refresh_token; grava em MCP_TOKEN_FILE)
//	MCP_TOKEN_FILE           modo file: lê o token corrente desse arquivo a cada requisição
//	MCP_HOST_HEADER          Host a enviar (padrão: 127.0.0.1:porta fora de loopback)
func FromEnv(get func(string) string) (*Client, string, error) {
	endpoint := strings.TrimSpace(get("MCP_URL"))
	if endpoint == "" {
		return nil, "", nil
	}
	modo := strings.ToLower(strings.TrimSpace(get("MCP_AUTH")))
	if modo == "" {
		switch {
		case strings.TrimSpace(get("MCP_BEARER_TOKEN")) != "":
			modo = "bearer"
		case strings.TrimSpace(get("MCP_KEYCLOAK_URL")) != "":
			modo = "keycloak"
		default:
			modo = "none"
		}
	}
	var tp TokenProvider
	switch modo {
	case "bearer":
		tok := strings.TrimSpace(get("MCP_BEARER_TOKEN"))
		if tok == "" {
			return nil, "", errors.New("MCP_AUTH=bearer exige MCP_BEARER_TOKEN")
		}
		tp = Static(tok)
	case "keycloak":
		tp = &Keycloak{TokenURL: strings.TrimSpace(get("MCP_KEYCLOAK_URL")), ClientID: strings.TrimSpace(get("MCP_KEYCLOAK_CLIENT_ID")), ClientSecret: strings.TrimSpace(get("MCP_KEYCLOAK_CLIENT_SECRET"))}
		if _, err := tp.(*Keycloak).validar(); err != nil {
			return nil, "", err
		}
	case "b2c":
		b := &B2C{
			BaseURL:     ou(get("MCP_B2C_URL"), "https://api.superlogica.com/spaces/v1"),
			Basic:       ou(get("MCP_B2C_BASIC"), "Basic Z3J1dmktYXBwOg=="), // gruvi-app, sem segredo (client público)
			Mobile:      strings.TrimSpace(get("MCP_B2C_MOBILE")),
			SMSCode:     ou(get("MCP_B2C_SMS_CODE"), "999999"),
			Validation:  ou(get("MCP_B2C_VALIDATION"), "dev"),
			Fingerprint: ou(get("MCP_B2C_FINGERPRINT"), "go-concierge"),
			ClientID:    ou(get("MCP_B2C_CLIENT_ID"), "app"),
			File:        strings.TrimSpace(get("MCP_TOKEN_FILE")),
		}
		if err := b.validar(); err != nil {
			return nil, "", err
		}
		tp = b
	case "file":
		f := strings.TrimSpace(get("MCP_TOKEN_FILE"))
		if f == "" {
			return nil, "", errors.New("MCP_AUTH=file exige MCP_TOKEN_FILE")
		}
		tp = Arquivo(f)
	case "none":
		tp = nil
	default:
		return nil, "", fmt.Errorf("MCP_AUTH desconhecido: %q (use bearer, b2c, file, keycloak ou none)", modo)
	}
	c := New(endpoint, tp)
	c.HostHeader = strings.TrimSpace(get("MCP_HOST_HEADER"))
	return c, modo, nil
}

func (k *Keycloak) validar() (bool, error) {
	if k.TokenURL == "" || k.ClientID == "" {
		return false, errors.New("MCP_AUTH=keycloak exige MCP_KEYCLOAK_URL e MCP_KEYCLOAK_CLIENT_ID")
	}
	return true, nil
}

func ou(v, padrao string) string {
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return padrao
}
