package camadas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-concierge/internal/okf"
)

const casoDeTeste = `id: agendar_mudanca
name: Agendamento de Mudança
summary: Agenda a mudança.
visibility: private
description: Quando o morador quer agendar a mudança.
keywords: [mudança]
examples:
  - quero agendar minha mudança
params:
  - name: data_mudanca
    title: Data da mudança
    ask: Para qual data?
    validate: date
steps:
  - collect: [data_mudanca]
  - confirm: "Confirma a mudança em {{.data_mudanca}}? (sim/não)"
  - resource: mudancas.agendar
    with:
      data: "{{.data_mudanca}}"
    save_as: agendamento
output: "Agendado. Protocolo: {{.agendamento.protocolo}}."
`

func documentoDoCaso(t *testing.T, titulo string) Documento {
	t.Helper()
	fluxo, err := okf.FluxoDeYAML([]byte(casoDeTeste), func(recurso string) bool { return strings.HasPrefix(recurso, "mudancas.") })
	if err != nil {
		t.Fatalf("FluxoDeYAML: %v", err)
	}
	return Documento{
		Textos: okf.Textos{
			Titulo: titulo, QuandoUsar: "Quando o morador quer agendar a mudança.",
			FrasesDeExemplo: []string{"quero agendar minha mudança"}, PalavrasChave: []string{"mudança"}, Visibilidade: "private",
		},
		Fluxo:       fluxo,
		Ferramentas: []string{"mudancas.agendar"},
	}
}

// comTitulo devolve o documento com o título trocado: é o que cada teste usa para saber de
// qual camada veio a skill efetiva.
func comTitulo(d Documento, titulo string) Documento {
	d.Textos.Titulo = titulo
	return d
}

func blocoInvalido(d Documento) Documento {
	blocos := append([]okf.Bloco(nil), d.Fluxo.Blocos...)
	blocos = append(blocos, okf.Bloco{ID: "e-ruim", Tipo: "tipo_que_nao_existe"})
	d.Fluxo.Blocos = blocos
	return d
}

func retratoDeTeste(t *testing.T) Retrato {
	t.Helper()
	base := documentoDoCaso(t, "Base")
	return Retrato{
		Bases: map[string]VersaoBase{skillMudanca: {Cabeca: Cabeca{VersaoID: 1, Documento: base}, CasoYAML: []byte(casoDeTeste)}},
		Administradoras: map[string]map[string]Cabeca{skillMudanca: {
			"A": {VersaoID: 2, Documento: comTitulo(base, "Adm A")},
		}},
		Condominios: map[string]map[string]Cabeca{skillMudanca: {
			"c1": {VersaoID: 3, Documento: comTitulo(base, "Cond c1")},
		}},
	}
}

func tituloEfetivo(t *testing.T, consulta func(string) (okf.SkillEfetiva, bool), skill string) string {
	t.Helper()
	ef, ok := consulta(skill)
	if !ok {
		t.Fatalf("skill %q não oferecida", skill)
	}
	return ef.Caso.Name
}

func TestEfetivarAplicaTextosEFluxo(t *testing.T) {
	d := documentoDoCaso(t, "Agendar a mudança")
	d.Orientacao = "Seja breve."
	for i := range d.Fluxo.Blocos {
		if d.Fluxo.Blocos[i].Tipo == okf.BlocoResponder {
			d.Fluxo.Blocos[i].Texto = "Feito."
		}
	}

	got, err := Efetivar([]byte(casoDeTeste), d)

	if err != nil {
		t.Fatalf("Efetivar: %v", err)
	}
	if got.Caso == nil || got.Caso.Name != "Agendar a mudança" || got.Caso.Output != "Feito." || got.Caso.ID != "agendar_mudanca" {
		t.Errorf("caso = %+v; quer título e resposta da camada, id da base", got.Caso)
	}
	if got.Descricao != d.Textos.QuandoUsar || got.Orientacao != "Seja breve." {
		t.Errorf("descrição/orientação = %q/%q", got.Descricao, got.Orientacao)
	}
	if !strings.Contains(got.Principal, "Agendar a mudança") || !strings.Contains(got.Principal, "1. ") {
		t.Errorf("principal = %q; quer o título e os passos numerados", got.Principal)
	}
}

func TestEfetivarSkillSemFluxo(t *testing.T) {
	d := Documento{Textos: okf.Textos{Titulo: "Saudação", QuandoUsar: "Quando o morador cumprimenta."}, Orientacao: "Cordial."}

	got, err := Efetivar(nil, d)

	want := okf.SkillEfetiva{Descricao: "Quando o morador cumprimenta.", Orientacao: "Cordial."}
	if err != nil || got != want {
		t.Errorf("Efetivar = %+v, %v; quer %+v", got, err, want)
	}
}

