package concierge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/core/api/execucao"
	"go-concierge/internal/harness/canais"
)

type Cliente struct {
	BaseURL    string
	Token      string
	UsuarioDev string
	HTTP       *http.Client
}

func (c Cliente) Configuracao(ctx context.Context, empresa string, canal uuid.UUID) (execucao.Configuracao, error) {
	if strings.TrimSpace(empresa) == "" || canal == uuid.Nil {
		return execucao.Configuracao{}, errors.New("configuração exige empresa e canal")
	}
	base, err := url.Parse(strings.TrimRight(c.BaseURL, "/"))
	if err != nil || base.Scheme != "http" && base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return execucao.Configuracao{}, errors.New("URL pública do Concierge inválida")
	}
	if (c.Token != "" || c.UsuarioDev != "") && base.Host != "localhost:8090" && base.Host != "127.0.0.1:8090" && base.Host != "[::1]:8090" {
		return execucao.Configuracao{}, errors.New("credenciais do CLI restritas ao Concierge local na porta 8090")
	}
	base.Path = strings.TrimRight(base.Path, "/") + execucao.RotaConfiguracao
	q := base.Query()
	q.Set("company_id", empresa)
	q.Set("channel_id", canal.String())
	base.RawQuery = q.Encode()
	ctx, cancelar := context.WithTimeout(ctx, 30*time.Second)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return execucao.Configuracao{}, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if c.UsuarioDev != "" {
		req.Header.Set("X-Dev-Usuario", c.UsuarioDev)
	}
	httpCliente := c.HTTP
	if httpCliente == nil {
		httpCliente = http.DefaultClient
	}
	seguro := *httpCliente
	seguro.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := seguro.Do(req)
	if err != nil {
		return execucao.Configuracao{}, fmt.Errorf("consultar configuração pública: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return execucao.Configuracao{}, canais.ErrDesconhecido
	}
	if res.StatusCode == http.StatusConflict {
		return execucao.Configuracao{}, canais.ErrDesabilitado
	}
	if res.StatusCode != http.StatusOK {
		return execucao.Configuracao{}, fmt.Errorf("consultar configuração pública: HTTP %d", res.StatusCode)
	}
	corpo, err := io.ReadAll(io.LimitReader(res.Body, (64<<20)+1))
	if err != nil || len(corpo) > 64<<20 {
		return execucao.Configuracao{}, errors.New("resposta pública inválida ou acima do limite")
	}
	var config execucao.Configuracao
	dec := json.NewDecoder(bytes.NewReader(corpo))
	dec.DisallowUnknownFields()
	if dec.Decode(&config) != nil || dec.Decode(new(any)) != io.EOF || config.Canal.ID != canal || config.Canal.CompanyID != empresa || config.Casos == nil || config.Arquivos == nil {
		return execucao.Configuracao{}, errors.New("configuração pública incompatível com o canal solicitado")
	}
	if !config.Canal.Habilitado {
		return execucao.Configuracao{}, canais.ErrDesabilitado
	}
	return config, nil
}
