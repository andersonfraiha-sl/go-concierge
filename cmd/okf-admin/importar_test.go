package main

import (
	"bytes"
	"context"
	"go-concierge/internal/escopo"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/importacao"
	"go-concierge/internal/memoria"
	"go-concierge/internal/pedidos"
)

func destinosEmMemoria() importacao.Destinos {
	return importacao.Destinos{Mapeamento: admin.NovaEmMemoria(), Memorias: memoria.NovoFake(), Pedidos: pedidos.NovoFake()}
}

func TestImprimirRelatorioUmaLinhaPorArea(t *testing.T) {
	var b strings.Builder
	imprimirRelatorio(&b, importacao.Relatorio{
		importacao.AreaMapeamento:              {Lidos: 6, Gravados: 6},
		importacao.AreaMemorias:                {Lidos: 2, Gravados: 1},
		importacao.AvisoProtocolosRedesenhados: {Lidos: 3, Gravados: 3},
	})
	quer := "mapeamento: lidos=6 gravados=6\nmemorias: lidos=2 gravados=1\npedidos: lidos=0 gravados=0\n"
	if !strings.HasPrefix(b.String(), quer) || !strings.Contains(b.String(), "aviso: 3 protocolo(s)") {
		t.Errorf("saída = %q", b.String())
	}
}

func TestExecutarImportacaoCodigoDeSaida(t *testing.T) {
	testdata := filepath.Join("..", "..", "internal", "importacao", "testdata")
	tests := []struct {
		nome        string
		dir         string
		destinos    func() importacao.Destinos
		querCodigo  int
		querNaSaida string
		querNoErro  string
	}{
		{"tudo gravado", testdata, destinosEmMemoria, 0, "pedidos: lidos=2 gravados=2", ""},
		{"pasta inexistente é pasta vazia", filepath.Join(t.TempDir(), "nada"), destinosEmMemoria, 0, "mapeamento: lidos=0 gravados=0", ""},
		{"store que não guarda", testdata, func() importacao.Destinos {
			d := destinosEmMemoria()
			d.Pedidos = pedidosQueNaoGuarda{d.Pedidos}
			return d
		}, 1, "pedidos: lidos=2 gravados=0", "gravou menos"},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			var saida, erro bytes.Buffer

			codigo := executarImportacao(t.Context(), tt.dir, tt.destinos(), &saida, &erro)

			if codigo != tt.querCodigo {
				t.Errorf("código = %d, quer %d (stderr %q)", codigo, tt.querCodigo, erro.String())
			}
			if !strings.Contains(saida.String(), tt.querNaSaida) {
				t.Errorf("saída %q sem %q", saida.String(), tt.querNaSaida)
			}
			if !strings.Contains(erro.String(), tt.querNoErro) {
				t.Errorf("stderr %q sem %q", erro.String(), tt.querNoErro)
			}
		})
	}
}

func TestExecutarImportacaoErroSaiComUm(t *testing.T) {
	dir := t.TempDir()
	// memória de uma account que o mapeamento não conhece
	escreverArquivo(t, filepath.Join(dir, "memorias", "sem-dono", "5511999990001.json"), `{"fatos":[]}`)
	var saida, erro bytes.Buffer

	codigo := executarImportacao(t.Context(), dir, destinosEmMemoria(), &saida, &erro)

	if codigo != 1 || !strings.Contains(erro.String(), "sem-dono") {
		t.Errorf("código = %d, stderr %q; quer 1 listando a account órfã", codigo, erro.String())
	}
}

// pedidosQueNaoGuarda aceita a gravação e não guarda nada.
type pedidosQueNaoGuarda struct{ pedidos.Store }

func (pedidosQueNaoGuarda) Gravar(context.Context, escopo.Contato, pedidos.Carteira) error {
	return nil
}

func escreverArquivo(t *testing.T, caminho, conteudo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}
