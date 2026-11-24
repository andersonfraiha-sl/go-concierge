package dados

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/core/banco"
	auditoriapg "go-concierge/internal/core/dados/postgres/auditoria"
	atende "go-concierge/internal/harness/atendimento"
)

const rotaReterAuditoria = "/interno/dados/v1/manutencao/auditoria/reter"

type OperacoesManutencao interface {
	ReterAuditoria(context.Context) (int, error)
}

type ManutencaoPostgres struct {
	banco banco.Banco
	agora func() time.Time
}

func NovaManutencaoPostgres(b banco.Banco) *ManutencaoPostgres {
	return &ManutencaoPostgres{banco: b, agora: time.Now}
}

func (m *ManutencaoPostgres) ReterAuditoria(ctx context.Context) (int, error) {
	return auditoriapg.Apagar(ctx, m.banco.Ent, m.agora())
}

type Manutencao struct {
	base, token string
	http        *http.Client
}

func (s *Servidor) Manutencao() *Manutencao { return s.manutencao }

type pedidoRetencao struct {
	Operacao string `json:"operacao"`
}
type respostaRetencao struct {
	Operacao  string `json:"operacao"`
	Resultado string `json:"resultado"`
	Apagadas  int    `json:"apagadas"`
}

func registrarManutencao(mux *http.ServeMux, store OperacoesManutencao, token string) {
	mux.HandleFunc("POST "+rotaReterAuditoria, func(w http.ResponseWriter, r *http.Request) {
		var p pedidoRetencao
		if !lerPedido(w, r, token, 16<<10, &p) {
			return
		}
		if _, err := uuid.Parse(p.Operacao); err != nil {
			http.Error(w, "operação inválida", http.StatusBadRequest)
			return
		}
		if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(35 * time.Second)); err != nil {
			http.Error(w, "prazo de manutenção indisponível", http.StatusServiceUnavailable)
			return
		}
		ctx, cancelar := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancelar()
		n, err := store.ReterAuditoria(ctx)
		res := respostaRetencao{Operacao: p.Operacao, Resultado: "concluida", Apagadas: n}
		status := http.StatusOK
		if err != nil {
			res.Resultado, status = "incerta", http.StatusInternalServerError
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	})
}

func (m *Manutencao) ReterAuditoria(ctx context.Context) (int, error) {
	if m == nil {
		return 0, errors.New("API de manutenção indisponível")
	}
	op, err := uuid.NewRandom()
	if err != nil {
		return 0, err
	}
	var res respostaRetencao
	status, enviada, err := requisitarJSON(ctx, m.http, m.base+rotaReterAuditoria, m.token, pedidoRetencao{Operacao: op.String()}, &res, 16<<10, 4096, 30*time.Second)
	if err != nil {
		if enviada {
			return 0, erroIncerto(op, err)
		}
		return 0, errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	if res.Operacao != op.String() || res.Apagadas < 0 || status != http.StatusOK || res.Resultado != "concluida" {
		return res.Apagadas, erroIncerto(op, errors.New("retenção sem confirmação válida"))
	}
	return res.Apagadas, nil
}
