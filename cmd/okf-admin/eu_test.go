package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestEu(t *testing.T) {
	amb := novoAmbienteRotas(t)
	casos := []struct {
		usuario          string
		status           int
		papel            string
		administradoraID *string
		tem, naoTem      []string
	}{
		{usuario: "admin", status: 200, papel: "admin_superlogica", administradoraID: nil, tem: []string{"skills.criar", "studio.avancado", "administradoras.editar"}},
		{usuario: "construtor-a", status: 200, papel: "construtor_superlogica", administradoraID: ptr("adm-a"), tem: []string{"skills.editar_fluxo", "studio.avancado"}},
		{usuario: "gestor-a", status: 200, papel: "gestor_administradora", administradoraID: ptr("adm-a"), tem: []string{"condominios.habilitar"}, naoTem: []string{"studio.avancado", "skills.criar"}},
		{usuario: "sem-papel", status: 200, papel: "", administradoraID: nil, naoTem: []string{"skills.ler"}},
		{usuario: "gestor-ab", status: 409},
	}
	for _, c := range casos {
		t.Run(c.usuario, func(t *testing.T) {
			w := amb.pedir(t, c.usuario, "GET", "/api/eu", "")
			if w.Code != c.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if c.status == http.StatusConflict {
				if !strings.Contains(w.Body.String(), "ADMINISTRADORA_AMBIGUA") {
					t.Fatalf("corpo = %s", w.Body.String())
				}
				return
			}
			if strings.Contains(w.Body.String(), `"permissoes":null`) {
				t.Fatalf("permissoes null: %s", w.Body.String())
			}
			var got eu
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.ID != c.usuario || got.Nome != "Nome de "+c.usuario || got.Papel != c.papel || !got.Dev {
				t.Fatalf("eu = %+v", got)
			}
			if (got.AdministradoraID == nil) != (c.administradoraID == nil) || (got.AdministradoraID != nil && *got.AdministradoraID != *c.administradoraID) {
				t.Fatalf("administradoraId = %v, quer %v", got.AdministradoraID, c.administradoraID)
			}
			for _, p := range c.tem {
				if !slices.Contains(got.Permissoes, p) {
					t.Errorf("falta %s em %v", p, got.Permissoes)
				}
			}
			for _, p := range c.naoTem {
				if slices.Contains(got.Permissoes, p) {
					t.Errorf("sobra %s em %v", p, got.Permissoes)
				}
			}
		})
	}
}

func TestEuSemUsuarioDa401(t *testing.T) {
	w := novoAmbienteRotas(t).pedir(t, "", "GET", "/api/eu", "")
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "NAO_AUTENTICADO") {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
}

func TestEuIgnoraCabecalhoDePapel(t *testing.T) {
	amb := novoAmbienteRotas(t)
	r := httptest.NewRequest("GET", "/api/eu", nil)
	r.Header.Set("X-Dev-Usuario", "gestor-a")
	r.Header.Set("X-Studio-Papel", "construtor")
	w := httptest.NewRecorder()
	amb.handler.ServeHTTP(w, r)
	var got eu
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Papel != "gestor_administradora" || got.AdministradoraID == nil || *got.AdministradoraID != "adm-a" || slices.Contains(got.Permissoes, "skills.criar") {
		t.Fatalf("eu = %+v", got)
	}
}
