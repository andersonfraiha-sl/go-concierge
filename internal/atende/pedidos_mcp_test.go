package atende

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/mcp"
	"go-concierge/internal/pedidos"
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
	case "gruvi.bff.building-operation.create-visit-controller-handle":
		m.escritasVisita++
		return resultadoMCPFluxo(`{"data":{"visit":{"clearance_status":"AUTHORIZED"}}}`), nil
	default:
		return resultadoMCPFluxo(`{}`), nil
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
	svc.Config.MCP = fake
	return svc, fake
}

func fixarAgoraDoFluxo(t *testing.T) {
	t.Helper()
	original := pedidos.Agora
	pedidos.Agora = func() time.Time { return time.Date(2026, 10, 29, 0, 30, 0, 0, time.UTC) }
	t.Cleanup(func() { pedidos.Agora = original })
}

func TestCasoRealListaUnidadesPeloMCP(t *testing.T) {
	fixarAgoraDoFluxo(t)
	svc, fake := servicoComMCPFluxo(t)
	fake.resultadoEmpresa = resultadoMCPFluxo(`{"data":{"name":"Residencial Exemplo","units":[{"id":"unit-109","id_external_gruvi":"op-109","id_fastpass":"109","block":"A","unit":"A - 101","resident":{"name":"Vanderlei Martins","mobile_number":"5519981621916"}},{"id":"unit-110","id_external_gruvi":"op-110","id_fastpass":"110","block":"B","unit":"B - 202","resident":{"name":"Outra pessoa","mobile_number":"5511999999999"}}]}}`)
	sess := falar(t, svc, "mcp-unidades", "quais são minhas unidades?")
	texto := ultimoConcierge(sess).Texto
	if !strings.Contains(texto, "A - 101") || strings.Contains(texto, "B - 202") {
		t.Fatalf("resposta deve incluir só a unidade vinculada ao telefone: %q", texto)
	}
	if len(fake.chamadas) != 1 || fake.chamadas[0].nome != "gruvi.bff.building-operation.get-company-by-document-controller-handle" {
		t.Fatalf("listagem de unidades não foi pelo MCP: %+v", fake.chamadas)
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
			args := fake.argsPorNome["gruvi.bff.building-operation.fetch-unit-packages-controller-handle"]
			if args == nil || args["filter_status"] != teste.status {
				t.Fatalf("filter_status enviado ao MCP: %+v, esperado %s; chamadas=%+v", args, teste.status, fake.chamadas)
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
	if !strings.Contains(ultimoConcierge(sess).Texto, "Não consegui concluir") || !strings.Contains(ultimoConcierge(sess).Texto, "bff indisponível") {
		t.Fatalf("falha do MCP precisa chegar ao morador: %q", ultimoConcierge(sess).Texto)
	}
	if len(fake.chamadas) != 2 || fake.chamadas[0].nome != "gruvi.bff.building-operation.get-company-by-document-controller-handle" || fake.chamadas[1].nome != packages {
		t.Fatalf("não deve executar fallback após falha MCP: %+v", fake.chamadas)
	}
}
