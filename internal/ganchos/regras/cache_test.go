package regras

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/ent"
	"go-concierge/internal/ganchos"
)

// compilarContando é o compilador fake: o índice só conta as ativas, em duas chaves que
// precisam bater (um índice pela metade teria as duas diferentes).
func compilarContando(rs []RegraGravada) (*ganchos.Indice, []ErroDeCarga) {
	return &ganchos.Indice{Ativas: map[ganchos.NomeDoEvento]int{
		ganchos.EventoMensagemRecebida: len(rs),
		ganchos.EventoAntesDeResponder: len(rs),
	}}, nil
}

func ativasNoIndice(t *testing.T, c *Cache) int {
	t.Helper()
	idx := c.Indice()
	if idx == nil {
		t.Fatal("Indice nil depois de Aquecer")
	}
	return idx.Ativas[ganchos.EventoMensagemRecebida]
}

func TestAquecerComBancoForaDevolveErro(t *testing.T) {
	c := NovoCache(&Postgres{}, compilarContando, nil)
	c.lerAtivas = func(context.Context) ([]RegraGravada, error) { return nil, errors.New("banco fora") }
	if err := c.Aquecer(t.Context()); err == nil {
		t.Fatal("Aquecer com o banco fora devolveu nil")
	}
	if c.Indice() != nil {
		t.Fatal("erro de leitura publicou um índice")
	}
}

func aquecido(t *testing.T, p *Postgres, compilar Compilador, regras []RegraGravada) *Cache {
	t.Helper()
	c := NovoCache(p, compilar, nil)
	c.lerAtivas = func(context.Context) ([]RegraGravada, error) { return regras, nil }
	if err := c.Aquecer(t.Context()); err != nil {
		t.Fatal(err)
	}
	return c
}

// O prazo que chega à query do boot é o de Aquecer (5 s), não o teto de 3 s das rotas.
func TestAquecerDaAQueryOPrazoDeCincoSegundos(t *testing.T) {
	cli := bancotest.Novo(t).Ent
	var sobra time.Duration
	cli.GanchosRegra.Intercept(ent.TraverseFunc(func(ctx context.Context, _ ent.Query) error {
		if prazo, ok := ctx.Deadline(); ok {
			sobra = time.Until(prazo)
		}
		return nil
	}))
	c := NovoCache(NovoPostgres(cli), compilarContando, nil)
	if err := c.Aquecer(t.Context()); err != nil {
		t.Fatal(err)
	}
	if sobra <= timeoutConsulta || sobra > prazoAquecer {
		t.Fatalf("prazo na query = %v, quer entre %v e %v", sobra, timeoutConsulta, prazoAquecer)
	}
}

func TestAvisoAntesDeAquecerNaoPublicaIndiceParcial(t *testing.T) {
	p := &Postgres{}
	c := NovoCache(p, compilarContando, nil)
	p.aoAlterar(RegraGravada{ID: "salva-no-boot", Versao: 1, Ativa: true})
	if c.Indice() != nil {
		t.Fatal("aviso antes de Aquecer publicou um índice")
	}
	c.lerAtivas = func(context.Context) ([]RegraGravada, error) {
		return []RegraGravada{{ID: "lida-no-boot", Versao: 1, Ativa: true}}, nil
	}
	if err := c.Aquecer(t.Context()); err != nil {
		t.Fatal(err)
	}
	if n := ativasNoIndice(t, c); n != 2 {
		t.Fatalf("ativas depois de Aquecer = %d, quer 2 (a lida e a avisada antes)", n)
	}
}

func TestSalvarTrocaOIndiceSemRelerOBanco(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	salvar(t, p, novaRegra("antes do boot", true), 0)
	c := NovoCache(p, compilarContando, nil)
	if err := c.Aquecer(t.Context()); err != nil {
		t.Fatal(err)
	}
	lidas := 0
	c.lerAtivas = func(ctx context.Context) ([]RegraGravada, error) { lidas++; return p.Ativas(ctx) }
	antes := c.Indice()
	nova := salvar(t, p, novaRegra("depois do boot", true), 0)
	if n := ativasNoIndice(t, c); n != 2 || lidas != 0 || c.Indice() == antes {
		t.Fatalf("ativas no índice = %d, leituras do banco = %d; quer 2 e 0, com ponteiro novo", n, lidas)
	}
	nova.Ativa = false
	salvar(t, p, nova, nova.Versao)
	if n := ativasNoIndice(t, c); n != 1 {
		t.Fatalf("ativas depois de desativar = %d, quer 1", n)
	}
}

