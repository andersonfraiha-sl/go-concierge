package auditoria

import (
	"context"
	"slices"
	"sync"
	"time"

	"go-concierge/internal/banco"
	"go-concierge/internal/ganchos"
)

// prazoGravacao limita a gravação feita na hora, fora do lote pós resposta.
const prazoGravacao = 2 * time.Second

// Auditor é o observador do Conjunto que vira auditoria. Cada observação vai a um destino
// só: o Coletor do ctx, quando o turno tem um (o lote pós resposta grava), ou o banco na
// hora (encerramento, transbordo pedido pelo atendente, qualquer avaliação fora do turno).
type Auditor struct {
	banco    banco.Banco
	aoFalhar func(origem string)
	agora    func() time.Time
}

// NovoAuditor grava em b; aoFalhar conta cada linha perdida (pode ser nil).
func NovoAuditor(b banco.Banco, aoFalhar func(origem string), agora func() time.Time) *Auditor {
	return &Auditor{banco: b, aoFalhar: aoFalhar, agora: agora}
}

// Observador é a opção que liga o auditor ao Conjunto.
func (a *Auditor) Observador() ganchos.Opcao { return ganchos.ComObservador(a.observar) }

// Gravar grava as linhas agora, com o fallback linha a linha de Gravar.
func (a *Auditor) Gravar(ctx context.Context, linhas []Linha) error {
	return Gravar(ctx, a.banco, linhas, a.aoFalhar)
}

func (a *Auditor) observar(ctx context.Context, obs ganchos.Observacao) {
	linhas := LinhasDe(obs.Registros, a.base(ctx, obs))
	if len(linhas) == 0 {
		return
	}
	if c, ok := ctx.Value(chaveColetor{}).(*Coletor); ok && c.guardar(linhas) {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), prazoGravacao)
	defer cancel()
	// Gravar já loga em ERROR e conta em aoFalhar cada linha perdida
	_ = a.Gravar(ctx, linhas)
}

// base é a do ctx, com a hora da decisão e o alvo que a checagem resolveu.
func (a *Auditor) base(ctx context.Context, obs ganchos.Observacao) Linha {
	base, _ := ctx.Value(chaveBase{}).(Linha)
	base.CriadoEm = a.agora()
	if base.Origem == "" {
		base.Origem = ganchos.OrigemConcierge
	}
	if obs.AdministradoraID != "" {
		base.AdministradoraID = obs.AdministradoraID
	}
	if obs.CondominioID != "" {
		base.CondominioID = obs.CondominioID
	}
	return base
}

type (
	chaveBase    struct{}
	chaveColetor struct{}
)

// ComBase põe no ctx os campos que a observação não traz: origem, sessão, protocolo e
// usuário. Alvo e hora vêm da observação.
func ComBase(ctx context.Context, base Linha) context.Context {
	return context.WithValue(ctx, chaveBase{}, base)
}

// Coletor guarda as linhas de um turno para o lote pós resposta. Depois de Fechar, o que
// chegar (um gancho atrasado) vai ao banco na hora: nada se perde nem se grava duas vezes.
type Coletor struct {
	mu      sync.Mutex
	linhas  []Linha
	fechado bool
}

// ComColetor abre o coletor do turno no ctx.
func ComColetor(ctx context.Context) (context.Context, *Coletor) {
	c := &Coletor{}
	return context.WithValue(ctx, chaveColetor{}, c), c
}

// Fechar devolve o que o turno coletou e passa a recusar linhas. Coletor nil devolve nil.
func (c *Coletor) Fechar() []Linha {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fechado = true
	return slices.Clone(c.linhas)
}

func (c *Coletor) guardar(linhas []Linha) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fechado {
		return false
	}
	c.linhas = append(c.linhas, linhas...)
	return true
}
