package memoria

import (
	"math"
	"slices"
	"testing"
	"time"
)

const umDia = 24 * time.Hour

var agoraDoTeste = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

func padraoEm(caso string, valor float64, atualizadoEm time.Time) Padrao {
	return Padrao{Caso: caso, CasoNome: caso, CondominioID: "cond-1", UnidadeID: "u12", Valor: valor, AtualizadoEm: atualizadoEm}
}

func pertoDe(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestValorEmMeiaVidaDivideAoMeio(t *testing.T) {
	p := padraoEm("segunda_via", 4, agoraDoTeste)

	got := ValorEm(p, agoraDoTeste.Add(45*umDia), AjustesPadrao())

	if !pertoDe(got, 2.0) {
		t.Fatalf("ValorEm = %v, want 2.0", got)
	}
}

func TestValorEmComDataNoFuturoNaoCresce(t *testing.T) {
	p := padraoEm("segunda_via", 4, agoraDoTeste.Add(3*time.Hour))

	if got := ValorEm(p, agoraDoTeste, AjustesPadrao()); got != p.Valor {
		t.Fatalf("ValorEm = %v, want %v", got, p.Valor)
	}
}

func TestRegistrarCriaPadraoComUm(t *testing.T) {
	o := Ocorrencia{Caso: "segunda_via", CasoNome: "Segunda via", CondominioID: "cond-1", UnidadeID: "u12"}

	p, mudou := Registrar(nil, o, agoraDoTeste, AjustesPadrao())

	want := Padrao{Caso: "segunda_via", CasoNome: "Segunda via", CondominioID: "cond-1", UnidadeID: "u12", Valor: 1, AtualizadoEm: agoraDoTeste}
	if !mudou || p != want {
		t.Fatalf("Registrar = %+v %v, want %+v true", p, mudou, want)
	}
}

func TestRegistrarDecaiESoma(t *testing.T) {
	atual := padraoEm("segunda_via", 1, agoraDoTeste.Add(-45*umDia))
	o := Ocorrencia{Caso: "segunda_via", CondominioID: "cond-1", UnidadeID: "u12"}

	p, mudou := Registrar([]Padrao{atual}, o, agoraDoTeste, AjustesPadrao())

	if !mudou || !pertoDe(p.Valor, 1.5) || !p.AtualizadoEm.Equal(agoraDoTeste) {
		t.Fatalf("Registrar = %+v %v, want valor 1.5 em agora", p, mudou)
	}
	if p.CasoNome != "segunda_via" {
		t.Fatalf("ocorrência sem nome apagou CasoNome: %q", p.CasoNome)
	}
}

func TestRegistrarRespeitaOIntervaloMinimo(t *testing.T) {
	o := Ocorrencia{Caso: "segunda_via", CondominioID: "cond-1", UnidadeID: "u12"}
	tests := []struct {
		name      string
		haQuanto  time.Duration
		wantMudou bool
		wantValor float64
	}{
		{"onze horas conta uma só", 11 * time.Hour, false, 1},
		{"doze horas ja conta", 12 * time.Hour, true, 1 + math.Pow(0.5, float64(12*time.Hour)/float64(45*umDia))},
		{"treze horas conta", 13 * time.Hour, true, 1 + math.Pow(0.5, float64(13*time.Hour)/float64(45*umDia))},
		{"gravado no futuro nao soma duas vezes", -3 * time.Hour, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atual := padraoEm("segunda_via", 1, agoraDoTeste.Add(-tt.haQuanto))

			p, mudou := Registrar([]Padrao{atual}, o, agoraDoTeste, AjustesPadrao())

			if mudou != tt.wantMudou || !pertoDe(p.Valor, tt.wantValor) {
				t.Fatalf("Registrar = valor %v mudou %v, want %v %v", p.Valor, mudou, tt.wantValor, tt.wantMudou)
			}
		})
	}
}

func TestRegistrarPreservaSupressao(t *testing.T) {
	atual := padraoEm("segunda_via", 1, agoraDoTeste.Add(-20*umDia))
	atual.SuprimidoAte = agoraDoTeste.Add(umDia)
	o := Ocorrencia{Caso: "segunda_via", CondominioID: "cond-1", UnidadeID: "u12"}

	p, _ := Registrar([]Padrao{atual}, o, agoraDoTeste, AjustesPadrao())

	if !p.SuprimidoAte.Equal(atual.SuprimidoAte) {
		t.Fatalf("SuprimidoAte = %v, want %v", p.SuprimidoAte, atual.SuprimidoAte)
	}
}

func TestRecusarDivideESuprime(t *testing.T) {
	p := padraoEm("segunda_via", 4, agoraDoTeste)

	got := Recusar(p, agoraDoTeste, AjustesPadrao())

	if !pertoDe(got.Valor, 2) || !got.SuprimidoAte.Equal(agoraDoTeste.Add(14*umDia)) || !got.AtualizadoEm.Equal(agoraDoTeste) {
		t.Fatalf("Recusar = %+v", got)
	}
}

func chavesDe(ps []Padrao) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Caso)
	}
	return out
}

