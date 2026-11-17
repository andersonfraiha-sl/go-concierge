package main

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/escopo"
	"go-concierge/internal/memoria"
	"go-concierge/internal/pedidos"
)

const telefoneDoMorador = "5519981621916"

// ambienteDeMorador liga a memória do morador em adm-a e devolve o ambiente de integração.
func ambienteDeMorador(t *testing.T) ambienteRotas {
	t.Helper()
	amb := ambienteDeIntegracao(t)
	contato, err := escopo.NovoContato("adm-a", admin.Padrao, telefoneDoMorador)
	if err != nil {
		t.Fatal(err)
	}
	fatos := memoria.Memoria{Fatos: []memoria.Fato{{Chave: "pets", Texto: "tem um cachorro"}, {Chave: "unidade", Texto: "apto 12"}}}
	if err := amb.s.memorias.Gravar(t.Context(), contato, fatos); err != nil {
		t.Fatal(err)
	}
	return amb
}

func TestRotasDeMoradorExigemChaveDeIntegracao(t *testing.T) {
	amb := ambienteDeMorador(t)
	consulta := "?administradora=adm-a&phone=" + telefoneDoMorador
	for _, rota := range []struct{ metodo, url string }{
		{http.MethodGet, "/integracao/v1/memorias?administradora=adm-a"},
		{http.MethodGet, "/integracao/v1/memoria" + consulta},
		{http.MethodDelete, "/integracao/v1/memoria" + consulta},
		{http.MethodGet, "/integracao/v1/pedidos" + consulta},
	} {
		for _, autorizacao := range []string{"", "Bearer outra-chave-qualquer-de-32-caracteres"} {
			t.Run(rota.metodo+" "+rota.url+" com "+autorizacao, func(t *testing.T) {
				// o usuário de dev válido não abre a rota no lugar da chave
				r, _ := http.NewRequest(rota.metodo, rota.url, nil)
				r.Header.Set("X-Dev-Usuario", "admin")
				if autorizacao != "" {
					r.Header.Set("Authorization", autorizacao)
				}
				w := httptest.NewRecorder()
				amb.handler.ServeHTTP(w, r)
				if w.Code != http.StatusUnauthorized {
					t.Fatalf("status %d, quer 401", w.Code)
				}
			})
		}
	}
	// a memória segue intacta depois do DELETE recusado
	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/memoria"+consulta, ""); !strings.Contains(w.Body.String(), "tem um cachorro") {
		t.Fatalf("memória alterada por chamada sem chave: %s", w.Body.String())
	}
}

func TestRotasDeMoradorExigemAdministradora(t *testing.T) {
	amb := ambienteDeMorador(t)
	for _, rota := range []struct{ metodo, url string }{
		{http.MethodGet, "/integracao/v1/memorias"},
		{http.MethodGet, "/integracao/v1/memoria?phone=" + telefoneDoMorador},
		{http.MethodDelete, "/integracao/v1/memoria?phone=" + telefoneDoMorador},
		{http.MethodGet, "/integracao/v1/pedidos?phone=" + telefoneDoMorador},
	} {
		t.Run(rota.metodo+" "+rota.url, func(t *testing.T) {
			w := chamarIntegracao(amb, rota.metodo, rota.url, "")
			if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "ADMINISTRADORA_OBRIGATORIA") {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
		})
	}
	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/memoria?administradora=../x&phone="+telefoneDoMorador, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("administradora inválida: status %d", w.Code)
	}
	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/memoria?administradora=adm-a", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("sem phone: status %d", w.Code)
	}
}

func TestMemoriaPelaIntegracaoLeListaEEsquece(t *testing.T) {
	amb := ambienteDeMorador(t)
	consulta := "?administradora=adm-a&phone=" + telefoneDoMorador

	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/memorias?administradora=adm-a", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), telefoneDoMorador) {
		t.Fatalf("listar: status %d: %s", w.Code, w.Body.String())
	}
	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/memorias?administradora=adm-b", ""); w.Code != http.StatusOK || strings.Contains(w.Body.String(), telefoneDoMorador) {
		t.Fatalf("a listagem de adm-b não pode trazer contato de adm-a: %d %s", w.Code, w.Body.String())
	}
	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/memoria"+consulta, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "tem um cachorro") {
		t.Fatalf("ler: status %d: %s", w.Code, w.Body.String())
	}
	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/memoria?administradora=adm-b&phone="+telefoneDoMorador, ""); strings.Contains(w.Body.String(), "cachorro") {
		t.Fatalf("outra administradora leu a memória: %s", w.Body.String())
	}

	if w := chamarIntegracao(amb, http.MethodDelete, "/integracao/v1/memoria"+consulta+"&chave=pets", ""); w.Code != http.StatusOK || strings.Contains(w.Body.String(), "cachorro") {
		t.Fatalf("esquecer um fato: status %d: %s", w.Code, w.Body.String())
	}
	if w := chamarIntegracao(amb, http.MethodDelete, "/integracao/v1/memoria"+consulta+"&chave=pets", ""); w.Code != http.StatusNotFound {
		t.Fatalf("esquecer fato que já não existe: status %d: %s", w.Code, w.Body.String())
	}
	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/memoria"+consulta, ""); !strings.Contains(w.Body.String(), "apto 12") || strings.Contains(w.Body.String(), "cachorro") {
		t.Fatalf("a leitura seguinte devia ter só a unidade: %s", w.Body.String())
	}
	if w := chamarIntegracao(amb, http.MethodDelete, "/integracao/v1/memoria"+consulta, ""); w.Code != http.StatusOK || strings.Contains(w.Body.String(), "apto 12") {
		t.Fatalf("esquecer tudo: status %d: %s", w.Code, w.Body.String())
	}
}

