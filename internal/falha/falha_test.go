package falha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"testing"
)

var errSentinela = errors.New("sentinela")

func lerDoBanco() error { return ComPilha(fmt.Errorf("ler do banco: %w", fs.ErrClosed)) }

func TestComPilhaCapturaUmaVezEPreservaACadeia(t *testing.T) {
	err := lerDoBanco()
	acima := ComPilha(fmt.Errorf("serviço: %w", err))

	if !errors.Is(acima, fs.ErrClosed) {
		t.Fatal("errors.Is perdeu a causa")
	}
	if _, ok := errors.AsType[*comPilha](acima); !ok {
		t.Fatal("errors.As não acha a pilha")
	}
	if Pilha(acima) != Pilha(err) {
		t.Fatal("a segunda anotação trocou a pilha da primeira")
	}
	if !strings.Contains(Pilha(acima), "falha.lerDoBanco") {
		t.Fatalf("a pilha não começa onde o erro nasceu:\n%s", Pilha(acima))
	}
	if acima.Error() != "serviço: ler do banco: file already closed" {
		t.Fatalf("Error() = %q", acima.Error())
	}
	if completo := fmt.Sprintf("%+v", err); !strings.Contains(completo, "falha.lerDoBanco") {
		t.Fatalf("%%+v sem pilha: %s", completo)
	}
	if curto := fmt.Sprintf("%v", err); strings.Contains(curto, "\n") {
		t.Fatalf("%%v com pilha: %s", curto)
	}
	if ComPilha(nil) != nil {
		t.Fatal("ComPilha(nil) deve ser nil")
	}
	if Pilha(errSentinela) != "" {
		t.Fatal("sentinela sem ComPilha não tem pilha")
	}
}

func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(ComFonte(slog.NewJSONHandler(&buf, nil))))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	return &buf
}

func linhaUnica(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	linhas := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(linhas) != 1 {
		t.Fatalf("quero uma linha, veio:\n%s", buf.String())
	}
	var linha map[string]any
	if err := json.Unmarshal([]byte(linhas[0]), &linha); err != nil {
		t.Fatal(err)
	}
	return linha
}

func TestRecuperarRegistraPanicoComPilhaESegue(t *testing.T) {
	buf := capturarLog(t)
	feito := make(chan struct{})
	go func() {
		defer close(feito)
		defer Recuperar(context.Background(), "teste")
		trabalhoQueEntraEmPanico()
	}()
	<-feito

	linha := linhaUnica(t, buf)
	if linha["level"] != "ERROR" || linha["operacao"] != "teste" || linha["panico"] != "quebrou" {
		t.Fatalf("linha: %v", linha)
	}
	if pilha, _ := linha["stack"].(string); !strings.Contains(pilha, "trabalhoQueEntraEmPanico") {
		t.Fatalf("stack sem a função que entrou em pânico: %v", linha["stack"])
	}
}

func trabalhoQueEntraEmPanico() { panic("quebrou") }

func TestComFonteSoNoError(t *testing.T) {
	buf := capturarLog(t)
	slog.Info("enxuto")
	var info map[string]any
	if err := json.Unmarshal(buf.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if _, ok := info["fonte"]; ok {
		t.Fatalf("INFO ganhou fonte: %v", info)
	}
	buf.Reset()
	slog.Error("quebrou")
	linha := linhaUnica(t, buf)
	if fonte, _ := linha["fonte"].(string); !strings.Contains(fonte, "falha_test.go:") {
		t.Fatalf("fonte = %v", linha["fonte"])
	}
	if _, ok := linha["pilha"]; !ok {
		t.Fatalf("ERROR sem stack ganha a pilha curta: %v", linha)
	}
}

func TestRegistrarSentinelaSemPilha(t *testing.T) {
	buf := capturarLog(t)
	Registrar(context.Background(), "falhou", fmt.Errorf("turno: %w", errSentinela))
	linha := linhaUnica(t, buf)
	if _, ok := linha["stack"]; ok {
		t.Fatalf("sentinela não leva stack: %v", linha)
	}
	if chamador, _ := linha["chamador"].(string); !strings.Contains(chamador, "falha_test.go:") {
		t.Fatalf("chamador = %v", linha["chamador"])
	}
	if linha["erro"] != "turno: sentinela" {
		t.Fatalf("erro = %v", linha["erro"])
	}
}
