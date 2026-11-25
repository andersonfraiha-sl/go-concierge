package conciergeapi

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go-concierge/internal/core/banco/bancotest"
)

func TestProcessoConciergeAuxiliar(t *testing.T) {
	if os.Getenv("CONCIERGE_TESTE_PROCESSO") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("concierge", flag.ExitOnError)
	os.Args = []string{"concierge", "-root", os.Getenv("CONCIERGE_TESTE_RAIZ"), "-addr", os.Getenv("CONCIERGE_TESTE_ENDERECO"), "-env", filepath.Join(os.Getenv("CONCIERGE_TESTE_RAIZ"), "ausente")}
	Executar()
}

func TestProcessoConciergeUsaDadosHTTPNoBootEEncerra(t *testing.T) {
	b := bancotest.Novo(t)
	canal := uuid.New()
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,'empresa',42,'WEB','test','smoke','INHERIT',true,'agentes/concierge',now())`, canal); err != nil {
		t.Fatal(err)
	}
	raiz := t.TempDir()
	for _, pasta := range []string{"agentes", "nexus"} {
		if err := os.CopyFS(filepath.Join(raiz, pasta), os.DirFS(filepath.Join("..", "..", "..", "..", "okf", pasta))); err != nil {
			t.Fatal(err)
		}
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessoConciergeAuxiliar$")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "TZ=UTC", "CONCIERGE_TESTE_PROCESSO=1", "CONCIERGE_TESTE_RAIZ=" + raiz, "CONCIERGE_TESTE_ENDERECO=" + addr, "DATABASE_URL=" + b.Pool.Config().ConnString(), "AMBIENTE=dev", "AUTH_MODE=dev", "AUTH_DEV_USUARIO=admin-superlogica", "BOOTSTRAP_DEV_ATENDE=false", "NEXUS_COMPANY_ID=", "MCP_AUTH=none", "MCP_URL=", "NEXUS_BASE_URL=", "DEFAULT_ACCOUNT_ID=cond-001", "LLM_MODE=mock", "PEDIDOS=off"}
	var saida bytes.Buffer
	cmd.Stdout, cmd.Stderr = &saida, &saida
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	fim := make(chan error, 1)
	go func() { fim <- cmd.Wait() }()
	encerrado := false
	t.Cleanup(func() {
		if !encerrado {
			_ = cmd.Process.Kill()
			<-fim
		}
	})
	cliente := &http.Client{Timeout: time.Second}
	base := "http://" + addr
	prazo := time.Now().Add(15 * time.Second)
	for {
		res, e := cliente.Get(base + "/readyz")
		if e == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				break
			}
		}
		select {
		case e := <-fim:
			encerrado = true
			t.Fatalf("processo encerrou no boot: %v\n%s", e, saida.String())
		default:
		}
		if time.Now().After(prazo) {
			t.Fatal("processo não ficou pronto no prazo")
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, rota := range []string{"/api/eu", "/api/admin/estado?company_id=empresa&condo_id=42", "/api/atende/config", "/api/ganchos/regras", "/api/runtime/configuracao?company_id=empresa&channel_id=" + canal.String()} {
		req, e := http.NewRequestWithContext(t.Context(), http.MethodGet, base+rota, nil)
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("X-Dev-Usuario", "admin-superlogica")
		res, e := cliente.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		corpo, e := io.ReadAll(io.LimitReader(res.Body, 1024))
		_ = res.Body.Close()
		if e != nil || res.StatusCode != http.StatusOK {
			t.Fatalf("%s: status=%d erro=%v corpo=%s", rota, res.StatusCode, e, corpo)
		}
	}
	corpoWebhook := `{"event":"com.echo.atende.user.protocol.created","id":"evt-smoke-processo","tenant":"tenant-teste","data":{"protocol":{"number":"protocolo-smoke-processo"},"contact":{"phone":"5511999999999","name":"Teste"},"channel":{"type":"whatsapp","displayNumber":"5511900000000"},"message":{"id":"msg-smoke","type":"text","content":"Olá"}}}`
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/emulador/atende/webhook?company_id=empresa&channel_id="+canal.String(), strings.NewReader(corpoWebhook))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Dev-Usuario", "admin-superlogica")
	req.Header.Set("Content-Type", "application/json")
	res, err := cliente.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var aceite struct {
		Accepted  bool   `json:"accepted"`
		Protocol  string `json:"protocol"`
		SessionID string `json:"session_id"`
	}
	err = json.NewDecoder(res.Body).Decode(&aceite)
	_ = res.Body.Close()
	if err != nil || res.StatusCode != http.StatusAccepted || !aceite.Accepted || aceite.Protocol != "protocolo-smoke-processo" || aceite.SessionID == "" {
		t.Fatalf("aceite do smoke: status=%d resposta=%+v erro=%v", res.StatusCode, aceite, err)
	}
	prazo = time.Now().Add(15 * time.Second)
	var respostaEsperada string
	for {
		var dados []byte
		err := b.Pool.QueryRow(t.Context(), `SELECT data FROM sessions WHERE company_id=$1 AND channel_id=$2 AND protocol=$3`, "empresa", canal, aceite.Protocol).Scan(&dados)
		if err != nil && err != pgx.ErrNoRows {
			t.Fatal(err)
		}
		if err == nil {
			var snapshot struct {
				State struct {
					Historico []struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"historico"`
				} `json:"state"`
			}
			if err := json.Unmarshal(dados, &snapshot); err != nil {
				t.Fatal(err)
			}
			for _, item := range snapshot.State.Historico {
				if item.Role == "assistant" && item.Content != "" {
					respostaEsperada = item.Content
					break
				}
			}
		}
		if respostaEsperada != "" {
			break
		}
		if time.Now().After(prazo) {
			t.Fatal("smoke não persistiu resposta do assistente")
		}
		time.Sleep(250 * time.Millisecond)
	}
	req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/api/atende/saidas", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Dev-Usuario", "admin-superlogica")
	res, err = cliente.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var caixa struct {
		Saidas []struct {
			Protocolo string `json:"protocolo"`
			Texto     string `json:"texto"`
		} `json:"saidas"`
	}
	err = json.NewDecoder(res.Body).Decode(&caixa)
	_ = res.Body.Close()
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("caixa do smoke: status=%d erro=%v", res.StatusCode, err)
	}
	entregue := false
	for _, item := range caixa.Saidas {
		if item.Protocolo == aceite.Protocol && item.Texto == respostaEsperada {
			entregue = true
		}
	}
	if !entregue {
		t.Fatal("resposta persistida não consta na saída do mesmo protocolo")
	}
	t.Logf("smoke emulado: aceite, saída e histórico persistido no protocolo %s, sessão %s", aceite.Protocol, aceite.SessionID)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-fim:
		encerrado = true
		if err != nil {
			t.Fatalf("encerramento: %v\n%s", err, saida.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("encerramento não drenou no prazo")
	}
}
