package atende

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/entrada"
	"go-concierge/internal/mcp"
	"go-concierge/internal/okf"
	"go-concierge/internal/pedidos"
	"go-concierge/internal/recursos"
)

type chamadaMCPFluxo struct {
	nome   string
	params map[string]any
}

type mcpFluxoFalso struct {
	chamadas         []chamadaMCPFluxo
	argsPorNome      map[string]map[string]any
	falhas           map[string]error
	resultadoEmpresa *mcp.CallResult
	pacotesStatus    map[string][]any
	motivosVisita    []any
	tentativasVisita int
	escritasVisita   int
	escritasChamado  int
	utcp             *utcpFluxoFalso
}

func (m *mcpFluxoFalso) ValidarToken(context.Context) error { return nil }

func (m *mcpFluxoFalso) CallTool(_ context.Context, nome string, params map[string]any) (*mcp.CallResult, error) {
	args := map[string]any{}
	for chave, valor := range params {
		args[chave] = valor
	}
	m.chamadas = append(m.chamadas, chamadaMCPFluxo{nome: nome, params: args})
	if nome == "gruvi.bff.building-operation.create-visit-controller-handle" {
		m.tentativasVisita++
	}
	if m.argsPorNome == nil {
		m.argsPorNome = map[string]map[string]any{}
	}
	m.argsPorNome[nome] = args
	if err := m.falhas[nome]; err != nil {
		return nil, err
	}
	switch nome {
	case "gruvi.bff.building-operation.get-company-by-document-controller-handle":
		if m.resultadoEmpresa != nil {
			return m.resultadoEmpresa, nil
		}
		return resultadoMCPFluxo(`{"data":{"units":[{"id":"unit-109","id_fastpass":"109","label":"A - 101","unit":"A - 101","resident":{"name":"Vanderlei Martins","mobile_number":"5519981621916"}}]}}`), nil
	case "gruvi.bff.building-operation.fetch-unit-packages-controller-handle":
		status := strings.ToLower(strings.TrimSpace(fmt.Sprint(params["filter_status"])))
		pacotes, existe := m.pacotesStatus[status]
		if !existe {
			pacotes = []any{map[string]any{"id": 42, "description": "Encomenda teste"}}
		}
		conteudo, _ := json.Marshal(map[string]any{"data": pacotes})
		return resultadoMCPFluxo(string(conteudo)), nil
	case "gruvi.bff.building-operation.get-package-delivery-code-controller-handle":
		return resultadoMCPFluxo(`{"data":{"delivery_code":"4831"}}`), nil
	case "gruvi.bff.building-operation.fetch-visit-reasons-controller-handle":
		motivos := m.motivosVisita
		if motivos == nil {
			motivos = []any{map[string]any{"id": 5, "reason": "Visita familiar", "type_provider": "GUEST"}}
		}
		conteudo, _ := json.Marshal(map[string]any{"data": motivos})
		return resultadoMCPFluxo(string(conteudo)), nil
	case "gruvi.bff.mobile.fetch-fees-controller-handle-1":
		if params["filter[paid]"] == true {
			return resultadoMCPFluxo(`{"data":[{"id":"f-pago","due_date":"2026-10-10","value":498.7}]}`), nil
		}
		return resultadoMCPFluxo(`{"links":{},"meta":{"count":2},"data":[{"id":"f-jurid","due_date":"2026-06-10","value":512.3,"billet_unavailable":true},{"id":"f-nov","due_date":"2026-11-10","value":498.7,"billet_unavailable":false}]}`), nil
	case "gruvi.bff.mobile.get-fee-by-id-controller-handle-2":
		if params["feeId"] == "f-pago" {
			return resultadoMCPFluxo(`{"data":{"id":"f-pago","unit_description":"A - 101","due_date":"2026-10-10","value":498.7,"paid_date":"2026-10-08","payment_method":"Pix"}}`), nil
		}
		return resultadoMCPFluxo(`{"data":{"id":"f-nov","unit_description":"A - 101","due_date":"2026-11-10","value":498.7,"billet_code":"23790.00009 00000.000000 00000.000000 1 00000000049870","pix_code":"PIX-SINTETICO"}}`), nil
	case "gruvi.bff.mobile.fetch-documents-controller-handle-1":
		return resultadoMCPFluxo(`{"data":[{"id":"d-ata","file_name":"ata-assembleia-out.pdf","title":"Ata AGO"}]}`), nil
	case "gruvi.bff.mobile.get-document-by-id-controller-handle-1":
		return resultadoMCPFluxo(`{"data":{"link":"https://docs.exemplo.test/ata.pdf"}}`), nil
	case "gruvi.bff.mobile.get-parameters-controller-handle-get-ticket-parameters-c4fee62606":
		return resultadoMCPFluxo(`{"data":{"has_title":false,"has_subject":true,"has_attachment":false,"subjects":[{"id":"s-1","description":"Assunto"}],"types":[{"id":"t-manut","description":"Manutenção"}]}}`), nil
	case "gruvi.bff.mobile.create-ticket-controller-handle":
		m.escritasChamado++
		return resultadoMCPFluxo(`{"data":{"id":"CH-77","status":"OPEN"}}`), nil
	case "gruvi.bff.building-operation.create-visit-controller-handle":
		m.escritasVisita++
		return resultadoMCPFluxo(`{"data":{"visit":{"clearance_status":"AUTHORIZED"}}}`), nil
	default:
		return resultadoMCPFluxo(`{}`), nil
	}
}

