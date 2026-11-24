package identificacao

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Unidade do morador no condomínio.
type Unidade struct {
	ID         string `json:"id"` // id_fastpass (o id que as operações usam); sem ele, o id do BFF
	IDInterno  string `json:"id_interno,omitempty"`
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

func IdentidadeDoResultadoMCP(phone, companyID string, data any) Identidade {
	id, _ := IdentidadeComDadosVinculadosMCP(phone, companyID, data)
	return id
}

func IdentidadeComDadosVinculadosMCP(phone, companyID string, data any) (Identidade, map[string]any) {
	tel := NormalizarTelefone(phone)
	id := Identidade{Phone: tel, CompanyID: strings.TrimSpace(companyID), Units: []Unidade{}, Condominios: []Condominio{}}
	var unidadesVinculadas []any
	var condominios []map[string]any
	switch value := data.(type) {
	case map[string]any:
		condominios = []map[string]any{value}
	case []any:
		condominios = Lista(value)
	}
	for _, raw := range condominios {
		nome := ""
		if condo, ok := raw["condominium"].(map[string]any); ok {
			nome = Str(condo["name"])
		}
		if nome == "" {
			nome = PrimeiraString(raw, "name", "trade_name", "company_name")
		}
		if id.CompanyName == "" {
			id.CompanyName = nome
		}
		responsavelVerificado := false
		for _, responsavel := range Lista(raw["legal_responsibles"]) {
			if !TelefoneEm(responsavel["documents"], tel) {
				continue
			}
			responsavelVerificado = true
			if id.Name == "" {
				id.Name = Str(responsavel["name"])
			}
			if position, ok := responsavel["position"].(map[string]any); ok && Str(position["name"]) != "" {
				id.Role = PapelDe(Str(position["name"]))
			}
		}
		condo := Condominio{Nome: nome, Unidades: []Unidade{}}
		for _, unit := range Lista(raw["units"]) {
			resident, hasResident := unit["resident"].(map[string]any)
			residentMatches := false
			if hasResident {
				residentMatches = NormalizarTelefone(Str(resident["mobile_number"])) == tel || TelefoneEm(resident["documents"], tel)
				if residentMatches && id.Name == "" {
					id.Name = Str(resident["name"])
				}
			}
			if !responsavelVerificado && !residentMatches {
				continue
			}
			interno := Str(unit["id"])
			fastpass := Str(unit["id_fastpass"])
			operacional := fastpass
			if operacional == "" {
				operacional = interno
			}
			if operacional == "" {
				continue
			}
			unidade := Unidade{ID: operacional, IDInterno: interno, Label: RotuloUnidade(unit), Bloco: Str(unit["block"]), Condominio: nome}
			condo.Unidades = append(condo.Unidades, unidade)
			id.Units = append(id.Units, unidade)
			unidadesVinculadas = append(unidadesVinculadas, unidadeMCPVinculada(unit))
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
	dadosVinculados := map[string]any{"units": unidadesVinculadas}
	if id.CompanyName != "" {
		dadosVinculados["name"] = id.CompanyName
		dadosVinculados["condominium"] = map[string]any{"name": id.CompanyName}
	}
	return id, dadosVinculados
}

func unidadeMCPVinculada(unit map[string]any) map[string]any {
	filtrada := map[string]any{}
	for _, campo := range []string{"id", "id_external_gruvi", "id_fastpass", "block", "unit", "label"} {
		if valor, existe := unit[campo]; existe {
			filtrada[campo] = valor
		}
	}
	if residente, ok := unit["resident"].(map[string]any); ok {
		if nome, existe := residente["name"]; existe {
			filtrada["resident"] = map[string]any{"name": nome}
		}
	}
	return filtrada
}

// telefoneEm: algum documento MOBILE_NUMBER da lista bate com o telefone normalizado.
func TelefoneEm(docs any, tel string) bool {
	for _, d := range Lista(docs) {
		if strings.EqualFold(Str(d["type"]), "MOBILE_NUMBER") && NormalizarTelefone(Str(d["value"])) == tel {
			return true
		}
	}
	return false
}

// papelDe traduz o cargo do responsável legal no papel do Admin (sindico, proprietario, morador).
func PapelDe(cargo string) string {
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

func Desembrulhar(v any) any {
	if m, ok := v.(map[string]any); ok {
		if d, ok := m["data"]; ok {
			return d
		}
	}
	return v
}

func Lista(v any) []map[string]any {
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

func RotuloUnidade(u map[string]any) string {
	if Str(u["unit"]) == "" && Str(u["block"]) == "" {
		if ext := Str(u["id_external_gruvi"]); ext != "" {
			return "unidade " + ext
		}
		return "unidade"
	}
	unit, block := Str(u["unit"]), Str(u["block"])
	if block != "" && unit != "" && !strings.Contains(strings.ToLower(unit), strings.ToLower(block)) {
		return block + " - " + unit
	}
	if unit == "" {
		return block
	}
	return unit
}

func Str(v any) string {
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

func PrimeiraString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := Str(m[k]); s != "" {
			return s
		}
	}
	return ""
}
