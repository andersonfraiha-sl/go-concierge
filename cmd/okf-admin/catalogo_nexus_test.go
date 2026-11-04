package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"go-concierge/internal/catalogonexus"
	"go-concierge/internal/mcp"
	"go-concierge/internal/okf"
)

type fakeCatalogoNexusCaller struct {
	names []string
	args  []map[string]any
}

func (f *fakeCatalogoNexusCaller) CallTool(_ context.Context, name string, args map[string]any) (*mcp.CallResult, error) {
	f.names = append(f.names, name)
	f.args = append(f.args, args)
	text := "1 operação(ões) para \"listar itens\".\n- GET /items · ListItems · Listar itens\n  serviço: API de Exemplo · serviceId: service-1 · score 0.9"
	if name == catalogonexus.ToolDetail {
		text = "GET /items · ListItems · Listar itens\nResumo: Listar itens\nDescrição: Consulta itens.\nComo chamar (1 exposição):\n- URL: https://api.example.test/items | Auth: não informado\nDefinição OpenAPI da operação ($ref resolvidos):\n{\"parameters\":[]}"
	}
	return &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: text}}}, nil
}

func TestHandlersCatalogoNexusChamamSomentePesquisaEDetalhe(t *testing.T) {
	caller := &fakeCatalogoNexusCaller{}
	s := &servidor{catalogoNexusMCP: catalogonexus.New(caller)}

	busca := httptest.NewRecorder()
	s.buscarCatalogoNexus(busca, httptest.NewRequest(http.MethodPost, "/api/catalogo-nexus/search", bytes.NewBufferString(`{"q":"listar itens"}`)))
	if busca.Code != http.StatusOK {
		t.Fatalf("busca: status=%d corpo=%s", busca.Code, busca.Body.String())
	}

	detalhe := httptest.NewRecorder()
	s.detalharCatalogoNexus(detalhe, httptest.NewRequest(http.MethodPost, "/api/catalogo-nexus/detail", bytes.NewBufferString(`{"serviceId":"service-1","operationId":"ListItems"}`)))
	if detalhe.Code != http.StatusOK || !bytes.Contains(detalhe.Body.Bytes(), []byte("Auth: não informado")) {
		t.Fatalf("detalhe não preservou a resposta do catálogo: status=%d corpo=%s", detalhe.Code, detalhe.Body.String())
	}
	if !reflect.DeepEqual(caller.names, []string{catalogonexus.ToolSearch, catalogonexus.ToolDetail}) {
		t.Fatalf("tools chamadas fora da lista permitida: %#v", caller.names)
	}
	if caller.args[0]["q"] != "listar itens" || caller.args[1]["serviceId"] != "service-1" || caller.args[1]["operationId"] != "ListItems" {
		t.Fatalf("argumentos MCP inesperados: %#v", caller.args)
	}
}

func TestHandlersCatalogoNexusCRUD409EIsolamento(t *testing.T) {
	root := t.TempDir()
	storeA, err := okf.NewCatalogoNexusStore(root, "account-a")
	if err != nil {
		t.Fatal(err)
	}
	storeB, err := okf.NewCatalogoNexusStore(root, "account-b")
	if err != nil {
		t.Fatal(err)
	}
	s := &servidor{account: "account-a", catalogoNexusStore: storeA}
	resource := okf.Recurso{Name: "itens.listar", Kind: okf.KindAPI, Visibility: "private", Params: []okf.ParamMeta{}, Definicoes: []okf.Definicao{{Name: "limite", Label: "Limite"}}}
	save := func(original string, value okf.Recurso) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]any{"original": original, "recurso": value})
		if err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		s.salvarCatalogoNexus(response, httptest.NewRequest(http.MethodPut, "/api/catalogo-nexus/recurso", bytes.NewReader(body)))
		return response
	}
	if response := save("", resource); response.Code != http.StatusOK {
		t.Fatalf("criar: %d %s", response.Code, response.Body.String())
	}
	if response := save("", resource); response.Code != http.StatusConflict {
		t.Fatalf("duplicado: %d %s", response.Code, response.Body.String())
	}
	edited := resource
	edited.Description = "atualizado"
	edited.Definicoes = nil
	if response := save(resource.Name, edited); response.Code != http.StatusOK {
		t.Fatalf("editar: %d %s", response.Code, response.Body.String())
	}
	list, err := storeA.Listar()
	if err != nil || len(list) != 1 || len(list[0].Definicoes) != 1 || list[0].Description != "atualizado" {
		t.Fatalf("edição perdeu dados: %#v, %v", list, err)
	}
	other, err := storeB.Listar()
	if err != nil || len(other) != 0 {
		t.Fatalf("accounts compartilharam catálogo: %#v, %v", other, err)
	}
	remove := httptest.NewRecorder()
	s.removerCatalogoNexus(remove, httptest.NewRequest(http.MethodDelete, "/api/catalogo-nexus/recurso?nome=itens.listar", nil))
	if remove.Code != http.StatusOK {
		t.Fatalf("remover: %d %s", remove.Code, remove.Body.String())
	}
}
