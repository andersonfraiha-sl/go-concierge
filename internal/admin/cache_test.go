package admin

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"go-concierge/internal/banco/bancotest"
)

func lerCache(t *testing.T, c *Cache) Mapeamento {
	t.Helper()
	m, err := c.Ler(t.Context())
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}
	return m
}

func temAdministradora(m Mapeamento, id string) bool {
	for _, a := range m.Administradoras {
		if a.ID == id {
			return true
		}
	}
	return false
}

// Uma escrita do Admin aparece no cache quente sem reler o banco.
func TestCacheRecebeEscritaDoAdmin(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	c := NovoCache(p)
	lerCache(t, c)
	lidas := 0
	c.lerDoBanco = func(ctx context.Context) (Mapeamento, error) { lidas++; return p.Ler(ctx) }
	if _, err := p.SalvarAdministradora(t.Context(), Administradora{ID: "adm-n", Nome: "Nova"}); err != nil {
		t.Fatal(err)
	}
	if m := lerCache(t, c); !temAdministradora(m, "adm-n") || lidas != 0 {
		t.Fatalf("cache sem a escrita (lidas do banco: %d): %+v", lidas, m.Administradoras)
	}
}

// Erro de leitura não fica guardado: a leitura seguinte vai ao banco de novo.
func TestCacheNaoGuardaErro(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	c := NovoCache(p)
	falhar := true
	c.lerDoBanco = func(ctx context.Context) (Mapeamento, error) {
		if falhar {
			return Mapeamento{}, errors.New("banco fora")
		}
		return p.Ler(ctx)
	}
	if _, err := c.Ler(t.Context()); err == nil {
		t.Fatal("Ler com o banco fora devolveu nil")
	}
	falhar = false
	lerCache(t, c)
}

// Uma leitura fria que voltou depois de uma escrita não troca a escrita pelo que leu antes.
func TestCacheLeituraAntigaNaoSobrescreveEscrita(t *testing.T) {
	c := NovoCache(&Postgres{})
	antigo := Mapeamento{Administradoras: []Administradora{{ID: "adm-velha"}}}
	novo := Mapeamento{Administradoras: []Administradora{{ID: "adm-nova"}}}
	c.lerDoBanco = func(context.Context) (Mapeamento, error) {
		c.guardar(novo, 1) // a escrita chega enquanto a leitura está no banco
		return antigo, nil
	}
	lerCache(t, c)
	if m, _ := c.guardado(); !temAdministradora(m, "adm-nova") {
		t.Fatalf("guardado %+v, want a escrita", m.Administradoras)
	}
}

// Duas escritas fora de ordem: fica a de versão maior.
func TestCacheEscritaMaisVelhaNaoSobrescreve(t *testing.T) {
	c := NovoCache(&Postgres{})
	c.guardar(Mapeamento{Administradoras: []Administradora{{ID: "v2"}}}, 2)
	c.guardar(Mapeamento{Administradoras: []Administradora{{ID: "v1"}}}, 1)
	if m, _ := c.guardado(); !temAdministradora(m, "v2") {
		t.Fatalf("guardado %+v, want a versão 2", m.Administradoras)
	}
}

// Escritas simultâneas terminam com o cache igual ao banco.
func TestCacheEscritasSimultaneasIgualAoBanco(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	c := NovoCache(p)
	lerCache(t, c)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			if _, err := p.SalvarAdministradora(context.Background(), Administradora{ID: fmt.Sprintf("adm-%d", i), Nome: "A"}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	banco, err := p.Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cache := lerCache(t, c); !reflect.DeepEqual(cache, banco) {
		t.Fatalf("cache %+v\nbanco %+v", cache.Administradoras, banco.Administradoras)
	}
}
