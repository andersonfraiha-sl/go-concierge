package okf

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func escritaSempre(string) bool { return true }
func escritaNunca(string) bool  { return false }

func TestFluxoDeYAMLMapeiaPassos(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		escrita func(string) bool
		want    Bloco
	}{
		{
			name: "confirmar",
			yaml: "steps:\n  - confirm: \"Ok? (sim/não)\"\n",
			want: Bloco{ID: "b1", Tipo: BlocoConfirmar, Texto: "Ok? (sim/não)"},
		},
		{
			name: "montar texto",
			yaml: "steps:\n  - compose: \"Olá {{.nome}}\"\n",
			want: Bloco{ID: "b1", Tipo: BlocoMontarTexto, Texto: "Olá {{.nome}}"},
		},
		{
			name: "perguntar com texto vira lista",
			yaml: "steps:\n  - collect: status\n",
			want: Bloco{ID: "b1", Tipo: BlocoPerguntar, Campos: []string{"status"}},
		},
		{
			name: "perguntar com lista",
			yaml: "steps:\n  - collect: [a, b]\n",
			want: Bloco{ID: "b1", Tipo: BlocoPerguntar, Campos: []string{"a", "b"}},
		},
		{
			name: "reaproveitar com varios slots",
			yaml: "steps:\n  - reuse: {slots: [a, b], confirm: c}\n",
			want: Bloco{ID: "b1", Tipo: BlocoReaproveitar, Campos: []string{"a", "b"}, Confirmacao: "c"},
		},
		{
			name: "reaproveitar com um slot",
			yaml: "steps:\n  - reuse: {slot: a, confirm: c, from: f, label: l, value: v, ask: q, extra: 1}\n",
			want: Bloco{
				ID: "b1", Tipo: BlocoReaproveitar, Campos: []string{"a"}, Confirmacao: "c",
				De: "f", Rotulo: "l", Valor: "v", Pergunta: "q", Extra: map[string]any{"extra": 1},
			},
		},
		{
			name:    "recurso de escrita grava",
			yaml:    "steps:\n  - resource: x\n",
			escrita: escritaSempre,
			want:    Bloco{ID: "b1", Tipo: BlocoGravar, Recurso: "x"},
		},
		{
			name: "recurso de leitura busca",
			yaml: "steps:\n  - resource: x\n    save_as: r\n    cache: 5\n",
			want: Bloco{ID: "b1", Tipo: BlocoBuscar, Recurso: "x", GuardarEm: "r", Extra: map[string]any{"cache": 5}},
		},
		{
			name: "with preserva a ordem",
			yaml: "steps:\n  - resource: x\n    with:\n      b: 1\n      a: 2\n",
			want: Bloco{ID: "b1", Tipo: BlocoBuscar, Recurso: "x", Com: []CampoValor{{"b", 1}, {"a", 2}}},
		},
		{
			name: "escolha com campos extras",
			yaml: "steps:\n  - choose:\n      param: p\n      title: T\n      from: f\n      label: l\n      value: v\n      ask: q\n      empty: e\n      reuse: always\n      strict: true\n",
			want: Bloco{
				ID: "b1", Tipo: BlocoEscolha, Param: "p", Titulo: "T", De: "f", Rotulo: "l", Valor: "v",
				Pergunta: "q", SeVazio: "e", Reaproveitar: "always", Extra: map[string]any{"strict": true},
			},
		},
		{
			name: "formulario",
			yaml: "steps:\n  - form:\n      fields_from: campos\n      save_as: dados\n      modo: x\n",
			want: Bloco{ID: "b1", Tipo: BlocoFormulario, De: "campos", GuardarEm: "dados", Extra: map[string]any{"modo": "x"}},
		},
		{
			name: "responder vem por ultimo",
			yaml: "output: Pronto\nsteps:\n  - confirm: a\n",
			want: Bloco{ID: "b2", Tipo: BlocoResponder, Texto: "Pronto"},
		},
		{
			name: "texto vazio vai para o extra sem se perder",
			yaml: "steps:\n  - choose:\n      param: p\n      empty: \"\"\n",
			want: Bloco{ID: "b1", Tipo: BlocoEscolha, Param: "p", Extra: map[string]any{"empty": ""}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			escrita := tt.escrita
			if escrita == nil {
				escrita = escritaNunca
			}
			f, err := FluxoDeYAML([]byte(tt.yaml), escrita)
			if err != nil {
				t.Fatalf("FluxoDeYAML: %v", err)
			}
			got := f.Blocos[len(f.Blocos)-1]
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("bloco\n got  %#v\n want %#v", got, tt.want)
			}
		})
	}
}

