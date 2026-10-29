package harness

import "testing"

func TestResolverProvedorLuna(t *testing.T) {
	vals := map[string]string{"LLM_MODE": "luna", "LLM_LUNA_OPENAI_URL": "https://gateway.example/v1", "LLM_LUNA_GATEWAY_TOKEN": "token", "LLM_LUNA_MODEL": "modelo-luna"}
	env := func(k string) string { return vals[k] }
	p := ResolverProvedor(env)
	if p.Nome != "luna" || p.BaseURL != vals["LLM_LUNA_OPENAI_URL"] || p.Chave != "token" || p.Modelo != "modelo-luna" {
		t.Fatalf("configuração incorreta: %+v", p)
	}
	vals["MODEL_PLANNER"] = "outro-modelo"
	if ResolverProvedor(env).Modelo != "outro-modelo" {
		t.Fatal("MODEL_PLANNER deve ter precedência")
	}
	delete(vals, "MODEL_PLANNER")
	delete(vals, "LLM_LUNA_OPENAI_URL")
	delete(vals, "LLM_LUNA_MODEL")
	p = ResolverProvedor(env)
	if p.BaseURL != BaseURLLuna || p.Modelo != ModeloLuna {
		t.Fatal("valores padrão incorretos")
	}
}