type utcpFluxoFalso struct {
	bootstrap        *mcpFluxoFalso
	chamadas         []string
	argsPorNome      map[string]map[string]any
	resultadoEmpresa any
}

const (
	ferramentaEmpresaUTCPFluxo = "agent_utcp.building-operation.GetCompanyByDocumentController_handle"
	ferramentaPacotesUTCPFluxo = "agent_utcp.building-operation.FetchUnitPackagesController_handle"
)

func (f *utcpFluxoFalso) Descrever(_ context.Context, name string) (recursos.Definicao, error) {
	var schema string
	switch name {
	case ferramentaEmpresaUTCPFluxo:
		schema = `{"type":"object","properties":{"companyId":{"type":"string","pattern":"^[0-9]+$"},"filter[mobile-number]":{"type":"string","pattern":"^55\\d{10,11}$"}},"required":["companyId","filter[mobile-number]"],"additionalProperties":false}`
	case ferramentaPacotesUTCPFluxo:
		schema = `{"type":"object","properties":{"companyId":{"type":"string","pattern":"^[0-9]+$"},"unitId":{"type":"string","pattern":"^[0-9]+$"},"filter[mobile-number]":{"type":"string","pattern":"^55\\d{10,11}$"},"filter[status]":{"type":"string","enum":["waiting","WAITING","delivered","DELIVERED","canceled","CANCELED"]}},"required":["companyId","unitId","filter[mobile-number]"],"additionalProperties":false}`
	default:
		return recursos.Definicao{}, fmt.Errorf("recurso UTCP desconhecido")
	}
	return recursos.Definicao{Nome: name, Schema: json.RawMessage(schema)}, nil
}

func (f *utcpFluxoFalso) Executar(_ context.Context, name string, params map[string]any) (any, error) {
	if f.argsPorNome == nil {
		f.argsPorNome = map[string]map[string]any{}
	}
	args := map[string]any{}
	for key, value := range params {
		args[key] = value
	}
	f.argsPorNome[name] = args
	f.chamadas = append(f.chamadas, name)
	switch name {
	case ferramentaEmpresaUTCPFluxo:
		if f.resultadoEmpresa != nil {
			return f.resultadoEmpresa, nil
		}
		if f.bootstrap.resultadoEmpresa != nil {
			return f.bootstrap.resultadoEmpresa.Valor(), nil
		}
		return map[string]any{"data": map[string]any{"units": []any{map[string]any{"id": "unit-109", "id_fastpass": "109", "unit": "A - 101", "resident": map[string]any{"mobile_number": "5519981621916"}}}}}, nil
	case ferramentaPacotesUTCPFluxo:
		if err := f.bootstrap.falhas["gruvi.bff.building-operation.fetch-unit-packages-controller-handle"]; err != nil {
			return nil, err
		}
		status := strings.ToLower(strings.TrimSpace(fmt.Sprint(params["filter[status]"])))
		packages, exists := f.bootstrap.pacotesStatus[status]
		if !exists {
			packages = []any{map[string]any{"id": 42, "description": "Encomenda teste"}}
		}
		return map[string]any{"data": packages}, nil
	default:
		return nil, fmt.Errorf("recurso UTCP desconhecido")
	}
}