func TestFluxoDeYAMLMapeiaParametros(t *testing.T) {
	raw := `params:
  - name: a
    title: A
    ask: Qual?
    type: select
    required: false
    options:
      - {value: x, label: X}
      - {value: y, label: Y}
  - name: b
    options: [Sala, Cozinha]
`
	f, err := FluxoDeYAML([]byte(raw), escritaNunca)
	if err != nil {
		t.Fatalf("FluxoDeYAML: %v", err)
	}
	want := []Informacao{
		{
			Nome: "a", Titulo: "A", Pergunta: "Qual?", Tipo: "select",
			Opcoes: []Opcao{{"x", "X"}, {"y", "Y"}},
			Extra:  map[string]any{"required": false},
		},
		{
			Nome:          "b",
			Opcoes:        []Opcao{{"Sala", "Sala"}, {"Cozinha", "Cozinha"}},
			OpcoesSimples: true,
		},
	}
	if !reflect.DeepEqual(f.Informacoes, want) {
		t.Errorf("informacoes\n got  %#v\n want %#v", f.Informacoes, want)
	}
}

func TestFluxoDeYAMLRecusaPassoDesconhecido(t *testing.T) {
	_, err := FluxoDeYAML([]byte("steps: [{voar: x}]\n"), escritaNunca)
	if err == nil || !strings.Contains(err.Error(), "passo 1") {
		t.Fatalf("erro = %v, quer mencionar \"passo 1\"", err)
	}
}

func TestFluxoDeYAMLRecusaFormaInvalida(t *testing.T) {
	tests := []struct{ name, yaml, quer string }{
		{"chave a mais fora de resource", "steps:\n  - confirm: a\n    save_as: x\n", "passo 1"},
		{"confirm que nao e texto", "steps:\n  - confirm: [a]\n", "passo 1"},
		{"compose que nao e texto", "steps:\n  - confirm: a\n  - compose: 3\n", "passo 2"},
		{"output que nao e texto", "output: [a]\n", "output"},
		{"valor de opcao que nao e texto", "params:\n  - name: a\n    options:\n      - {value: 1, label: Um}\n", "valor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FluxoDeYAML([]byte(tt.yaml), escritaNunca)
			if err == nil || !strings.Contains(err.Error(), tt.quer) {
				t.Fatalf("erro = %v, quer mencionar %q", err, tt.quer)
			}
		})
	}
}

func casosDoCatalogo(t *testing.T) map[string][]byte {
	t.Helper()
	casos := map[string][]byte{}
	err := filepath.WalkDir(filepath.Join("..", "..", "okf", "agentes"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "caso.yaml" {
			return err
		}
		raw, err := os.ReadFile(p)
		casos[p] = raw
		return err
	})
	if err != nil {
		t.Fatalf("percorrer catálogo: %v", err)
	}
	if len(casos) < 8 {
		t.Fatalf("catálogo com %d casos, quer ao menos 8", len(casos))
	}
	return casos
}

// normalizar iguala as duas grafias de collect (texto ou lista de um) para comparar.
func normalizar(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if s, ok := val.(string); ok && k == "collect" {
				out[k] = []any{s}
				continue
			}
			out[k] = normalizar(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = normalizar(val)
		}
		return out
	default:
		return v
	}
}

func decodificar(t *testing.T, raw []byte) any {
	t.Helper()
	var v any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return normalizar(v)
}

func TestFluxoIdaEVoltaCatalogoReal(t *testing.T) {
	for p, raw := range casosDoCatalogo(t) {
		t.Run(p, func(t *testing.T) {
			f, err := FluxoDeYAML(raw, escritaNunca)
			if err != nil {
				t.Fatalf("FluxoDeYAML: %v", err)
			}
			saida, err := AplicarFluxo(raw, f)
			if err != nil {
				t.Fatalf("AplicarFluxo: %v", err)
			}
			if !reflect.DeepEqual(decodificar(t, raw), decodificar(t, saida)) {
				t.Errorf("ida e volta divergiu\n%s", saida)
			}
		})
	}
}

