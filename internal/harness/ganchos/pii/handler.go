package pii

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
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
	// Número também pode ser CPF ou telefone (int64(12345678909)).
	if texto := v.String(); len(achar(texto)) > 0 {
		return slog.String(a.Key, mascararAnonimo(texto))
	}
	return slog.Attr{Key: a.Key, Value: v}
}

// mascararQualquer limpa um atributo KindAny com limparParaLog.
func mascararQualquer(chave string, v any) slog.Attr {
	return slog.Any(chave, limparParaLog(v))
}

// limparParaLog devolve v sem CPF nem telefone:
//   - texto, []byte e erro pela máscara do texto;
//   - map[string]any e []any item a item, mantendo a forma;
//   - ponteiro seguido (nil vira nil), Stringer pelo String();
//   - número e bool como estão, a menos que o texto deles seja PII;
//   - o resto (struct, slice ou map tipado) pelo JSON que o sink escreveria, que
//     segue ponteiros aninhados. Perde a forma, mas nada passa cru.
func limparParaLog(v any) any {
	return limparComLimite(v, 0)
}

// profundidadeMaxima corta estrutura cíclica (map ou slice que contém a si mesmo),
// que sem limite estouraria a pilha e derrubaria o processo.
const profundidadeMaxima = 32

func limparComLimite(v any, profundidade int) any {
	if profundidade > profundidadeMaxima {
		return "[profundo demais]"
	}
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil
	}
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return mascararAnonimo(x)
	case []byte:
		return mascararAnonimo(string(x))
	case error:
		return mascararAnonimo(x.Error())
	case fmt.Stringer:
		return mascararAnonimo(x.String())
	case map[string]any:
		limpo := make(map[string]any, len(x))
		for k, item := range x {
			limpo[k] = limparComLimite(item, profundidade+1)
		}
		return limpo
	case []any:
		limpo := make([]any, len(x))
		for i, item := range x {
			limpo[i] = limparComLimite(item, profundidade+1)
		}
		return limpo
	}
	rv := reflect.ValueOf(v)
	switch {
	case rv.Kind() == reflect.Pointer:
		return limparComLimite(rv.Elem().Interface(), profundidade+1)
	case ehEscalar(rv.Kind()):
		if texto := fmt.Sprint(v); len(achar(texto)) > 0 {
			return mascararAnonimo(texto)
		}
		return v
	}
	return mascararAnonimo(renderizar(v))
}

func ehEscalar(k reflect.Kind) bool {
	return k >= reflect.Bool && k <= reflect.Complex128
}

// renderizar escreve v como o JSONHandler escreveria; sem JSON, como o fmt.
func renderizar(v any) string {
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return fmt.Sprintf("%+v", v)
}