func TestRanquearOrdenaPeloValorDecaido(t *testing.T) {
	antigoForte := padraoEm("a", 3, agoraDoTeste.Add(-90*umDia)) // 0,75 hoje
	recente := padraoEm("b", 1, agoraDoTeste)
	mesmoValorMaisNovo := padraoEm("c", 1, agoraDoTeste)
	mesmoValorMaisNovo.AtualizadoEm = agoraDoTeste // mesmo instante: desempata pela chave
	empateAntigo := padraoEm("d", 1, agoraDoTeste.Add(-time.Second))
	empateAntigo.Valor = 1 / math.Pow(0.5, float64(time.Second)/float64(45*umDia)) // decai para 1 exato em agora

	tests := []struct {
		name string
		in   []Padrao
		want []string
	}{
		{"valor decaido manda", []Padrao{antigoForte, recente}, []string{"b", "a"}},
		{"empate pelo mais recente", []Padrao{empateAntigo, recente}, []string{"b", "d"}},
		{"empate total pela chave", []Padrao{mesmoValorMaisNovo, recente}, []string{"b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chavesDe(Ranquear(tt.in, agoraDoTeste, AjustesPadrao())); !slices.Equal(got, tt.want) {
				t.Fatalf("Ranquear = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRanquearNaoAlteraAEntrada(t *testing.T) {
	in := []Padrao{padraoEm("a", 1, agoraDoTeste), padraoEm("b", 5, agoraDoTeste)}

	Ranquear(in, agoraDoTeste, AjustesPadrao())

	if in[0].Caso != "a" {
		t.Fatalf("entrada reordenada: %v", chavesDe(in))
	}
}

func TestDoAlvoFiltraCondominioEUnidade(t *testing.T) {
	semUnidade := Padrao{Caso: "sem_unidade", CondominioID: "cond-1", Valor: 1}
	daU12 := Padrao{Caso: "u12", CondominioID: "cond-1", UnidadeID: "u12", Valor: 1}
	daU15 := Padrao{Caso: "u15", CondominioID: "cond-1", UnidadeID: "u15", Valor: 1}
	deOutroCondominio := Padrao{Caso: "outro", CondominioID: "cond-2", Valor: 1}
	todos := []Padrao{semUnidade, daU12, daU15, deOutroCondominio}

	tests := []struct {
		name string
		alvo Alvo
		want []string
	}{
		{"unidade no vinculo fica", Alvo{CondominioID: "cond-1", Unidades: map[string]string{"u12": "Apto 12"}}, []string{"sem_unidade", "u12"}},
		{"unidade fora do vinculo sai", Alvo{CondominioID: "cond-1", Unidades: map[string]string{"u15": "Apto 15"}}, []string{"sem_unidade", "u15"}},
		{"sem vinculo fica so sem unidade", Alvo{CondominioID: "cond-1"}, []string{"sem_unidade"}},
		{"outro condominio sai", Alvo{CondominioID: "cond-2"}, []string{"outro"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chavesDe(DoAlvo(todos, tt.alvo)); !slices.Equal(got, tt.want) {
				t.Fatalf("DoAlvo = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEscolherOferta(t *testing.T) {
	alvo := Alvo{CondominioID: "cond-1", Unidades: map[string]string{"u12": "Apto 12"}}
	forte := func(caso string, valor float64) Padrao { return padraoEm(caso, valor, agoraDoTeste) }
	suprimido := func(p Padrao) Padrao { p.SuprimidoAte = agoraDoTeste.Add(umDia); return p }
	outroCondominio := func(p Padrao) Padrao { p.CondominioID = "cond-2"; return p }

	tests := []struct {
		name string
		in   []Padrao
		want string // vazio: não oferece
	}{
		{"um forte oferece", []Padrao{forte("a", 2.4)}, "a"},
		{"um fraco nao oferece", []Padrao{forte("a", 1.9)}, ""},
		{"nao e o dobro do segundo", []Padrao{forte("a", 2.4), forte("b", 1.8)}, ""},
		{"exatamente o dobro oferece", []Padrao{forte("a", 4.0), forte("b", 2.0)}, "a"},
		{"suprimido sai e o resto e fraco", []Padrao{suprimido(forte("a", 4.0)), forte("b", 1.5)}, ""},
		{"suprimido sai e o seguinte oferece", []Padrao{suprimido(forte("a", 4.0)), forte("b", 2.2), forte("c", 1.0)}, "b"},
		{"outro condominio nao conta como segundo", []Padrao{forte("a", 2.4), outroCondominio(forte("b", 2.3))}, "a"},
		{"sem padrao nao oferece", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ofereceu := EscolherOferta(tt.in, alvo, agoraDoTeste, AjustesPadrao())

			if ofereceu != (tt.want != "") || got.Caso != tt.want {
				t.Fatalf("EscolherOferta = %q %v, want %q", got.Caso, ofereceu, tt.want)
			}
		})
	}
}

func TestEscolherOfertaVoltaQuandoASupressaoVence(t *testing.T) {
	p := padraoEm("a", 3, agoraDoTeste)
	p.SuprimidoAte = agoraDoTeste
	alvo := Alvo{CondominioID: "cond-1", Unidades: map[string]string{"u12": "Apto 12"}}

	if _, ofereceu := EscolherOferta([]Padrao{p}, alvo, agoraDoTeste, AjustesPadrao()); !ofereceu {
		t.Fatal("supressão que acabou agora ainda silencia a oferta")
	}
}
