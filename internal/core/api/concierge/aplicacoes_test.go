package conciergeapi

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	canaiscore "go-concierge/internal/core/clientes/canais"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
)

type canalConsoleTeste struct{ canal canais.Canal }

func (c canalConsoleTeste) Resolver(context.Context, canais.Identidade) (canais.Canal, error) {
	return c.canal, nil
}
func (c canalConsoleTeste) PorID(_ context.Context, company string, id uuid.UUID) (canais.Canal, error) {
	if company != c.canal.CompanyID || id != c.canal.ID {
		return canais.Canal{}, canais.ErrDesconhecido
	}
	return c.canal, nil
}
func (c canalConsoleTeste) Casos(context.Context, canais.Canal) ([]canais.Caso, error) {
	return nil, nil
}

func TestConsoleSeparadoProcessaConversa(t *testing.T) {
	if testing.Short() {
		t.Skip("integração entre processos")
	}
	bin := filepath.Join(t.TempDir(), "console")
	build := exec.Command("go", "build", "-o", bin, "./apps/console/cmd/console")
	build.Dir = filepath.Join("..", "..", "..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compilar Console: %v\n%s", err, out)
	}
	amb := novoAmbienteRotas(t)
	amb.s.autenticador = autenticadorDev{padrao: "admin"}
	canal := canais.Canal{ID: uuid.New(), CompanyID: "adm-a", Habilitado: true, Agente: "agentes/concierge", Modo: "INHERIT"}
	amb.s.atende.Canais = canalConsoleTeste{canal: canal}
	amb.s.atende.Mapeamento = nil
	servidorHarness, clienteHarness, err := atende.IniciarEntradaHTTP(amb.s.atende)
	if err != nil {
		t.Fatal(err)
	}
	amb.s.entradaHarness = clienteHarness
	t.Cleanup(func() {
		ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelar()
		if err := servidorHarness.Fechar(ctx); err != nil {
			t.Error(err)
		}
	})
	nucleo := httptest.NewServer(amb.s.rotear(nil))
	defer nucleo.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	base := "http://" + addr
	amb.s.outbox.Emulado = &canaiscore.HTTPSender{URL: base + "/emulador/atende/messages"}
	cmd := exec.Command(bin, "-addr", addr, "-env", filepath.Join(t.TempDir(), "ausente"))
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "CONCIERGE_URL=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	cmd.Env = append(cmd.Env, "CONCIERGE_URL="+nucleo.URL)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Signal(os.Interrupt); _ = cmd.Wait() })
	client := &http.Client{Timeout: 2 * time.Second}
	pronto := false
	for fim := time.Now().Add(10 * time.Second); time.Now().Before(fim); {
		r, err := client.Get(base + "/healthz")
		if err == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				pronto = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !pronto {
		t.Fatal("Console não iniciou")
	}
	corpo := `{"event":"com.echo.atende.user.protocol.created","id":"evt-separacao","tenant":"tenant-1","data":{"protocol":{"number":"teste-separacao"},"contact":{"phone":"5511999990000","name":"Contato de teste"},"channel":{"type":"whatsapp","wabaId":"waba-1","displayNumber":"551940402710"},"message":{"id":"msg-separacao","type":"text","content":"Qual o horário da piscina?"}}}`
	resp, err := client.Post(base+"/emulador/atende/webhook?company_id=adm-a&channel_id="+canal.ID.String(), "application/json", strings.NewReader(corpo))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 202 {
		t.Fatalf("webhook: %d %s", resp.StatusCode, raw)
	}
	var resposta struct {
		Recebidas []struct {
			Texto string `json:"message"`
		} `json:"recebidas"`
	}
	for fim := time.Now().Add(15 * time.Second); time.Now().Before(fim); {
		resp, err = client.Get(base + "/api/atende/sessao?protocolo=teste-separacao&company_id=adm-a")
		if err != nil {
			t.Fatal(err)
		}
		raw, err = io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		err = json.Unmarshal(raw, &resposta)
		resp.Body.Close()
		if resp.StatusCode == 200 && err == nil && len(resposta.Recebidas) > 0 {
			if resposta.Recebidas[0].Texto == "" {
				t.Fatal("entrega vazia")
			}
			humano := strings.Replace(corpo, "com.echo.atende.user.protocol.created", "com.echo.atende.attendant.message", 1)
			humano = strings.Replace(humano, "evt-separacao", "evt-humano", 1)
			humano = strings.Replace(humano, "Qual o horário da piscina?", "Vou cuidar disso", 1)
			resp, err = client.Post(base+"/emulador/atende/webhook?company_id=adm-a&channel_id="+canal.ID.String(), "application/json", strings.NewReader(humano))
			if err != nil {
				t.Fatal(err)
			}
			raw, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusAccepted {
				t.Fatalf("evento humano: %d %s", resp.StatusCode, raw)
			}
			resp, err = client.Get(base + "/api/atende/sessao?protocolo=teste-separacao&company_id=adm-a")
			if err != nil {
				t.Fatal(err)
			}
			raw, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			var estado struct {
				Sessao struct {
					Humano bool `json:"atendimento_humano"`
					Turnos []struct {
						Quem  string `json:"quem"`
						Texto string `json:"texto"`
					} `json:"turnos"`
				} `json:"sessao"`
			}
			if err := json.Unmarshal(raw, &estado); err != nil || !estado.Sessao.Humano {
				t.Fatalf("estado após intervenção: %s erro=%v", raw, err)
			}
			encontrou := false
			for _, turno := range estado.Sessao.Turnos {
				encontrou = encontrou || turno.Quem == "atendente" && turno.Texto == "Vou cuidar disso"
			}
			if !encontrou {
				t.Fatalf("mensagem humana ausente: %s", raw)
			}
			morador := strings.Replace(corpo, "com.echo.atende.user.protocol.created", "com.echo.atende.user.message", 1)
			morador = strings.Replace(morador, "evt-separacao", "evt-morador-humano", 1)
			morador = strings.Replace(morador, `"number":"teste-separacao"`, `"number":"teste-separacao","isHumanAttendance":true`, 1)
			morador = strings.Replace(morador, "Qual o horário da piscina?", "Obrigado", 1)
			resp, err = client.Post(base+"/emulador/atende/webhook?company_id=adm-a&channel_id="+canal.ID.String(), "application/json", strings.NewReader(morador))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusAccepted {
				t.Fatalf("mensagem durante atendimento humano: %d", resp.StatusCode)
			}
			resp, err = client.Get(base + "/api/atende/sessao?protocolo=teste-separacao&company_id=adm-a")
			if err != nil {
				t.Fatal(err)
			}
			raw, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			if err := json.Unmarshal(raw, &estado); err != nil || !estado.Sessao.Humano || !strings.Contains(string(raw), "Obrigado") {
				t.Fatalf("mensagem do morador no atendimento humano: %s erro=%v", raw, err)
			}
			t.Logf("smoke HTTP pelo Console: webhook do morador 202, resposta recebida, webhook humano 202 e intervenção registrada no protocolo teste-separacao")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	amb.s.turnos.Wait()
	t.Fatalf("resposta não percorreu Concierge → Console: status %d corpo %s saídas %+v", resp.StatusCode, raw, amb.s.outbox.Itens(""))
}