func TestCacheIgnoraVersaoMenorOuIgualAGuardada(t *testing.T) {
	tests := []struct {
		name   string
		avisos []RegraGravada
		quer   int
	}{
		{"versão menor reativando", []RegraGravada{{ID: "a", Versao: 2}, {ID: "a", Versao: 1, Ativa: true}}, 0},
		{"versão igual desativando", []RegraGravada{{ID: "a", Versao: 2, Ativa: true}, {ID: "a", Versao: 2}}, 1},
		{"versão maior desativando", []RegraGravada{{ID: "a", Versao: 1, Ativa: true}, {ID: "a", Versao: 2}}, 0},
		{"regras diferentes", []RegraGravada{{ID: "a", Versao: 3, Ativa: true}, {ID: "b", Versao: 1, Ativa: true}}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Postgres{}
			c := aquecido(t, p, compilarContando, nil)
			for _, r := range tt.avisos {
				p.aoAlterar(r)
			}
			if n := ativasNoIndice(t, c); n != tt.quer {
				t.Fatalf("ativas = %d, quer %d", n, tt.quer)
			}
		})
	}
}

func TestRecargaAvisaOResultado(t *testing.T) {
	tests := []struct {
		name  string
		erros []ErroDeCarga
		quer  string
	}{
		{"limpa", nil, "ok"},
		{"com regra que não compila", []ErroDeCarga{{RegraID: "a", Err: errors.New("condição inválida")}}, "parcial"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resultados []string
			compilar := func([]RegraGravada) (*ganchos.Indice, []ErroDeCarga) { return &ganchos.Indice{}, tt.erros }
			c := NovoCache(&Postgres{}, compilar, func(r string) { resultados = append(resultados, r) })
			c.lerAtivas = func(context.Context) ([]RegraGravada, error) { return nil, nil }
			if err := c.Aquecer(t.Context()); err != nil {
				t.Fatal(err)
			}
			if len(resultados) != 1 || resultados[0] != tt.quer {
				t.Fatalf("aoRecarregar = %v, quer [%s]", resultados, tt.quer)
			}
		})
	}
}

// Review Focus 5: com 10 regras salvas no meio de turnos, cada leitura vê uma das gravações (0
// a 10 ativas) e nunca volta para um índice mais velho. As duas chaves iguais pegam um índice
// publicado que alguém mexeu no lugar; o -race pega a mesma coisa pelo outro lado.
func TestTurnoVeIndiceInteiroEnquantoRegrasSaoSalvas(t *testing.T) {
	const escritores, leitores, leiturasMinimas = 10, 50, 1000
	p := NovoPostgres(bancotest.Novo(t).Ent)
	c := NovoCache(p, compilarContando, nil)
	if err := c.Aquecer(t.Context()); err != nil {
		t.Fatal(err)
	}
	var terminou atomic.Bool
	var leituras atomic.Int64
	falhas := make(chan string, leitores)
	var lendo sync.WaitGroup
	for range leitores {
		lendo.Go(func() { falhas <- lerAteTerminar(c, &terminou, &leituras, escritores) })
	}
	var salvando sync.WaitGroup
	for i := range escritores {
		salvando.Go(func() {
			if _, err := p.Salvar(context.Background(), novaRegra(fmt.Sprintf("regra %d", i), true), 0); err != nil {
				t.Errorf("Salvar %d: %v", i, err)
			}
		})
	}
	salvando.Wait()
	terminou.Store(true)
	lendo.Wait()
	close(falhas)
	for f := range falhas {
		if f != "" {
			t.Error(f)
		}
	}
	if n := ativasNoIndice(t, c); n != escritores {
		t.Fatalf("ativas no fim = %d, quer %d", n, escritores)
	}
	if n := leituras.Load(); n < leiturasMinimas {
		t.Fatalf("só %d leituras durante as gravações, quer ao menos %d", n, leiturasMinimas)
	}
}

// lerAteTerminar lê o índice em laço e devolve a primeira inconsistência ("" se nenhuma).
func lerAteTerminar(c *Cache, terminou *atomic.Bool, leituras *atomic.Int64, maximo int) string {
	ultimo := 0
	for !terminou.Load() {
		idx := c.Indice()
		n, espelho := idx.Ativas[ganchos.EventoMensagemRecebida], idx.Ativas[ganchos.EventoAntesDeResponder]
		if n != espelho || n < ultimo || n > maximo {
			return fmt.Sprintf("índice pela metade ou velho: %d e %d ativas, última leitura %d", n, espelho, ultimo)
		}
		ultimo = n
		leituras.Add(1)
		runtime.Gosched() // 50 laços apertados não podem tomar a CPU das gravações
	}
	return ""
}