func resultadoMCPFluxo(texto string) *mcp.CallResult {
	return &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: texto}}}
}

type extratorPedidosFunc func(context.Context, pedidos.Entrada) (pedidos.Extracao, error)

func (f extratorPedidosFunc) Extrair(ctx context.Context, entrada pedidos.Entrada) (pedidos.Extracao, error) {
	return f(ctx, entrada)
}

func extratorVisitaMCP() pedidos.Extrator {
	return extratorPedidosFunc(func(ctx context.Context, entrada pedidos.Entrada) (pedidos.Extracao, error) {
		if strings.Contains(entrada.Mensagem, "liberar Ana Souza") {
			return pedidos.Extracao{Pedidos: []pedidos.PedidoExtraido{{Caso: "liberacao_visitante", Valores: map[string]any{
				"visitante_nome": "Ana Souza", "visitante_telefone": "5511999999999", "data_visita": "2026-10-28", "categoria_motivo": "GUEST",
			}}}}, nil
		}
		return (pedidos.Regras{}).Extrair(ctx, entrada)
	})
}

func servicoComMCPFluxo(t *testing.T) (*Servico, *mcpFluxoFalso) {
	t.Helper()
	svc, _, _ := servicoComPedidos(t)
	fake := &mcpFluxoFalso{falhas: map[string]error{}, pacotesStatus: map[string][]any{}}
	fake.utcp = &utcpFluxoFalso{bootstrap: fake}
	svc.Config.MCP = fake
	svc.Config.Executores = map[string]recursos.Cliente{okf.OrigemUTCP: fake.utcp}
	return svc, fake
}

func fixarAgoraDoFluxo(t *testing.T) {
	t.Helper()
	original := pedidos.Agora
	pedidos.Agora = func() time.Time { return time.Date(2026, 10, 29, 0, 30, 0, 0, time.UTC) }
	t.Cleanup(func() { pedidos.Agora = original })
}

func TestCasoRealListaUnidadesPeloUTCP(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	fake.resultadoEmpresa = resultadoMCPFluxo(`{"data":{"name":"Residencial Exemplo","units":[{"id":"unit-109","id_external_gruvi":"op-109","id_fastpass":"109","block":"A","unit":"A - 101","resident":{"name":"Vanderlei Martins","mobile_number":"5519981621916"}},{"id":"unit-110","id_external_gruvi":"op-110","id_fastpass":"110","block":"B","unit":"B - 202","resident":{"name":"Outra pessoa","mobile_number":"5511999999999"}}]}}`)
	sess := falar(t, svc, "mcp-unidades", "quais são minhas unidades?")
	texto := ultimoConcierge(sess).Texto
	if !strings.Contains(texto, "A - 101") || strings.Contains(texto, "B - 202") {
		t.Fatalf("resposta deve incluir só a unidade vinculada ao telefone: %q", texto)
	}
	if len(fake.chamadas) != 1 || fake.chamadas[0].nome != "gruvi.bff.building-operation.get-company-by-document-controller-handle" || len(fake.utcp.chamadas) != 1 || fake.utcp.chamadas[0] != ferramentaEmpresaUTCPFluxo {
		t.Fatalf("bootstrap MCP e consulta UTCP deveriam ser chamadas separadamente: MCP=%+v UTCP=%+v", fake.chamadas, fake.utcp.chamadas)
	}
	if !sess.Identidade.Authenticated || len(sess.Identidade.Units) != 1 {
		t.Fatalf("vínculo comprovado deveria autenticar o telefone e as unidades: %+v", sess.Identidade)
	}
	if len(sess.resultadosMCP()) != 1 {
		t.Fatalf("resposta de vínculo deveria ficar em cache por sessão: %+v", sess.resultadosMCP())
	}
	cached := sess.resultadosMCP()["gruvi.bff.building-operation.get-company-by-document-controller-handle"].(map[string]any)
	if units, ok := cached["units"].([]any); !ok || len(units) != 1 {
		t.Fatalf("cache deve conter somente a unidade vinculada: %+v", cached)
	}
}

