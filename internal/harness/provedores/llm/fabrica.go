package llm

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
)

type Fabrica struct {
	Env func(string) string
}

func (f Fabrica) ModeloPadrao() string {
	env := f.Env
	if env == nil {
		env = os.Getenv
	}
	return ResolverProvedor(env).Modelo
}

func (f Fabrica) Criar(ctx context.Context, finalidade, nome string, prazo time.Duration) (model.ToolCallingChatModel, string, string, bool, error) {
	env := f.Env
	if env == nil {
		env = os.Getenv
	}
	prov := ResolverProvedor(env)
	if nome == "" {
		switch finalidade {
		case "barato":
			nome = env("MODEL_EXTRACT")
			if nome == "" {
				nome = env("MODEL_CHEAP")
			}
		case "copiloto":
			nome = strings.TrimSpace(env("MODEL_COPILOTO"))
		}
	}
	if nome == "" {
		nome = prov.Modelo
	}
	if finalidade == "atendimento" && prov.Nome == "luna" && prov.Chave == "" {
		return nil, nome, prov.Nome, false, errors.New("harness: configure LLM_LUNA_GATEWAY_TOKEN para o gateway Luna")
	}
	if prov.semAutenticacao(prov.Chave) || prov.Nome == "mock" {
		return nil, nome, prov.Nome, false, nil
	}
	base := prov.BaseURL
	if finalidade == "atendimento" && base == "" && prov.Nome != "vertex" {
		base = BaseURLOpenRouter
	}
	m, err := NovoModelo(ctx, prov, prov.Chave, base, nome, prazo)
	return m, nome, prov.Nome, err == nil, err
}