func TestEfetivarRecusaFluxoQueNaoCompoe(t *testing.T) {
	tests := []struct {
		name string
		caso []byte
		doc  Documento
	}{
		{"bloco de tipo inválido", []byte(casoDeTeste), blocoInvalido(documentoDoCaso(t, "X"))},
		{"caso.yaml ilegível", []byte("id: [\n"), documentoDoCaso(t, "X")},
		{"visibilidade inválida", []byte(casoDeTeste), alterado(documentoDoCaso(t, "X"), func(d *Documento) { d.Textos.Visibilidade = "secreta" })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Efetivar(tt.caso, tt.doc); err == nil {
				t.Error("Efetivar compôs o que não devia")
			}
		})
	}
}

func alterado(d Documento, fn func(*Documento)) Documento {
	fn(&d)
	return d
}

func TestDoCondominioPrefereCondominioDepoisAdministradoraDepoisBase(t *testing.T) {
	e := NovasEfetivas(nil)
	e.Trocar(retratoDeTeste(t))
	tests := []struct {
		name                       string
		administradora, condominio string
		want                       string
	}{
		{"condomínio com camada própria", "A", "c1", "Cond c1"},
		{"condomínio sem camada herda a administradora", "A", "c2", "Adm A"},
		{"administradora sem camada herda a base", "B", "c9", "Base"},
		{"condomínio com camada vale mesmo sem administradora conhecida", "B", "c1", "Cond c1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consulta := e.DoCondominio(tt.administradora, tt.condominio, time.Now())

			if got := tituloEfetivo(t, consulta, skillMudanca); got != tt.want {
				t.Errorf("título efetivo = %q, quer %q", got, tt.want)
			}
		})
	}
}

func TestDoCondominioEscondeSkillEncerrada(t *testing.T) {
	encerra := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		encerraEm time.Time
		agora     time.Time
		skill     string
		want      bool
	}{
		{"sem data de fim", time.Time{}, encerra.AddDate(5, 0, 0), skillMudanca, true},
		{"antes do fim", encerra, encerra.Add(-time.Nanosecond), skillMudanca, true},
		{"no instante do fim", encerra, encerra, skillMudanca, false},
		{"depois do fim", encerra, encerra.Add(time.Hour), skillMudanca, false},
		{"skill sem versão base publicada", time.Time{}, encerra, "outra_skill", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := retratoDeTeste(t)
			base := r.Bases[skillMudanca]
			base.EncerraEm = tt.encerraEm
			r.Bases[skillMudanca] = base
			e := NovasEfetivas(nil)
			e.Trocar(r)

			_, got := e.DoCondominio("A", "c1", tt.agora)(tt.skill)

			if got != tt.want {
				t.Errorf("oferecida = %v, quer %v", got, tt.want)
			}
		})
	}
}

func TestDoCondominioEscondeCamadaDeSkillSemBase(t *testing.T) {
	r := retratoDeTeste(t)
	r.Bases = map[string]VersaoBase{}
	e := NovasEfetivas(nil)
	e.Trocar(r)

	if _, ok := e.DoCondominio("A", "c1", time.Now())(skillMudanca); ok {
		t.Error("camada sem base foi oferecida")
	}
}

func TestDoCondominioAntesDeTrocarDevolveNil(t *testing.T) {
	if consulta := NovasEfetivas(nil).DoCondominio("A", "c1", time.Now()); consulta != nil {
		t.Error("antes da primeira troca a consulta deveria ser nil (o turno usa os arquivos)")
	}
}

