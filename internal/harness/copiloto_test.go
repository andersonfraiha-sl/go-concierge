package harness

import (
	"reflect"
	"testing"

	"go-concierge/internal/copiloto"
)

func TestLerRespostaDoModelo(t *testing.T) {
	ligar := true
	esperada := RespostaDoModelo{
		Mensagem: "Liguei a reserva.",
		Operacoes: []copiloto.Operacao{{
			Tipo: copiloto.HabilitarSkill, Skill: "reserva-area-comum", Condominio: "vila-verde", Habilitado: &ligar,
		}},
	}
	puro := `{"mensagem":"Liguei a reserva.","operacoes":[{"tipo":"habilitar_skill","skill":"reserva-area-comum","condominio":"vila-verde","habilitado":true}]}`
	casos := []struct {
		nome    string
		raw     string
		quer    RespostaDoModelo
		querErr bool
	}{
		{"le json puro", puro, esperada, false},
		{"le json dentro de cerca", "```json\n" + puro + "\n```", esperada, false},
		{"le json dentro de cerca sem linguagem", "  ```\n" + puro + "\n```  ", esperada, false},
		{"le resposta sem operacoes", `{"mensagem":"Oi."}`, RespostaDoModelo{Mensagem: "Oi."}, false},
		{"recusa lixo", "não sei responder isso", RespostaDoModelo{}, true},
		{"recusa json sem mensagem", `{"operacoes":[]}`, RespostaDoModelo{}, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := LerRespostaDoModelo(c.raw)
			if (err != nil) != c.querErr {
				t.Fatalf("erro = %v, querErr %v", err, c.querErr)
			}
			if !reflect.DeepEqual(got, c.quer) {
				t.Fatalf("= %+v, quer %+v", got, c.quer)
			}
		})
	}
}

func TestNovoCopilotoSemProvedor(t *testing.T) {
	semChave := func(k string) string { return map[string]string{"LLM_MODE": "gemini"}[k] }
	if _, ok := NovoCopiloto(Config{Env: semChave}); ok {
		t.Fatal("sem chave não há Copiloto")
	}
	comChave := func(k string) string { return map[string]string{"LLM_MODE": "gemini", "GEMINI_API_KEY": "x"}[k] }
	if _, ok := NovoCopiloto(Config{Env: comChave, Mock: true}); ok {
		t.Fatal("mock não tem Copiloto")
	}
	if _, ok := NovoCopiloto(Config{Env: comChave}); !ok {
		t.Fatal("com chave há Copiloto")
	}
}

func TestModeloDoCopiloto(t *testing.T) {
	casos := []struct {
		nome string
		env  map[string]string
		quer string
	}{
		{"usa MODEL_COPILOTO", map[string]string{"GEMINI_API_KEY": "x", "MODEL_COPILOTO": "c", "MODEL_PLANNER": "p"}, "c"},
		{"cai no MODEL_PLANNER", map[string]string{"GEMINI_API_KEY": "x", "MODEL_PLANNER": "p"}, "p"},
		{"cai no padrao do provedor", map[string]string{"LLM_MODE": "gemini", "GEMINI_API_KEY": "x"}, ModeloGemini},
		{"vertex sem chave", map[string]string{"LLM_MODE": "vertex"}, ModeloVertex},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			cfg := Config{Env: func(k string) string { return c.env[k] }}
			if got := modeloDoCopiloto(cfg, ResolverProvedor(cfg.env)); got != c.quer {
				t.Fatalf("modelo = %q, quer %q", got, c.quer)
			}
		})
	}
}