func TestTokenB2CSemVinculoNaoAutentica(t *testing.T) {
	svc, fake := servicoComMCPFluxo(t)
	fake.resultadoEmpresa = resultadoMCPFluxo(`{"data":{"units":[{"id":"unit-109","id_fastpass":"109","resident":{"name":"Outra pessoa","mobile_number":"5511999999999"}}]}}`)
	ev, err := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "oi", 1)))
	if err != nil {
		t.Fatal(err)
	}
	ev.ID = "mcp-identidade-sem-vinculo"
	sess, err := svc.Processar(context.Background(), ev.Normalizar())
	if err != nil {
		t.Fatalf("contato sem vínculo pode continuar como público: %v", err)
	}
	if sess.Identidade.Authenticated || sess.Identidade.Role != "" || len(sess.Identidade.Units) != 0 {
		t.Fatalf("token válido sem correspondência telefônica não eleva identidade: %+v", sess.Identidade)
	}
	if len(fake.chamadas) != 1 || len(sess.resultadosMCP()) != 0 {
		t.Fatalf("resultado sem vínculo não deve ser reutilizado como contexto privado: chamadas=%+v cache=%+v", fake.chamadas, sess.resultadosMCP())
	}
}

func TestFalhaMCPNaValidacaoDeVinculoMantemSessaoNaoAutenticada(t *testing.T) {
	svc, fake := servicoComMCPFluxo(t)
	const company = "gruvi.bff.building-operation.get-company-by-document-controller-handle"
	fake.falhas[company] = errors.New("MCP indisponível")
	ev, err := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "oi", 1)))
	if err != nil {
		t.Fatal(err)
	}
	ev.ID = "mcp-identidade-falha"
	sess, err := svc.Processar(context.Background(), ev.Normalizar())
	if err == nil || !strings.Contains(err.Error(), "MCP indisponível") || sess.Identidade.Authenticated {
		t.Fatalf("falha ao comprovar vínculo deve manter identidade não autenticada: sessão=%+v erro=%v", sess.Identidade, err)
	}
	if len(fake.chamadas) != 1 || len(sess.resultadosMCP()) != 0 {
		t.Fatalf("falha de vínculo não deve usar cache nem fallback: chamadas=%+v cache=%+v", fake.chamadas, sess.resultadosMCP())
	}
}

func TestCasoRealEncomendasUsaStatusExplicitoOuWaitingPadrao(t *testing.T) {
	testes := []struct {
		nome, id, mensagem, status string
		vazio                      bool
	}{
		{"waiting padrão", "waiting-padrao", "minhas encomendas?", "waiting", false},
		{"delivered", "delivered", "minhas encomendas entregues", "delivered", false},
		{"canceled", "canceled", "minhas encomendas canceladas", "canceled", false},
		{"sem encomendas", "sem-encomendas", "minhas encomendas?", "waiting", true},
	}
	for _, teste := range testes {
		t.Run(teste.nome, func(t *testing.T) {
			fixarAgoraDoFluxo(t)
			svc, fake := servicoComMCPFluxo(t)
			if teste.vazio {
				fake.pacotesStatus[teste.status] = []any{}
			}
			sess := falar(t, svc, "mcp-encomendas-"+teste.id, teste.mensagem)
			args := fake.utcp.argsPorNome[ferramentaPacotesUTCPFluxo]
			if args == nil || args["filter[status]"] != teste.status {
				t.Fatalf("filter[status] enviado ao UTCP: %+v, esperado %s; chamadas=%+v", args, teste.status, fake.utcp.chamadas)
			}
			if _, ok := args["unitId"].(string); !ok {
				t.Fatalf("unitId deve chegar como texto segundo a OpenAPI: %+v", args)
			}
			texto := ultimoConcierge(sess).Texto
			if teste.vazio {
				if !strings.Contains(texto, "Não há encomendas aguardando retirada") {
					t.Fatalf("resposta vazia para o status padrão: %q", texto)
				}
			} else if !strings.Contains(texto, "Encomenda teste") {
				t.Fatalf("encomenda não apareceu na resposta: %q", texto)
			}
		})
	}
}

