package camadas

import (
	"reflect"
	"testing"

	"go-concierge/internal/okf"
)

func documentoDeTeste() Documento {
	return Documento{
		Textos: okf.Textos{
			Titulo: "Agendar mudança", QuandoUsar: "Quando o morador vai se mudar.",
			FrasesDeExemplo: []string{"quero agendar mudança"}, PalavrasChave: []string{"mudança"},
		},
		Fluxo: okf.Fluxo{
			Blocos: []okf.Bloco{
				{ID: "e-1", Tipo: okf.BlocoPerguntar, Campos: []string{"data"}},
				{ID: "e-2", Tipo: okf.BlocoConfirmar, Texto: "Confirma a data?"},
				{ID: "e-3", Tipo: okf.BlocoGravar, Recurso: "mudancas.agendar"},
			},
			Informacoes: []okf.Informacao{{Nome: "data", Titulo: "Data da mudança"}},
		},
		Ferramentas: []string{"mudancas.agendar"},
	}
}

// com devolve o documento de teste alterado por fn.
func com(fn func(*Documento)) Documento {
	d := documentoDeTeste()
	fn(&d)
	return d
}

func semBloco(d *Documento, id string) {
	var blocos []okf.Bloco
	for _, b := range d.Fluxo.Blocos {
		if b.ID != id {
			blocos = append(blocos, b)
		}
	}
	d.Fluxo.Blocos = blocos
}

func editarBloco(d *Documento, id string, fn func(*okf.Bloco)) {
	for i := range d.Fluxo.Blocos {
		if d.Fluxo.Blocos[i].ID == id {
			fn(&d.Fluxo.Blocos[i])
		}
	}
}

func situacoes(ds []Diferenca) map[TrechoID]Situacao {
	r := map[TrechoID]Situacao{}
	for _, d := range ds {
		r[d.Trecho] = d.Situacao
	}
	return r
}

func TestCompararSoBaseMudouVemComoSoBase(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { d.Textos.Titulo = "Agendar a mudança" })

	got := Comparar(antes, depois, antes)

	if len(got) != 1 {
		t.Fatalf("diferenças = %+v, quer 1", got)
	}
	d := got[0]
	if d.Trecho != IDTitulo || d.Situacao != SoBase || d.Tipo != TrechoTexto || d.Rotulo != "Título" || d.Obrigatorio {
		t.Errorf("diferença = %+v", d)
	}
	if *d.Antes != "Agendar mudança" || *d.Depois != "Agendar a mudança" || *d.Minha != "Agendar mudança" {
		t.Errorf("antes/depois/minha = %q/%q/%q", *d.Antes, *d.Depois, *d.Minha)
	}
}

func TestCompararSoAPessoaMudouNaoAparece(t *testing.T) {
	antes := documentoDeTeste()
	minha := com(func(d *Documento) {
		d.Textos.Titulo = "Minha mudança"
		d.Orientacao = "Falar com a portaria."
	})

	if got := Comparar(antes, antes, minha); len(got) != 0 {
		t.Errorf("diferenças = %+v, quer nenhuma", got)
	}
}

func TestCompararAsDuasMudaramDiferenteEhConflito(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { d.Textos.Titulo = "Da base" })
	minha := com(func(d *Documento) { d.Textos.Titulo = "Da pessoa" })

	got := Comparar(antes, depois, minha)

	if len(got) != 1 || got[0].Situacao != Conflito || *got[0].Minha != "Da pessoa" {
		t.Errorf("diferenças = %+v, quer 1 conflito", got)
	}
}

func TestCompararAsDuasMudaramIgualNaoAparece(t *testing.T) {
	antes := documentoDeTeste()
	igual := com(func(d *Documento) { d.Textos.Titulo = "Igual" })

	if got := Comparar(antes, igual, igual); len(got) != 0 {
		t.Errorf("diferenças = %+v, quer nenhuma", got)
	}
}

