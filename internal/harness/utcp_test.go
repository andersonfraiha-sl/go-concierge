package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"go-concierge/internal/okf"
	"go-concierge/internal/recursos"
)

type fakeRecursoExecutor struct {
	descriptions atomic.Int32
	calls        atomic.Int32
	definition   recursos.Definicao
	definitions  map[string]recursos.Definicao
	result       any
	results      map[string]any
	callsByName  map[string]map[string]any
}

func (f *fakeRecursoExecutor) Descrever(_ context.Context, name string) (recursos.Definicao, error) {
	f.descriptions.Add(1)
	if definition, ok := f.definitions[name]; ok {
		return definition, nil
	}
	return f.definition, nil
}

func (f *fakeRecursoExecutor) Executar(_ context.Context, name string, args map[string]any) (any, error) {
	f.calls.Add(1)
	if f.callsByName == nil {
		f.callsByName = map[string]map[string]any{}
	}
	copy := map[string]any{}
	for key, value := range args {
		copy[key] = value
	}
	f.callsByName[name] = copy
	if result, ok := f.results[name]; ok {
		return result, nil
	}
	return f.result, nil
}

func TestUTCPSemPoliticaPrivacidadeFalhaAntesDeQualquerExecutorOuLocal(t *testing.T) {
	const name = "agent_utcp.building-operation.FetchUnitPackagesController_handle"
	fake := &fakeRecursoExecutor{}
	localCalls := atomic.Int32{}
	ag := &Agente{cfg: Config{
		Executores: map[string]recursos.Cliente{okf.OrigemUTCP: fake},
		Locais: map[string]func(context.Context, map[string]any) (any, error){name: func(context.Context, map[string]any) (any, error) {
			localCalls.Add(1)
			return "local", nil
		}},
	}}
	resource := okf.Recurso{Name: name, Kind: okf.KindAPI, Visibility: "private", Origem: okf.OrigemUTCP}
	out, err := ag.executar(context.Background(), chamada{nome: name, achado: &Permissao{RecursoDeclarado: okf.RecursoDeclarado{Nome: name, OK: true, Recurso: &resource}, Permitido: true}, params: map[string]any{"unitId": "109"}})
	if err != nil {
		t.Fatal(err)
	}
	if out["error"] == nil {
		t.Fatalf("esperava bloqueio por política ausente, veio %v", out)
	}
	if fake.descriptions.Load() != 0 || fake.calls.Load() != 0 || localCalls.Load() != 0 {
		t.Fatalf("política ausente acionou executor/local: describes=%d calls=%d locals=%d", fake.descriptions.Load(), fake.calls.Load(), localCalls.Load())
	}
}

func fakePilotExecutor() *fakeRecursoExecutor {
	company := json.RawMessage(`{"type":"object","properties":{"companyId":{"type":"string","pattern":"^[0-9]+$"},"filter[mobile-number]":{"type":"string","pattern":"^55\\d{10,11}$"}},"required":["companyId","filter[mobile-number]"],"additionalProperties":false}`)
	packages := json.RawMessage(`{"type":"object","properties":{"companyId":{"type":"string","pattern":"^[0-9]+$"},"unitId":{"type":"string","pattern":"^[0-9]+$"},"filter[mobile-number]":{"type":"string","pattern":"^55\\d{10,11}$"},"filter[status]":{"type":"string","enum":["waiting","WAITING","delivered","DELIVERED","canceled","CANCELED"]}},"required":["companyId","unitId","filter[mobile-number]"],"additionalProperties":false}`)
	return &fakeRecursoExecutor{definitions: map[string]recursos.Definicao{
		"agent_utcp.building-operation.GetCompanyByDocumentController_handle": {Nome: "agent_utcp.building-operation.GetCompanyByDocumentController_handle", Schema: company},
		"agent_utcp.building-operation.FetchUnitPackagesController_handle":    {Nome: "agent_utcp.building-operation.FetchUnitPackagesController_handle", Schema: packages},
	}}
}

