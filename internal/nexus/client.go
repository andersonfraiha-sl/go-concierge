// Package nexus é o cliente do gateway de recursos do condomínio: identidade do
// morador, catálogo de recursos da account e invocação com idempotência. O mesmo
// contrato do sl-concierge (e do seu nexus-mock). Sem NEXUS_BASE_URL, o catálogo de
// recursos vem de um instantâneo em arquivo (okf/nexus/<account>.yaml), para a
// interface e o harness funcionarem sem o Nexus no ar.
package nexus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go-concierge/internal/okf"
)

// Client fala HTTP com o Nexus.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New cria um cliente; baseURL vazia = sem Nexus (só instantâneo).
func New(baseURL, token string) *Client {
	return &Client{BaseURL: baseURL, Token: token, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// Identity é o resultado do AUTH.
type Identity struct {
	Authenticated bool   `json:"authenticated"`
	PersonID      string `json:"person_id,omitempty"`
	Name          string `json:"name,omitempty"`
	Phone         string `json:"phone"`
	AccountID     string `json:"account_id"`
	AccountName   string `json:"account_name,omitempty"`
	Role          string `json:"role,omitempty"`
	Units         []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"units,omitempty"`
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	if c.BaseURL == "" {
		return fmt.Errorf("nexus: NEXUS_BASE_URL não configurado")
	}
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		return &Erro{Status: res.StatusCode, Corpo: string(raw)}
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// Erro é uma resposta não-2xx (4xx = recusa de negócio; 5xx = falha técnica).
type Erro struct {
	Status int
	Corpo  string
}

func (e *Erro) Error() string { return fmt.Sprintf("nexus: HTTP %d: %s", e.Status, e.Corpo) }

// Identity resolve o telefone (AUTH). Sem cadastro, vem Authenticated=false.
func (c *Client) Identity(ctx context.Context, phone, defaultAccount string) (Identity, error) {
	var id Identity
	err := c.do(ctx, http.MethodGet, "/v1/identity?phone="+url.QueryEscape(phone)+"&account="+url.QueryEscape(defaultAccount), nil, &id)
	return id, err
}

// Recursos lista o catálogo da account.
func (c *Client) Recursos(ctx context.Context, account string) ([]okf.Recurso, error) {
	var out []okf.Recurso
	err := c.do(ctx, http.MethodGet, "/v1/accounts/"+url.PathEscape(account)+"/resources", nil, &out)
	return out, err
}

// Invoke chama um recurso com chave de idempotência.
func (c *Client) Invoke(ctx context.Context, name, account string, params map[string]any, idem string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.do(ctx, http.MethodPost, "/v1/resources/"+url.PathEscape(name)+"/invoke", map[string]any{
		"account_id": account, "params": params, "idempotency_key": idem,
	}, &out)
	return out, err
}

// Fonte devolve a função de recursos que o okf.Catalogo usa: tenta o Nexus (com
// cache de 30 s) e, sem Nexus ou com falha, o instantâneo em <dirFixtures>/<account>.yaml.
// Devolve nil se não há nem Nexus nem instantâneo, para o catálogo saber que não
// pode conferir declarações.
func Fonte(c *Client, account, dirFixtures string) func() []okf.Recurso {
	fixture := filepath.Join(dirFixtures, account+".yaml")
	_, temFixture := os.Stat(fixture)
	if (c == nil || c.BaseURL == "") && temFixture != nil {
		return nil
	}
	var (
		mu     sync.Mutex
		cache  []okf.Recurso
		quando time.Time
		mtime  time.Time
	)
	return func() []okf.Recurso {
		mu.Lock()
		defer mu.Unlock()
		if cache != nil && time.Since(quando) < 30*time.Second {
			// instantâneo editado pela interface: relê na hora
			if st, err := os.Stat(fixture); err != nil || st.ModTime().Equal(mtime) {
				return cache
			}
		}
		if c != nil && c.BaseURL != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			lista, err := c.Recursos(ctx, account)
			cancel()
			if err == nil {
				cache, quando = lista, time.Now()
				return cache
			}
		}
		if lista, err := okf.LerRecursosFixture(fixture); err == nil {
			cache, quando = lista, time.Now()
			if st, err := os.Stat(fixture); err == nil {
				mtime = st.ModTime()
			}
			return cache
		}
		return cache
	}
}
