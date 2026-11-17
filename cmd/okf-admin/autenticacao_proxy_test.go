package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-concierge/internal/acesso"
)

func TestAutenticadorProxy(t *testing.T) {
	casos := []struct {
		nome      string
		cabecalho []string // valores de X-Authenticated-User-Email; nil = ausente
		dev       string   // valor de X-Dev-Usuario
		quer      string
		querErro  bool
	}{
		{nome: "email valido", cabecalho: []string{"ana@superlogica.com"}, quer: "ana@superlogica.com"},
		{nome: "espacos e maiusculas", cabecalho: []string{"  Ana.Silva@SuperLogica.COM \t"}, quer: "ana.silva@superlogica.com"},
		{nome: "ausente", querErro: true},
		{nome: "vazio", cabecalho: []string{""}, querErro: true},
		{nome: "so espacos", cabecalho: []string{"   "}, querErro: true},
		{nome: "sem arroba", cabecalho: []string{"ana"}, querErro: true},
		{nome: "sem dominio", cabecalho: []string{"ana@"}, querErro: true},
		{nome: "com nome de exibicao", cabecalho: []string{"Ana <ana@superlogica.com>"}, querErro: true},
		{nome: "entre colchetes", cabecalho: []string{"<ana@superlogica.com>"}, querErro: true},
		{nome: "dois enderecos", cabecalho: []string{"ana@superlogica.com, bia@superlogica.com"}, querErro: true},
		{nome: "espaco no meio", cabecalho: []string{"ana silva@superlogica.com"}, querErro: true},
		{nome: "cabecalho repetido", cabecalho: []string{"ana@superlogica.com", "bia@superlogica.com"}, querErro: true},
		{nome: "grande demais", cabecalho: []string{strings.Repeat("a", 250) + "@superlogica.com"}, querErro: true},
		{nome: "usuario semeado de dev nao tem forma de email", cabecalho: []string{usuarioSemeadoDev}, querErro: true},
		{nome: "X-Dev-Usuario ignorado com email valido", cabecalho: []string{"ana@superlogica.com"}, dev: "admin-superlogica", quer: "ana@superlogica.com"},
		{nome: "X-Dev-Usuario sozinho nao autentica", dev: "ana@superlogica.com", querErro: true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/eu", nil)
			for _, v := range c.cabecalho {
				r.Header.Add(cabecalhoUsuarioProxy, v)
			}
			if c.dev != "" {
				r.Header.Set(cabecalhoUsuarioDev, c.dev)
			}
			got, err := autenticadorProxy{}.Autenticar(r)
			if c.querErro {
				if !errors.Is(err, errSemUsuario) {
					t.Fatalf("quero errSemUsuario, veio %q, %v", got, err)
				}
				return
			}
			if err != nil || got != c.quer {
				t.Fatalf("quero %q, veio %q, %v", c.quer, got, err)
			}
		})
	}
}

func TestErroDoProxyNaoVazaOEmail(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(cabecalhoUsuarioProxy, "segredo@@superlogica.com")
	_, err := autenticadorProxy{}.Autenticar(r)
	if err == nil || strings.Contains(err.Error(), "segredo") {
		t.Fatalf("erro %v não pode carregar o valor do cabeçalho", err)
	}
}

func TestNovoAutenticadorEscolhePeloModo(t *testing.T) {
	casos := []struct {
		modo string
		quer Autenticador
	}{
		{modo: "", quer: autenticadorFechado{}},
		{modo: authModeDev, quer: autenticadorDev{padrao: "u"}},
		{modo: authModeProxy, quer: autenticadorProxy{}},
	}
	for _, c := range casos {
		if got := novoAutenticador(c.modo, "u"); got != c.quer {
			t.Errorf("modo %q: %#v, quer %#v", c.modo, got, c.quer)
		}
	}
}

// pedirComoProxy faz a requisição só com o cabeçalho do proxy (e, se dev != "", o de dev).
func pedirComoProxy(amb ambienteRotas, email, dev string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/api/eu", nil)
	if email != "" {
		r.Header.Set(cabecalhoUsuarioProxy, email)
	}
	if dev != "" {
		r.Header.Set(cabecalhoUsuarioDev, dev)
	}
	w := httptest.NewRecorder()
	amb.handler.ServeHTTP(w, r)
	return w
}

