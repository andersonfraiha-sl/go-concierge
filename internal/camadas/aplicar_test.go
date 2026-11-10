package camadas

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"go-concierge/internal/okf"
)

func todos(difs []Diferenca, escolha Escolha) []Decisao {
	decisoes := make([]Decisao, len(difs))
	for i, d := range difs {
		decisoes[i] = Decisao{Trecho: d.Trecho, Escolha: escolha}
	}
	return decisoes
}

func idsDasEtapas(d Documento) []string { return idsDosBlocos(d.Fluxo.Blocos) }

func comResponder(d *Documento) {
	d.Fluxo.Blocos = append(d.Fluxo.Blocos, okf.Bloco{ID: okf.IDDaResposta, Tipo: okf.BlocoResponder, Texto: "Pronto."})
}

func TestAplicarAceitarTudoSemPersonalizacaoDevolveADepois(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) {
		d.Textos.Titulo = "Da base"
		d.Orientacao = "Orientação nova"
		editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "Confirma mesmo?" })
		d.Fluxo.Informacoes[0].Titulo = "Dia"
	})
	difs := Comparar(antes, depois, antes)

	got, err := Aplicar(antes, depois, difs, todos(difs, Aceitar))

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(Trechos(got), Trechos(depois)) {
		t.Errorf("composto = %+v, quer %+v", Trechos(got), Trechos(depois))
	}
}

func TestAplicarManterTudoDevolveAMinha(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { d.Textos.Titulo = "Da base"; d.Textos.Resumo = "Resumo da base" })
	minha := com(func(d *Documento) { d.Textos.Titulo = "Da pessoa" })
	difs := Comparar(antes, depois, minha)

	got, err := Aplicar(minha, depois, difs, todos(difs, Manter))

	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(Trechos(got), Trechos(minha)) {
		t.Errorf("composto = %+v, quer %+v", Trechos(got), Trechos(minha))
	}
}

func TestAplicarSoBaseSemDecisaoMantem(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { d.Textos.Titulo = "Da base" })
	difs := Comparar(antes, depois, antes)

	got, err := Aplicar(antes, depois, difs, nil)

	if err != nil || got.Textos.Titulo != antes.Textos.Titulo {
		t.Errorf("título = %q, err = %v; quer o original", got.Textos.Titulo, err)
	}
}

func TestAplicarConflitoSemDecisaoFalha(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { d.Textos.Titulo = "Da base"; d.Textos.Resumo = "R" })
	minha := com(func(d *Documento) { d.Textos.Titulo = "Da pessoa" })
	difs := Comparar(antes, depois, minha)

	_, err := Aplicar(minha, depois, difs, []Decisao{{Trecho: IDResumo, Escolha: Aceitar}})

	var semEscolha *ErrConflitoSemEscolha
	if !errors.As(err, &semEscolha) || !slices.Equal(semEscolha.Trechos, []TrechoID{IDTitulo}) {
		t.Errorf("err = %v, quer ErrConflitoSemEscolha com texto:titulo", err)
	}
}

func TestAplicarMisturaDeTexto(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { d.Textos.Titulo = "Da base" })
	minha := com(func(d *Documento) { d.Textos.Titulo = "Da pessoa" })
	difs := Comparar(antes, depois, minha)

	got, err := Aplicar(minha, depois, difs, []Decisao{{Trecho: IDTitulo, Escolha: Misturar, Mistura: "Dos dois"}})

	if err != nil || got.Textos.Titulo != "Dos dois" {
		t.Errorf("título = %q, err = %v", got.Textos.Titulo, err)
	}
}

