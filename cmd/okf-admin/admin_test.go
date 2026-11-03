package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-concierge/internal/admin"
)

func TestSemearMapeamento(t *testing.T) {
	exemplo := admin.Condominio{ID: "cond-001", Nome: "Condomínio exemplo", Administradora: "adm-exemplo", Account: "cond-001"}
	casos := []struct {
		nome          string
		ambiente      string
		exemplo       admin.Condominio
		jaTem         bool
		querCondos    int
		querAdministr int
	}{
		{nome: "dev vazio semeia", ambiente: "", exemplo: exemplo, querCondos: 1, querAdministr: 1},
		{nome: "dev explícito semeia", ambiente: "dev", exemplo: exemplo, querCondos: 1, querAdministr: 1},
		{nome: "fora de dev não grava", ambiente: "prod", exemplo: exemplo},
		{nome: "account fora da regra não derruba nem grava", ambiente: "dev", exemplo: admin.Condominio{ID: "Cond 001", Nome: "X", Administradora: "adm-exemplo"}},
		{nome: "mapeamento existente fica como está", ambiente: "dev", exemplo: exemplo, jaTem: true, querAdministr: 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			repo := admin.NovaEmMemoria()
			if c.jaTem {
				if _, err := repo.SalvarAdministradora(t.Context(), admin.Administradora{ID: "adm-real", Nome: "Real"}); err != nil {
					t.Fatal(err)
				}
			}
			if err := semearMapeamento(repo, c.ambiente, c.exemplo); err != nil {
				t.Fatal(err)
			}
			m, err := repo.Ler(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Condominios) != c.querCondos || len(m.Administradoras) != c.querAdministr {
				t.Fatalf("condomínios %+v, administradoras %+v", m.Condominios, m.Administradoras)
			}
			if c.querCondos == 1 && m.Condominios[0].Administradora != "adm-exemplo" {
				t.Fatalf("exemplo sem dona: %+v", m.Condominios[0])
			}
		})
	}
}

func TestResponderErroAdmin(t *testing.T) {
	textoDoDriver := "ERROR: could not serialize access (SQLSTATE 40001)"
	casos := []struct {
		nome       string
		err        error
		querStatus int
		querCodigo string
	}{
		{nome: "regra recusa", err: fmt.Errorf("salvar: %w: nome vazio", admin.ErrInvalido), querStatus: http.StatusBadRequest},
		{nome: "conflito concorrente", err: fmt.Errorf("salvar: %w: %w", admin.ErrConflitoConcorrente, errors.New(textoDoDriver)), querStatus: http.StatusServiceUnavailable, querCodigo: "CONFLITO_CONCORRENTE"},
		{nome: "falha do servidor", err: errors.New("banco fora"), querStatus: http.StatusInternalServerError},
	}
	s := &servidor{log: log.New(io.Discard, "", 0)}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			s.responderErroAdmin(w, c.err)
			var corpo map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
				t.Fatal(err)
			}
			if w.Code != c.querStatus || corpo["codigo"] != c.querCodigo {
				t.Fatalf("status %d, corpo %v", w.Code, corpo)
			}
			if strings.Contains(w.Body.String(), "SQLSTATE") {
				t.Fatalf("texto do driver vazou: %s", w.Body.String())
			}
		})
	}
}