func TestRotasEmModoProxy(t *testing.T) {
	amb := novoAmbienteRotas(t)
	atribuicoes := usuariosTeste()
	atribuicoes["ana@superlogica.com"] = atribuicoes["admin"]
	amb.s.acessos = acessosFake{atribuicoes: atribuicoes, mapeamento: amb.repo}
	amb.s.autenticador, amb.s.authMode = autenticadorProxy{}, authModeProxy

	casos := []struct {
		nome, email, dev string
		quer             int
	}{
		{nome: "cadastrado", email: "Ana@Superlogica.com ", quer: http.StatusOK},
		{nome: "sem cadastro", email: "bia@superlogica.com", quer: http.StatusUnauthorized},
		{nome: "sem cabecalho", quer: http.StatusUnauthorized},
		{nome: "X-Dev-Usuario nao vale", dev: "admin", quer: http.StatusUnauthorized},
		{nome: "X-Dev-Usuario nao troca o email", email: "bia@superlogica.com", dev: "admin", quer: http.StatusUnauthorized},
		{nome: "semeado de dev no cabecalho do proxy", email: usuarioSemeadoDev, quer: http.StatusUnauthorized},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := pedirComoProxy(amb, c.email, c.dev)
			if w.Code != c.quer {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if w.Code == http.StatusUnauthorized && strings.Contains(w.Body.String(), `"dev"`) {
				t.Fatalf("o 401 fora de dev não deve oferecer o Ver como: %s", w.Body.String())
			}
		})
	}
}

func TestGarantirAdminInicial(t *testing.T) {
	type gravado struct {
		usuario acesso.Usuario
		atrib   acesso.Atribuicao
	}
	querAtrib := acesso.Atribuicao{PapelID: "admin_superlogica", Escopo: acesso.Escopo{Tipo: acesso.TipoSuperlogica}}
	erroBanco := errors.New("banco fora")

	casos := []struct {
		nome     string
		bruto    string
		falha    error
		querErro error
		quer     []gravado
	}{
		{nome: "vazio nao cria nada", bruto: ""},
		{nome: "email valido", bruto: "ana@superlogica.com",
			quer: []gravado{{acesso.Usuario{ID: "ana@superlogica.com", Nome: "ana@superlogica.com"}, querAtrib}}},
		{nome: "normaliza antes de gravar", bruto: " Ana@SuperLogica.com ",
			quer: []gravado{{acesso.Usuario{ID: "ana@superlogica.com", Nome: "ana@superlogica.com"}, querAtrib}}},
		{nome: "email invalido", bruto: "ana", querErro: errAdminInicialInvalido},
		{nome: "erro do banco sobe", bruto: "ana@superlogica.com", falha: erroBanco, querErro: erroBanco},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var got []gravado
			atribuir := func(_ context.Context, u acesso.Usuario, at acesso.Atribuicao) error {
				if c.falha != nil {
					return c.falha
				}
				got = append(got, gravado{u, at})
				return nil
			}
			err := garantirAdminInicial(t.Context(), c.bruto, atribuir)
			if !errors.Is(err, c.querErro) {
				t.Fatalf("erro %v, quer %v", err, c.querErro)
			}
			if len(got) != len(c.quer) || (len(got) == 1 && got[0] != c.quer[0]) {
				t.Fatalf("gravado %+v, quer %+v", got, c.quer)
			}
		})
	}
}

// Repetir o boot grava o mesmo usuário e a mesma atribuição: quem garante a idempotência no
// banco é Atribuir (ON CONFLICT DO NOTHING); aqui a chamada precisa ser sempre idêntica.
func TestGarantirAdminInicialRepeteAMesmaChamada(t *testing.T) {
	chamadas := map[string]acesso.Atribuicao{}
	n := 0
	atribuir := func(_ context.Context, u acesso.Usuario, at acesso.Atribuicao) error {
		chamadas[u.ID] = at
		n++
		return nil
	}
	for range 3 {
		if err := garantirAdminInicial(t.Context(), "ana@superlogica.com", atribuir); err != nil {
			t.Fatal(err)
		}
	}
	if n != 3 || len(chamadas) != 1 {
		t.Fatalf("%d chamadas para %d usuários, quer 3 para 1", n, len(chamadas))
	}
}
