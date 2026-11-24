package atende

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go-concierge/internal/canais"
	"go-concierge/internal/gruvi"
)

var errContextoExterno = errors.New("não foi possível comprovar o contexto condominial da integração")

func restringirIdentidadeGruvi(ctx context.Context, dados any) (any, error) {
	c, ok := canalDoContexto(ctx)
	if !ok || c.CondoID == nil {
		return dados, nil
	}
	if c.Integracoes.GruviCondominiumID == "" {
		return nil, errContextoExterno
	}
	var entradas []any
	switch v := dados.(type) {
	case []any:
		entradas = v
	case map[string]any:
		entradas = []any{v}
	default:
		return nil, errContextoExterno
	}
	out := []any{}
	for _, v := range entradas {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		condo, ok := m["condominium"].(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(condo["id"]) == c.Integracoes.GruviCondominiumID {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, errContextoExterno
	}
	return out, nil
}

func validarFerramentaDoCanal(c canais.Canal, id gruvi.Identidade, cache map[string]any) func(string, map[string]any) error {
	if c.CondoID == nil {
		return nil
	}
	unidades := map[string]bool{}
	for _, u := range id.Units {
		unidades[u.ID] = true
		if u.IDInterno != "" {
			unidades[u.IDInterno] = true
		}
	}
	return func(nome string, params map[string]any) error {
		company, ok := params["companyId"]
		if !ok || c.Integracoes.GruviCompanyID == "" || fmt.Sprint(company) != c.Integracoes.GruviCompanyID {
			return errContextoExterno
		}
		if _, ok := cache[nome]; ok {
			return nil
		}
		if !strings.HasPrefix(nome, "gruvi.bff.") {
			return errContextoExterno
		}
		parametro := map[string]string{
			"gruvi.bff.building-operation.fetch-unit-packages-controller-handle":                 "unitId",
			"gruvi.bff.building-operation.get-package-delivery-code-controller-handle":           "unitId",
			"gruvi.bff.building-operation.create-visit-controller-handle":                        "unitId",
			"gruvi.bff.mobile.get-parameters-controller-handle-get-ticket-parameters-c4fee62606": "filter[unit.id]",
			"gruvi.bff.mobile.create-ticket-controller-handle":                                   "unit_id",
		}[nome]
		if parametro == "" || !unidades[fmt.Sprint(params[parametro])] {
			return errContextoExterno
		}
		vinculado := false
		for _, chave := range []string{"unitId", "unit_id", "id_unit", "unit", "filter[unit.id]"} {
			if v, ok := params[chave]; ok {
				if !unidades[fmt.Sprint(v)] {
					return errContextoExterno
				}
				vinculado = true
			}
		}
		if !vinculado {
			return errContextoExterno
		}
		if v, ok := params["companyId"]; ok && fmt.Sprint(v) != c.Integracoes.GruviCompanyID {
			return errContextoExterno
		}
		return nil
	}
}
