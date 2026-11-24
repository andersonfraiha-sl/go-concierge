package admin

import (
	"errors"
	"reflect"
	"testing"

	"go-concierge/internal/banco/bancotest"
)

func TestConteudoV2CompoeSemCongelarHerancaEPreservaOutrosAlvos(t *testing.T) {
	b := bancotest.Novo(t)
	p := NovoPostgres(b.Ent)
	ctx := t.Context()
	company := "Empresa Nexus / A"
	entries := []struct {
		alvo Alvo
		data map[string]any
	}{
		{Alvo{Condominio: Padrao}, map[string]any{"base": "global", "override": "global"}},
		{Alvo{Dona: company, Condominio: Padrao}, map[string]any{"company": "a", "override": "company"}},
		{Alvo{Dona: company, Condominio: "101"}, map[string]any{"condo": "alpha", "override": "alpha"}},
		{Alvo{Dona: company, Condominio: "102"}, map[string]any{"condo": "beta"}},
		{Alvo{Dona: "Empresa B", Condominio: "101"}, map[string]any{"condo": "outra empresa"}},
	}
	for _, entry := range entries {
		if _, err := p.SalvarPayload(ctx, entry.alvo, "resource", entry.data); err != nil {
			t.Fatal(err)
		}
	}
	for _, entry := range []struct {
		alvo Alvo
		text string
	}{{Alvo{Condominio: Padrao}, "base"}, {Alvo{Dona: company, Condominio: Padrao}, "empresa"}, {Alvo{Dona: company, Condominio: "101"}, "alpha"}} {
		if _, err := p.SalvarOrientacao(ctx, entry.alvo, Orientacao{Geral: entry.text}); err != nil {
			t.Fatal(err)
		}
	}
	alpha := 101
	m, err := p.Conteudo(ctx, company, &alpha)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"base": "global", "company": "a", "condo": "alpha", "override": "alpha"}
	if !reflect.DeepEqual(m.PayloadsDe("101")["resource"], want) {
		t.Fatalf("composição: %#v", m.PayloadsDe("101"))
	}
	if got := m.OrientacaoDe("101").Geral; got != "base\n\nempresa\n\nalpha" {
		t.Fatalf("orientação: %q", got)
	}
	exact, err := p.ConteudoConfigurado(ctx, company, &alpha)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(exact.PayloadsDe("101")["resource"], entries[2].data) {
		t.Fatalf("herança vazou para edição: %#v", exact.Payloads)
	}
	if _, err := p.SalvarAjustes(ctx, Alvo{Dona: company, Condominio: "101"}, map[string]map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.SalvarPayload(ctx, Alvo{Dona: company, Condominio: "101"}, "resource", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		company string
		condo   int
		want    string
	}{{company, 102, "beta"}, {"Empresa B", 101, "outra empresa"}} {
		m, err := p.ConteudoConfigurado(ctx, entry.company, &entry.condo)
		if err != nil {
			t.Fatal(err)
		}
		if got := m.PayloadsDe("" + func() string {
			if entry.condo == 102 {
				return "102"
			}
			return "101"
		}())["resource"]["condo"]; got != entry.want {
			t.Fatalf("outro alvo alterado: %v", got)
		}
	}
	global, err := p.ConteudoConfigurado(ctx, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if global.PayloadsDe(Padrao)["resource"]["base"] != "global" {
		t.Fatal("base global alterada")
	}
}

func TestPostgresV2NaoRecriaCadastrosLocaisENaoAtivaCasoAusente(t *testing.T) {
	b := bancotest.Novo(t)
	p := NovoPostgres(b.Ent)
	if _, err := p.SalvarAdministradora(t.Context(), Administradora{ID: "A"}); !errors.Is(err, ErrCadastroExterno) {
		t.Fatal(err)
	}
	if _, err := p.SalvarCondominio(t.Context(), Condominio{ID: "101", Administradora: "A"}); !errors.Is(err, ErrCadastroExterno) {
		t.Fatal(err)
	}
	m, err := p.Conteudo(t.Context(), "A", nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Habilitado(Padrao, "caso-a", "") {
		t.Fatal("caso sem configuração ficou ativo")
	}
}

func TestOrientacaoV2PadraoSemDuplicacaoERetornoNoAlvoEditado(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	ctx := t.Context()
	casos := []struct {
		nome  string
		alvo  Alvo
		texto string
		chave string
	}{
		{"GLOBAL", Alvo{Condominio: Padrao}, "global", Padrao},
		{"COMPANY", Alvo{Dona: "empresa-a", Condominio: Padrao}, "empresa", Padrao},
		{"CONDO", Alvo{Dona: "empresa-a", Condominio: "101"}, "condomínio", "101"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			m, err := p.SalvarOrientacao(ctx, c.alvo, Orientacao{Geral: c.texto, PorCaso: map[string]string{"caso-a": c.texto}})
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Orientacoes) != 1 || m.Orientacoes[c.chave].Geral != c.texto || m.Orientacoes[c.chave].PorCaso["caso-a"] != c.texto {
				t.Fatalf("edição retornou conteúdo herdado ou de outro alvo: %#v", m.Orientacoes)
			}
			if c.chave == Padrao {
				o := m.OrientacaoDe(Padrao)
				if o.Geral != c.texto || o.PorCaso["caso-a"] != c.texto {
					t.Fatalf("padrão duplicado: %#v", o)
				}
			}
		})
	}
}
