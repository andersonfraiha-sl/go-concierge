package pii

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

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
		{"número que não é cpf fica", func(l *slog.Logger) { l.Info("ok", "protocolo", "20261103000123") }, `"protocolo":"20261103000123"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var saida strings.Builder
			tt.log(slog.New(NovoHandler(slog.NewJSONHandler(&saida, nil))))
			if !strings.Contains(saida.String(), tt.want) {
				t.Errorf("log = %s, want conter %s", saida.String(), tt.want)
			}
			if strings.Contains(saida.String(), "789-09") || strings.Contains(saida.String(), "2710") {
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