func TestUTCPPoliticaDeUnidadeRejeitaIDNaoVinculadoAntesDaExecucao(t *testing.T) {
	name := "agent_utcp.building-operation.FetchUnitPackagesController_handle"
	fake := fakePilotExecutor()
	allowed := map[string]bool{"109": true}
	ag := &Agente{cfg: Config{
		Executores: map[string]recursos.Cliente{okf.OrigemUTCP: fake},
		ValidarParametros: map[string]func(map[string]any) error{name: func(params map[string]any) error {
			if !allowed[fmt.Sprint(params["unitId"])] {
				return errors.New("unidade não vinculada")
			}
			return nil
		}},
		TransformarResultados: map[string]func(any) (any, error){name: func(value any) (any, error) { return value, nil }},
	}}
	params := map[string]any{"companyId": "5982", "unitId": "110", "filter[mobile-number]": "5519981621916", "filter[status]": "waiting"}
	out, err := ag.executarExecutor(context.Background(), chamada{nome: name, params: params}, okf.OrigemUTCP)
	if err != nil || out["error"] == nil {
		t.Fatalf("unidade não vinculada deveria ser recusada: out=%v err=%v", out, err)
	}
	if fake.calls.Load() != 0 {
		t.Fatalf("a requisição de unidade não autorizada chegou ao executor: %d", fake.calls.Load())
	}
	params["unitId"] = "109"
	out, err = ag.executarExecutor(context.Background(), chamada{nome: name, params: params}, okf.OrigemUTCP)
	if err != nil || out["error"] != nil || fake.calls.Load() != 1 {
		t.Fatalf("unidade vinculada deveria executar: out=%v err=%v calls=%d", out, err, fake.calls.Load())
	}
}

func TestNormalizarResultadoUTCPSeparaEnvelopeEFalhaFuncional(t *testing.T) {
	for _, value := range []any{map[string]any{"error": nil, "data": []any{}}, map[string]any{"data": []any{}}} {
		result, err := NormalizarResultadoUTCP(value)
		if err != nil {
			t.Fatalf("envelope vazio válido foi recusado: %v", err)
		}
		if list, ok := result.([]any); !ok || len(list) != 0 {
			t.Fatalf("lista vazia não foi preservada: %T %v", result, result)
		}
	}
	for _, value := range []any{
		map[string]any{"ok": false, "message": "falha"},
		map[string]any{"data": map[string]any{"success": false, "message": "falha"}},
		map[string]any{"isError": true},
		map[string]any{"statusCode": float64(500)},
	} {
		if _, err := NormalizarResultadoUTCP(value); err == nil {
			t.Fatalf("falha funcional aceita: %v", value)
		}
	}
}

func TestValidarJSONSchemaPreservaRefsRequiredPatternEnumEAdicionais(t *testing.T) {
	schema := json.RawMessage(`{
		"$defs":{"id":{"type":"string","pattern":"^[0-9]+$"}},
		"type":"object","properties":{
			"companyId":{"$ref":"#/$defs/id"},
			"filter[mobile-number]":{"type":"string","pattern":"^55\\d{10,11}$"},
			"filter[status]":{"type":"string","enum":["waiting","WAITING","delivered","DELIVERED","canceled","CANCELED"]}
		},"required":["companyId","filter[mobile-number]"],"additionalProperties":false
	}`)
	valid := map[string]any{"companyId": "5982", "filter[mobile-number]": "5519981621916", "filter[status]": "waiting"}
	if err := validarJSONSchema(schema, valid); err != nil {
		t.Fatalf("schema recusou os valores válidos: %v", err)
	}
	unitSchema := json.RawMessage(`{"type":"object","properties":{"unitId":{"type":"string","pattern":"^[0-9]+$"}},"required":["unitId"],"additionalProperties":false}`)
	unitParams := map[string]any{"unitId": 109}
	if err := validarJSONSchema(unitSchema, unitParams); err != nil || unitParams["unitId"] != "109" {
		t.Fatalf("unitId numérico seguro deveria normalizar para string: params=%v err=%v", unitParams, err)
	}
	for _, invalid := range []map[string]any{
		{"filter[mobile-number]": "5519981621916"},
		{"companyId": "5982", "filter[mobile-number]": "119981621916"},
		{"companyId": "5982", "filter[mobile-number]": "5519981621916", "filter[status]": "pending"},
		{"companyId": "5982", "filter[mobile-number]": "5519981621916", "extra": "x"},
	} {
		if err := validarJSONSchema(schema, invalid); err == nil {
			t.Fatalf("schema aceitou argumentos inválidos: %v", invalid)
		}
	}
	if err := validarJSONSchema(schema, nil); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("esperava rejeição do objeto ausente, veio %v", err)
	}
}
