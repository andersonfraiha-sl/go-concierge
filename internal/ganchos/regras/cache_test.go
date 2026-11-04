package regras

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"go-concierge/internal/banco/bancotest"
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
			c := NovoCache(p, compilarContando, nil)
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

// Review Focus 5: com 10 regras salvas no meio de turnos, cada leitura vê um índice inteiro de
// uma das gravações (0 a 10 ativas, as duas chaves iguais), e nunca volta para um mais velho.
func TestTurnoVeIndiceInteiroEnquantoRegrasSaoSalvas(t *testing.T) {
	const escritores, leitores = 10, 50
	p := NovoPostgres(bancotest.Novo(t).Ent)
	c := NovoCache(p, compilarContando, nil)
	if err := c.Aquecer(t.Context()); err != nil {
		t.Fatal(err)
	}
	var terminou atomic.Bool
	falhas := make(chan string, leitores)
	var lendo sync.WaitGroup
	for range leitores {
		lendo.Go(func() { falhas <- lerAteTerminar(c, &terminou, escritores) })
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
}

// lerAteTerminar lê o índice em laço e devolve a primeira inconsistência ("" se nenhuma).
func lerAteTerminar(c *Cache, terminou *atomic.Bool, maximo int) string {
	ultimo := 0
	for !terminou.Load() {
		idx := c.Indice()
		n, espelho := idx.Ativas[ganchos.EventoMensagemRecebida], idx.Ativas[ganchos.EventoAntesDeResponder]
		if n != espelho || n < ultimo || n > maximo {
			return fmt.Sprintf("índice pela metade ou velho: %d e %d ativas, última leitura %d", n, espelho, ultimo)
		}
		ultimo = n
		runtime.Gosched() // 50 laços apertados não podem tomar a CPU das gravações
	}
	return ""
}