func TestFluxoIdaEVoltaPeloJSON(t *testing.T) {
	for p, raw := range casosDoCatalogo(t) {
		t.Run(p, func(t *testing.T) {
			f, err := FluxoDeYAML(raw, escritaNunca)
			if err != nil {
				t.Fatalf("FluxoDeYAML: %v", err)
			}
			corpo, err := json.Marshal(f)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var voltou Fluxo
			if err := json.Unmarshal(corpo, &voltou); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			saida, err := AplicarFluxo(raw, voltou)
			if err != nil {
				t.Fatalf("AplicarFluxo: %v", err)
			}
			if !reflect.DeepEqual(decodificar(t, raw), decodificar(t, saida)) {
				t.Errorf("ida e volta pelo JSON divergiu\n%s", saida)
			}
		})
	}
}

func TestAplicarFluxoIdempotente(t *testing.T) {
	for p, raw := range casosDoCatalogo(t) {
		t.Run(p, func(t *testing.T) {
			f, err := FluxoDeYAML(raw, escritaNunca)
			if err != nil {
				t.Fatalf("FluxoDeYAML: %v", err)
			}
			saida, err := AplicarFluxo(raw, f)
			if err != nil {
				t.Fatalf("AplicarFluxo: %v", err)
			}
			f2, err := FluxoDeYAML(saida, escritaNunca)
			if err != nil {
				t.Fatalf("FluxoDeYAML da saída: %v", err)
			}
			de2, err := AplicarFluxo(saida, f2)
			if err != nil {
				t.Fatalf("AplicarFluxo da saída: %v", err)
			}
			if !bytes.Equal(saida, de2) {
				t.Errorf("segunda aplicação mudou os bytes\n--- primeira\n%s\n--- segunda\n%s", saida, de2)
			}
		})
	}
}

func TestAplicarFluxoPreservaChavesForaDoFluxo(t *testing.T) {
	raw := casosDoCatalogo(t)[filepath.Join("..", "..", "okf", "agentes", "concierge", "agentes", "agendar-mudanca", "caso.yaml")]
	if raw == nil {
		t.Fatal("agendar-mudanca/caso.yaml não encontrado")
	}
	f, err := FluxoDeYAML(raw, escritaNunca)
	if err != nil {
		t.Fatalf("FluxoDeYAML: %v", err)
	}
	for i, b := range f.Blocos {
		if b.Tipo == BlocoConfirmar {
			f.Blocos[i].Texto = "Texto novo?"
		}
	}
	saida, err := AplicarFluxo(raw, f)
	if err != nil {
		t.Fatalf("AplicarFluxo: %v", err)
	}
	var antes, depois map[string]any
	if err := yaml.Unmarshal(raw, &antes); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(saida, &depois); err != nil {
		t.Fatal(err)
	}
	for _, chave := range []string{"id", "name", "keywords", "examples", "icon"} {
		if !reflect.DeepEqual(antes[chave], depois[chave]) {
			t.Errorf("%s mudou: %v -> %v", chave, antes[chave], depois[chave])
		}
	}
	if !strings.HasPrefix(string(saida), "# Trazido do sl-concierge") {
		t.Errorf("comentário do topo perdido:\n%.80s", saida)
	}
	if !strings.Contains(string(saida), "Texto novo?") {
		t.Error("texto novo do confirmar não foi gravado")
	}
}

func TestAplicarFluxoSemResponderRemoveOutput(t *testing.T) {
	raw := []byte("id: x\nsteps:\n  - confirm: a\noutput: fim\n")
	f, err := FluxoDeYAML(raw, escritaNunca)
	if err != nil {
		t.Fatal(err)
	}
	f.Blocos = f.Blocos[:len(f.Blocos)-1]
	saida, err := AplicarFluxo(raw, f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saida), "output") {
		t.Errorf("output deveria ter sido removido:\n%s", saida)
	}
}

func TestAplicarFluxoSemInformacoesRemoveParams(t *testing.T) {
	raw := []byte("id: x\nparams:\n  - name: a\nsteps:\n  - confirm: a\n")
	saida, err := AplicarFluxo(raw, Fluxo{Blocos: []Bloco{{Tipo: BlocoConfirmar, Texto: "a"}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saida), "params") {
		t.Errorf("params deveria ter sido removido:\n%s", saida)
	}
}

func TestAplicarFluxoRecusaTipoDesconhecido(t *testing.T) {
	_, err := AplicarFluxo([]byte("id: x\n"), Fluxo{Blocos: []Bloco{{Tipo: "voar"}}})
	if err == nil || !strings.Contains(err.Error(), "bloco 1") {
		t.Fatalf("erro = %v, quer mencionar \"bloco 1\"", err)
	}
}
