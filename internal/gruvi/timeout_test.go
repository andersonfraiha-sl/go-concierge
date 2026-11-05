package gruvi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestIdentidadeFalhaRapidoComBFFLento confere que uma dependência externa lenta não trava
// o turno: o cliente tem Timeout (ver New), e a chamada precisa falhar dentro dele, com um
// erro claro, nunca travar a chamadora.
func TestIdentidadeFalhaRapidoComBFFLento(t *testing.T) {
	// a resposta demora mais que o Timeout do client, mas um tempo finito: srv.Close() (no
	// defer) espera o handler terminar, e um handler bloqueado para sempre travaria o teste.
	const atrasoServidor = 300 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(atrasoServidor)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL+"/bff", "5982", "5519981621916", &tokenTeste{})
	c.HTTP = &http.Client{Timeout: 100 * time.Millisecond} // o mesmo client.Do, timeout curto só para o teste não esperar 25s

	inicio := time.Now()
	_, err := c.Identidade(context.Background(), "19 98162-1916")
	decorrido := time.Since(inicio)

	if err == nil {
		t.Fatal("BFF que nunca responde deveria falhar, não travar a chamada")
	}
	if decorrido > 2*time.Second {
		t.Fatalf("a chamada devia falhar perto do Timeout do client (100ms), levou %s", decorrido)
	}
}
