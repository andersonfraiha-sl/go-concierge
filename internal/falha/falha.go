// Package falha guarda a pilha de chamadas onde um erro inesperado nasce (a borda de I/O:
// Postgres, HTTP de saída) e a entrega ao log de ERROR, que sai uma vez só, na borda de
// cima (o handler HTTP, o turno, o trabalho de fundo). Erro esperado de negócio (sentinela,
// validação) não passa por aqui: sem pilha, a linha de log fica enxuta.
package falha

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

const (
	maxQuadros       = 32
	maxQuadrosCurtos = 10
)

type comPilha struct {
	err     error
	quadros []uintptr
}

func (e *comPilha) Error() string { return e.err.Error() }
func (e *comPilha) Unwrap() error { return e.err }

// Format: %+v escreve a mensagem e a pilha; os demais verbos, só a mensagem.
func (e *comPilha) Format(f fmt.State, verbo rune) {
	_, _ = io.WriteString(f, e.err.Error())
	if verbo == 'v' && f.Flag('+') {
		_, _ = io.WriteString(f, "\n"+formatar(e.quadros))
	}
}

// ComPilha anota err com a pilha de quem chamou. Só a primeira anotação vale: um erro que já
// traz pilha volta como veio, e nil continua nil.
func ComPilha(err error) error {
	if err == nil || temPilha(err) {
		return err
	}
	quadros := make([]uintptr, maxQuadros)
	n := runtime.Callers(2, quadros)
	return &comPilha{err: err, quadros: quadros[:n]}
}

func temPilha(err error) bool {
	_, ok := errors.AsType[*comPilha](err)
	return ok
}

// Pilha é a pilha que ComPilha capturou em algum ponto da cadeia; vazia se ninguém capturou.
func Pilha(err error) string {
	c, ok := errors.AsType[*comPilha](err)
	if !ok {
		return ""
	}
	return formatar(c.quadros)
}

// Registrar escreve a linha de ERROR de uma falha na borda: a cadeia de erros, a pilha de
// onde nasceu (se capturada) e o chamador. Chamar uma vez por falha, no ponto em que ela
// deixa de subir.
func Registrar(ctx context.Context, mensagem string, err error, atributos ...slog.Attr) {
	atributos = append(atributos, slog.String("erro", err.Error()), slog.String("chamador", Chamador(1)))
	if pilha := Pilha(err); pilha != "" {
		atributos = append(atributos, slog.String("stack", pilha))
	}
	slog.Default().LogAttrs(ctx, slog.LevelError, mensagem, atributos...)
}

// Recuperar, adiado no topo de uma goroutine de fundo, troca o pânico por uma linha de ERROR
// com a pilha; a goroutine termina e o processo segue.
func Recuperar(ctx context.Context, operacao string) {
	p := recover()
	if p == nil {
		return
	}
	slog.Default().LogAttrs(ctx, slog.LevelError, "pânico em segundo plano",
		slog.String("operacao", operacao), slog.String("panico", fmt.Sprint(p)), slog.String("stack", string(debug.Stack())))
}

// Chamador é "arquivo:linha" de quem está pular quadros acima de quem chamou Chamador.
func Chamador(pular int) string {
	_, arquivo, linha, ok := runtime.Caller(pular + 1)
	if !ok {
		return ""
	}
	return arquivo + ":" + strconv.Itoa(linha)
}

// PilhaDoProjeto é a pilha curta (até 10 quadros, só código deste módulo) de quem está pular
// quadros acima de quem chamou: o contexto de um ERROR sem pilha capturada.
func PilhaDoProjeto(pular int) string {
	quadros := make([]uintptr, maxQuadros)
	n := runtime.Callers(pular+2, quadros)
	var b strings.Builder
	frames := runtime.CallersFrames(quadros[:n])
	for contados := 0; contados < maxQuadrosCurtos; {
		f, mais := frames.Next()
		if ehDoProjeto(f.Function) {
			fmt.Fprintf(&b, "%s\n\t%s:%d\n", f.Function, f.File, f.Line)
			contados++
		}
		if !mais {
			break
		}
	}
	return b.String()
}

// ehDoProjeto: pacotes do módulo go-concierge (o main do okf-admin aparece como "main."),
// menos este, que só registra.
func ehDoProjeto(funcao string) bool {
	if strings.HasPrefix(funcao, "go-concierge/internal/falha.") {
		return false
	}
	return strings.HasPrefix(funcao, "go-concierge/") || strings.HasPrefix(funcao, "main.")
}

func formatar(quadros []uintptr) string {
	var b strings.Builder
	frames := runtime.CallersFrames(quadros)
	for {
		f, mais := frames.Next()
		fmt.Fprintf(&b, "%s\n\t%s:%d\n", f.Function, f.File, f.Line)
		if !mais {
			return b.String()
		}
	}
}

// ComFonte acrescenta aos registros de ERROR o arquivo:linha de quem logou (fonte) e, quando
// o registro não traz stack nem chamador, a pilha curta do projeto (pilha). INFO e WARN
// passam enxutos.
func ComFonte(h slog.Handler) slog.Handler { return comFonte{h} }

type comFonte struct{ slog.Handler }

func (h comFonte) Handle(ctx context.Context, r slog.Record) error {
	if r.Level < slog.LevelError {
		return h.Handler.Handle(ctx, r)
	}
	r = r.Clone()
	if r.PC != 0 {
		f, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		r.AddAttrs(slog.String("fonte", f.File+":"+strconv.Itoa(f.Line)))
	}
	if !temLocalizacao(r) {
		r.AddAttrs(slog.String("pilha", PilhaDoProjeto(0)))
	}
	return h.Handler.Handle(ctx, r)
}

func (h comFonte) WithAttrs(atributos []slog.Attr) slog.Handler {
	return comFonte{h.Handler.WithAttrs(atributos)}
}

func (h comFonte) WithGroup(nome string) slog.Handler { return comFonte{h.Handler.WithGroup(nome)} }

func temLocalizacao(r slog.Record) bool {
	tem := false
	r.Attrs(func(a slog.Attr) bool {
		tem = a.Key == "stack" || a.Key == "chamador"
		return !tem
	})
	return tem
}
