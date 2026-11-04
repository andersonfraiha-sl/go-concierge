package ganchos

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const gancoDeRegra = "regra"

// ComObservabilidade registra cada avaliação em log, métrica e evento do span do ctx.
// Só leva nomes, decisões e durações: nunca texto, params, resultado nem PII.
func ComObservabilidade(log *slog.Logger, m metric.Meter) (Opcao, error) {
	decisoes, err := m.Int64Counter("ganchos.decisoes")
	if err != nil {
		return nil, fmt.Errorf("contador ganchos.decisoes: %w", err)
	}
	falhas, err := m.Int64Counter("ganchos.falhas")
	if err != nil {
		return nil, fmt.Errorf("contador ganchos.falhas: %w", err)
	}
	duracao, err := m.Float64Histogram("ganchos.duracao", metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(0.1, 0.25, 0.5, 1, 2, 5, 10, 25))
	if err != nil {
		return nil, fmt.Errorf("histograma ganchos.duracao: %w", err)
	}
	if log == nil {
		log = slog.Default()
	}
	o := observador{log: log, decisoes: decisoes, falhas: falhas, duracao: duracao}
	return ComObservador(o.observar), nil
}

type observador struct {
	log              *slog.Logger
	decisoes, falhas metric.Int64Counter
	duracao          metric.Float64Histogram
}

func (o observador) observar(ctx context.Context, obs Observacao) {
	evento := attribute.String("evento", string(obs.Evento))
	o.duracao.Record(ctx, milissegundos(obs.Duracao), metric.WithAttributes(evento))
	span := trace.SpanFromContext(ctx)
	for _, r := range obs.Registros {
		attrs := metric.WithAttributes(evento, attribute.String("gancho", nomeMetrica(r)),
			attribute.String("decisao", string(r.Decisao.Tipo)))
		o.decisoes.Add(ctx, 1, attrs)
		if r.Falhou {
			o.falhas.Add(ctx, 1, metric.WithAttributes(evento, attribute.String("gancho", nomeMetrica(r)),
				attribute.String("causa", string(r.Causa))))
		}
		if decidiuAlgo(r) {
			o.registrar(ctx, obs, r)
			span.AddEvent("gancho", trace.WithAttributes(evento,
				attribute.String("gancho", nomeMetrica(r)),
				attribute.String("decisao", string(r.Decisao.Tipo)),
				attribute.Bool("falhou", r.Falhou)))
		}
	}
}

func (o observador) registrar(ctx context.Context, obs Observacao, r Registro) {
	origem := r.Decisao.Origem
	args := []any{
		"operacao", string(obs.Evento), "gancho", nomeMetrica(r),
		"regra_id", origem.RegraID, "regra_versao", origem.RegraVersao,
		"decisao", string(r.Decisao.Tipo), "duracao_ms", milissegundos(r.Duracao),
		"administradora_id", obs.AdministradoraID, "condominio_id", obs.CondominioID,
	}
	if !r.Falhou {
		o.log.InfoContext(ctx, "gancho decidiu", args...)
		return
	}
	args = append(args, "causa", string(r.Causa))
	nivel := slog.LevelError
	if !origem.Fixo {
		nivel = slog.LevelWarn
	}
	o.log.Log(ctx, nivel, "gancho falhou", args...)
}

// nomeMetrica serve a métrica, log e span: regra entra como "regra", nunca pelo id nem
// pelo nome, que o usuário digitou.
func nomeMetrica(r Registro) string {
	if r.Decisao.Origem.RegraID != "" {
		return gancoDeRegra
	}
	return r.Decisao.Origem.Gancho
}

func milissegundos(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// Valores aceitos nos rótulos de Contadores; qualquer outro vira RotuloOutro.
const (
	RecargaOk      = "ok"
	RecargaParcial = "parcial"
	RecargaErro    = "erro"

	OrigemConcierge = "concierge"
	OrigemCopiloto  = "copiloto"

	RotuloOutro = "outro"
)

func rotulo(v string, aceitos ...string) string {
	if slices.Contains(aceitos, v) {
		return v
	}
	return RotuloOutro
}

// Contadores cria as métricas de cache, auditoria e regras ativas. Devolve funções simples
// para o cache (Task 8) e a auditoria (Task 9) usarem sem importar OTel.
func Contadores(m metric.Meter, indice func() *Indice) (recarga func(resultado string), auditoriaFalhou func(origem string), err error) {
	recargas, err1 := m.Int64Counter("ganchos.cache.recargas")
	falhasDeAuditoria, err2 := m.Int64Counter("ganchos.auditoria.falhas")
	_, err3 := m.Int64ObservableGauge("ganchos.regras.ativas", metric.WithInt64Callback(
		func(ctx context.Context, o metric.Int64Observer) error {
			return observarAtivas(o, indice)
		}))
	if err := errors.Join(err1, err2, err3); err != nil {
		return nil, nil, fmt.Errorf("criar contadores dos ganchos: %w", err)
	}
	recarga = func(resultado string) {
		recargas.Add(context.Background(), 1, metric.WithAttributes(attribute.String("resultado", rotulo(resultado, RecargaOk, RecargaParcial, RecargaErro))))
	}
	auditoriaFalhou = func(origem string) {
		falhasDeAuditoria.Add(context.Background(), 1, metric.WithAttributes(attribute.String("origem", rotulo(origem, OrigemConcierge, OrigemCopiloto))))
	}
	return recarga, auditoriaFalhou, nil
}

func observarAtivas(o metric.Int64Observer, indice func() *Indice) error {
	if indice == nil {
		return nil
	}
	i := indice()
	if i == nil {
		return nil
	}
	for evento, n := range i.Ativas {
		o.Observe(int64(n), metric.WithAttributes(attribute.String("evento", string(evento))))
	}
	return nil
}

// decidiuAlgo deixa de fora Deixar e o Ajustar sem efeito dos fixos de contexto, que
// ajustam toda mensagem: logar isso seria ruído, como na auditoria.
func decidiuAlgo(r Registro) bool {
	if r.Falhou {
		return true
	}
	return r.Decisao.Tipo != Deixar && !(r.Decisao.Tipo == Ajustar && r.Decisao.Quantidade == 0)
}
