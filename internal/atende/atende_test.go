package atende

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/entrada"
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

func raizDoRepo(t testing.TB) string {
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
	ignorado, err := Parse([]byte(`{"event":"x","data":{}}`))
	if err != nil || ignorado.Classificar() != EventoIgnorado || ignorado.Validar(EventoIgnorado) != nil {
		t.Fatalf("evento não tratado deveria ser classificado e ignorado: evento=%+v erro=%v", ignorado, err)
	}
}

func TestClassificacaoEValidacaoPorTipo(t *testing.T) {
	criado, err := Parse([]byte(eventoExemplo))
	if err != nil || criado.Classificar() != EventoElegivel || criado.Validar(EventoElegivel) != nil {
		t.Fatalf("evento elegível: class=%v err=%v", criado.Classificar(), err)
	}
	humanoRaw := strings.Replace(eventoExemplo, EventoProtocoloCriado, EventoMensagemAtendente, 1)
	humanoRaw = strings.Replace(humanoRaw, `"phone": "5519981621916"`, `"phone": ""`, 1)
	humano, err := Parse([]byte(humanoRaw))
	if err != nil || humano.Classificar() != EventoHumano || humano.Validar(EventoHumano) != nil {
		t.Fatalf("evento humano não deve exigir telefone: class=%v err=%v", humano.Classificar(), err)
	}
	fechadoRaw := strings.Replace(eventoExemplo, EventoProtocoloCriado, EventoProtocoloFechadoSistema, 1)
	fechadoRaw = strings.Replace(fechadoRaw, `"phone": "5519981621916"`, `"phone": ""`, 1)
	fechado, err := Parse([]byte(fechadoRaw))
	if err != nil || fechado.Classificar() != EventoFechado || fechado.Validar(EventoFechado) != nil {
		t.Fatalf("evento de fechamento não deve exigir telefone: class=%v err=%v", fechado.Classificar(), err)
	}
	semTelefone := strings.Replace(eventoExemplo, `"phone": "5519981621916"`, `"phone": ""`, 1)
	semTelefoneEv, err := Parse([]byte(semTelefone))
	if err != nil || semTelefoneEv.Validar(EventoElegivel) == nil {
		t.Fatal("evento de mensagem elegível sem telefone deveria falhar")
	}
}

func TestSessoesIsolamMesmoProtocoloEntreConectores(t *testing.T) {
	sessoes := NewSessoes()
	real := entrada.Mensagem{ConversaID: "protocolo-igual", Tenant: "tenant", Canal: map[string]string{"displayNumber": "5511"}, Contato: entrada.Contato{Phone: "5511999"}}
	emulado := real
	emulado.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
	realSessao := sessoes.Abrir(real)
	emuladoSessao := sessoes.Abrir(emulado)
	if realSessao.ID == emuladoSessao.ID || realSessao.ID == "" || emuladoSessao.ID == "" {
		t.Fatalf("IDs internos não foram isolados: real=%q emulado=%q", realSessao.ID, emuladoSessao.ID)
	}
	if _, ok := sessoes.Get(realSessao.ID); !ok {
		t.Fatal("consulta por ID interno falhou")
	}
	if _, ok := sessoes.Get("protocolo-igual"); ok {
		t.Fatal("protocolo externo ambíguo deveria exigir ID interno")
	}
}

func TestOutboxFiltraPorIDInternoSemCruzarProtocolo(t *testing.T) {
	out := &Outbox{}
	real := entrada.Destino{Conector: "atende", Protocolo: "P-1", SessaoID: "atende:tenant:canal:P-1"}
	emulado := entrada.Destino{Conector: "atende-emulado", Protocolo: "P-1", SessaoID: "atende-emulado:console:P-1"}
	if err := out.Enviar(context.Background(), real, "resposta real"); err != nil {
		t.Fatal(err)
	}
	if err := out.Enviar(context.Background(), emulado, "resposta emulada"); err != nil {
		t.Fatal(err)
	}
	if got := out.Itens(real.SessaoID); len(got) != 1 || got[0].Texto != "resposta real" {
		t.Fatalf("saída real cruzou a sessão: %+v", got)
	}
	if got := out.Itens(emulado.SessaoID); len(got) != 1 || got[0].Texto != "resposta emulada" {
		t.Fatalf("saída emulada cruzou a sessão: %+v", got)
	}
	if got := out.Itens("P-1"); len(got) != 0 {
		t.Fatalf("protocolo ambíguo não deve misturar saídas: %+v", got)
	}
}