func TestMoradorPelaIntegracaoRespeitaAImplantacaoFixa(t *testing.T) {
	amb := ambienteDeMorador(t)
	amb.s.implantacao = escopo.Implantacao{AdministradoraFixa: "adm-a"}
	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/memoria?phone="+telefoneDoMorador, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "cachorro") {
		t.Fatalf("a fixa vale sem parâmetro: status %d: %s", w.Code, w.Body.String())
	}
	for _, rota := range []struct{ metodo, url string }{
		{http.MethodGet, "/integracao/v1/memorias?administradora=adm-b"},
		{http.MethodGet, "/integracao/v1/memoria?administradora=adm-b&phone=" + telefoneDoMorador},
		{http.MethodDelete, "/integracao/v1/memoria?administradora=adm-b&phone=" + telefoneDoMorador},
		{http.MethodGet, "/integracao/v1/pedidos?administradora=adm-b&phone=" + telefoneDoMorador},
	} {
		t.Run(rota.metodo+" "+rota.url, func(t *testing.T) {
			if w := chamarIntegracao(amb, rota.metodo, rota.url, ""); w.Code != http.StatusForbidden {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestPedidosPelaIntegracao(t *testing.T) {
	amb := ambienteDeMorador(t)
	url := "/integracao/v1/pedidos?administradora=adm-a&phone=" + telefoneDoMorador

	if w := chamarIntegracao(amb, http.MethodGet, url, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ligado":false`) {
		t.Fatalf("pedidos desligados: status %d: %s", w.Code, w.Body.String())
	}

	registro := pedidos.NovoFake()
	amb.s.atende.Pedidos = registro
	contato, err := escopo.NovoContato("adm-a", admin.Padrao, telefoneDoMorador)
	if err != nil {
		t.Fatal(err)
	}
	cart := pedidos.Carteira{Pedidos: []*pedidos.Pedido{{Protocolo: "PED-1", Caso: "boleto", Estado: pedidos.Coletando}}}
	if err := registro.Gravar(t.Context(), contato, cart); err != nil {
		t.Fatal(err)
	}
	if w := chamarIntegracao(amb, http.MethodGet, url, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"ligado":true`) || !strings.Contains(w.Body.String(), "PED-1") {
		t.Fatalf("pedidos do contato: status %d: %s", w.Code, w.Body.String())
	}
	if w := chamarIntegracao(amb, http.MethodGet, "/integracao/v1/pedidos?administradora=adm-b&phone="+telefoneDoMorador, ""); strings.Contains(w.Body.String(), "PED-1") {
		t.Fatalf("outra administradora leu o pedido: %s", w.Body.String())
	}
}

// O telefone da consulta nunca chega ao log inteiro, em nenhuma das rotas de morador.
func TestRotasDeMoradorMascaramOTelefoneNoLog(t *testing.T) {
	amb := ambienteDeMorador(t)
	var saida bytes.Buffer
	amb.s.registro = slog.New(slog.NewJSONHandler(&saida, nil))
	amb.handler = amb.s.rotear(nil)
	consulta := "?administradora=adm-a&phone=" + telefoneDoMorador
	for _, rota := range []struct{ metodo, url string }{
		{http.MethodGet, "/integracao/v1/memoria" + consulta},
		{http.MethodDelete, "/integracao/v1/memoria" + consulta + "&chave=nao-existe"},
		{http.MethodGet, "/integracao/v1/pedidos" + consulta},
	} {
		chamarIntegracao(amb, rota.metodo, rota.url, "")
	}
	log := saida.String()
	if !strings.Contains(log, "integracao/v1/memoria") {
		t.Fatalf("a requisição não gerou linha de log: %q", log)
	}
	if strings.Contains(log, telefoneDoMorador) {
		t.Fatalf("telefone sem máscara no log: %s", log)
	}
	if !strings.Contains(log, escopo.MascararTelefone(telefoneDoMorador)) {
		t.Errorf("a linha de log devia trazer o telefone mascarado: %s", log)
	}
}
