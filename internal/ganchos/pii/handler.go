package pii

import (
	"context"
	"fmt"
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
		return mascararQualquer(a.Key, v.Any())
	}
	return slog.Attr{Key: a.Key, Value: v}
}

// mascararQualquer: erro pela mensagem, map[string]any e []any pelo conteúdo, e o
// resto (struct, slice tipado, Stringer) pelo texto renderizado, perdendo a forma.
func mascararQualquer(chave string, v any) slog.Attr {
	switch x := v.(type) {
	case nil:
		return slog.Any(chave, nil)
	case error:
		return slog.String(chave, mascararAnonimo(x.Error()))
	case map[string]any, []any:
		limpo := mapearStrings(x, mascararAnonimo)
		// Valor tipado lá dentro ([]string, struct) não passa por mapearStrings.
		if texto := fmt.Sprint(limpo); len(achar(texto)) > 0 {
			return slog.String(chave, mascararAnonimo(texto))
		}
		return slog.Any(chave, limpo)
	}
	return slog.String(chave, mascararAnonimo(fmt.Sprint(v)))
}