func TestCompararApagarEAlterarEhConflito(t *testing.T) {
	editada := func(d *Documento) {
		editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "Posso agendar?" })
	}
	apagada := func(d *Documento) { semBloco(d, "e-2") }
	tests := []struct {
		name                  string
		depois, minha         Documento
		depoisNulo, minhaNula bool
	}{
		{"base apaga, pessoa edita", com(apagada), com(editada), true, false},
		{"pessoa apaga, base edita", com(editada), com(apagada), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Comparar(documentoDeTeste(), tt.depois, tt.minha)

			var etapa *Diferenca
			for i := range got {
				if got[i].Trecho == IDDaEtapa("e-2") {
					etapa = &got[i]
				}
			}
			if etapa == nil || etapa.Situacao != Conflito {
				t.Fatalf("etapa e-2 deveria ser conflito: %+v", got)
			}
			if (etapa.Depois == nil) != tt.depoisNulo || (etapa.Minha == nil) != tt.minhaNula {
				t.Errorf("depois nulo = %v, minha nula = %v", etapa.Depois == nil, etapa.Minha == nil)
			}
		})
	}
}

func TestCompararApagarEtapaIntocadaVemComoSoBase(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { semBloco(d, "e-2") })

	got := Comparar(antes, depois, antes)

	if situacoes(got)[IDDaEtapa("e-2")] != SoBase {
		t.Fatalf("apagar etapa intocada deveria ser so_base: %+v", got)
	}
	for _, d := range got {
		if d.Trecho == IDDaEtapa("e-2") && d.Depois != nil {
			t.Errorf("depois deveria ser nil para etapa apagada: %+v", d)
		}
	}
}

func TestCompararBaseCriaEtapaNovaVemComoSoBase(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) {
		d.Fluxo.Blocos = append(d.Fluxo.Blocos, okf.Bloco{ID: "e-9", Tipo: okf.BlocoMontarTexto, Texto: "Novo"})
	})

	got := situacoes(Comparar(antes, depois, antes))

	want := map[TrechoID]Situacao{IDDaEtapa("e-9"): SoBase, IDOrdem: SoBase}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diferenças = %v, quer %v", got, want)
	}
}

func TestCompararReordenarSoMudaOTrechoDeOrdem(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) {
		b := d.Fluxo.Blocos
		d.Fluxo.Blocos = []okf.Bloco{b[1], b[0], b[2]}
	})

	got := Comparar(antes, depois, antes)

	if len(got) != 1 || got[0].Trecho != IDOrdem || got[0].Tipo != TrechoOrdem || got[0].Situacao != SoBase {
		t.Errorf("diferenças = %+v, quer só fluxo:ordem", got)
	}
}

func TestCompararRenomearEtapaNaoMudaAIdentidade(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) {
		editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "Tudo certo com a data?" })
	})

	got := Comparar(antes, depois, antes)

	if len(got) != 1 || got[0].Trecho != IDDaEtapa("e-2") || got[0].Situacao != SoBase {
		t.Errorf("diferenças = %+v, quer só etapa:e-2 como so_base", got)
	}
}

func TestCompararInformacaoUsaONomeComoIdentidade(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { d.Fluxo.Informacoes[0].Titulo = "Dia da mudança" })

	got := Comparar(antes, depois, antes)

	if len(got) != 1 || got[0].Trecho != IDDaInformacao("data") || got[0].Rotulo != "Informação: Dia da mudança" {
		t.Errorf("diferenças = %+v", got)
	}
}

func TestCompararSemMudancasNaoDevolveNada(t *testing.T) {
	d := documentoDeTeste()
	if got := Comparar(d, d, d); len(got) != 0 {
		t.Errorf("diferenças = %+v, quer nenhuma", got)
	}
}

// A ordem segue o documento novo; o que só a pessoa ou só o documento antigo tem vem depois.
func TestCompararOrdenaPeloDocumentoNovoEDepoisPeloResto(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) {
		d.Textos.Titulo = "B"
		semBloco(d, "e-3")
	})
	minha := com(func(d *Documento) {
		editarBloco(d, "e-3", func(b *okf.Bloco) { b.Recurso = "outra.coisa" })
	})

	var ordem []TrechoID
	for _, d := range Comparar(antes, depois, minha) {
		ordem = append(ordem, d.Trecho)
	}

	want := []TrechoID{IDTitulo, IDOrdem, IDDaEtapa("e-3")}
	if !reflect.DeepEqual(ordem, want) {
		t.Errorf("ordem = %v, quer %v", ordem, want)
	}
}