func TestMontarCaiParaACamadaDeCimaQuandoAComposicaoFalha(t *testing.T) {
	base := documentoDoCaso(t, "Base")
	quebrada := func(titulo string) Cabeca { return Cabeca{Documento: blocoInvalido(comTitulo(base, titulo))} }
	tests := []struct {
		name                 string
		administradora, cond *Cabeca
		wantTitulo           string
		wantLogs             int
	}{
		{"condomínio quebrado cai para a administradora", ptr(Cabeca{VersaoID: 2, Documento: comTitulo(base, "Adm A")}), ptr(withID(quebrada("Cond"), 3)), "Adm A", 1},
		{"os dois quebrados caem para a base", ptr(withID(quebrada("Adm"), 2)), ptr(withID(quebrada("Cond"), 3)), "Base", 2},
		{"administradora quebrada não derruba condomínio bom", ptr(withID(quebrada("Adm"), 2)), ptr(Cabeca{VersaoID: 3, Documento: comTitulo(base, "Cond")}), "Cond", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var saida bytes.Buffer
			e := NovasEfetivas(slog.New(slog.NewJSONHandler(&saida, nil)))
			e.Trocar(Retrato{
				Bases:           map[string]VersaoBase{skillMudanca: {Cabeca: Cabeca{VersaoID: 1, Documento: base}, CasoYAML: []byte(casoDeTeste)}},
				Administradoras: map[string]map[string]Cabeca{skillMudanca: {"A": *tt.administradora}},
				Condominios:     map[string]map[string]Cabeca{skillMudanca: {"c1": *tt.cond}},
			})

			got := tituloEfetivo(t, e.DoCondominio("A", "c1", time.Now()), skillMudanca)

			if got != tt.wantTitulo {
				t.Errorf("título efetivo = %q, quer %q", got, tt.wantTitulo)
			}
			conferirLogsDeErro(t, saida.Bytes(), tt.wantLogs)
		})
	}
}

func conferirLogsDeErro(t *testing.T, saida []byte, quer int) {
	t.Helper()
	linhas := bytes.Split(bytes.TrimSpace(saida), []byte("\n"))
	if len(saida) == 0 {
		linhas = nil
	}
	if len(linhas) != quer {
		t.Fatalf("logs = %d, quer %d: %s", len(linhas), quer, saida)
	}
	for _, linha := range linhas {
		var campos map[string]any
		if err := json.Unmarshal(linha, &campos); err != nil {
			t.Fatalf("log ilegível %q: %v", linha, err)
		}
		if campos["level"] != "ERROR" || campos["operacao"] != "camadas.compor" || campos["skill"] != skillMudanca || campos["erro"] == nil {
			t.Errorf("log = %v; quer ERROR de camadas.compor com skill e erro", campos)
		}
	}
}

func ptr[T any](v T) *T { return &v }

func withID(c Cabeca, id int) Cabeca {
	c.VersaoID = id
	return c
}

// contador devolve um Efetivar que conta quantas vezes foi chamado.
func contador() (func([]byte, Documento) (okf.SkillEfetiva, error), *atomic.Int32) {
	var chamadas atomic.Int32
	return func(caso []byte, d Documento) (okf.SkillEfetiva, error) {
		chamadas.Add(1)
		return Efetivar(caso, d)
	}, &chamadas
}

func TestTrocarCompoeCadaVersaoUmaVez(t *testing.T) {
	base := documentoDoCaso(t, "Base")
	e := NovasEfetivas(nil)
	efetivar, chamadas := contador()
	e.efetivar = efetivar
	r := retratoDeTeste(t)

	e.Trocar(r)
	e.Trocar(r)
	if got := chamadas.Load(); got != 3 {
		t.Fatalf("composições depois de duas trocas iguais = %d, quer 3 (base, administradora, condomínio)", got)
	}

	r.Condominios[skillMudanca]["c1"] = Cabeca{VersaoID: 4, Documento: comTitulo(base, "Cond novo")}
	e.Trocar(r)
	if got := chamadas.Load(); got != 4 {
		t.Errorf("composições depois de mudar só o condomínio = %d, quer 4", got)
	}
	if got := tituloEfetivo(t, e.DoCondominio("A", "c1", time.Now()), skillMudanca); got != "Cond novo" {
		t.Errorf("título efetivo = %q, quer Cond novo", got)
	}
}

func TestTrocarRecompoeCamadasQuandoABaseMuda(t *testing.T) {
	e := NovasEfetivas(nil)
	efetivar, chamadas := contador()
	e.efetivar = efetivar
	r := retratoDeTeste(t)
	e.Trocar(r)

	base := r.Bases[skillMudanca]
	base.VersaoID = 10
	r.Bases[skillMudanca] = base
	e.Trocar(r)

	if got := chamadas.Load(); got != 6 {
		t.Errorf("composições = %d, quer 6 (as camadas se compõem sobre o caso.yaml da base nova)", got)
	}
}

func TestTrocarNaoRepeteALogDeUmaCamadaQuebradaJaConhecida(t *testing.T) {
	var saida bytes.Buffer
	e := NovasEfetivas(slog.New(slog.NewJSONHandler(&saida, nil)))
	r := retratoDeTeste(t)
	r.Condominios[skillMudanca]["c1"] = Cabeca{VersaoID: 3, Documento: blocoInvalido(documentoDoCaso(t, "Cond"))}

	e.Trocar(r)
	e.Trocar(r)

	conferirLogsDeErro(t, saida.Bytes(), 1)
}

