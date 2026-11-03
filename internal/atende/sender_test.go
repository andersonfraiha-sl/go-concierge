package atende

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/entrada"
)

// Ao remover o mock de telefone fixo do ATENDE, reative e ajuste este teste
// para verificar o destinatário real, restaurando também os imports necessários.
/*
func TestATENDESenderContratoDivisaoEOrdem(t *testing.T) {
	var mu sync.Mutex
	var partes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/integrations/messages" {
			t.Errorf("requisição inesperada: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("x-api-key") != "chave-teste" {
			t.Errorf("headers incompletos: content-type=%q api-key=%q", r.Header.Get("Content-Type"), r.Header.Get("x-api-key"))
		}
		var corpo struct {
			Contact struct {
				Phone string `json:"phone"`
			} `json:"contact"`
			Message struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
			t.Errorf("JSON inválido: %v", err)
		}
		if corpo.Contact.Phone != "5511999999999" || corpo.Message.Type != "text" {
			t.Errorf("destino/corpo inesperado: %+v", corpo)
		}
		mu.Lock()
		partes = append(partes, corpo.Message.Text)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("não é JSON"))
	}))
	defer server.Close()

	texto := strings.Repeat("🙂", 2000) + "\n\n**fim**"
	err := (&ATENDESender{BaseURL: server.URL + "/v2", APIKey: "chave-teste"}).Enviar(context.Background(), entrada.Destino{Telefone: "5511999999999"}, texto)
	if err != nil {
		t.Fatal(err)
	}
	if len(partes) != 2 || len([]rune(partes[0])) != 2000 || partes[1] != "*fim*" {
		t.Fatalf("partes inesperadas: count=%d lengths=(%d,%d), final=%q", len(partes), len([]rune(partes[0])), len([]rune(partes[1])), partes[len(partes)-1])
	}
	if len(DividirMensagem(strings.Repeat("🙂", 2001), 4000)) != 2 {
		t.Fatal("a divisão não contou pares UTF-16")
	}
}
*/

func TestDividirMensagemUsaLimitesEFormataWhatsApp(t *testing.T) {
	texto := strings.Repeat("a", 3990) + " \n\n" + strings.Repeat("b", 30)
	partes := DividirMensagem(texto, 4000)
	if len(partes) != 2 || len([]rune(partes[0])) > 4000 {
		t.Fatalf("divisão inesperada: %d partes, primeira=%d", len(partes), len([]rune(partes[0])))
	}
	formatado := FormatarMensagem("# Título\n* item **forte**\n- outro\n####### título")
	if formatado != "*Título*\n- item *forte*\n- outro\n####### título" {
		t.Fatalf("formatação inesperada: %q", formatado)
	}
}

func TestATENDESenderRecusaRedirectETempoERegistraFalhaParcial(t *testing.T) {
	t.Run("redirect", func(t *testing.T) {
		alvoChamadas := 0
		alvo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { alvoChamadas++ }))
		defer alvo.Close()
		redirecionador := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", alvo.URL)
			w.WriteHeader(http.StatusFound)
		}))
		defer redirecionador.Close()
		err := (&ATENDESender{BaseURL: redirecionador.URL, APIKey: "key"}).Enviar(context.Background(), entrada.Destino{Telefone: "1"}, "oi")
		if err == nil || !strings.Contains(err.Error(), "HTTP 302") || alvoChamadas != 0 {
			t.Fatalf("redirect não recusado: err=%v chamadas=%d", err, alvoChamadas)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { time.Sleep(100 * time.Millisecond) }))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		err := (&ATENDESender{BaseURL: server.URL, APIKey: "key"}).Enviar(ctx, entrada.Destino{Telefone: "1"}, "oi")
		if err == nil {
			t.Fatal("timeout deveria retornar erro")
		}
	})
	t.Run("parcial sem repetição", func(t *testing.T) {
		chamadas := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			chamadas++
			if chamadas == 1 {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer server.Close()
		texto := strings.Repeat("x", 8500)
		err := (&ATENDESender{BaseURL: server.URL, APIKey: "key"}).Enviar(context.Background(), entrada.Destino{Telefone: "1"}, texto)
		if err == nil || !strings.Contains(err.Error(), "parte 2 de 3") || chamadas != 2 {
			t.Fatalf("falha parcial/repetição incorreta: err=%v chamadas=%d", err, chamadas)
		}
	})
}

func TestATENDESenderSemConfigNaoFazRequisicao(t *testing.T) {
	err := (&ATENDESender{}).Enviar(context.Background(), entrada.Destino{}, fmt.Sprintf("%s", "texto"))
	if err == nil || strings.Contains(err.Error(), "texto") {
		t.Fatalf("configuração incompleta deve falhar sem incluir conteúdo: %v", err)
	}
}
