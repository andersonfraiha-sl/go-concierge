package pii

import (
	"context"
	"log/slog"
)

// handler embrulha outro slog.Handler e troca CPF e telefone por «cpf» e «tel» na
// mensagem e em todo atributo de texto ou erro, inclusive dentro de grupo.
type handler struct{ proximo slog.Handler }

func NovoHandler(h slog.Handler) slog.Handler { return handler{proximo: h} }

func (h handler) Enabled(ctx context.Context, nivel slog.Level) bool {
	return h.proximo.Enabled(ctx, nivel)
}

func (h handler) Handle(ctx context.Context, r slog.Record) error {
	limpo := slog.NewRecord(r.Time, r.Level, mascararAnonimo(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		limpo.AddAttrs(mascararAttr(a))
		return true
	})
	return h.proximo.Handle(ctx, limpo)
}

func (h handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	limpos := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		limpos[i] = mascararAttr(a)
	}
	return handler{proximo: h.proximo.WithAttrs(limpos)}
}

func (h handler) WithGroup(nome string) slog.Handler {
	return handler{proximo: h.proximo.WithGroup(nome)}
}

func mascararAttr(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, mascararAnonimo(v.String()))
	case slog.KindGroup:
		grupo := v.Group()
		limpos := make([]slog.Attr, len(grupo))
		for i, g := range grupo {
			limpos[i] = mascararAttr(g)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(limpos...)}
	case slog.KindAny:
		if err, ok := v.Any().(error); ok {
			return slog.String(a.Key, mascararAnonimo(err.Error()))
		}
	}
	return slog.Attr{Key: a.Key, Value: v}
}
