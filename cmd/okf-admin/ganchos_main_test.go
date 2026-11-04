package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/metric/noop"

	"go-concierge/internal/atende"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/entrada"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/regras"
	"go-concierge/internal/harness"
	"go-concierge/internal/okf"
)

// servidorComGanchos é o servidor com o banco e as regras de um Postgres próprio e o
// Conjunto que o main monta.
func servidorComGanchos(t *testing.T) (*servidor, *ganchos.Conjunto, *atende.Servico) {
	t.Helper()
	b := bancotest.Novo(t)
	s := &servidor{banco: b, regras: regras.NovoPostgres(b.Ent), log: log.New(io.Discard, "", 0)}
	conjunto, auditor, err := s.montarGanchos(t.Context(), noop.NewMeterProvider().Meter("ganchos"))
	if err != nil {
		t.Fatal(err)
	}
	return s, conjunto, &atende.Servico{Ganchos: conjunto, Auditoria: auditor}
}

func TestConjuntoDoMainTemTodosOsFixos(t *testing.T) {
	_, conjunto, _ := servidorComGanchos(t)
	// spec §5, na ordem em que rodam (a checagem de antes_da_ferramenta é permitida,
	// confirmada, contexto)
	quer := map[ganchos.NomeDoEvento][]string{
		ganchos.EventoMensagemRecebida:         {"escopo_da_conversa", "pii_na_entrada"},
		ganchos.EventoAntesDaFerramenta:        {"ferramenta_permitida", "escrita_confirmada", "contexto_b2c"},
		ganchos.EventoDepoisDaFerramenta:       {"pii_no_resultado"},
		ganchos.EventoAntesDeResponder:         {"pii_na_resposta"},
		ganchos.EventoTransbordoOuEncerramento: {"transbordo_sem_pii"},
	}

	nomes := conjunto.NomesDosFixos()

	if len(nomes) != len(quer) {
		t.Fatalf("eventos = %v, quer %v", nomes, quer)
	}
	for evento, esperados := range quer {
		if !slices.Equal(nomes[evento], esperados) {
			t.Errorf("%s: fixos %v, quer %v", evento, nomes[evento], esperados)
		}
	}
}

var (
	cpfNoLog      = regexp.MustCompile(`\d{3}\.?\d{3}\.?\d{3}-?\d{2}`)
	telefoneNoLog = regexp.MustCompile(`\(?\d{2}\)?[ -]?9\d{4}-?\d{4}`)
)

// falasComPII são as mensagens do morador da bateria, com CPF e telefone em formatos variados.
var falasComPII = []string{
	"meu cpf é 529.982.247-25, pode consultar?",
	"cpf 11144477735 e telefone (11) 91234-5678",
	"me liga no 19 98765-4321, meu documento é 123.456.789-09",
	"telefone 11977776666",
}

func TestLogsSemPII(t *testing.T) {
	var saida bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(novoRegistro(&saida))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	s, conjunto, svc := servidorComGanchos(t)
	cat, err := okf.NewCatalogo(filepath.Join("..", "..", "okf"))
	if err != nil {
		t.Fatal(err)
	}
	fix, err := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cat.Recursos = func() []okf.Recurso { return fix }
	svc.Cat, svc.Agente, svc.Mapeamento = cat, "agentes/concierge", mapeamentoTeste(t)
	svc.Sessoes, svc.Sender = atende.NewSessoes(atende.NovaSessaoEmMemoria()), &atende.Outbox{}
	svc.Config = harness.Config{Mock: true, Env: func(string) string { return "" }, Ganchos: conjunto}
	s.atende = svc

	for i := range 20 {
		fala := falasComPII[i%len(falasComPII)]
		if _, err := svc.Processar(context.Background(), mensagemComPII(t, i, fala)); err != nil {
			t.Fatalf("turno %d: %v", i, err)
		}
		// vazamento simulado, num atributo e no log legado: o turno de hoje não loga o
		// texto, então só isto prova que a máscara do main está na frente de tudo
		slog.Info("fala do morador", "texto", fala)
		log.Printf("legado: %s", fala)
	}
	svc.Aguardar()

	logs := saida.String()
	if !strings.Contains(logs, "gancho decidiu") {
		t.Fatalf("o handler do main não capturou os logs dos ganchos:\n%s", logs)
	}
	for nome, re := range map[string]*regexp.Regexp{"CPF": cpfNoLog, "telefone": telefoneNoLog} {
		if achados := re.FindAllString(logs, -1); len(achados) > 0 {
			t.Errorf("%s no log: %v", nome, achados)
		}
	}
}

// mensagemComPII é a mensagem i do morador no condomínio cond-x, como o webhook emulado a
// monta, cada uma no seu protocolo.
func mensagemComPII(t *testing.T, i int, texto string) entrada.Mensagem {
	t.Helper()
	corpo, err := json.Marshal(map[string]any{
		"event": "com.echo.atende.user.protocol.created", "id": fmt.Sprintf("evt-pii-%d", i), "tenant": "tenant-x",
		"data": map[string]any{
			"protocol": map[string]any{"number": fmt.Sprintf("protocolo-pii-%d", i)},
			"contact":  map[string]any{"phone": "5519981621916", "name": "Morador"},
			"channel":  map[string]any{"type": "whatsapp", "wabaId": "waba-1", "displayNumber": "551940402710"},
			"message":  map[string]any{"id": fmt.Sprintf("msg-pii-%d", i), "type": "text", "content": texto},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := atende.Parse(corpo)
	if err != nil {
		t.Fatal(err)
	}
	msg := ev.Normalizar()
	msg.Tenant, msg.CompanyID = "tenant-x", "123"
	msg.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
	msg.Destino = entrada.Destino{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console", Protocolo: msg.ConversaID, Telefone: msg.Contato.Phone, Canal: msg.Canal}
	return msg
}