func TestCasoEncomendasMantemAllowlistUTCPEntreTurnos(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	fake.resultadoEmpresa = resultadoMCPFluxo(`{"data":{"units":[{"id":"interno-109","id_fastpass":"109","unit":"A - 101","resident":{"mobile_number":"5519981621916"}}]}}`)
	fake.utcp.resultadoEmpresa = map[string]any{"data": map[string]any{"units": []any{
		map[string]any{"id": "interno-109", "id_fastpass": float64(109), "unit": "A - 101", "resident": map[string]any{"mobile_number": "5519981621916"}},
		map[string]any{"id": "interno-110", "id_fastpass": float64(110), "unit": "B - 202", "resident": map[string]any{"mobile_number": "5519981621916"}},
	}}}
	sess := falar(t, svc, "utcp-escolha-unidade", "minhas encomendas?")
	if !strings.Contains(ultimoConcierge(sess).Texto, "De qual unidade?") {
		t.Fatalf("o primeiro turno deveria solicitar unidade: %q", ultimoConcierge(sess).Texto)
	}
	if !sess.UTCPVinculoResolvido || len(sess.UTCPUnidades) != 2 {
		t.Fatalf("allowlist UTCP não foi salva na sessão: %+v", sess.UTCPUnidades)
	}
	sess = falar(t, svc, "utcp-escolha-unidade-2", "B - 202")
	args := fake.utcp.argsPorNome[ferramentaPacotesUTCPFluxo]
	if args["unitId"] != "110" {
		t.Fatalf("unidade selecionada não chegou como id_fastpass string: args=%+v resposta=%q", args, ultimoConcierge(sess).Texto)
	}
}

func TestSessaoNaoRestauraUnidadesUTCPDoBootstrapDepoisDeVinculoVazio(t *testing.T) {
	sess := &Sessao{ID: "s", Contato: entrada.Contato{Phone: "5519981621916"}, UTCPCompanyID: "5982", UTCPPhone: "5519981621916", UTCPVinculoResolvido: true, UTCPUnidades: []string{}}
	bootstrap := map[string]struct{}{"109": {}}
	if units := sess.unidadesUTCP("5982", "5519981621916", bootstrap); len(units) != 0 {
		t.Fatalf("vínculo UTCP vazio foi reabilitado pelo bootstrap MCP: %v", units)
	}
	if units := sess.unidadesUTCP("9999", "5519981621916", bootstrap); len(units) != 0 {
		t.Fatalf("troca de empresa reutilizou unidades antigas: %v", units)
	}
	if units := sess.unidadesUTCP("5982", "5511999999999", bootstrap); len(units) != 0 {
		t.Fatalf("troca de telefone reutilizou unidades antigas: %v", units)
	}
	record, err := sess.Registro()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := RestaurarSessao(record)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.UTCPVinculoResolvido || restored.UTCPPhone != "5519981621916" || len(restored.UTCPUnidades) != 0 {
		t.Fatalf("roundtrip perdeu allowlist vazia: %+v", restored)
	}
	if units := restored.unidadesUTCP("5982", "5519981621916", bootstrap); len(units) != 0 {
		t.Fatalf("restauração reviveu bootstrap após vínculo vazio: %v", units)
	}
	if units := idsFastpassVinculados(map[string]any{"units": []any{map[string]any{"id": "109"}}}); len(units) != 0 {
		t.Fatalf("ID interno sem id_fastpass não deve autorizar encomendas: %v", units)
	}
	if units := idsFastpassVinculados(map[string]any{"units": []any{map[string]any{"id": "interno", "id_fastpass": float64(109)}}}); len(units) != 1 {
		t.Fatalf("id_fastpass numérico não foi normalizado: %v", units)
	}
}

