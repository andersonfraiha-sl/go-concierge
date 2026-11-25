package dados

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	atendeapi "go-concierge/internal/core/api/atende"
	"go-concierge/internal/harness/canais"
)

const rotaCanal = "/interno/dados/v1/canais/consultar"
const rotaCasos = "/interno/dados/v1/canais/casos"
const rotaWebhook = "/interno/dados/v1/entrada/verificar-webhook"
const prazoCanais = 3 * time.Second
const limiteConsultaCanais = 16 << 10
const limiteRespostaCanais = 32 << 20
const limiteWebhook = 3 << 20

var ErrDadosIndisponiveis = errors.New("dados indisponíveis")
var ErrWebhookNaoAutorizado = errors.New("webhook não autorizado")

type Entrada struct {
	base  string
	token string
	http  *http.Client
}

type EntradaLocal struct {
	repo canais.Repositorio
}

func NovaEntradaLocal(repo canais.Repositorio) *EntradaLocal {
	return &EntradaLocal{repo: repo}
}

func (e *EntradaLocal) VerificarWebhook(ctx context.Context, id canais.Identidade, corpo []byte, assinatura string) (canais.Canal, error) {
	ctx, cancelar := context.WithTimeout(ctx, prazoCanais)
	defer cancelar()
	canal, err := e.repo.Resolver(ctx, id)
	if err != nil {
		if errors.Is(err, canais.ErrDesconhecido) || errors.Is(err, canais.ErrInvalido) || errors.Is(err, canais.ErrDesabilitado) {
			return canais.Canal{}, ErrWebhookNaoAutorizado
		}
		return canais.Canal{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	if !canal.Habilitado || atendeapi.VerificarAssinatura(canal.WebhookSecret, assinatura, corpo) != nil {
		return canais.Canal{}, ErrWebhookNaoAutorizado
	}
	return semCredenciais(canal), nil
}

type escopoCanal struct {
	CompanyID string    `json:"company_id"`
	ChannelID uuid.UUID `json:"channel_id"`
}

type pedidoWebhook struct {
	Identidade canais.Identidade `json:"identidade"`
	Corpo      []byte            `json:"corpo"`
	Assinatura string            `json:"assinatura"`
}

type respostaCanal struct {
	Canal *canais.Canal `json:"canal,omitempty"`
	Casos []canais.Caso `json:"casos"`
	Erro  string        `json:"erro,omitempty"`
}

func registrarCanais(mux *http.ServeMux, repo canais.Repositorio, token, tokenEntrada string) {
	mux.HandleFunc("POST "+rotaCanal, func(w http.ResponseWriter, r *http.Request) {
		var e escopoCanal
		if !lerPedido(w, r, token, limiteConsultaCanais, &e) {
			return
		}
		if strings.TrimSpace(e.CompanyID) == "" || e.ChannelID == uuid.Nil {
			responderCanal(w, respostaCanal{Erro: "desconhecido"}, http.StatusNotFound)
			return
		}
		canal, err := repo.PorID(r.Context(), e.CompanyID, e.ChannelID)
		if err != nil {
			responderErroCanal(w, err)
			return
		}
		canal = semCredenciais(canal)
		responderCanal(w, respostaCanal{Canal: &canal}, http.StatusOK)
	})
	mux.HandleFunc("POST "+rotaCasos, func(w http.ResponseWriter, r *http.Request) {
		var e escopoCanal
		if !lerPedido(w, r, token, limiteConsultaCanais, &e) {
			return
		}
		if strings.TrimSpace(e.CompanyID) == "" || e.ChannelID == uuid.Nil {
			responderCanal(w, respostaCanal{Erro: "desconhecido"}, http.StatusNotFound)
			return
		}
		canal, err := repo.PorID(r.Context(), e.CompanyID, e.ChannelID)
		if err != nil {
			responderErroCanal(w, err)
			return
		}
		casos, err := repo.Casos(r.Context(), canal)
		if err != nil {
			responderErroCanal(w, err)
			return
		}
		if casos == nil {
			casos = []canais.Caso{}
		}
		responderCanal(w, respostaCanal{Casos: casos}, http.StatusOK)
	})
	mux.HandleFunc("POST "+rotaWebhook, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoWebhook
		if !lerPedido(w, r, tokenEntrada, limiteWebhook, &p) {
			return
		}
		canal, err := NovaEntradaLocal(repo).VerificarWebhook(r.Context(), p.Identidade, p.Corpo, p.Assinatura)
		if err != nil {
			if errors.Is(err, ErrWebhookNaoAutorizado) {
				responderCanal(w, respostaCanal{Erro: "webhook_nao_autorizado"}, http.StatusUnauthorized)
			} else {
				responderErroCanal(w, err)
			}
			return
		}
		responderCanal(w, respostaCanal{Canal: &canal}, http.StatusOK)
	})
}

func lerPedido(w http.ResponseWriter, r *http.Request, token string, limite int64, destino any) bool {
	if !hmac.Equal([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) {
		http.Error(w, "não autorizado", http.StatusUnauthorized)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limite)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if dec.Decode(destino) != nil {
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return false
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return false
	}
	return true
}

func semCredenciais(c canais.Canal) canais.Canal {
	c.APIBaseURL, c.APIKey, c.WebhookSecret = "", "", ""
	return c
}

func responderErroCanal(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, canais.ErrDesconhecido):
		responderCanal(w, respostaCanal{Erro: "desconhecido"}, http.StatusNotFound)
	case errors.Is(err, canais.ErrInvalido):
		responderCanal(w, respostaCanal{Erro: "invalido"}, http.StatusUnprocessableEntity)
	case errors.Is(err, canais.ErrDesabilitado):
		responderCanal(w, respostaCanal{Erro: "desabilitado"}, http.StatusForbidden)
	default:
		responderCanal(w, respostaCanal{Erro: "indisponivel"}, http.StatusServiceUnavailable)
	}
}

