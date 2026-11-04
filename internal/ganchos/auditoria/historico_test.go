package auditoria_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/ganchos/auditoria"
)

func TestDecodificarCursorRecusaOQueListarNaoDevolveu(t *testing.T) {
	valido := auditoria.Cursor{CriadoEm: time.Now(), ID: "6f1b1f0e-6d1c-4a63-9d44-3a8f6a3b9c11"}
	volta, err := auditoria.DecodificarCursor(valido.String())
	if err != nil || volta.ID != valido.ID || !volta.CriadoEm.Equal(valido.CriadoEm) {
		t.Fatalf("ida e volta = %+v, %v", volta, err)
	}
	for _, texto := range []string{"", "%%%", "bm8tcGlwZQ", "MjAyNi0xMS0wNHxudW0", "aXNzby1uYW8tZS1kYXRh"} {
		if _, err := auditoria.DecodificarCursor(texto); !errors.Is(err, auditoria.ErrCursorInvalido) {
			t.Errorf("cursor %q: erro = %v, quer ErrCursorInvalido", texto, err)
		}
	}
}

// 120 linhas em apenas 3 instantes: o desempate por id decide e nada repete nem pula.
func TestListarPaginaPorCriadoEmEIdSemRepetirNemPular(t *testing.T) {
	b := bancotest.Novo(t)
	base := time.Now().UTC().Add(-time.Hour).Round(time.Second)
	var linhas []auditoria.Linha
	for i := range 120 {
		l := linha("regra", base.Add(time.Duration(i%3)*time.Minute))
		l.Motivo = fmt.Sprint(i)
		linhas = append(linhas, l)
	}
	if err := auditoria.Gravar(t.Context(), b, linhas, nil); err != nil {
		t.Fatal(err)
	}
	f := auditoria.Filtro{Desde: base.Add(-time.Hour), Ate: time.Now()}

	var tamanhos []int
	vistos := map[string]bool{}
	var depois *auditoria.Cursor
	for range 5 {
		pagina, proximo, err := auditoria.Listar(t.Context(), b.Ent, auditoria.Escopo{Tudo: true}, f, depois, 50)
		if err != nil {
			t.Fatal(err)
		}
		if len(pagina) == 0 {
			break
		}
		tamanhos = append(tamanhos, len(pagina))
		for _, l := range pagina {
			if vistos[l.ID] {
				t.Fatalf("linha %s repetida", l.ID)
			}
			vistos[l.ID] = true
		}
		if depois = proximo; depois == nil {
			break
		}
	}

	if fmt.Sprint(tamanhos) != "[50 50 20]" || len(vistos) != 120 {
		t.Fatalf("páginas = %v, distintas = %d, quer [50 50 20] e 120", tamanhos, len(vistos))
	}
}

func TestListarAplicaEscopoEFiltrosNaQuery(t *testing.T) {
	b := bancotest.Novo(t)
	agora := time.Now().UTC()
	de := func(adm, cond, decisao string) auditoria.Linha {
		l := linha("regra", agora.Add(-time.Minute))
		l.AdministradoraID, l.CondominioID, l.Decisao = adm, cond, decisao
		return l
	}
	gravadas := []auditoria.Linha{
		de("adm-a", "cond-a", "bloquear"), de("adm-a", "cond-a2", "falhou"),
		de("adm-b", "cond-b", "bloquear"), de("", "", "ajustar"),
	}
	if err := auditoria.Gravar(t.Context(), b, gravadas, nil); err != nil {
		t.Fatal(err)
	}
	periodo := auditoria.Filtro{Desde: agora.Add(-time.Hour), Ate: agora}
	tests := []struct {
		nome   string
		esc    auditoria.Escopo
		filtro func(*auditoria.Filtro)
		quer   int
	}{
		{"tudo", auditoria.Escopo{Tudo: true}, nil, 4},
		{"administradora A", auditoria.Escopo{Administradoras: []string{"adm-a"}}, nil, 2},
		{"condomínio avulso", auditoria.Escopo{Condominios: []string{"cond-b"}}, nil, 1},
		{"administradora e condomínio avulso", auditoria.Escopo{Administradoras: []string{"adm-a"}, Condominios: []string{"cond-b"}}, nil, 3},
		{"escopo vazio não vê nada", auditoria.Escopo{}, nil, 0},
		{"filtro de B dentro do escopo de A", auditoria.Escopo{Administradoras: []string{"adm-a"}},
			func(f *auditoria.Filtro) { f.AdministradoraID = "adm-b" }, 0},
		{"decisão falhou", auditoria.Escopo{Tudo: true}, func(f *auditoria.Filtro) { f.Decisao = "falhou" }, 1},
		{"condomínio", auditoria.Escopo{Tudo: true}, func(f *auditoria.Filtro) { f.CondominioID = "cond-a" }, 1},
		{"fora do período", auditoria.Escopo{Tudo: true}, func(f *auditoria.Filtro) { f.Ate = agora.Add(-time.Hour) }, 0},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			f := periodo
			if tt.filtro != nil {
				tt.filtro(&f)
			}
			pagina, proximo, err := auditoria.Listar(t.Context(), b.Ent, tt.esc, f, nil, 50)
			if err != nil || proximo != nil || len(pagina) != tt.quer {
				t.Fatalf("linhas = %d, próximo = %v, erro = %v; quer %d", len(pagina), proximo, err, tt.quer)
			}
		})
	}
}
