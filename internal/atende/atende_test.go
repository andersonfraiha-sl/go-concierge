package atende

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/mcp"
	"go-concierge/internal/memoria"
	"go-concierge/internal/okf"
)

const eventoExemplo = `{
  "event": "com.echo.atende.user.protocol.created",
  "id": "evt_68fb1427-896b-44b3-99d5-a2f39565184d",
  "created": "2026-03-06T20:15:13.677Z",
  "specversion": "1.0",
  "tenant": "69a5a131e67bd8e2f54e5981",
  "data": {
    "protocol": {"number": "344200011995", "status": "open", "isHumanAttendance": false, "isReceptive": true, "attendant": null, "createdAt": "2026-04-09T12:49:10.534Z", "updatedAt": "2026-04-09T12:49:10.785Z", "closedAt": null, "attendanceStartedAt": null, "tag": null, "tags": []},
    "contact": {"phone": "5519981621916", "name": "Vanderlei Martins", "email": null},
    "channel": {"type": "whatsapp", "wabaId": "806679752450694", "phoneNumberId": "1049048231621427", "displayNumber": "551940402710"},
    "creation": {"createdBy": "user", "trigger": "incoming_message", "createdByUser": null, "externalReference": null},
    "message": {"id": "wamid.X", "type": "text", "direction": "incoming", "status": "received", "sentAt": "2026-03-06T20:15:12.862Z", "metadata": null, "error": null, "content": "posso fazer barulho depois das 22h?"}
  }
}`

func raizDoRepo(t *testing.T) string {
	_, arq, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(arq), "..", "..")
}

type mcpServicoFalso struct {
	erroToken        error
	erroEmpresa      error
	resultadoEmpresa *mcp.CallResult
	chamadas         []string
}

func (m *mcpServicoFalso) ValidarToken(context.Context) error { return m.erroToken }

func (m *mcpServicoFalso) CallTool(_ context.Context, nome string, _ map[string]any) (*mcp.CallResult, error) {
	m.chamadas = append(m.chamadas, nome)
	if nome == "gruvi.bff.building-operation.get-company-by-document-controller-handle" {
		if m.erroEmpresa != nil {
			return nil, m.erroEmpresa
		}
		if m.resultadoEmpresa != nil {
			return m.resultadoEmpresa, nil
		}
		return &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: `{"data":{"units":[{"id":"109","id_fastpass":"109","label":"A - 101","unit":"A - 101","resident":{"name":"Vanderlei Martins","mobile_number":"5519981621916"}}]}}`}}}, nil
	}
	return &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: `{"ok":true}`}}}, nil
}

func adicionarToolsMCP(cat *okf.Catalogo) {
	original := cat.Recursos
	cat.Recursos = func() []okf.Recurso {
		out := original()
		parametros := map[string][]okf.ParamMeta{
			"gruvi.bff.building-operation.get-company-by-document-controller-handle": {
				{Name: "companyId", Type: "string", Required: true},
				{Name: "filter_mobile_number", Type: "string", Required: true},
			},
			"gruvi.bff.building-operation.fetch-unit-packages-controller-handle": {
				{Name: "companyId", Type: "string", Required: true},
				{Name: "unitId", Type: "string", Required: true},
				{Name: "filter_status", Type: "string"},
				{Name: "filter_mobile_number", Type: "string", Required: true},
			},
			"gruvi.bff.building-operation.get-package-delivery-code-controller-handle": {
				{Name: "companyId", Type: "string", Required: true},
				{Name: "unitId", Type: "string", Required: true},
				{Name: "packageId", Type: "string", Required: true},
				{Name: "filter_mobile_number", Type: "string", Required: true},
			},
			"gruvi.bff.building-operation.fetch-visit-reasons-controller-handle": {
				{Name: "companyId", Type: "string", Required: true},
				{Name: "filter_visitor_type", Type: "string"},
			},
			"gruvi.bff.building-operation.create-visit-controller-handle": {
				{Name: "companyId", Type: "string", Required: true},
				{Name: "unitId", Type: "string", Required: true},
				{Name: "start_datetime", Type: "string", Required: true},
				{Name: "end_datetime", Type: "string", Required: true},
				{Name: "resident", Type: "object", Required: true},
				{Name: "reason", Type: "number", Required: true},
				{Name: "visitors", Type: "array", Required: true},
			},
		}
		for _, nome := range []string{
			"gruvi.bff.building-operation.get-company-by-document-controller-handle",
			"gruvi.bff.building-operation.fetch-unit-packages-controller-handle",
			"gruvi.bff.building-operation.get-package-delivery-code-controller-handle",
			"gruvi.bff.building-operation.fetch-visit-reasons-controller-handle",
			"gruvi.bff.building-operation.create-visit-controller-handle",
		} {
			out = append(out, okf.Recurso{Name: nome, Kind: okf.KindAPI, Visibility: "private", Origem: okf.OrigemMCP, Write: strings.HasSuffix(nome, "create-visit-controller-handle"), Params: parametros[nome]})
		}
		return out
	}
}

func TestParse(t *testing.T) {
	ev, err := Parse([]byte(eventoExemplo))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Event != EventoProtocoloCriado || ev.Data.Protocol.Number != "344200011995" || ev.Data.Contact.Phone != "5519981621916" || !strings.Contains(ev.Texto(), "barulho") {
		t.Fatalf("%+v", ev)
	}
	if _, err := Parse([]byte(`{"event":"x","data":{}}`)); err == nil {
		t.Fatal("sem protocolo deveria falhar")
	}
}