func TestCasoRealCodigoRetiradaConsultaSomenteWaiting(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	sess := falar(t, svc, "mcp-codigo", "qual o código de retirada da minha encomenda?")
	if texto := ultimoConcierge(sess).Texto; !strings.Contains(texto, "4831") {
		t.Fatalf("código de retirada não apareceu: %q", texto)
	}
	packages := fake.argsPorNome["gruvi.bff.building-operation.fetch-unit-packages-controller-handle"]
	if packages == nil || packages["filter_status"] != "waiting" {
		t.Fatalf("código deve buscar apenas encomendas aguardando retirada: %+v", packages)
	}
	codeArgs := fake.argsPorNome["gruvi.bff.building-operation.get-package-delivery-code-controller-handle"]
	if packageID, ok := codeArgs["packageId"].(string); !ok || packageID != "42" {
		t.Fatalf("packageId deve ser string e usar a encomenda listada: %+v", codeArgs)
	}
	if _, ok := packages["unitId"].(string); !ok {
		t.Fatalf("unitId deve chegar como texto segundo a OpenAPI: %+v", packages)
	}
}

func TestCasoRealVisitanteConfirmaEEscreveUmaVezPeloMotor(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	svc.ExtratorPedidos = extratorVisitaMCP()
	sess := falar(t, svc, "mcp-visita-1", "quero liberar Ana Souza hoje")
	if !strings.Contains(ultimoConcierge(sess).Texto, "Confirma a liberação") || fake.tentativasVisita != 0 {
		t.Fatalf("a primeira etapa deve pedir confirmação sem escrever: resposta=%q chamadas=%+v", ultimoConcierge(sess).Texto, fake.chamadas)
	}
	sess = falar(t, svc, "mcp-visita-2", "sim")
	if fake.tentativasVisita != 1 || fake.escritasVisita != 1 || !strings.Contains(ultimoConcierge(sess).Texto, "Prontinho") {
		t.Fatalf("confirmação válida deve criar uma visita: resposta=%q chamadas=%+v", ultimoConcierge(sess).Texto, fake.chamadas)
	}
	falar(t, svc, "mcp-visita-3", "sim")
	if fake.tentativasVisita != 1 || fake.escritasVisita != 1 {
		t.Fatalf("a visita concluída não pode ser escrita novamente: chamadas=%+v", fake.chamadas)
	}
	args := fake.argsPorNome["gruvi.bff.building-operation.create-visit-controller-handle"]
	if args == nil || args["unitId"] != "109" || fmt.Sprint(args["reason"]) != "5" {
		t.Fatalf("a escrita deve usar unidade e motivo escolhidos pelo caso: %+v", args)
	}
	if _, ok := args["unitId"].(string); !ok {
		t.Fatalf("unitId deve chegar como texto segundo a OpenAPI: %+v", args)
	}
	if _, ok := args["reason"].(float64); !ok {
		t.Fatalf("reason deve chegar como número: %+v", args)
	}
	if _, ok := args["resident"].(map[string]any); !ok {
		t.Fatalf("resident deve chegar como objeto: %+v", args)
	}
	if _, ok := args["visitors"].([]any); !ok {
		t.Fatalf("visitors deve chegar como lista: %+v", args)
	}
}

