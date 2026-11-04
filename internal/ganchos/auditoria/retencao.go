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

const lotePadrao = 10_000

// Apagar remove as linhas com mais de Retencao, em lotes de 10 mil ids por comando, até não
// sobrar nada ou o ctx acabar. Devolve quantas saíram (também quando para no meio).
func Apagar(ctx context.Context, cli *ent.Client, agora time.Time) (int, error) {
	return apagarEmLotes(ctx, cli, agora, lotePadrao)
}

func apagarEmLotes(ctx context.Context, cli *ent.Client, agora time.Time, tamanho int) (int, error) {
	corte := ganchosauditoria.CriadoEmLT(agora.Add(-Retencao))
	total := 0
	for ctx.Err() == nil {
		ids, err := cli.GanchosAuditoria.Query().Where(corte).Limit(tamanho).IDs(ctx)
		if err != nil {
			return total, fmt.Errorf("listar auditoria vencida: %w", err)
		}
		if len(ids) == 0 {
			return total, nil
		}
		n, err := cli.GanchosAuditoria.Delete().Where(ganchosauditoria.IDIn(ids...)).Exec(ctx)
		total += n
		if err != nil {
			return total, fmt.Errorf("apagar auditoria vencida: %w", err)
		}
	}
	return total, fmt.Errorf("apagar auditoria vencida: %w", ctx.Err())
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
	if ctx.Err() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutDaRodada)
	defer cancel()
	n, err := Apagar(ctx, cli, relogio())
	if err != nil {
		slog.ErrorContext(ctx, "retenção da auditoria falhou", "operacao", "reter_auditoria", "erro", err)
		return
	}
	slog.InfoContext(ctx, "auditoria vencida apagada", "operacao", "reter_auditoria", "apagadas", n)
}
