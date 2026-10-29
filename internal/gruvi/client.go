// Package gruvi fala com a plataforma Gruvi (staging/produção) para identificar o
// morador que chegou pelo ATENDE por meio de uma consulta ao BFF com token B2C.
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

// Unidade do morador no condomínio.
type Unidade struct {
	ID         string `json:"id"` // id_fastpass (o id que as operações usam); sem ele, o id do BFF
	Label      string `json:"label"`
	Bloco      string `json:"bloco,omitempty"`
	Condominio string `json:"condominio,omitempty"` // nome do condomínio dono da unidade
}

// Condominio é um condomínio em que o telefone tem unidades. O BFF devolve um
// (data como objeto) ou vários (data como lista).
type Condominio struct {
	Nome     string    `json:"nome"`
	Unidades []Unidade `json:"unidades"`
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
	Units         []Unidade `json:"units"` // todas as unidades, de todos os condomínios
	// Condominios agrupa as unidades por condomínio: com mais de um condomínio ou mais
	// de uma unidade, o concierge pergunta de qual se trata o pedido.
	Condominios []Condominio `json:"condominios"`
	// Bruto é a resposta do BFF (data), para a bancada do emulador.
	Bruto map[string]any `json:"bruto,omitempty"`
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
			uid := str(u["id_fastpass"])
			if uid == "" {
				uid = str(u["id"])
			}
			if uid == "" {
				continue
			}
			un := Unidade{ID: uid, Label: rotuloUnidade(u), Bloco: str(u["block"]), Condominio: nome}
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

// telefoneEm: algum documento MOBILE_NUMBER da lista bate com o telefone normalizado.
func telefoneEm(docs any, tel string) bool {
	for _, d := range lista(docs) {
		if strings.EqualFold(str(d["type"]), "MOBILE_NUMBER") && NormalizarTelefone(str(d["value"])) == tel {
			return true
		}
	}
	return false
}

// papelDe traduz o cargo do responsável legal no papel do Admin (sindico, proprietario, morador).
func papelDe(cargo string) string {
	c := strings.ToLower(cargo)
	switch {
	case strings.Contains(c, "síndic") || strings.Contains(c, "sindic"):
		return "sindico"
	case strings.Contains(c, "propriet"):
		return "proprietario"
	}
	return "morador"
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
	if str(u["unit"]) == "" && str(u["block"]) == "" {
		if ext := str(u["id_external_gruvi"]); ext != "" {
			return "unidade " + ext
		}
		return "unidade"
	}
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
