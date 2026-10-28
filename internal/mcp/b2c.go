package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// B2C obtém o token de morador (Keycloak "persons") pelo fluxo de dev do Spaces:
//
//	POST {BaseURL}/auth/sms    validation=dev&mobile_number=…        → session_id
//	POST {BaseURL}/auth/token  grant_type=password&session_id=…&sms_code=… → access_token (5 min) + refresh_token (60 dias)
//
// e o renova pelo refresh_token antes de expirar (se o refresh falhar, refaz o SMS). O
// token corrente é gravado em File (quando definido): o servidor MCP da entrega só
// aceita o bearer igual ao que ele "pina", e com B2C_TOKEN_FILE ele passa a ler o
// mesmo arquivo, então o concierge e o MCP sempre carregam o MESMO token.
type B2C struct {
	BaseURL     string // ex.: https://api.superlogica.com/spaces/v1
	Basic       string // valor do header Authorization (Basic …) do app
	Mobile      string
	SMSCode     string
	Validation  string // dev
	Fingerprint string
	ClientID    string // app
	File        string // onde gravar o token corrente; vazio = não grava
	HTTP        *http.Client
	Now         func() time.Time

	mu      sync.Mutex
	token   string
	refresh string
	expires time.Time
}

func (b *B2C) validar() error {
	switch {
	case strings.TrimSpace(b.BaseURL) == "":
		return errors.New("mcp b2c: MCP_B2C_URL é obrigatória")
	case strings.TrimSpace(b.Mobile) == "":
		return errors.New("mcp b2c: MCP_B2C_MOBILE é obrigatório")
	case strings.TrimSpace(b.Basic) == "":
		return errors.New("mcp b2c: MCP_B2C_BASIC é obrigatório")
	}
	if u, err := url.Parse(b.BaseURL); err != nil || u.Host == "" || (u.Scheme != "https" && !ehLoopback(u.Hostname())) {
		return fmt.Errorf("mcp b2c: MCP_B2C_URL deve ser https (ou loopback): %q", b.BaseURL)
	}
	return nil
}

func ehLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (b *B2C) agora() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

// Token devolve o token corrente, renovando quando faltar menos de 45 s para expirar.
func (b *B2C) Token(ctx context.Context) (string, error) {
	if err := b.validar(); err != nil {
		return "", err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.token != "" && b.agora().Before(b.expires) {
		return b.token, nil
	}
	var tok tokenB2C
	var err error
	if b.refresh != "" {
		tok, err = b.grant(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {b.refresh}})
	}
	if b.refresh == "" || err != nil {
		tok, err = b.porSMS(ctx)
	}
	if err != nil {
		return "", err
	}
	ttl := time.Duration(tok.ExpiresIn)*time.Second - 45*time.Second
	if ttl < 30*time.Second {
		ttl = 30 * time.Second
	}
	b.token, b.refresh, b.expires = tok.AccessToken, tok.RefreshToken, b.agora().Add(ttl)
	if b.File != "" {
		if err := gravarToken(b.File, b.token); err != nil {
			return "", fmt.Errorf("mcp b2c: gravar %s: %w", b.File, err)
		}
	}
	return b.token, nil
}

type tokenB2C struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (b *B2C) porSMS(ctx context.Context) (tokenB2C, error) {
	validation := b.Validation
	if validation == "" {
		validation = "dev"
	}
	raw, err := b.post(ctx, "/auth/sms", url.Values{"validation": {validation}, "mobile_number": {b.Mobile}})
	if err != nil {
		return tokenB2C{}, err
	}
	var sms struct {
		SessionID string `json:"session_id"`
	}
	if json.Unmarshal(raw, &sms) != nil || sms.SessionID == "" {
		return tokenB2C{}, errors.New("mcp b2c: /auth/sms sem session_id")
	}
	code := b.SMSCode
	if code == "" {
		code = "999999"
	}
	return b.grant(ctx, url.Values{"grant_type": {"password"}, "session_id": {sms.SessionID}, "sms_code": {code}, "mobile_number": {b.Mobile}})
}

func (b *B2C) grant(ctx context.Context, form url.Values) (tokenB2C, error) {
	client := b.ClientID
	if client == "" {
		client = "app"
	}
	fp := b.Fingerprint
	if fp == "" {
		fp = "go-concierge"
	}
	form.Set("client_id", client)
	form.Set("fingerprint", fp)
	raw, err := b.post(ctx, "/auth/token", form)
	if err != nil {
		return tokenB2C{}, err
	}
	var tok tokenB2C
	if json.Unmarshal(raw, &tok) != nil || tok.AccessToken == "" {
		return tokenB2C{}, errors.New("mcp b2c: /auth/token sem access_token")
	}
	if tok.ExpiresIn <= 0 {
		tok.ExpiresIn = 300
	}
	return tok, nil
}

func (b *B2C) post(ctx context.Context, caminho string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(b.BaseURL, "/")+caminho, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", b.Basic)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	hc := b.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp b2c: %s: %w", caminho, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp b2c: %s: HTTP %d", caminho, res.StatusCode) // corpo omitido: pode ecoar credenciais
	}
	return raw, nil
}

// gravarToken escreve o token num arquivo só do dono, por troca atômica (o servidor
// MCP lê o arquivo a cada verificação e nunca vê um token pela metade).
func gravarToken(caminho, token string) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return err
	}
	tmp := caminho + ".tmp"
	if err := os.WriteFile(tmp, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, caminho)
}

// LerTokenArquivo é o inverso: um token corrente gravado por outro processo (modo file).
type Arquivo string

func (a Arquivo) Token(context.Context) (string, error) {
	raw, err := os.ReadFile(string(a))
	if err != nil {
		return "", fmt.Errorf("mcp: token em %s: %w", string(a), err)
	}
	tok := strings.TrimSpace(string(raw))
	if tok == "" {
		return "", fmt.Errorf("mcp: token em %s vazio", string(a))
	}
	return tok, nil
}
