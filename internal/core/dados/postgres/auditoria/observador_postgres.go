package auditoria

import (
	"context"
	"go-concierge/internal/core/banco"
	"time"
)

func NovoAuditor(b banco.Banco, aoFalhar func(origem string), agora func() time.Time) *Auditor {
	return NovoAuditorComGravador(func(ctx context.Context, linhas []Linha) {
		_ = Gravar(ctx, b, linhas, aoFalhar)
	}, agora)
}
