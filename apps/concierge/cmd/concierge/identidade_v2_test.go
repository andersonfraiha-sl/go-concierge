package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
	"go-concierge/internal/escopo"
)

func TestEmpresaDaConsultaPreservaIdentidadeNexus(t *testing.T) {
	for _, tc := range []struct {
		nome, consulta, empresa string
		erro                    bool
	}{
		{"canonica", "company_id=Company-ABC", "Company-ABC", false},
		{"alias", "administradora=Company-ABC", "Company-ABC", false},
		{"iguais", "company_id=Company-ABC&administradora=Company-ABC", "Company-ABC", false},
		{"conflito", "company_id=Company-ABC&administradora=company-abc", "", true},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/?"+tc.consulta, nil)
			got, err := administradoraPedida(r, escopo.Implantacao{})
			if (err != nil) != tc.erro || got != tc.empresa {
				t.Fatalf("empresa=%q erro=%v; esperado %q erro=%v", got, err, tc.empresa, tc.erro)
			}
		})
	}
}

func TestAlvoCondominialExigeEmpresaERecusaConflitos(t *testing.T) {
	for _, tc := range []struct {
		nome, consulta, corpo string
		status                int
	}{
		{"sem_empresa", "", `{"condo_id":101}`, 400},
		{"conflito_empresa", "company_id=A", `{"company_id":"B","condo_id":101}`, 400},
		{"conflito_condominio", "company_id=A", `{"condominio":"102","condo_id":101}`, 400},
		{"id_local", "company_id=A", `{"condominio":"cond-001"}`, 400},
		{"canonico", "company_id=Company-ABC", `{"condo_id":101}`, 0},
	} {
		t.Run(tc.nome, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/?"+tc.consulta, strings.NewReader(tc.corpo))
			alvo, err := alvoCondominioDoCorpo(r)
			if tc.status == 0 {
				company, condo, ok := acesso.CondominioDoEscopo(alvo)
				if err != nil || !ok || company != "Company-ABC" || condo != "101" {
					t.Fatalf("alvo=%+v erro=%v", alvo, err)
				}
				return
			}
			var e erroDeAlvo
			if !errors.As(err, &e) || e.status != tc.status {
				t.Fatalf("erro=%v; esperado status %d", err, tc.status)
			}
		})
	}
}

func TestVisaoCondominialNaoIncluiMesmoIDDeOutraEmpresa(t *testing.T) {
	a := acesso.Acesso{
		Papeis: map[string]acesso.Papel{"operador": {ID: "operador", Permissoes: []acesso.Permissao{acesso.SkillsLer}}},
		Atribuicoes: []acesso.Atribuicao{{
			PapelID: "operador", AdministradoraID: "Company-A", Escopo: acesso.EscopoCondominio("Company-A", "101"),
		}},
	}
	m := admin.Mapeamento{
		Administradoras: []admin.Administradora{{ID: "Company-A"}, {ID: "Company-B"}},
		Condominios: []admin.Condominio{
			{ID: "101", Administradora: "Company-A", Nome: "Alpha"},
			{ID: "101", Administradora: "Company-B", Nome: "Beta"},
		},
	}
	r := httptest.NewRequest(http.MethodGet, "/api/admin/estado", nil)
	r = r.WithContext(context.WithValue(r.Context(), chaveAcesso{}, a))
	v, _, err := visaoDoEstado(r, m)
	if err != nil || len(v.Condominios) != 1 || v.Condominios[0].Administradora != "Company-A" {
		t.Fatalf("visão=%+v erro=%v", v, err)
	}
	r = httptest.NewRequest(http.MethodGet, "/?company_id=Company-B", nil)
	r = r.WithContext(context.WithValue(r.Context(), chaveAcesso{}, a))
	if _, _, err := visaoDoEstado(r, m); err == nil {
		t.Fatal("consulta explícita à empresa alheia deveria ser recusada")
	}
}

func TestAlvoSessaoSemEmpresaRecusaAntesDeConsultarStore(t *testing.T) {
	s := &servidor{}
	r := httptest.NewRequest(http.MethodGet, "/api/atende/sessao?id=qualquer", nil)
	_, err := s.alvoSessaoDaConsulta(r)
	var e erroDeAlvo
	if !errors.As(err, &e) || e.status != http.StatusBadRequest {
		t.Fatalf("erro=%v; esperado company_id obrigatório antes de carregar sessão", err)
	}
}

func TestRotasDeCadastroLocalForamRetiradas(t *testing.T) {
	amb := novoAmbienteRotas(t)
	for _, path := range []string{"/api/admin/administradora", "/api/admin/condominio", "/api/admin/habilitacao"} {
		for _, metodo := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			w := amb.pedir(t, "admin", metodo, path+"?company_id=adm-a", `{}`)
			if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("rota retirada %s %s respondeu %d: %s", metodo, path, w.Code, w.Body.String())
			}
		}
	}
}
