package pii

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

type pessoa struct{ CPF string }

type morador struct{ Pessoa *pessoa }

func slogJSON(w io.Writer) *slog.Logger { return slog.New(slog.NewJSONHandler(w, nil)) }

func TestHandlerMascara(t *testing.T) {
	tests := []struct {
		name string
		log  func(*slog.Logger)
		want string
	}{
		{"atributo", func(l *slog.Logger) { l.Info("ok", "texto", "cpf 123.456.789-09") }, `"texto":"cpf «cpf»"`},
		{"telefone", func(l *slog.Logger) { l.Info("ok", "motivo", "ligar +55 (19) 94040-2710") }, `"motivo":"ligar «tel»"`},
		{"grupo", func(l *slog.Logger) {
			l.Info("ok", slog.Group("morador", slog.String("doc", "12345678909")))
		}, `"morador":{"doc":"«cpf»"}`},
		{"grupo do logger", func(l *slog.Logger) { l.WithGroup("g").Info("ok", "doc", "12345678909") }, `"g":{"doc":"«cpf»"}`},
		{"With", func(l *slog.Logger) { l.With("doc", "12345678909").Info("ok") }, `"doc":"«cpf»"`},
		{"erro", func(l *slog.Logger) { l.Error("ok", "erro", errors.New("cpf 12345678909 inválido")) }, `"erro":"cpf «cpf» inválido"`},
		{"mensagem", func(l *slog.Logger) { l.Info("cpf 12345678909") }, `"msg":"cpf «cpf»"`},
		{"map", func(l *slog.Logger) {
			l.Info("ok", "params", map[string]any{"cpf": "12345678909", "unidade": 1203})
		}, `"params":{"cpf":"«cpf»","unidade":1203}`},
		{"slice", func(l *slog.Logger) { l.Info("ok", "lista", []any{"19 98162-1916", "apto 12"}) }, `"lista":["«tel»","apto 12"]`},
		{"slice tipado", func(l *slog.Logger) { l.Info("ok", "lista", []string{"19 98162-1916"}) }, `"lista":"[\"«tel»\"]"`},
		{"struct", func(l *slog.Logger) { l.Info("ok", "st", struct{ Doc string }{"12345678909"}) }, `"st":"{\"Doc\":\"«cpf»\"}"`},
		{"map com valor tipado", func(l *slog.Logger) {
			l.Info("ok", "params", map[string]any{"tels": []string{"19 98162-1916"}})
		}, `"params":{"tels":"[\"«tel»\"]"}`},
		{"ponteiro", func(l *slog.Logger) { l.Info("ok", "p", &pessoa{CPF: "12345678909"}) }, `"p":"{\"CPF\":\"«cpf»\"}"`},
		{"ponteiro dentro de map", func(l *slog.Logger) {
			l.Info("ok", "params", map[string]any{"p": &pessoa{CPF: "12345678909"}})
		}, `"params":{"p":"{\"CPF\":\"«cpf»\"}"}`},
		{"ponteiro dentro de slice", func(l *slog.Logger) { l.Info("ok", "lista", []any{&pessoa{CPF: "12345678909"}}) }, `"lista":["{\"CPF\":\"«cpf»\"}"]`},
		{"ponteiro aninhado em struct", func(l *slog.Logger) {
			l.Info("ok", "params", map[string]any{"m": morador{Pessoa: &pessoa{CPF: "12345678909"}}})
		}, `«cpf»`},
		{"ponteiro nil", func(l *slog.Logger) { l.Info("ok", "params", map[string]any{"p": (*pessoa)(nil)}) }, `"params":{"p":null}`},
		{"bytes", func(l *slog.Logger) { l.Info("ok", "corpo", []byte(`{"cpf":"12345678909"}`)) }, `"corpo":"{\"cpf\":\"«cpf»\"}"`},
		{"bytes dentro de map", func(l *slog.Logger) { l.Info("ok", "params", map[string]any{"b": []byte("tel 19 98162-1916")}) }, `"params":{"b":"tel «tel»"}`},
		{"número que é cpf", func(l *slog.Logger) { l.Info("ok", "doc", int64(12345678909)) }, `"doc":"«cpf»"`},
		{"número comum fica", func(l *slog.Logger) { l.Info("ok", "unidade", map[string]any{"n": 1203}) }, `"unidade":{"n":1203}`},
		{"nil", func(l *slog.Logger) { l.Info("ok", "nada", nil) }, `"nada":null`},
		{"número que não é cpf fica", func(l *slog.Logger) { l.Info("ok", "protocolo", "20261103000123") }, `"protocolo":"20261103000123"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var saida strings.Builder
			tt.log(slog.New(NovoHandler(slog.NewJSONHandler(&saida, nil))))
			if !strings.Contains(saida.String(), tt.want) {
				t.Errorf("log = %s, want conter %s", saida.String(), tt.want)
			}
			if s := saida.String(); strings.Contains(s, "789-09") || strings.Contains(s, "2710") || strings.Contains(s, "12345678909") || strings.Contains(s, "1916") {
				t.Errorf("log vazou PII: %s", saida.String())
			}
		})
	}
}

func TestHandlerRespeitaNivel(t *testing.T) {
	var saida strings.Builder
	l := slog.New(NovoHandler(slog.NewJSONHandler(&saida, &slog.HandlerOptions{Level: slog.LevelWarn})))
	l.Info("ignorado")
	if saida.Len() != 0 {
		t.Errorf("log abaixo do nível saiu: %s", saida.String())
	}
}
