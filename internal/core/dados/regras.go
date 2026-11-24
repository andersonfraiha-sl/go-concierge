package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/core/administracao/regras"
)

const rotaRegrasAtivas = "/interno/dados/v1/regras/ativas"
const prazoRegras = 5 * time.Second

type pedidoRegrasAtivas struct {
	Operacao string `json:"operacao"`
}

type respostaRegrasAtivas struct {
	Operacao  string                `json:"operacao"`
	Resultado string                `json:"resultado"`
	Regras    []regras.RegraGravada `json:"regras"`
}

func registrarRegrasAtivas(mux *http.ServeMux, fonte regras.FonteDoIndice, token string) {
	mux.HandleFunc("POST "+rotaRegrasAtivas, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoRegrasAtivas
		if !lerPedido(w, r, token, 16<<10, &p) {
			return
		}
		if _, err := uuid.Parse(p.Operacao); err != nil {
			http.Error(w, "operação inválida", http.StatusBadRequest)
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), prazoRegras)
		defer cancelar()
		gravadas, err := fonte.Ativas(ctx)
		res := respostaRegrasAtivas{Operacao: p.Operacao, Resultado: "lidas", Regras: gravadas}
		status := http.StatusOK
		if err != nil {
			res.Resultado, res.Regras, status = "indisponivel", nil, http.StatusServiceUnavailable
		} else if res.Regras == nil {
			res.Regras = []regras.RegraGravada{}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func (c *Cliente) Ativas(ctx context.Context) ([]regras.RegraGravada, error) {
	op, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	p := pedidoRegrasAtivas{Operacao: op.String()}
	var res respostaRegrasAtivas
	status, _, err := requisitarJSON(ctx, c.http, c.base+rotaRegrasAtivas, c.token, p, &res, 16<<10, 64<<20, prazoRegras)
	if err != nil || status != http.StatusOK || res.Operacao != p.Operacao || res.Resultado != "lidas" || res.Regras == nil {
		return nil, errors.Join(ErrDadosIndisponiveis, err)
	}
	return res.Regras, nil
}

func (c *Cliente) ObservarAlteracoes(observar func([]regras.RegraGravada)) {
	c.regrasMu.Lock()
	defer c.regrasMu.Unlock()
	c.aoAlterarRegras = observar
}

func (c *Cliente) notificarRegras(gravadas []regras.RegraGravada) {
	c.regrasMu.Lock()
	observar := c.aoAlterarRegras
	c.regrasMu.Unlock()
	if observar != nil {
		observar(gravadas)
	}
}
