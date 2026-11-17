package harness

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"
)

func novoModeloVertex(ctx context.Context, prov Provedor, base, nome string, timeout time.Duration) (model.ToolCallingChatModel, error) {
	if prov.Projeto == "" {
		return nil, errors.New("vertex: configure GOOGLE_CLOUD_PROJECT")
	}
	opts := &credentials.DetectOptions{
		Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
		Client: &http.Client{Timeout: timeout},
	}
	var cred *auth.Credentials
	var err error
	if prov.ArquivoCredenciais != "" {
		cred, err = credentials.NewCredentialsFromFile(credentials.ServiceAccount, prov.ArquivoCredenciais, opts)
	} else {
		cred, err = credentials.DetectDefault(opts)
	}
	if err != nil {
		return nil, fmt.Errorf("vertex: carregar credenciais Google: %w", err)
	}
	tentativas := int32(3)
	cliente, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:     genai.BackendVertexAI,
		Project:     prov.Projeto,
		Location:    prov.Localizacao,
		Credentials: cred,
		HTTPOptions: genai.HTTPOptions{
			BaseURL:      base,
			APIVersion:   "v1",
			Timeout:      &timeout,
			RetryOptions: &genai.HTTPRetryOptions{Attempts: &tentativas},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("vertex: criar cliente Google: %w", err)
	}
	return gemini.NewChatModel(ctx, &gemini.Config{Client: cliente, Model: nome})
}
