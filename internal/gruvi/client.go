// Package gruvi fala com a plataforma Gruvi (staging/produção) para identificar o
// morador que chegou pelo ATENDE: um token OAuth client_credentials (Basic auth no
// endpoint de token) e a consulta do BFF por telefone, que devolve o condomínio e as
// unidades do contato.
//
//	POST {GRUVI_TOKEN_URL}  Authorization: Basic {GRUVI_CLIENT_BASIC}  grant_type=client_credentials
//	GET  {GRUVI_BFF_URL}/building-operation/companies/{GRUVI_COMPANY_ID}?filter[mobile-number]={telefone}
//	     Authorization: Bearer {access_token}
package gruvi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	TokenURLPadrao = "https://api.gruvi.app/staging/gruvi/v1/clients/auth/token"
	BFFURLPadrao   = "https://api.gruvi.app/staging/bff/v2"
)

// Client guarda as credenciais e o token em cache.
type Client struct {
	TokenURL  string
	BFFURL    string
	Basic     string // base64 de client_id:client_secret (o valor do header Authorization: Basic …)
	CompanyID string
	HTTP      *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

// New monta o cliente; valores vazios caem nos padrões de staging.
func New(tokenURL, bffURL, basic, company string) *Client {
	if tokenURL == "" {
		tokenURL = TokenURLPadrao
	}
	if bffURL == "" {
		bffURL = BFFURLPadrao
	}
	return &Client{TokenURL: tokenURL, BFFURL: strings.TrimRight(bffURL, "/"), Basic: strings.TrimSpace(basic), CompanyID: strings.TrimSpace(company), HTTP: &http.Client{Timeout: 25 * time.Second}}
}

// Configurado diz se há credencial e company para consultar.
func (c *Client) Configurado() bool { return c != nil && c.Basic != "" && c.CompanyID != "" }

// Token devolve o access_token (com cache até expirar).
func (c *Client) Token(ctx context.Context) (string, error) {
	if !c.Configurado() {
		return "", errors.New("gruvi: GRUVI_CLIENT_BASIC e GRUVI_COMPANY_ID não configurados")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expires) {
		return c.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Basic "+c.Basic)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("gruvi token: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gruvi token: HTTP %d", res.StatusCode) // corpo omitido: pode ecoar credenciais
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if json.Unmarshal(raw, &tok) != nil || tok.AccessToken == "" {
		return "", errors.New("gruvi token: resposta sem access_token")
	}
	ttl := tok.ExpiresIn - 30
	if ttl < 30 {
		ttl = 30
	}
	c.token, c.expires = tok.AccessToken, time.Now().Add(time.Duration(ttl)*time.Second)
	return c.token, nil
}

// Unidade do morador no condomínio.
type Unidade struct {
	ID    string `json:"id"` // id_fastpass (o id que as operações usam)
	Label string `json:"label"`
	Bloco string `json:"bloco,omitempty"`
}

// Identidade é o que o BFF disse do telefone.
type Identidade struct {
	Authenticated bool      `json:"authenticated"`
	PersonID      string    `json:"person_id,omitempty"`
	Name          string    `json:"name,omitempty"`
	Phone         string    `json:"phone"`
	CompanyID     string    `json:"company_id"`
	CompanyName   string    `json:"company_name,omitempty"`
	Role          string    `json:"role,omitempty"`
	Units         []Unidade `json:"units"`
	// Bruto é a resposta do BFF (data), para a bancada do emulador.
	Bruto map[string]any `json:"bruto,omitempty"`
}

// Identidade consulta o BFF pelo telefone (API1 do contrato).
func (c *Client) Identidade(ctx context.Context, phone string) (Identidade, error) {
	id := Identidade{Phone: phone, CompanyID: c.CompanyID, Units: []Unidade{}}
	tok, err := c.Token(ctx)
	if err != nil {
		return id, err
	}
	q := url.Values{"filter[mobile-number]": {NormalizarTelefone(phone)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BFFURL+"/building-operation/companies/"+url.PathEscape(c.CompanyID)+"?"+q.Encode(), nil)
	if err != nil {
		return id, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return id, fmt.Errorf("gruvi bff: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		return id, fmt.Errorf("gruvi bff: HTTP %d: %s", res.StatusCode, truncar(string(raw), 300))
	}
	var corpo any
	if err := json.Unmarshal(raw, &corpo); err != nil {
		return id, fmt.Errorf("gruvi bff: resposta não é JSON")
	}
	data, _ := desembrulhar(corpo).(map[string]any)
	id.Bruto = data
	id.CompanyName = primeiraString(data, "name", "trade_name", "company_name")
	for _, u := range lista(data["units"]) {
		if res, ok := u["resident"].(map[string]any); ok && id.Name == "" {
			id.Name = str(res["name"])
		}
		if fp := str(u["id_fastpass"]); fp != "" {
			id.Units = append(id.Units, Unidade{ID: fp, Label: rotuloUnidade(u), Bloco: str(u["block"])})
		}
	}
	if len(id.Units) > 0 {
		id.Authenticated = true
		id.PersonID = "gruvi:" + NormalizarTelefone(phone)
		id.Role = "morador"
	}
	return id, nil
}

var naoDigitos = regexp.MustCompile(`\D`)

// NormalizarTelefone: só dígitos, com 55 e o nono dígito.
func NormalizarTelefone(p string) string {
	d := naoDigitos.ReplaceAllString(p, "")
	if len(d) <= 11 {
		d = "55" + d
	}
	if len(d) == 12 {
		d = d[:4] + "9" + d[4:]
	}
	return d
}

func desembrulhar(v any) any {
	if m, ok := v.(map[string]any); ok {
		if d, ok := m["data"]; ok {
			return d
		}
	}
	return v
}

func lista(v any) []map[string]any {
	var out []map[string]any
	if arr, ok := v.([]any); ok {
		for _, it := range arr {
			if m, ok := it.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

func rotuloUnidade(u map[string]any) string {
	unit, block := str(u["unit"]), str(u["block"])
	if block != "" && unit != "" && !strings.Contains(strings.ToLower(unit), strings.ToLower(block)) {
		return block + " - " + unit
	}
	if unit == "" {
		return block
	}
	return unit
}

func str(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return fmt.Sprint(v)
}

func primeiraString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := str(m[k]); s != "" {
			return s
		}
	}
	return ""
}

func truncar(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
