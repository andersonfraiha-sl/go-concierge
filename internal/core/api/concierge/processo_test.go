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
	raiz := filepath.Join(t.TempDir(), "sem-semente")
	if _, err := abrirCatalogo(filepath.Join("..", "..", "..", "..", "okf"), b); err != nil {
		t.Fatal(err)
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
	if _, err := os.Stat(filepath.Join(raiz, "var", "catalogo")); !os.IsNotExist(err) {
		t.Fatalf("boot criou cópia do catálogo: %v", err)
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

func TestProcessoStudioSalvaEReabreDepoisDeReiniciarSemSemente(t *testing.T) {
	b := bancotest.Novo(t)
	if _, err := abrirCatalogo(filepath.Join("..", "..", "..", "..", "okf"), b); err != nil {
		t.Fatal(err)
	}
	raiz := filepath.Join(t.TempDir(), "sem-semente")
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	base := "http://" + addr
	apiBase := base
	cliente := &http.Client{Timeout: 2 * time.Second}
	var processo *exec.Cmd
	var fim chan error
	var saida bytes.Buffer
	iniciar := func() {
		t.Helper()
		processo = exec.Command(os.Args[0], "-test.run=^TestProcessoConciergeAuxiliar$")
		processo.Env = []string{"PATH=" + os.Getenv("PATH"), "TZ=UTC", "CONCIERGE_TESTE_PROCESSO=1", "CONCIERGE_TESTE_RAIZ=" + raiz, "CONCIERGE_TESTE_ENDERECO=" + addr, "DATABASE_URL=" + b.Pool.Config().ConnString(), "AMBIENTE=dev", "AUTH_MODE=dev", "AUTH_DEV_USUARIO=admin-superlogica", "BOOTSTRAP_DEV_ATENDE=false", "NEXUS_COMPANY_ID=", "MCP_AUTH=none", "MCP_URL=", "NEXUS_BASE_URL=", "DEFAULT_ACCOUNT_ID=cond-001", "LLM_MODE=mock", "PEDIDOS=off"}
		processo.Stdout, processo.Stderr = &saida, &saida
		if err := processo.Start(); err != nil {
			t.Fatal(err)
		}
		fim = make(chan error, 1)
		go func(p *exec.Cmd, resultado chan error) { resultado <- p.Wait() }(processo, fim)
		limite := time.Now().Add(20 * time.Second)
		for time.Now().Before(limite) {
			res, err := cliente.Get(base + "/readyz")
			if err == nil {
				_ = res.Body.Close()
				if res.StatusCode == 200 {
					return
				}
			}
			select {
			case err := <-fim:
				processo = nil
				t.Fatalf("processo encerrou: %v\n%s", err, saida.String())
			default:
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("boot não ficou pronto: %s", saida.String())
	}
	parar := func() {
		t.Helper()
		if processo == nil {
			return
		}
		if err := processo.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-fim:
			if err != nil {
				t.Fatalf("encerrar: %v\n%s", err, saida.String())
			}
		case <-time.After(15 * time.Second):
			_ = processo.Process.Kill()
			<-fim
			t.Fatal("processo não encerrou")
		}
		processo = nil
	}
	t.Cleanup(func() {
		if processo != nil {
			_ = processo.Process.Kill()
			<-fim
		}
	})
	pedir := func(metodo, caminho string, dados []byte) []byte {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), metodo, apiBase+caminho, bytes.NewReader(dados))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Dev-Usuario", "admin-superlogica")
		res, err := cliente.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		corpo, err := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s %s: %d %s", metodo, caminho, res.StatusCode, corpo)
		}
		return corpo
	}
	iniciar()
	projeto, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binario := filepath.Join(t.TempDir(), "backdoor")
	build := exec.Command("go", "build", "-o", binario, "./apps/backdoor/cmd/backdoor")
	build.Dir = projeto
	if saida, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compilar Backdoor: %v\n%s", err, saida)
	}
	portaBackdoor, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	enderecoBackdoor := portaBackdoor.Addr().String()
	_ = portaBackdoor.Close()
	backdoor := exec.Command(binario, "-addr", enderecoBackdoor, "-env", filepath.Join(raiz, "ausente"))
	backdoor.Env = []string{"PATH=" + os.Getenv("PATH"), "CONCIERGE_URL=" + base}
	var logBackdoor bytes.Buffer
	backdoor.Stdout, backdoor.Stderr = &logBackdoor, &logBackdoor
	if err := backdoor.Start(); err != nil {
		t.Fatal(err)
	}
	terminouBackdoor := make(chan error, 1)
	go func() { terminouBackdoor <- backdoor.Wait() }()
	backdoorAtivo := true
	t.Cleanup(func() {
		if backdoorAtivo {
			_ = backdoor.Process.Kill()
			<-terminouBackdoor
		}
	})
	apiBase = "http://" + enderecoBackdoor
	limiteBackdoor := time.Now().Add(20 * time.Second)
	for {
		res, err := cliente.Get(apiBase + "/studio/")
		if err == nil {
			corpo, lerErr := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if lerErr != nil {
				t.Fatal(lerErr)
			}
			if res.StatusCode == http.StatusOK && bytes.Contains(corpo, []byte("<html")) {
				t.Log("Studio servido pelo Backdoor isolado")
				break
			}
			if res.StatusCode == http.StatusServiceUnavailable {
				if _, err := os.Stat(filepath.Join(projeto, "apps", "backdoor", "web", "studio", "dist", "index.html")); err == nil {
					t.Fatalf("Studio compilado retornou 503: %s", corpo)
				}
				t.Log("Studio estático não compilado; o proxy da API continua sob teste")
				break
			}
			t.Fatalf("Studio: %d %s", res.StatusCode, corpo)
		}
		select {
		case err := <-terminouBackdoor:
			backdoorAtivo = false
			t.Fatalf("Backdoor encerrou: %v\n%s", err, logBackdoor.String())
		default:
		}
		if time.Now().After(limiteBackdoor) {
			t.Fatal("Backdoor não ficou pronto")
		}
		time.Sleep(50 * time.Millisecond)
	}
	rota := "/api/skills/agendar-mudanca"
	var lido textosLidos
	if err := json.Unmarshal(pedir(http.MethodGet, rota, nil), &lido); err != nil {
		t.Fatal(err)
	}
	lido.Textos.Resumo = "Salvo antes de reiniciar"
	corpo, err := json.Marshal(textosParaGravar{Versao: lido.Versao, Textos: lido.Textos})
	if err != nil {
		t.Fatal(err)
	}
	pedir(http.MethodPut, rota+"/textos", corpo)
	parar()
	iniciar()
	var relido textosLidos
	if err := json.Unmarshal(pedir(http.MethodGet, rota, nil), &relido); err != nil {
		t.Fatal(err)
	}
	if relido.Textos.Resumo != lido.Textos.Resumo {
		t.Fatalf("resumo após reiniciar: %q", relido.Textos.Resumo)
	}
	t.Log("edição pela API do Backdoor permaneceu após reinício do core sem semente")
	if _, err := os.Stat(filepath.Join(raiz, "var", "catalogo")); !os.IsNotExist(err) {
		t.Fatalf("cópia criada: %v", err)
	}
	parar()
}
