package gruvi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-concierge/internal/falha"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	BFFURLPadrao = "https://b2c-nexus.superlogica.net/api/gruvi"
)

type TokenProvider interface {
	Token(context.Context) (string, error)
}

type Client struct {
	BFFURL    string
	Token     TokenProvider
	CompanyID string
	Telefone  string
	HTTP      *http.Client
}

func New(bffURL, company, telefone string, token TokenProvider) *Client {
	if bffURL == "" {
		bffURL = BFFURLPadrao
	}
	return &Client{BFFURL: strings.TrimRight(bffURL, "/"), Token: token, CompanyID: strings.TrimSpace(company), Telefone: strings.TrimSpace(telefone), HTTP: &http.Client{Timeout: 25 * time.Second}}
}

func (c *Client) Configurado() bool {
	return c != nil && c.Token != nil && c.CompanyID != "" && c.Telefone != ""
}

func (c *Client) PermiteTelefone(phone string) bool {
	return c.Configurado() && strings.TrimSpace(phone) != "" && NormalizarTelefone(phone) == NormalizarTelefone(c.Telefone)
}

// Identidade consulta o BFF pelo telefone (API1 do contrato).
func (c *Client) Identidade(ctx context.Context, phone string) (Identidade, error) {
	id := Identidade{Phone: phone, CompanyID: c.CompanyID, Units: []Unidade{}, Condominios: []Condominio{}}
	if !c.Configurado() {
		return id, errors.New("gruvi: token B2C e GRUVI_COMPANY_ID não configurados")
	}
	if !c.PermiteTelefone(phone) {
		return id, errors.New("gruvi: telefone diferente do morador configurado para o token B2C")
	}
	tok, err := c.Token.Token(ctx)
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
		return id, falha.ComPilha(fmt.Errorf("gruvi bff: %w", err))
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		return id, falha.ComPilha(fmt.Errorf("gruvi bff: HTTP %d: %s", res.StatusCode, truncar(string(raw), 300)))
	}
	var corpo any
	if err := json.Unmarshal(raw, &corpo); err != nil {
		return id, fmt.Errorf("gruvi bff: resposta não é JSON")
	}
	data := desembrulhar(corpo)
	// data é um condomínio (objeto) ou vários (lista)
	var condos []map[string]any
	switch d := data.(type) {
	case map[string]any:
		condos = []map[string]any{d}
		id.Bruto = d
	case []any:
		condos = lista(d)
		id.Bruto = map[string]any{"data": d}
	}
	tel := NormalizarTelefone(phone)
	for _, cd := range condos {
		nome := ""
		if cm, ok := cd["condominium"].(map[string]any); ok {
			nome = str(cm["name"])
		}
		if nome == "" {
			nome = primeiraString(cd, "name", "trade_name", "company_name")
		}
		if id.CompanyName == "" {
			id.CompanyName = nome
		}
		condo := Condominio{Nome: nome, Unidades: []Unidade{}}
		for _, u := range lista(cd["units"]) {
			if res, ok := u["resident"].(map[string]any); ok && id.Name == "" {
				id.Name = str(res["name"])
			}
			interno := str(u["id"])
			fastpass := str(u["id_fastpass"])
			uid := fastpass
			if uid == "" {
				uid = interno
			}
			if uid == "" {
				continue
			}
			un := Unidade{ID: uid, IDInterno: interno, Label: rotuloUnidade(u), Bloco: str(u["block"]), Condominio: nome}
			condo.Unidades = append(condo.Unidades, un)
			id.Units = append(id.Units, un)
		}
		// responsável legal (síndico etc.) com este telefone: o papel sobe
		for _, lr := range lista(cd["legal_responsibles"]) {
			if telefoneEm(lr["documents"], tel) {
				if id.Name == "" {
					id.Name = str(lr["name"])
				}
				if pos, ok := lr["position"].(map[string]any); ok && str(pos["name"]) != "" {
					id.Role = papelDe(str(pos["name"]))
				}
			}
		}
		if len(condo.Unidades) > 0 {
			id.Condominios = append(id.Condominios, condo)
		}
	}
	if len(id.Units) > 0 {
		id.Authenticated = true
		id.PersonID = "gruvi:" + tel
		if id.Role == "" {
			id.Role = "morador"
		}
	}
	return id, nil
}

func truncar(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
