package camadas

import (
	"math"
	"slices"
	"strings"
	"testing"

	"go-concierge/internal/okf"
)

func idsDe(trechos []Trecho) []TrechoID {
	ids := make([]TrechoID, len(trechos))
	for i, t := range trechos {
		ids[i] = t.ID
	}
	return ids
}

func TestTrechosTemIDsUnicosEOrdemEstavel(t *testing.T) {
	d := com(func(d *Documento) {
		d.Fluxo.Blocos = append(d.Fluxo.Blocos, okf.Bloco{ID: okf.IDDaResposta, Tipo: okf.BlocoResponder, Texto: "Pronto"})
	})

	ids := idsDe(Trechos(d))

	vistos := map[TrechoID]bool{}
	for _, id := range ids {
		if vistos[id] {
			t.Errorf("id repetido: %s", id)
		}
		vistos[id] = true
	}
	want := []TrechoID{
		IDTitulo, IDResumo, IDQuandoUsar, IDFrasesDeExemplo, IDPalavrasChave, IDOrientacao, IDOrdem,
		IDDaEtapa("e-1"), IDDaEtapa("e-2"), IDDaEtapa("e-3"), IDDaEtapa("responder"), IDDaInformacao("data"),
	}
	if !slices.Equal(ids, want) {
		t.Errorf("ids = %v\nquer %v", ids, want)
	}
	if !slices.Equal(ids, idsDe(Trechos(d))) {
		t.Error("a ordem deveria ser a mesma em duas chamadas")
	}
}

func TestTrechosCamposVaziosContamComoTrechoVazio(t *testing.T) {
	porID := map[TrechoID]Trecho{}
	for _, tr := range Trechos(Documento{}) {
		porID[tr.ID] = tr
	}
	for _, id := range []TrechoID{IDTitulo, IDResumo, IDFrasesDeExemplo, IDOrientacao} {
		if tr, ok := porID[id]; !ok || tr.Conteudo != "" {
			t.Errorf("%s = %+v, ok=%v; quer trecho com conteúdo vazio", id, tr, ok)
		}
	}
	if porID[IDOrdem].Conteudo != "[]" {
		t.Errorf("ordem de fluxo vazio = %q, quer []", porID[IDOrdem].Conteudo)
	}
}

func TestTrechosRotulosDasEtapasContamPorTipo(t *testing.T) {
	d := com(func(d *Documento) {
		d.Fluxo.Blocos = append(d.Fluxo.Blocos, okf.Bloco{ID: "e-4", Tipo: okf.BlocoConfirmar, Texto: "De novo?"})
	})
	rotulos := map[TrechoID]string{}
	for _, tr := range Trechos(d) {
		rotulos[tr.ID] = tr.Rotulo
	}
	if rotulos[IDDaEtapa("e-2")] != "Etapa: Confirmar 1" || rotulos[IDDaEtapa("e-4")] != "Etapa: Confirmar 2" {
		t.Errorf("rótulos = %v", rotulos)
	}
}

func TestTrechoDaEtapaNaoCarregaOID(t *testing.T) {
	for _, tr := range Trechos(documentoDeTeste()) {
		if tr.Tipo == TrechoEtapa && strings.Contains(tr.Conteudo, `"id"`) {
			t.Errorf("conteúdo de %s tem id: %s", tr.ID, tr.Conteudo)
		}
	}
}

func TestTrechoDeEtapaComValorForaDoJSONContinuaEstavel(t *testing.T) {
	d := com(func(d *Documento) {
		editarBloco(d, "e-2", func(b *okf.Bloco) { b.Extra = map[string]any{"limite": math.NaN(), "a": 1} })
	})
	primeiro, segundo := Trechos(d)[8], Trechos(d)[8]
	if primeiro.ID != IDDaEtapa("e-2") || primeiro.Conteudo == "" || primeiro.Conteudo != segundo.Conteudo {
		t.Errorf("conteúdo instável ou vazio: %q / %q", primeiro.Conteudo, segundo.Conteudo)
	}
}

func TestMudouTextosEMudouEtapas(t *testing.T) {
	tests := []struct {
		name           string
		mudar          func(*Documento)
		textos, etapas bool
	}{
		{"nada", func(*Documento) {}, false, false},
		{"título", func(d *Documento) { d.Textos.Titulo = "Outro" }, true, false},
		{"frases de exemplo", func(d *Documento) { d.Textos.FrasesDeExemplo = nil }, true, false},
		{"visibilidade não é trecho", func(d *Documento) { d.Textos.Visibilidade = "public" }, false, false},
		{"ferramentas não são trecho", func(d *Documento) { d.Ferramentas = nil }, false, false},
		{"orientação", func(d *Documento) { d.Orientacao = "Ligar antes." }, false, true},
		{"texto de uma etapa", func(d *Documento) { editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "x" }) }, false, true},
		{"apagar etapa", func(d *Documento) { semBloco(d, "e-2") }, false, true},
		{"reordenar", func(d *Documento) {
			d.Fluxo.Blocos[0], d.Fluxo.Blocos[1] = d.Fluxo.Blocos[1], d.Fluxo.Blocos[0]
		}, false, true},
		{"informação", func(d *Documento) { d.Fluxo.Informacoes[0].Pergunta = "Qual dia?" }, false, true},
		{"texto e orientação", func(d *Documento) {
			d.Textos.Resumo = "r"
			d.Orientacao = "o"
		}, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			antes, depois := documentoDeTeste(), com(tt.mudar)
			if got := MudouTextos(antes, depois); got != tt.textos {
				t.Errorf("MudouTextos = %v, quer %v", got, tt.textos)
			}
			if got := MudouEtapas(antes, depois); got != tt.etapas {
				t.Errorf("MudouEtapas = %v, quer %v", got, tt.etapas)
			}
		})
	}
}
