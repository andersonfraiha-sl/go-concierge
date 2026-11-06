package harness

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"go-concierge/internal/responses"
)

// Provedor de modelo, escolhido pelas mesmas variáveis do sl-concierge:
//
//	LLM_MODE=gemini      GEMINI_API_KEY, endpoint OpenAI-compatível do Google
//	LLM_MODE=openrouter  OPENROUTER_API_KEY, https://openrouter.ai/api/v1
//	LLM_MODE=local       modelo local OpenAI-compatível (Ollama), LLM_LOCAL_URL e LLM_LOCAL_MODEL, sem custo de token
//	LLM_MODE=mock        modelo falso, sem rede
//
// Sem LLM_MODE: gemini se houver GEMINI_API_KEY, senão openrouter se houver
// OPENROUTER_API_KEY, senão mock. O modelo padrão vem de MODEL_PLANNER, senão do
// padrão do provedor.
type Provedor struct {
	Nome    string // gemini | openrouter | luna | local | mock
	BaseURL string
	Chave   string
	Modelo  string // modelo padrão do provedor (quando nem Config nem agent.yaml dizem)
}

const (
	BaseURLGemini     = "https://generativelanguage.googleapis.com/v1beta/openai/"
	BaseURLOpenRouter = "https://openrouter.ai/api/v1"
	ModeloGemini      = "gemini-2.5-flash-lite"
	ModeloOpenRouter  = "openai/gpt-4o-mini"
	BaseURLLuna       = "https://kong-ai-gtw.idp.nexus.superlogica.net/v1"
	ModeloLuna        = "gpt-6-luna"
	BaseURLLocal      = "http://127.0.0.1:11434/v1"
	ModeloLocal       = "llama3.1:8b"
	// chaveLocal: o Ollama ignora a chave, mas sem chave o harness cai no mock.
	chaveLocal = "local"
)

// ResolverProvedor lê o ambiente (env) e decide o provedor.
func ResolverProvedor(env func(string) string) Provedor {
	modo := strings.ToLower(strings.TrimSpace(env("LLM_MODE")))
	gemini := strings.TrimSpace(env("GEMINI_API_KEY"))
	router := strings.TrimSpace(env("OPENROUTER_API_KEY"))
	if modo == "" {
		switch {
		case gemini != "":
			modo = "gemini"
		case router != "":
			modo = "openrouter"
		default:
			modo = "mock"
		}
	}
	padrao := strings.TrimSpace(env("MODEL_PLANNER"))
	switch modo {
	case "gemini":
		if padrao == "" {
			padrao = ModeloGemini
		}
		return Provedor{Nome: "gemini", BaseURL: BaseURLGemini, Chave: gemini, Modelo: padrao}
	case "openrouter":
		if padrao == "" {
			padrao = ModeloOpenRouter
		}
		return Provedor{Nome: "openrouter", BaseURL: BaseURLOpenRouter, Chave: router, Modelo: padrao}
	case "luna":
		base := strings.TrimSpace(env("LLM_LUNA_OPENAI_URL"))
		if base == "" {
			base = BaseURLLuna
		}
		if padrao == "" {
			padrao = strings.TrimSpace(env("LLM_LUNA_MODEL"))
		}
		if padrao == "" {
			padrao = ModeloLuna
		}
		return Provedor{Nome: "luna", BaseURL: base, Chave: strings.TrimSpace(env("LLM_LUNA_GATEWAY_TOKEN")), Modelo: padrao}
	case "local":
		return provedorLocal(env, padrao)
	default:
		return Provedor{Nome: "mock", Modelo: "mock"}
	}
}

func provedorLocal(env func(string) string, padrao string) Provedor {
	base := strings.TrimSpace(env("LLM_LOCAL_URL"))
	if base == "" {
		base = BaseURLLocal
	}
	if padrao == "" {
		padrao = strings.TrimSpace(env("LLM_LOCAL_MODEL"))
	}
	if padrao == "" {
		padrao = ModeloLocal
	}
	return Provedor{Nome: "local", BaseURL: base, Chave: chaveLocal, Modelo: padrao}
}

func novoModelo(ctx context.Context, prov Provedor, chave, base, nome string, timeout time.Duration) (model.ToolCallingChatModel, error) {
	if prov.Nome == "luna" {
		return &responses.Model{URL: base, Token: chave, Name: nome, Client: &http.Client{Timeout: timeout}}, nil
	}
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{APIKey: chave, BaseURL: base, Model: nome, Timeout: timeout})
}

// CarregarDotEnv lê arquivos NOME=valor e põe no ambiente o que ainda não existe
// (o ambiente do sistema tem precedência). Arquivo ausente não é erro. Aceita
// comentários (#), aspas simples ou duplas no valor e comentário no fim da linha.
func CarregarDotEnv(caminhos ...string) {
	for _, c := range caminhos {
		f, err := os.Open(c)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			linha := strings.TrimSpace(sc.Text())
			if linha == "" || strings.HasPrefix(linha, "#") {
				continue
			}
			linha = strings.TrimPrefix(linha, "export ")
			k, v, ok := strings.Cut(linha, "=")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			switch {
			case len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\''):
				v = v[1 : len(v)-1]
			default:
				if i := strings.Index(v, " #"); i >= 0 {
					v = strings.TrimSpace(v[:i])
				}
			}
			if k != "" && os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
		f.Close()
	}
}
