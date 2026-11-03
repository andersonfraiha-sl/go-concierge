package atende

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/escopo"
	"go-concierge/internal/memoria"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

// TestGravarConcorrente: Ler → Upsert → Gravar sob o lock do contato não perde fato, mesmo
// com o Gravar trocando a lista inteira de fatos do contato no Postgres.
func TestGravarConcorrente(t *testing.T) {
	const gravacoes = 20
	s := &Servico{Memorias: memoria.NovoPostgres(bancotest.Novo(t))}
	c, err := escopo.NovoContato("adm-a", "cond-1", "5519981621916")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	erros := make(chan error, gravacoes)
	for i := range gravacoes {
		wg.Go(func() { erros <- gravarFato(t, s, c, fmt.Sprintf("chave_%02d", i)) })
	}
	wg.Wait()
	close(erros)
	for err := range erros {
		if err != nil {
			t.Fatal(err)
		}
	}

	m, err := s.Memorias.Ler(t.Context(), c)
	if err != nil || len(m.Fatos) != gravacoes {
		t.Fatalf("Ler = %d fatos, %v; want %d", len(m.Fatos), err, gravacoes)
	}
}

func gravarFato(t *testing.T, s *Servico, c escopo.Contato, chave string) error {
	desbloquear := s.bloquearMemoria(c)
	defer desbloquear()
	m, err := s.Memorias.Ler(t.Context(), c)
	if err != nil {
		return err
	}
	m, _ = m.Upsert(memoria.Fato{Chave: chave, Texto: "fato " + chave})
	return s.Memorias.Gravar(t.Context(), c, m)
}
