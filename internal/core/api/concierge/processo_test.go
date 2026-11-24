package conciergeapi

import (
	"bytes"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
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
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "TZ=UTC", "CONCIERGE_TESTE_PROCESSO=1", "CONCIERGE_TESTE_RAIZ=" + raiz, "CONCIERGE_TESTE_ENDERECO=" + addr, "DATABASE_URL=" + b.Pool.Config().ConnString(), "AMBIENTE=dev", "AUTH_MODE=dev", "AUTH_DEV_USUARIO=admin-superlogica", "BOOTSTRAP_DEV_ATENDE=false", "NEXUS_COMPANY_ID=", "MCP_AUTH=none", "MCP_URL=", "NEXUS_BASE_URL=", "DEFAULT_ACCOUNT_ID=cond-001"}
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
