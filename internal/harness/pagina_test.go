package harness

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudwego/eino/components/tool/utils"
)

// Modelos menores (e às vezes os grandes) mandam a página como texto; o turno não pode cair por isso.
func TestPaginaAceitaNumeroOuTexto(t *testing.T) {
	casos := []struct {
		json string
		quer int
	}{
		{`{"caminho":"a.md","pagina":2}`, 2},
		{`{"caminho":"a.md","pagina":"3"}`, 3},
		{`{"caminho":"a.md","pagina":""}`, 0},
		{`{"caminho":"a.md"}`, 0},
	}
	for _, c := range casos {
		var in argsLerDado
		if err := json.Unmarshal([]byte(c.json), &in); err != nil {
			t.Fatalf("%s: %v", c.json, err)
		}
		if int(in.Pagina) != c.quer {
			t.Errorf("%s: pagina = %d, want %d", c.json, in.Pagina, c.quer)
		}
	}
	var in argsLerDado
	if err := json.Unmarshal([]byte(`{"caminho":"a.md","pagina":"dois"}`), &in); err == nil {
		t.Error("texto que não é número deve falhar")
	}
}

func TestSchemaDaPaginaContinuaInteiro(t *testing.T) {
	ferramenta, err := utils.InferTool("ler_dado", "d", func(context.Context, *argsLerDado) (saida, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	info, err := ferramenta.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	schema, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(schema)
	var lido struct {
		Properties map[string]struct{ Type string } `json:"properties"`
	}
	if err := json.Unmarshal(raw, &lido); err != nil || lido.Properties["pagina"].Type != "integer" {
		t.Fatalf("schema da página deve continuar inteiro: %s", raw)
	}
}

func TestParamsAceitaObjetoOuTexto(t *testing.T) {
	casos := []struct {
		json string
		quer int
	}{
		{`{"nome":"r","params":{"a":1}}`, 1},
		{`{"nome":"r","params":"{\"a\":1}"}`, 1},
		{`{"nome":"r","params":""}`, 0},
		{`{"nome":"r"}`, 0},
	}
	for _, c := range casos {
		var in argsRecurso
		if err := json.Unmarshal([]byte(c.json), &in); err != nil {
			t.Fatalf("%s: %v", c.json, err)
		}
		if len(in.Params) != c.quer {
			t.Errorf("%s: params = %v", c.json, in.Params)
		}
	}
	var in argsRecurso
	if err := json.Unmarshal([]byte(`{"nome":"r","params":"a=1"}`), &in); err == nil {
		t.Error("texto que não é objeto deve falhar")
	}
}

func TestSchemaDosParamsContinuaObjeto(t *testing.T) {
	ferramenta, err := utils.InferTool("chamar_recurso", "d", func(context.Context, *argsRecurso) (saida, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	info, _ := ferramenta.Info(context.Background())
	schema, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(schema)
	var lido struct {
		Properties map[string]struct{ Type string } `json:"properties"`
	}
	if err := json.Unmarshal(raw, &lido); err != nil || lido.Properties["params"].Type != "object" {
		t.Fatalf("schema dos params deve continuar objeto: %s", raw)
	}
}
