package conciergeapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-concierge/internal/acesso"
	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/dados"
	canaispg "go-concierge/internal/core/dados/postgres/canais"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	"go-concierge/internal/identidade"
)

func TestIdentidadeInternaVemDaAutenticacao(t *testing.T) {
	amb := novoAmbienteRotas(t)
	chamou := false
	h := amb.s.autenticado(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamou = true
		usuario, ok := identidade.Usuario(r.Context())
		if !ok || usuario != "gestor-a" {
			t.Errorf("usuário=%q presente=%v", usuario, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(cabecalhoUsuarioDev, "gestor-a")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent || !chamou {
		t.Fatalf("status=%d chamou=%v", w.Code, chamou)
	}
	chamou = false
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(cabecalhoUsuarioProxy, "gestor-a")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized || chamou {
		t.Fatalf("cabeçalho não autenticado: status=%d chamou=%v", w.Code, chamou)
	}
}

func TestAutenticacaoCarregaAcessoPorHTTP(t *testing.T) {
	amb := novoAmbienteRotas(t)
	b := bancotest.Novo(t)
	autoridade, err := identidade.NovaAutoridade()
	if err != nil {
		t.Fatal(err)
	}
	privado := dados.NovosAcessosPrivados(autoridade.Verificador(), amb.s.acessos, amb.s.implantacao)
	server, cliente, _, err := dados.IniciarCompleto(dados.NovoGravadorPostgres(b), canaispg.NovoPostgres(b.Pool), sessoespg.NovoSessaoPostgres(b), dados.NovasMemoriasPostgres(b), dados.NovosPedidosPostgres(b), dados.NovaAuditoriaPostgres(b), privado)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Fechar(context.Background()) })
	amb.s.autoridade, amb.s.acessos = autoridade, cliente
	h := amb.s.autenticado(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prova, ok := identidade.Prova(r.Context())
		usuario, valida := autoridade.Verificador().Verificar(prova)
		a, presente := acessoDe(r.Context())
		if !ok || !valida || usuario != "gestor-a" || !presente || !acesso.Permitido(a, acesso.PedidosVer, acesso.Escopo{Tipo: acesso.TipoAdministradora, ID: "adm-a"}) {
			t.Errorf("prova=%v válida=%v usuário=%q acesso=%v", ok, valida, usuario, presente)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(cabecalhoUsuarioDev, "gestor-a")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status=%d", w.Code)
	}
	if err := server.Fechar(context.Background()); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("servidor de dados desligado: status=%d", w.Code)
	}
}
