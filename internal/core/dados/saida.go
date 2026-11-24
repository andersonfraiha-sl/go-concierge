package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"go-concierge/internal/harness/canais"
)

const rotaCanalDeSaida = "/interno/dados/v1/saida/canal"

type CanaisDeSaida struct {
	base, token string
	http        *http.Client
}

type respostaCanalDeSaida struct {
	Canal *canais.Canal `json:"canal,omitempty"`
	Chave string        `json:"chave,omitempty"`
	Erro  string        `json:"erro,omitempty"`
}

func (s *Servidor) CanaisDeSaida() *CanaisDeSaida { return s.saida }

func registrarCanalDeSaida(mux *http.ServeMux, repo canais.Repositorio, token string) {
	mux.HandleFunc("POST "+rotaCanalDeSaida, func(w http.ResponseWriter, r *http.Request) {
		var p escopoCanal
		if !lerPedido(w, r, token, limiteConsultaCanais, &p) {
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoCanais)
		defer cancelar()
		c, err := repo.PorID(ctx, p.CompanyID, p.ChannelID)
		res := respostaCanalDeSaida{}
		status := http.StatusOK
		if errors.Is(err, canais.ErrDesconhecido) {
			res.Erro, status = "desconhecido", http.StatusNotFound
		} else if err != nil {
			res.Erro, status = "indisponivel", http.StatusServiceUnavailable
		} else if c.ID != p.ChannelID || c.CompanyID != p.CompanyID || c.ID == uuid.Nil {
			res.Erro, status = "invalido", http.StatusForbidden
		} else {
			c.WebhookSecret = ""
			res.Canal, res.Chave = &c, c.APIKey
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func (c *CanaisDeSaida) PorID(ctx context.Context, empresa string, id uuid.UUID) (canais.Canal, error) {
	var res respostaCanalDeSaida
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaCanalDeSaida, c.token, escopoCanal{CompanyID: empresa, ChannelID: id}, &res, limiteConsultaCanais, limiteRespostaCanais, prazoCanais)
	if err != nil {
		return canais.Canal{}, errors.Join(ErrDadosIndisponiveis, err)
	}
	if status == http.StatusNotFound && res.Erro == "desconhecido" {
		return canais.Canal{}, canais.ErrDesconhecido
	}
	if status != http.StatusOK || res.Canal == nil || res.Canal.ID != id || res.Canal.CompanyID != empresa {
		return canais.Canal{}, ErrDadosIndisponiveis
	}
	res.Canal.APIKey = res.Chave
	return *res.Canal, nil
}