func TestProcessarRecusaTelefoneDiferenteDoB2C(t *testing.T) {
	svc := &Servico{TelefoneB2C: "5511999999999", Sessoes: NewSessoes()}
	ev, err := Parse([]byte(eventoExemplo))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Processar(context.Background(), ev); err == nil || len(svc.Sessoes.Lista()) != 0 {
		t.Fatal("outro telefone deve ser recusado antes de criar uma sessão")
	}
	if !svc.PermiteTelefone("(11) 99999-9999") || svc.PermiteTelefone("") {
		t.Fatal("a validação deve normalizar o telefone e recusar contato vazio")
	}
}

func TestProcessarComMockEOutbox(t *testing.T) {
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	fix, _ := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	cat.Recursos = func() []okf.Recurso { return fix }
	adicionarToolsMCP(cat)
	out := &Outbox{}
	mcpFalso := &mcpServicoFalso{}
	svc := &Servico{
		Cat: cat, Agente: "agentes/concierge", Account: "cond-001", Sessoes: NewSessoes(), Sender: out,
		Config:      harness.Config{Mock: true, Env: func(string) string { return "" }, MCP: mcpFalso},
		TelefoneB2C: "5519981621916", Gruvi: &gruvi.Client{CompanyID: "5982"},
	}
	ev, _ := Parse([]byte(eventoExemplo))
	sess, err := svc.Processar(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if !sess.Identidade.Authenticated || sess.Identidade.CompanyID != "5982" {
		t.Fatalf("identidade: %+v", sess.Identidade)
	}
	if len(sess.Turnos) != 2 || sess.Turnos[0].Quem != "morador" || sess.Turnos[1].Quem != "concierge" || sess.Turnos[1].Texto == "" {
		t.Fatalf("turnos: %+v", sess.Turnos)
	}
	if sess.Turnos[1].Agente != "agentes/concierge/agentes/regimento" {
		t.Fatalf("deveria ter roteado para o regimento: %s", sess.Turnos[1].Agente)
	}
	itens := out.Itens("344200011995")
	if len(itens) != 1 || itens[0].Texto != sess.Turnos[1].Texto {
		t.Fatalf("caixa de saída: %+v", itens)
	}
	// segunda mensagem no mesmo protocolo mantém a sessão e o histórico
	ev2, _ := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "até que horas a piscina abre?", 1)))
	ev2.ID = "evt_2"
	sess2, err := svc.Processar(context.Background(), ev2)
	if err != nil {
		t.Fatal(err)
	}
	if sess2 != sess || len(sess.Turnos) != 4 || len(svc.Sessoes.Lista()) != 1 {
		t.Fatalf("sessão deveria continuar: %d turnos, %d sessões", len(sess.Turnos), len(svc.Sessoes.Lista()))
	}
	// memória relacional: o que o morador afirma sobre si fica guardado por telefone
	st := memoria.NewArquivos(t.TempDir())
	svcM := *svc
	svcM.Sessoes = NewSessoes()
	svcM.Memorias = st
	ev3, _ := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "na verdade só tenho um apartamento, tenho um cachorro", 1)))
	ev3.ID = "evt_3"
	s4, err := svcM.Processar(context.Background(), ev3)
	if err != nil {
		t.Fatal(err)
	}
	svcM.Aguardar()
	ult := s4.Turnos[len(s4.Turnos)-1]
	if len(ult.Memorias) != 2 {
		t.Fatalf("deveria ter capturado unidades e pets: %+v", ult.Memorias)
	}
	mem, _ := st.Ler(context.Background(), "cond-001", "5519981621916")
	if len(mem.Fatos) != 2 || mem.Fatos[0].Fonte != "344200011995" {
		t.Fatalf("memória gravada: %+v", mem)
	}
	// na conversa seguinte do mesmo telefone, a memória vai ao prompt (Contexto)
	var contexto string
	svcM.Sessoes = NewSessoes()
	svcM.Config.ChatModel = nil
	svcM.Config.Env = func(k string) string { return "" }
	// o harness recebe o contexto: conferimos pela instrução do agente montado
	dir, _ := cat.ResolveAgente("agentes/concierge")
	def, _ := cat.Load(dir)
	cfg := svcM.Config
	m2, _ := st.Ler(context.Background(), "cond-001", "5519981621916")
	cfg.Contexto = harness.ContextoDeMemoria(m2)
	ag, err := harness.Build(context.Background(), cat, def, cfg)
	if err != nil {
		t.Fatal(err)
	}
	contexto = ag.Instrucao
	if !strings.Contains(contexto, "Memória sobre o morador") || !strings.Contains(contexto, "unidades: tem só uma unidade") {
		t.Fatalf("contexto sem a memória:\n%s", contexto[len(contexto)-400:])
	}
	if sub := ag.Subagentes["regimento"]; sub == nil || !strings.Contains(sub.Instrucao, "Memória sobre o morador") {
		t.Fatal("a memória deveria chegar aos subagentes também")
	}

	// identidade falha: segue como público, com o erro registrado
	svc2 := *svc
	svc2.Sessoes = NewSessoes()
	svc2.Config.MCP = &mcpServicoFalso{erroToken: errors.New("bff fora")}
	s3, err := svc2.Processar(context.Background(), ev)
	if err == nil || s3.Identidade.Authenticated || !strings.Contains(s3.IdentErro, "bff fora") {
		t.Fatalf("token B2C inválido não deve autenticar a sessão: err=%v sessão=%+v", err, s3.Identidade)
	}
}

func TestContextoDeAgora(t *testing.T) {
	c := contextoDeAgora(time.Date(2026, 10, 17, 23, 5, 0, 0, time.UTC))
	if !strings.Contains(c, "sábado, 17 de outubro de 2026, 20:05") {
		t.Fatalf("agora: %s", c)
	}
}