func TestAplicarMisturaDeEtapa(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "Da base" }) })
	minha := com(func(d *Documento) { editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "Da pessoa" }) })
	difs := Comparar(antes, depois, minha)
	mistura := `{"tipo":"confirmar","texto":"Dos dois"}`

	got, err := Aplicar(minha, depois, difs, []Decisao{{Trecho: IDDaEtapa("e-2"), Escolha: Misturar, Mistura: mistura}})

	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(got.Fluxo.Blocos, func(b okf.Bloco) bool { return b.ID == "e-2" })
	if i < 0 || got.Fluxo.Blocos[i].Texto != "Dos dois" || got.Fluxo.Blocos[i].Tipo != okf.BlocoConfirmar {
		t.Errorf("blocos = %+v", got.Fluxo.Blocos)
	}
}

func TestAplicarMisturaInvalidaFalha(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "Da base" }) })
	minha := com(func(d *Documento) { editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "Da pessoa" }) })
	difs := Comparar(antes, depois, minha)
	tests := []struct {
		nome    string
		mistura string
	}{
		{"não é JSON", "texto solto"},
		{"campo desconhecido", `{"tipo":"confirmar","inventado":1}`},
		{"sem tipo", `{"texto":"x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			_, err := Aplicar(minha, depois, difs, []Decisao{{Trecho: IDDaEtapa("e-2"), Escolha: Misturar, Mistura: tt.mistura}})

			if !errors.Is(err, ErrDecisaoInvalida) {
				t.Errorf("err = %v, quer ErrDecisaoInvalida", err)
			}
		})
	}
}

func TestAplicarDecisaoInvalidaFalha(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { d.Textos.Titulo = "Da base" })
	difs := Comparar(antes, depois, antes)
	tests := []struct {
		nome     string
		decisoes []Decisao
	}{
		{"trecho fora da atualização", []Decisao{{Trecho: IDResumo, Escolha: Aceitar}}},
		{"escolha desconhecida", []Decisao{{Trecho: IDTitulo, Escolha: "talvez"}}},
		{"trecho decidido duas vezes", []Decisao{{Trecho: IDTitulo, Escolha: Aceitar}, {Trecho: IDTitulo, Escolha: Manter}}},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			if _, err := Aplicar(antes, depois, difs, tt.decisoes); !errors.Is(err, ErrDecisaoInvalida) {
				t.Errorf("err = %v, quer ErrDecisaoInvalida", err)
			}
		})
	}
}

func TestAplicarAceitarRemocaoApaga(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { semBloco(d, "e-2") })
	minha := com(func(d *Documento) { editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "Editada" }) })
	difs := Comparar(antes, depois, minha)

	got, err := Aplicar(minha, depois, difs, todos(difs, Aceitar))

	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"e-1", "e-3"}; !slices.Equal(idsDasEtapas(got), want) {
		t.Errorf("etapas = %v, quer %v", idsDasEtapas(got), want)
	}
}

func TestAplicarManterRemocaoGuardaAEtapa(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { semBloco(d, "e-2") })
	minha := com(func(d *Documento) { editarBloco(d, "e-2", func(b *okf.Bloco) { b.Texto = "Editada" }) })
	difs := Comparar(antes, depois, minha)

	got, err := Aplicar(minha, depois, difs, todos(difs, Manter))

	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"e-1", "e-2", "e-3"}; !slices.Equal(idsDasEtapas(got), want) {
		t.Errorf("etapas = %v, quer %v", idsDasEtapas(got), want)
	}
}

func TestAplicarOrdemComEtapaNovaDaBaseEntraAntesDaResposta(t *testing.T) {
	antes := com(comResponder)
	depois := com(func(d *Documento) {
		comResponder(d)
		novo := okf.Bloco{ID: "e-9", Tipo: okf.BlocoMontarTexto, Texto: "Novo"}
		d.Fluxo.Blocos = slices.Insert(d.Fluxo.Blocos, 1, novo)
	})
	minha := com(func(d *Documento) { comResponder(d); d.Textos.Titulo = "Minha" })
	difs := Comparar(antes, depois, minha)
	decisoes := []Decisao{{Trecho: IDDaEtapa("e-9"), Escolha: Aceitar}, {Trecho: IDOrdem, Escolha: Manter}}

	got, err := Aplicar(minha, depois, difs, decisoes)

	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"e-1", "e-2", "e-3", "e-9", "responder"}; !slices.Equal(idsDasEtapas(got), want) {
		t.Errorf("etapas = %v, quer %v", idsDasEtapas(got), want)
	}
}

func TestAplicarAceitarOrdemSegueABaseEMantemEtapaDaPessoa(t *testing.T) {
	antes := com(comResponder)
	depois := com(func(d *Documento) {
		comResponder(d)
		b := d.Fluxo.Blocos
		d.Fluxo.Blocos = []okf.Bloco{b[1], b[0], b[2], b[3]}
	})
	minha := com(func(d *Documento) {
		comResponder(d)
		propria := okf.Bloco{ID: "e-x", Tipo: okf.BlocoMontarTexto, Texto: "Minha"}
		d.Fluxo.Blocos = slices.Insert(d.Fluxo.Blocos, 3, propria)
	})
	difs := Comparar(antes, depois, minha)

	got, err := Aplicar(minha, depois, difs, []Decisao{{Trecho: IDOrdem, Escolha: Aceitar}})

	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"e-2", "e-1", "e-3", "e-x", "responder"}; !slices.Equal(idsDasEtapas(got), want) {
		t.Errorf("etapas = %v, quer %v", idsDasEtapas(got), want)
	}
}

func TestAplicarFerramentasEVisibilidadeSaoSempreDaBase(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) {
		d.Ferramentas = []string{"mudancas.agendar", "mudancas.listar"}
		d.Textos.Visibilidade = "interna"
	})

	got, err := Aplicar(antes, depois, Comparar(antes, depois, antes), nil)

	if err != nil || !slices.Equal(got.Ferramentas, depois.Ferramentas) || got.Textos.Visibilidade != "interna" {
		t.Errorf("ferramentas = %v, visibilidade = %q, err = %v", got.Ferramentas, got.Textos.Visibilidade, err)
	}
}

func TestAceitarPadraoSoPegaSoBase(t *testing.T) {
	difs := []Diferenca{
		{Trecho: IDTitulo, Situacao: SoBase},
		{Trecho: IDResumo, Situacao: Conflito},
		{Trecho: IDOrientacao, Situacao: SoBase},
	}

	got := AceitarPadrao(difs)

	want := []Decisao{{Trecho: IDTitulo, Escolha: Aceitar}, {Trecho: IDOrientacao, Escolha: Aceitar}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("decisões = %+v, quer %+v", got, want)
	}
}

func TestMarcarObrigatoriosNaoMudaAEntrada(t *testing.T) {
	difs := []Diferenca{{Trecho: IDTitulo}, {Trecho: IDResumo}}

	got := MarcarObrigatorios(difs, []TrechoID{IDResumo})

	if got[0].Obrigatorio || !got[1].Obrigatorio || difs[1].Obrigatorio {
		t.Errorf("marcadas = %+v, originais = %+v", got, difs)
	}
}

func TestAplicarObrigatorioTrocaMesmoPersonalizadoEDizQuais(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) {
		d.Textos.Titulo = "Da base"
		d.Textos.Resumo = "Resumo da base"
		d.Orientacao = "Segurança nova"
	})
	minha := com(func(d *Documento) { d.Textos.Titulo = "Da pessoa" })
	obrigatorios := []TrechoID{IDTitulo, IDResumo, IDOrientacao}

	got, trocados := AplicarObrigatorio(antes, minha, depois, obrigatorios)

	if got.Textos.Titulo != "Da base" || got.Textos.Resumo != "Resumo da base" || got.Orientacao != "Segurança nova" {
		t.Errorf("composto = %+v", got)
	}
	if want := []TrechoID{IDTitulo}; !slices.Equal(trocados, want) {
		t.Errorf("trocados = %v, quer só o que a pessoa editou: %v", trocados, want)
	}
}

func TestAplicarObrigatorioApagaEtapaQueABaseTirou(t *testing.T) {
	antes := documentoDeTeste()
	depois := com(func(d *Documento) { semBloco(d, "e-3") })
	minha := com(func(d *Documento) { editarBloco(d, "e-3", func(b *okf.Bloco) { b.Recurso = "outra.coisa" }) })

	got, trocados := AplicarObrigatorio(antes, minha, depois, []TrechoID{IDDaEtapa("e-3")})

	if slices.Contains(idsDasEtapas(got), "e-3") || !slices.Equal(trocados, []TrechoID{IDDaEtapa("e-3")}) {
		t.Errorf("etapas = %v, trocados = %v", idsDasEtapas(got), trocados)
	}
}

// Manter tudo tem de devolver a camada exatamente como está, para qualquer terceira via.
func TestAplicarManterTudoEhIdentidade(t *testing.T) {
	sorteio := rand.New(rand.NewPCG(7, 11))
	for range 300 {
		antes, depois, minha := mutar(sorteio, documentoDeTeste()), mutar(sorteio, documentoDeTeste()), mutar(sorteio, documentoDeTeste())
		difs := Comparar(antes, depois, minha)

		got, err := Aplicar(minha, depois, difs, todos(difs, Manter))

		if err != nil || !reflect.DeepEqual(Trechos(got), Trechos(minha)) {
			t.Fatalf("manter tudo mudou a camada (err = %v):\nantes  %+v\ndepois %+v\nminha  %+v\ngot    %+v", err, Trechos(antes), Trechos(depois), Trechos(minha), Trechos(got))
		}
	}
}

// Aceitar tudo sem personalização tem de devolver exatamente a versão nova.
func TestAplicarAceitarTudoSemPersonalizacaoEhIdentidadeDaDepois(t *testing.T) {
	sorteio := rand.New(rand.NewPCG(3, 5))
	for range 300 {
		antes, depois := mutar(sorteio, documentoDeTeste()), mutar(sorteio, documentoDeTeste())
		difs := Comparar(antes, depois, antes)

		got, err := Aplicar(antes, depois, difs, todos(difs, Aceitar))

		if err != nil || !reflect.DeepEqual(Trechos(got), Trechos(depois)) {
			t.Fatalf("aceitar tudo não deu a depois (err = %v):\nantes  %+v\ndepois %+v\ngot    %+v", err, Trechos(antes), Trechos(depois), Trechos(got))
		}
	}
}

// mutar sorteia alterações de texto, etapa (editar, apagar, acrescentar), informação e ordem,
// mantendo a resposta como último bloco.
func mutar(r *rand.Rand, d Documento) Documento {
	comResponder(&d)
	if r.IntN(2) == 0 {
		d.Textos.Titulo = "T" + string(rune('a'+r.IntN(5)))
	}
	if r.IntN(3) == 0 {
		d.Orientacao = "O" + string(rune('a'+r.IntN(5)))
	}
	if r.IntN(2) == 0 {
		editarBloco(&d, "e-2", func(b *okf.Bloco) { b.Texto = "C" + string(rune('a'+r.IntN(5))) })
	}
	if r.IntN(3) == 0 {
		semBloco(&d, []string{"e-1", "e-2", "e-3"}[r.IntN(3)])
	}
	if r.IntN(3) == 0 {
		novo := okf.Bloco{ID: "n-" + string(rune('a'+r.IntN(3))), Tipo: okf.BlocoMontarTexto, Texto: "N"}
		d.Fluxo.Blocos = slices.Insert(d.Fluxo.Blocos, r.IntN(len(d.Fluxo.Blocos)), novo)
	}
	if r.IntN(3) == 0 {
		r.Shuffle(len(d.Fluxo.Blocos)-1, func(i, j int) { d.Fluxo.Blocos[i], d.Fluxo.Blocos[j] = d.Fluxo.Blocos[j], d.Fluxo.Blocos[i] })
	}
	if r.IntN(3) == 0 {
		d.Fluxo.Informacoes = nil
	}
	return d
}