func TestEventoHumanoEFechamentoAtualizamSessaoExistente(t *testing.T) {
	sessoes := NewSessoes()
	msg := entrada.Mensagem{ID: "evt-1", ConversaID: "P-2", Tenant: "tenant", Contato: entrada.Contato{Phone: "5511999"}, Origem: entrada.Origem{Conector: "atende", Meio: "whatsapp", ConexaoID: "tenant:5511"}}
	sess := sessoes.Abrir(msg)
	svc := &Servico{Sessoes: sessoes}
	msg.Autor = "atendente"
	msg.Texto = "fala do atendente"
	if !svc.RegistrarHumano(msg) || !sess.AtendimentoHumano || len(sess.Turnos) != 1 || sess.Turnos[0].Quem != "atendente" {
		t.Fatalf("mensagem humana não foi registrada: sessão=%+v", sess)
	}
	if !svc.Encerrar(msg) || !sess.Encerrada || len(sess.Turnos) != 2 {
		t.Fatalf("fechamento não foi registrado: sessão=%+v", sess)
	}
	antes := len(sess.Turnos)
	if _, err := svc.Processar(context.Background(), msg); err != nil || len(sess.Turnos) != antes {
		t.Fatalf("mensagem posterior não foi ignorada: turnos=%d erro=%v", len(sess.Turnos), err)
	}
}

func TestEventoNormalizadoPreservaDadosDaSessao(t *testing.T) {
	ev, err := Parse([]byte(eventoExemplo))
	if err != nil {
		t.Fatal(err)
	}
	msg := ev.Normalizar()
	if msg.ID != ev.ID || msg.ConversaID != ev.Data.Protocol.Number || msg.Tenant != ev.Tenant || msg.Tipo != ev.Data.Message.Type || msg.Texto != ev.Data.Message.Content {
		t.Fatalf("mensagem normalizada: %+v", msg)
	}
	sess := NewSessoes().Abrir(msg)
	for nome, par := range map[string][2]any{
		"contato": {ev.Data.Contact, sess.Contato},
		"canal":   {ev.Data.Channel, sess.Canal},
	} {
		original, err := json.Marshal(par[0])
		if err != nil {
			t.Fatal(err)
		}
		convertido, err := json.Marshal(par[1])
		if err != nil {
			t.Fatal(err)
		}
		var a, b map[string]any
		if err := json.Unmarshal(original, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(convertido, &b); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s da sessão mudou: original=%s convertido=%s", nome, original, convertido)
		}
	}
}

func TestProcessarEntradaIndependenteDoAtende(t *testing.T) {
	svc, out, _ := servicoComPedidos(t)
	msg := entrada.Mensagem{
		ID:         "mensagem-1",
		ConversaID: "conversa-1",
		Tenant:     "tenant-1",
		Contato:    entrada.Contato{Phone: "5519981621916", Name: "Morador"},
		Canal:      map[string]string{"type": "outro-canal"},
		Tipo:       "text",
		Texto:      "  quais são minhas unidades?  ",
	}
	sess, err := svc.Processar(context.Background(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Protocolo != msg.ConversaID || sess.Tenant != msg.Tenant || sess.Canal["type"] != "outro-canal" || len(sess.Turnos) != 2 || sess.Turnos[0].Texto != "quais são minhas unidades?" || sess.Turnos[0].EventoID != msg.ID {
		t.Fatalf("sessão processada: %+v", sess)
	}
	if saidas := out.Itens(msg.ConversaID); len(saidas) != 1 || saidas[0].Texto != sess.Turnos[1].Texto {
		t.Fatalf("resposta para a conversa: %+v", saidas)
	}
}

func TestProcessarRecusaTelefoneDiferenteDoB2C(t *testing.T) {
	svc := &Servico{TelefoneB2C: "5511999999999", Sessoes: NewSessoes()}
	ev, err := Parse([]byte(eventoExemplo))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Processar(context.Background(), ev.Normalizar()); err == nil || len(svc.Sessoes.Lista()) != 0 {
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
	sess, err := svc.Processar(context.Background(), ev.Normalizar())
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
	sess2, err := svc.Processar(context.Background(), ev2.Normalizar())
	if err != nil {
		t.Fatal(err)
	}
	if sess2 != sess || len(sess.Turnos) != 4 || len(svc.Sessoes.Lista()) != 1 {
		t.Fatalf("sessão deveria continuar: %d turnos, %d sessões", len(sess.Turnos), len(svc.Sessoes.Lista()))
	}
	// memória relacional: o que o morador afirma sobre si fica guardado por telefone
	st := memoria.NewArquivos(t.TempDir())
	svcM := Servico{Cat: svc.Cat, Agente: svc.Agente, Account: svc.Account, Sessoes: NewSessoes(), Sender: svc.Sender, Config: svc.Config, TelefoneB2C: svc.TelefoneB2C, Gruvi: svc.Gruvi, Memorias: st}
	ev3, _ := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "na verdade só tenho um apartamento, tenho um cachorro", 1)))
	ev3.ID = "evt_3"
	s4, err := svcM.Processar(context.Background(), ev3.Normalizar())
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
	svc2 := Servico{Cat: svc.Cat, Agente: svc.Agente, Account: svc.Account, Sessoes: NewSessoes(), Sender: svc.Sender, Config: svc.Config, TelefoneB2C: svc.TelefoneB2C, Gruvi: svc.Gruvi}
	svc2.Config.MCP = &mcpServicoFalso{erroToken: errors.New("bff fora")}
	s3, err := svc2.Processar(context.Background(), ev.Normalizar())
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
