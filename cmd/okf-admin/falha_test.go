package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/banco/bancotest"
)

// capturarSlog troca o slog padrão pelo registro do processo (máscara de PII e fonte no
// ERROR) escrevendo num buffer.
func capturarSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var saida bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(novoRegistro(&saida))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	return &saida
}

func linhasDeLog(t *testing.T, saida string) []map[string]any {
	t.Helper()
	var linhas []map[string]any
	for _, bruta := range strings.Split(strings.TrimSpace(saida), "\n") {
		if bruta == "" {
			continue
		}
		var linha map[string]any
		if err := json.Unmarshal([]byte(bruta), &linha); err != nil {
			t.Fatalf("linha não é JSON: %v: %s", err, bruta)
		}
		linhas = append(linhas, linha)
	}
	return linhas
}

const cpfDoTeste = "529.982.247-25"

func exigirSemCPF(t *testing.T, saida string) {
	t.Helper()
	if strings.Contains(saida, cpfDoTeste) || strings.Contains(saida, "52998224725") {
		t.Fatalf("CPF sem máscara no log:\n%s", saida)
	}
}

func handlerQueEntraEmPanico(http.ResponseWriter, *http.Request) {
	panic("morador com cpf " + cpfDoTeste + " quebrou o handler")
}

func TestPanicoNoHandlerViraUmaLinhaDeErroCom500(t *testing.T) {
	var saida bytes.Buffer
	s := &servidor{registro: novoRegistro(&saida)}
	w := httptest.NewRecorder()
	s.registrar(http.HandlerFunc(handlerQueEntraEmPanico)).ServeHTTP(w, httptest.NewRequest("GET", "/api/quebra", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", w.Code)
	}
	var corpo map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &corpo); err != nil {
		t.Fatal(err)
	}
	exigirErroCom("ERRO_INTERNO", "Não deu para concluir. Tente de novo.")(t, corpo)
	linhas := linhasDeLog(t, saida.String())
	if len(linhas) != 1 {
		t.Fatalf("quero uma linha, veio:\n%s", saida.String())
	}
	linha := linhas[0]
	if linha["level"] != "ERROR" || linha["status"] != float64(500) || linha["request_id"] == "" || linha["metodo"] != "GET" {
		t.Fatalf("linha: %v", linha)
	}
	if pilha, _ := linha["stack"].(string); !strings.Contains(pilha, "handlerQueEntraEmPanico") {
		t.Fatalf("stack sem o handler: %v", linha["stack"])
	}
	exigirSemCPF(t, saida.String())
}

// A falha nasce no store do Postgres (banco fechado), sobe pelo handler e sai numa linha só,
// com a pilha do store, não só a do handler.
func TestErroDoBancoSaiComAPilhaDoStore(t *testing.T) {
	amb := novoAmbienteRotas(t)
	b := bancotest.Novo(t)
	if err := b.Fechar(); err != nil {
		t.Fatal(err)
	}
	amb.s.admin = admin.NovoPostgres(b.Ent)
	var saida bytes.Buffer
	amb.s.registro = novoRegistro(&saida)
	amb.handler = amb.s.rotear(nil)

	if w := amb.pedir(t, "admin-superlogica", "GET", "/api/admin/estado", ""); w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var erros []map[string]any
	for _, l := range linhasDeLog(t, saida.String()) {
		if l["level"] == "ERROR" {
			erros = append(erros, l)
		}
	}
	if len(erros) != 1 {
		t.Fatalf("quero uma linha de ERROR, veio:\n%s", saida.String())
	}
	pilha, _ := erros[0]["stack"].(string)
	if !strings.Contains(pilha, "go-concierge/internal/admin.(*Postgres).Ler") || !strings.Contains(pilha, "adminEstado") {
		t.Fatalf("stack sem o store ou sem o handler:\n%s", pilha)
	}
	if erros[0]["operacao"] != "GET /api/admin/estado" || erros[0]["usuario_id"] != "admin-superlogica" {
		t.Fatalf("linha sem a requisição: %v", erros[0])
	}
}

var errSentinelaDoTeste = errors.New("pedido recusado")

func TestErroSentinelaSaiSemPilhaECPFMascarado(t *testing.T) {
	var saida bytes.Buffer
	s := &servidor{registro: novoRegistro(&saida)}
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("morador %s: %w", cpfDoTeste, errSentinelaDoTeste))
	})
	s.registrar(h).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/qualquer", nil))

	linhas := linhasDeLog(t, saida.String())
	if len(linhas) != 1 || linhas[0]["level"] != "ERROR" {
		t.Fatalf("quero uma linha de ERROR, veio:\n%s", saida.String())
	}
	if _, ok := linhas[0]["stack"]; ok {
		t.Fatalf("sentinela não leva stack: %v", linhas[0])
	}
	if chamador, _ := linhas[0]["chamador"].(string); !strings.Contains(chamador, "falha_test.go:") {
		t.Fatalf("chamador = %v, quer a linha que respondeu o erro", linhas[0]["chamador"])
	}
	exigirSemCPF(t, saida.String())
}