func TestConsultaJaEntregueNaoVeTrocaPosterior(t *testing.T) {
	e := NovasEfetivas(nil)
	r := retratoDeTeste(t)
	e.Trocar(r)
	consulta := e.DoCondominio("A", "c1", time.Now())

	r.Condominios[skillMudanca]["c1"] = Cabeca{VersaoID: 4, Documento: comTitulo(documentoDoCaso(t, "x"), "Depois")}
	e.Trocar(r)

	if got := tituloEfetivo(t, consulta, skillMudanca); got != "Cond c1" {
		t.Errorf("consulta antiga = %q, quer Cond c1 (o turno termina com o que começou)", got)
	}
	if got := tituloEfetivo(t, e.DoCondominio("A", "c1", time.Now()), skillMudanca); got != "Depois" {
		t.Errorf("consulta nova = %q, quer Depois", got)
	}
}

func TestRecarregarTrocaPeloRetratoLido(t *testing.T) {
	e := NovasEfetivas(nil)
	r := retratoDeTeste(t)

	err := e.Recarregar(t.Context(), func(context.Context) (Retrato, error) { return r, nil })

	if err != nil {
		t.Fatalf("Recarregar: %v", err)
	}
	if got := tituloEfetivo(t, e.DoCondominio("A", "c1", time.Now()), skillMudanca); got != "Cond c1" {
		t.Errorf("título efetivo = %q, quer Cond c1", got)
	}
}

func TestRecarregarComLeituraQueFalhaMantemAComposicaoAnterior(t *testing.T) {
	e := NovasEfetivas(nil)
	e.Trocar(retratoDeTeste(t))
	falhou := errors.New("banco fora")

	err := e.Recarregar(t.Context(), func(context.Context) (Retrato, error) { return Retrato{}, falhou })

	if !errors.Is(err, falhou) {
		t.Errorf("erro = %v, quer o da leitura", err)
	}
	if got := tituloEfetivo(t, e.DoCondominio("A", "c1", time.Now()), skillMudanca); got != "Cond c1" {
		t.Errorf("título efetivo = %q, quer a composição anterior", got)
	}
}

// retratoDaGeracao: duas skills com o mesmo título de geração em todas as camadas, para o
// leitor conferir que uma consulta nunca mistura gerações.
func retratoDaGeracao(t *testing.T, geracao string) Retrato {
	t.Helper()
	base := documentoDoCaso(t, geracao)
	r := Retrato{
		Bases:           map[string]VersaoBase{},
		Administradoras: map[string]map[string]Cabeca{},
		Condominios:     map[string]map[string]Cabeca{},
	}
	for i, skill := range []string{"s1", "s2", "s3"} {
		r.Bases[skill] = VersaoBase{Cabeca: Cabeca{VersaoID: 10 + i, Documento: base}, CasoYAML: []byte(casoDeTeste)}
		r.Administradoras[skill] = map[string]Cabeca{"A": {VersaoID: 20 + i, Documento: base}}
		r.Condominios[skill] = map[string]Cabeca{"c1": {VersaoID: 30 + i, Documento: base}}
	}
	return r
}

func TestDoCondominioEmParaleloComTrocar(t *testing.T) {
	e := NovasEfetivas(nil)
	geracoes := []Retrato{retratoDaGeracao(t, "g0"), retratoDaGeracao(t, "g1"), retratoDaGeracao(t, "g2")}
	e.Trocar(geracoes[0])
	parar := make(chan struct{})
	var trocas sync.WaitGroup
	trocas.Go(func() {
		for i := 1; ; i++ {
			select {
			case <-parar:
				return
			default:
				e.Trocar(geracoes[i%len(geracoes)])
			}
		}
	})
	var leitores sync.WaitGroup
	erros := make(chan string, 8)
	for range 8 {
		leitores.Go(func() {
			for range 2000 {
				consulta := e.DoCondominio("A", "c1", time.Now())
				titulos := map[string]bool{}
				for _, skill := range []string{"s1", "s2", "s3"} {
					ef, ok := consulta(skill)
					if !ok || ef.Caso == nil {
						erros <- "skill " + skill + " sumiu durante a troca"
						return
					}
					titulos[ef.Caso.Name] = true
				}
				if len(titulos) != 1 {
					erros <- "a consulta misturou gerações"
					return
				}
			}
		})
	}
	leitores.Wait()
	close(parar)
	trocas.Wait()
	close(erros)

	for msg := range erros {
		t.Error(msg)
	}
}
