package concierge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"go-concierge/internal/core/api/execucao"
	"go-concierge/internal/core/catalogo"
	"go-concierge/internal/harness/canais"
)

type transporteTeste func(*http.Request) (*http.Response, error)

func (f transporteTeste) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestConfiguracaoPublicaProtegeCredenciaisDestinoEResposta(t *testing.T) {
	id := uuid.New()
	config := execucao.Configuracao{Canal: canais.Canal{ID: id, CompanyID: "empresa", Habilitado: true}, Casos: []canais.Caso{}, Arquivos: []catalogo.Arquivo{}}
	chamadas := 0
	redirecionar := false
	cliente := Cliente{BaseURL: "http://localhost:8090", Token: "token-teste", UsuarioDev: "admin-superlogica"}
	cliente.HTTP = &http.Client{Transport: transporteTeste(func(req *http.Request) (*http.Response, error) {
		chamadas++
		if req.URL.Host != "localhost:8090" || req.Header.Get("Authorization") != "Bearer token-teste" || req.Header.Get("X-Dev-Usuario") != "admin-superlogica" || req.URL.Query().Get("company_id") != "empresa" || req.URL.Query().Get("channel_id") != id.String() {
			t.Fatal("requisição fora do contrato público autenticado")
		}
		if redirecionar {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://destino-invalido.example"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
		corpo, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(corpo))), Request: req}, nil
	})}
	for _, destino := range []string{"https://destino-invalido.example", "http://localhost:8091", "http://localhost:8090@destino-invalido.example", "http://usuario@localhost:8090"} {
		invalido := cliente
		invalido.BaseURL = destino
		if _, err := invalido.Configuracao(context.Background(), "empresa", id); err == nil {
			t.Fatalf("destino não autorizado aceito: %s", destino)
		}
	}
	if chamadas != 0 {
		t.Fatal("credenciais chegaram a destino não autorizado")
	}
	if obtida, err := cliente.Configuracao(t.Context(), "empresa", id); err != nil || obtida.Canal.ID != id || chamadas != 1 {
		t.Fatalf("configuração válida: %+v erro=%v chamadas=%d", obtida, err, chamadas)
	}
	config.Canal.CompanyID = "outra"
	if _, err := cliente.Configuracao(t.Context(), "empresa", id); err == nil {
		t.Fatal("resposta de outra empresa aceita")
	}
	redirecionar = true
	antes := chamadas
	if _, err := cliente.Configuracao(t.Context(), "empresa", id); err == nil || chamadas != antes+1 {
		t.Fatalf("redirect seguido: erro=%v chamadas=%d", err, chamadas)
	}
}