func responderCanal(w http.ResponseWriter, resposta respostaCanal, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resposta)
}

func (c *Cliente) PorID(ctx context.Context, company string, id uuid.UUID) (canais.Canal, error) {
	res, err := consultarCanal(ctx, c.http, c.base+rotaCanal, c.token, escopoCanal{CompanyID: company, ChannelID: id}, limiteConsultaCanais)
	if err != nil {
		return canais.Canal{}, err
	}
	if res.Canal == nil || res.Canal.CompanyID != company || res.Canal.ID != id {
		return canais.Canal{}, ErrDadosIndisponiveis
	}
	return semCredenciais(*res.Canal), nil
}

func (c *Cliente) Casos(ctx context.Context, canal canais.Canal) ([]canais.Caso, error) {
	res, err := consultarCanal(ctx, c.http, c.base+rotaCasos, c.token, escopoCanal{CompanyID: canal.CompanyID, ChannelID: canal.ID}, limiteConsultaCanais)
	if err != nil {
		return nil, err
	}
	if res.Casos == nil {
		return nil, ErrDadosIndisponiveis
	}
	return res.Casos, nil
}

func (e *Entrada) VerificarWebhook(ctx context.Context, id canais.Identidade, corpo []byte, assinatura string) (canais.Canal, error) {
	res, err := consultarCanal(ctx, e.http, e.base+rotaWebhook, e.token, pedidoWebhook{Identidade: id, Corpo: corpo, Assinatura: assinatura}, limiteWebhook)
	if err != nil {
		return canais.Canal{}, err
	}
	if res.Canal == nil {
		return canais.Canal{}, ErrDadosIndisponiveis
	}
	return semCredenciais(*res.Canal), nil
}

func consultarCanal(ctx context.Context, cliente *http.Client, url, token string, pedido any, limite int) (respostaCanal, error) {
	var resposta respostaCanal
	dados, err := json.Marshal(pedido)
	if err != nil || len(dados) > limite {
		return resposta, ErrDadosIndisponiveis
	}
	ctx, cancelar := context.WithTimeout(ctx, prazoCanais)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(dados))
	if err != nil {
		return resposta, errors.Join(ErrDadosIndisponiveis, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res, err := cliente.Do(req)
	if err != nil {
		return resposta, errors.Join(ErrDadosIndisponiveis, err)
	}
	defer res.Body.Close()
	corpo, err := io.ReadAll(io.LimitReader(res.Body, limiteRespostaCanais+1))
	if err != nil || len(corpo) > limiteRespostaCanais {
		return resposta, ErrDadosIndisponiveis
	}
	dec := json.NewDecoder(bytes.NewReader(corpo))
	dec.DisallowUnknownFields()
	if dec.Decode(&resposta) != nil {
		return respostaCanal{}, ErrDadosIndisponiveis
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return respostaCanal{}, ErrDadosIndisponiveis
	}
	if res.StatusCode == http.StatusOK && resposta.Erro == "" {
		return resposta, nil
	}
	switch {
	case res.StatusCode == http.StatusNotFound && resposta.Erro == "desconhecido":
		return respostaCanal{}, canais.ErrDesconhecido
	case res.StatusCode == http.StatusUnprocessableEntity && resposta.Erro == "invalido":
		return respostaCanal{}, canais.ErrInvalido
	case res.StatusCode == http.StatusForbidden && resposta.Erro == "desabilitado":
		return respostaCanal{}, canais.ErrDesabilitado
	case res.StatusCode == http.StatusUnauthorized && resposta.Erro == "webhook_nao_autorizado":
		return respostaCanal{}, ErrWebhookNaoAutorizado
	default:
		return respostaCanal{}, fmt.Errorf("%w: HTTP %d", ErrDadosIndisponiveis, res.StatusCode)
	}
}
