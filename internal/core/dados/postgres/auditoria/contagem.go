package auditoria

import (
	"context"
	"fmt"
	"go-concierge/internal/core/banco/ent"
	"go-concierge/internal/core/banco/ent/ganchosauditoria"
	"time"
)

// timeoutContagem é o teto de query de requisição HTTP.
const timeoutContagem = 3 * time.Second

// DisparosPorRegra conta, numa consulta só, as decisões de cada regra de ids desde a data.
// Falha da regra não é disparo. Regra sem disparo fica fora do mapa.
func DisparosPorRegra(ctx context.Context, cli *ent.Client, desde time.Time, ids []string) (map[string]int, error) {
	if len(ids) == 0 {
		return map[string]int{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutContagem)
	defer cancel()
	var linhas []struct {
		RegraID string `json:"rule_id"`
		Count   int    `json:"count"`
	}
	err := cli.GanchosAuditoria.Query().
		Where(
			ganchosauditoria.RegraIDIn(ids...),
			ganchosauditoria.CriadoEmGTE(desde),
			ganchosauditoria.DecisaoNEQ(valorNoBanco(decisaoFalhou, decisoesDoBanco)),
		).
		GroupBy(ganchosauditoria.FieldRegraID).
		Aggregate(ent.Count()).
		Scan(ctx, &linhas)
	if err != nil {
		return nil, fmt.Errorf("contar disparos por regra: %w", err)
	}
	disparos := make(map[string]int, len(linhas))
	for _, l := range linhas {
		disparos[l.RegraID] = l.Count
	}
	return disparos, nil
}