func TestCasoRealVisitanteComListaDeMotivosVazia(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	fake.motivosVisita = []any{}
	svc.ExtratorPedidos = extratorVisitaMCP()
	sess := falar(t, svc, "mcp-visita-sem-motivos", "quero liberar Ana Souza hoje")
	texto := ultimoConcierge(sess).Texto
	if !strings.Contains(texto, "Não há motivos disponíveis para essa categoria") || fake.tentativasVisita != 0 {
		t.Fatalf("lista vazia de motivos deve encerrar o caso sem escrita: resposta=%q chamadas=%+v", texto, fake.chamadas)
	}
}

func TestCasoVisitanteResultadoIncertoNaoRepeteEscrita(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	const visita = "gruvi.bff.building-operation.create-visit-controller-handle"
	fake.falhas[visita] = errors.New("timeout")
	svc.ExtratorPedidos = extratorVisitaMCP()
	sess := falar(t, svc, "mcp-visita-incerta-1", "quero liberar Ana Souza hoje")
	if !strings.Contains(ultimoConcierge(sess).Texto, "Confirma a liberação") || fake.tentativasVisita != 0 {
		t.Fatalf("pedido deve aguardar confirmação antes da escrita: resposta=%q", ultimoConcierge(sess).Texto)
	}
	sess = falar(t, svc, "mcp-visita-incerta-2", "sim")
	if fake.tentativasVisita != 1 || !strings.Contains(ultimoConcierge(sess).Texto, "incerto após o envio") {
		t.Fatalf("timeout deve ser tratado como resultado incerto sem retry: resposta=%q chamadas=%+v", ultimoConcierge(sess).Texto, fake.chamadas)
	}
	falar(t, svc, "mcp-visita-incerta-3", "sim")
	if fake.tentativasVisita != 1 {
		t.Fatalf("resultado incerto não pode repetir a escrita: chamadas=%+v", fake.chamadas)
	}
}

func TestFalhaMCPDeEncomendasNaoUsaFallback(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	const packages = "gruvi.bff.building-operation.fetch-unit-packages-controller-handle"
	fake.falhas[packages] = errors.New("bff indisponível")
	sess := falar(t, svc, "mcp-falha", "minhas encomendas?")
	if !strings.Contains(ultimoConcierge(sess).Texto, "Não consegui concluir") {
		t.Fatalf("falha UTCP precisa chegar ao morador: %q", ultimoConcierge(sess).Texto)
	}
	if len(fake.chamadas) != 1 || len(fake.utcp.chamadas) != 2 || fake.utcp.chamadas[0] != ferramentaEmpresaUTCPFluxo || fake.utcp.chamadas[1] != ferramentaPacotesUTCPFluxo {
		t.Fatalf("não deve executar fallback MCP após falha UTCP: MCP=%+v UTCP=%+v", fake.chamadas, fake.utcp.chamadas)
	}
}

func TestCasoRealBoletoPulaCobrancaIndisponivelEFixaOMorador(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	sess := falar(t, svc, "mcp-boleto", "me manda a segunda via do boleto")
	texto := ultimoConcierge(sess).Texto
	for _, quer := range []string{"23790.00009", "PIX-SINTETICO", "2026-11-10"} {
		if !strings.Contains(texto, quer) {
			t.Fatalf("boleto sem %q: %q", quer, texto)
		}
	}
	detalhe := fake.argsPorNome["gruvi.bff.mobile.get-fee-by-id-controller-handle-2"]
	if detalhe["feeId"] != "f-nov" {
		t.Fatalf("deve pular a cobrança sem boleto e usar a disponível sem perguntar: %+v", detalhe)
	}
	lista := fake.argsPorNome["gruvi.bff.mobile.fetch-fees-controller-handle-1"]
	if lista["persona"] != "resident" || lista["companyId"] == nil {
		t.Fatalf("lista de cobranças deve sair como morador do condomínio do canal: %+v", lista)
	}
}

