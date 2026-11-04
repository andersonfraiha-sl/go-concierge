package auditoria

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/ganchosauditoria"
)

const (
	Retencao        = 365 * 24 * time.Hour // 12 meses
	intervaloReter  = 24 * time.Hour
	timeoutDaRodada = 30 * time.Second
)

// Apagar remove as linhas com mais de Retencao e devolve quantas saíram.
func Apagar(ctx context.Context, cli *ent.Client, agora time.Time) (int, error) {
	n, err := cli.GanchosAuditoria.Delete().
		Where(ganchosauditoria.CriadoEmLT(agora.Add(-Retencao))).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("apagar auditoria vencida: %w", err)
	}
	return n, nil
}

// Reter apaga o vencido agora e a cada 24 h, até ctx acabar. Falha de uma rodada vira ERROR e
// a próxima tenta de novo.
func Reter(ctx context.Context, cli *ent.Client, relogio func() time.Time) {
	rodar(ctx, cli, relogio)
	tique := time.NewTicker(intervaloReter)
	defer tique.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tique.C:
			rodar(ctx, cli, relogio)
		}
	}
}

func rodar(ctx context.Context, cli *ent.Client, relogio func() time.Time) {
	ctx, cancel := context.WithTimeout(ctx, timeoutDaRodada)
	defer cancel()
	n, err := Apagar(ctx, cli, relogio())
	if err != nil {
		slog.ErrorContext(ctx, "retenção da auditoria falhou", "operacao", "reter_auditoria", "erro", err)
		return
	}
	slog.InfoContext(ctx, "auditoria vencida apagada", "operacao", "reter_auditoria", "apagadas", n)
}