func TestCasoRealPagamentoConfereARegistradaComoMorador(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	sess := falar(t, svc, "mcp-pagamento", "já paguei o condomínio, consta aí?")
	texto := ultimoConcierge(sess).Texto
	for _, quer := range []string{"Pagamento registrado", "2026-10-08", "Pix"} {
		if !strings.Contains(texto, quer) {
			t.Fatalf("confirmação sem %q: %q", quer, texto)
		}
	}
	lista := fake.argsPorNome["gruvi.bff.mobile.fetch-fees-controller-handle-1"]
	if lista["filter[paid]"] != true || lista["persona"] != "resident" || lista["companyId"] == nil {
		t.Fatalf("deve listar só as pagas, como morador do condomínio do canal: %+v", lista)
	}
}

func TestCasoRealDebitoListaAbertasComIndisponivelEOfereceAtendente(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	sess := falar(t, svc, "mcp-debito", "quanto eu devo de condomínio?")
	texto := ultimoConcierge(sess).Texto
	for _, quer := range []string{"2026-06-10", "sem boleto disponível", "2026-11-10", "atendente"} {
		if !strings.Contains(texto, quer) {
			t.Fatalf("resumo do débito sem %q: %q", quer, texto)
		}
	}
	if lista := fake.argsPorNome["gruvi.bff.mobile.fetch-fees-controller-handle-1"]; lista["persona"] != "resident" || lista["filter[only-open]"] != true {
		t.Fatalf("débito deve listar só as abertas, como morador: %+v", lista)
	}
}

func TestCasoRealDocumentosPassaOLinkComoMorador(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	sess := falar(t, svc, "mcp-docs", "me manda a ata da última assembleia")
	if texto := ultimoConcierge(sess).Texto; !strings.Contains(texto, "https://docs.exemplo.test/ata.pdf") {
		t.Fatalf("documento sem link: %q", texto)
	}
	detalhe := fake.argsPorNome["gruvi.bff.mobile.get-document-by-id-controller-handle-1"]
	if detalhe["documentId"] != "d-ata" || detalhe["persona"] != "resident" || detalhe["companyId"] == nil {
		t.Fatalf("documento deve sair como morador do condomínio do canal: %+v", detalhe)
	}
}

func TestCasoRealChamadoConfirmaEAbreUmaVez(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	svc.ExtratorPedidos = extratorPedidosFunc(func(ctx context.Context, entrada pedidos.Entrada) (pedidos.Extracao, error) {
		if strings.Contains(entrada.Mensagem, "vazamento") {
			return pedidos.Extracao{Pedidos: []pedidos.PedidoExtraido{{Caso: "chamado", Valores: map[string]any{"descricao": "vazamento na garagem, vaga 12"}}}}, nil
		}
		return (pedidos.Regras{}).Extrair(ctx, entrada)
	})
	sess := falar(t, svc, "mcp-chamado-1", "tem um vazamento na garagem")
	if texto := ultimoConcierge(sess).Texto; !strings.Contains(texto, "Confirma a abertura do chamado de Manutenção") || fake.escritasChamado != 0 {
		t.Fatalf("deve pedir confirmação com o tipo antes de escrever: resposta=%q chamadas=%+v", texto, fake.chamadas)
	}
	sess = falar(t, svc, "mcp-chamado-2", "sim")
	if texto := ultimoConcierge(sess).Texto; fake.escritasChamado != 1 || !strings.Contains(texto, "CH-77") {
		t.Fatalf("confirmação deve abrir um chamado: resposta=%q chamadas=%+v", texto, fake.chamadas)
	}
	falar(t, svc, "mcp-chamado-3", "sim")
	if fake.escritasChamado != 1 {
		t.Fatalf("chamado aberto não pode ser criado de novo: chamadas=%+v", fake.chamadas)
	}
	args := fake.argsPorNome["gruvi.bff.mobile.create-ticket-controller-handle"]
	if args["unit_id"] != "unit-109" || args["type_id"] != "t-manut" || args["description"] != "vazamento na garagem, vaga 12" || args["persona"] != "resident" || args["companyId"] == nil {
		t.Fatalf("chamado deve usar unidade, tipo e descrição do caso, como morador do canal: %+v", args)
	}
}
