package admin

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/escopo"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func TestContratoMemoria(t *testing.T) {
	testarRepositorio(t, func(*testing.T) Repositorio { return NovaEmMemoria() })
}

func TestContratoArquivo(t *testing.T) {
	testarRepositorio(t, func(t *testing.T) Repositorio {
		return NovoArquivo(filepath.Join(t.TempDir(), "mapeamento.json"))
	})
}

func TestContratoPostgres(t *testing.T) {
	testarRepositorio(t, func(t *testing.T) Repositorio { return NovoPostgres(bancotest.Novo(t).Ent) })
}

// testarRepositorio é o contrato que toda implementação de Repositorio cumpre.
func testarRepositorio(t *testing.T, novo func(t *testing.T) Repositorio) {
	t.Run("habilitacoes e payloads", func(t *testing.T) { cenarioMapeamento(t, novo(t)) })
	t.Run("administradoras e visao", func(t *testing.T) { cenarioAdministradoras(t, novo(t)) })
	t.Run("orientacoes e ajustes", func(t *testing.T) { cenarioOrientacoes(t, novo(t)) })
	t.Run("padrao regravado", func(t *testing.T) { cenarioPadraoRegravado(t, novo(t)) })
	t.Run("ordem", func(t *testing.T) { cenarioOrdem(t, novo(t)) })
	t.Run("escritas simultaneas", func(t *testing.T) { cenarioEscritasSimultaneas(t, novo(t)) })
}

// cenarioOrdem: administradoras por id; condomínios por nome e, no empate, por id. ResolverCondominio
// devolve o primeiro que casa, então a ordem decide o condomínio.
func cenarioOrdem(t *testing.T, r Repositorio) {
	ctx := t.Context()
	ok := exigir(t)
	ok(r.SalvarAdministradora(ctx, Administradora{ID: "adm-z", Nome: "Aaa"}))
	ok(r.SalvarAdministradora(ctx, Administradora{ID: "adm-a", Nome: "Zzz"}))
	for _, c := range []Condominio{{ID: "c-z", Nome: "Mesmo", Administradora: "adm-a", Account: "acc", CompanyID: "1", Tenant: "t"}, {ID: "c-a", Nome: "Mesmo", Administradora: "adm-a", Account: "acc", CompanyID: "2", Tenant: "t"}, {ID: "c-m", Nome: "Alfa"}} {
		ok(r.SalvarCondominio(ctx, c))
	}
	m := ok(r.Ler(ctx))
	adms := []string{}
	for _, a := range m.Administradoras {
		adms = append(adms, a.ID)
	}
	condos := []string{}
	for _, c := range m.Condominios {
		condos = append(condos, c.ID)
	}
	if !slices.Equal(adms, []string{"adm-a", "adm-z"}) || !slices.Equal(condos, []string{"c-m", "c-a", "c-z"}) {
		t.Fatalf("ordem: administradoras %v, condomínios %v", adms, condos)
	}
	if c, err := ResolverCondominio(m, escopo.Implantacao{}, "", "t"); err != nil || c.ID != "c-a" {
		t.Fatalf("empate de tenant na mesma administradora resolve pelo menor id: %s %v", c.ID, err)
	}
}

// cenarioEscritasSimultaneas: duas escritas ao mesmo tempo em condomínios diferentes passam
// as duas (no Postgres, o conflito de serialização é refeito).
func cenarioEscritasSimultaneas(t *testing.T, r Repositorio) {
	ctx := t.Context()
	ok := exigir(t)
	condos := []string{"c1", "c2"}
	for _, id := range condos {
		ok(r.SalvarCondominio(ctx, Condominio{ID: id, Nome: strings.ToUpper(id)}))
	}
	const rodadas = 3
	for rodada := range rodadas {
		erros := make(chan error, len(condos))
		var wg sync.WaitGroup
		for _, id := range condos {
			wg.Go(func() {
				_, err := r.SalvarAjustes(ctx, id, map[string]map[string]any{"areas.reservar": {"rodada": rodada}})
				erros <- err
			})
		}
		wg.Wait()
		close(erros)
		for err := range erros {
			if err != nil {
				t.Fatalf("rodada %d: %v", rodada, err)
			}
		}
	}
	m := ok(r.Ler(ctx))
	for _, id := range condos {
		if m.Payloads[id]["areas.reservar"]["rodada"] != float64(rodadas-1) {
			t.Fatalf("escrita de %s perdida: %+v", id, m.Payloads)
		}
	}
}

// exigir falha o teste se a operação deu erro e devolve o mapeamento.
func exigir(t *testing.T) func(Mapeamento, error) Mapeamento {
	return func(m Mapeamento, err error) Mapeamento {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
}

// recusar falha o teste se a operação não deu ErrInvalido.
func recusar(t *testing.T, motivo string) func(Mapeamento, error) {
	return func(_ Mapeamento, err error) {
		t.Helper()
		if !errors.Is(err, ErrInvalido) {
			t.Fatalf("%s: quero ErrInvalido, veio %v", motivo, err)
		}
	}
}

func cenarioMapeamento(t *testing.T, r Repositorio) {
	ctx := t.Context()
	ok, nao := exigir(t), func(m string) func(Mapeamento, error) { return recusar(t, m) }
	ok(r.SalvarCondominio(ctx, Condominio{ID: "cond-x", Nome: "Residencial X", Account: "cond-001", CompanyID: "5982", Tenant: "t1"}))
	nao("id inválido")(r.SalvarCondominio(ctx, Condominio{ID: "Cond Y", Nome: "Y"}))
	// padrão: reserva desabilitada para todos; X reabilita só para proprietário
	ok(r.SalvarHabilitacao(ctx, Padrao, "reserva_area_comum", Habilitacao{Habilitado: false}, false))
	ok(r.SalvarHabilitacao(ctx, "cond-x", "reserva_area_comum", Habilitacao{Habilitado: true, Papeis: []string{"proprietario"}}, false))
	nao("condomínio desconhecido")(r.SalvarHabilitacao(ctx, "cond-z", "x", Habilitacao{}, false))
	// payloads: padrão e sobreposição do condomínio
	ok(r.SalvarPayload(ctx, Padrao, "areas.reservar", map[string]any{"antecedencia_min_dias": 2, "max_dias": 60}))
	ok(r.SalvarPayload(ctx, "cond-x", "areas.reservar", map[string]any{"max_dias": 30, "areas": []any{"salão", "churrasqueira"}}))
	ok(r.SalvarPayload(ctx, "cond-x", "unidades.listar", map[string]any{"company_id": "5982"}))

	m := ok(r.Ler(ctx))
	if c, achou := m.Condominio("cond-x"); !achou || c.Account != "cond-001" || c.CompanyID != "5982" || c.Tenant != "t1" {
		t.Fatalf("condomínio gravado: %+v %v", c, achou)
	}
	if !m.Habilitado("cond-x", "liberacao_visitante", "morador") {
		t.Fatal("sem entrada = habilitado")
	}
	if m.Habilitado("cond-outro", "reserva_area_comum", "morador") {
		t.Fatal("padrão desabilitou")
	}
	if m.Habilitado("cond-x", "reserva_area_comum", "morador") || !m.Habilitado("cond-x", "reserva_area_comum", "proprietario") {
		t.Fatal("papel do condomínio")
	}
	h, de := m.HabilitacaoEfetiva("cond-outro", "reserva_area_comum")
	if h.Habilitado || de != Padrao || h.Papeis == nil {
		t.Fatalf("efetiva: %+v %s", h, de)
	}
	p := m.PayloadsDe("cond-x")
	if p["areas.reservar"]["antecedencia_min_dias"] != float64(2) || p["areas.reservar"]["max_dias"] != float64(30) || p["unidades.listar"]["company_id"] != "5982" {
		t.Fatalf("payload remontado: %+v", p)
	}
	ctxPrompt := Contexto(Condominio{Nome: "Residencial X"}, p)
	if !strings.Contains(ctxPrompt, "Contexto do condomínio (Residencial X)") || !strings.Contains(ctxPrompt, "`areas.reservar`") {
		t.Fatalf("contexto:\n%s", ctxPrompt)
	}
	ok(r.SalvarHabilitacao(ctx, "cond-x", "reserva_area_comum", Habilitacao{}, true))
	if h, de := ok(r.Ler(ctx)).HabilitacaoEfetiva("cond-x", "reserva_area_comum"); de != Padrao || h.Habilitado {
		t.Fatalf("limpar devolve ao padrão: %+v %s", h, de)
	}
	ok(r.SalvarPayload(ctx, "cond-x", "unidades.listar", nil))
	if _, tem := ok(r.Ler(ctx)).Payloads["cond-x"]["unidades.listar"]; tem {
		t.Fatal("payload nil apaga o recurso")
	}
	ok(r.RemoverCondominio(ctx, "cond-x"))
	m = ok(r.Ler(ctx))
	if len(m.Condominios) != 0 || m.Payloads["cond-x"] != nil || m.Habilitacoes["cond-x"] != nil {
		t.Fatal("remover deveria limpar tudo do condomínio")
	}
	if !m.Habilitado("cond-x", "liberacao_visitante", "") || m.Payloads[Padrao]["areas.reservar"] == nil {
		t.Fatal("padrão continua")
	}
}

func cenarioAdministradoras(t *testing.T, r Repositorio) {
	ctx := t.Context()
	ok, nao := exigir(t), func(m string) func(Mapeamento, error) { return recusar(t, m) }
	ok(r.SalvarAdministradora(ctx, Administradora{ID: "adm-b", Nome: "Beta"}))
	ok(r.SalvarAdministradora(ctx, Administradora{ID: " ADM-A ", Nome: " Alfa "}))
	nao("administradora sem nome")(r.SalvarAdministradora(ctx, Administradora{ID: "adm-c"}))
	ok(r.SalvarCondominio(ctx, Condominio{ID: "c1", Nome: "C1", Administradora: "adm-a"}))
	ok(r.SalvarCondominio(ctx, Condominio{ID: "c2", Nome: "C2", Administradora: "adm-b"}))
	nao("administradora desconhecida")(r.SalvarCondominio(ctx, Condominio{ID: "c3", Nome: "C3", Administradora: "adm-zzz"}))
	ok(r.SalvarHabilitacao(ctx, Padrao, "faq", Habilitacao{Habilitado: false}, false))
	ok(r.SalvarHabilitacao(ctx, "c2", "faq", Habilitacao{Habilitado: true}, false))
	ok(r.SalvarPayload(ctx, "c1", "unidades.listar", map[string]any{"x": 1}))

	m := ok(r.Ler(ctx))
	if !slices.Equal(m.Administradoras, []Administradora{{ID: "adm-a", Nome: "Alfa"}, {ID: "adm-b", Nome: "Beta"}}) {
		t.Fatalf("administradoras normalizadas: %+v", m.Administradoras)
	}
	v := m.Visao("adm-a")
	if len(v.Administradoras) != 1 || v.Administradoras[0].ID != "adm-a" || len(v.Condominios) != 1 || v.Condominios[0].ID != "c1" {
		t.Fatalf("visão de adm-a: %+v", v)
	}
	if _, tem := v.Habilitacoes["c2"]; tem {
		t.Fatal("a visão de adm-a não pode ter o mapeamento de c2")
	}
	if _, tem := v.Habilitacoes[Padrao]; !tem {
		t.Fatal("o padrão é global e aparece na visão")
	}
	if _, tem := v.Payloads["c1"]; !tem {
		t.Fatal("o payload do próprio condomínio aparece")
	}
	nao("remover administradora com condomínio")(r.RemoverAdministradora(ctx, "adm-a"))
	ok(r.SalvarCondominio(ctx, Condominio{ID: "c1", Nome: "C1", Administradora: "adm-b"}))
	m = ok(r.RemoverAdministradora(ctx, "adm-a"))
	if _, tem := m.Administradora("adm-a"); tem || len(m.Administradoras) != 1 {
		t.Fatalf("adm-a removida: %+v", m.Administradoras)
	}
	if c, _ := m.Condominio("c1"); c.Administradora != "adm-b" {
		t.Fatalf("c1 mudou de dona: %+v", c)
	}
	m = ok(r.SalvarCondominio(ctx, Condominio{ID: "c2", Nome: "C2"}))
	if c, _ := m.Condominio("c2"); c.Administradora != "" {
		t.Fatalf("c2 ficou sem dona: %+v", c)
	}
}

func cenarioOrientacoes(t *testing.T, r Repositorio) {
	ctx := t.Context()
	ok, nao := exigir(t), func(m string) func(Mapeamento, error) { return recusar(t, m) }
	ok(r.SalvarCondominio(ctx, Condominio{ID: "c1", Nome: "C1"}))
	ok(r.SalvarOrientacao(ctx, Padrao, Orientacao{Geral: "Trate todos por você."}))
	ok(r.SalvarOrientacao(ctx, "c1", Orientacao{Geral: "A piscina fecha às 22h.", PorCaso: map[string]string{"reserva-area-comum": "Churrasqueira só aos fins de semana.", "faq": "  "}}))
	nao("condomínio desconhecido")(r.SalvarOrientacao(ctx, "zz", Orientacao{Geral: "x"}))
	nao("ajuste sem recurso")(r.SalvarAjustes(ctx, "c1", map[string]map[string]any{"": {"x": 1}}))
	ok(r.SalvarPayload(ctx, "c1", "areas.reservar", map[string]any{"do_backdoor": true, "max_dias": 60}))
	ok(r.SalvarAjustes(ctx, "c1", map[string]map[string]any{"areas.reservar": {"antecedencia_min_dias": 2, "max_dias": ""}, "unidades.listar": {"company_id": "5982"}}))

	m := ok(r.Ler(ctx))
	o := m.OrientacaoDe("c1")
	if !strings.HasPrefix(o.Geral, "Trate todos") || !strings.HasSuffix(o.Geral, "22h.") {
		t.Fatalf("geral junta padrão e condomínio: %q", o.Geral)
	}
	if o.PorCaso["reserva-area-comum"] == "" || len(o.PorCaso) != 1 {
		t.Fatalf("por caso: %+v (texto vazio não entra)", o.PorCaso)
	}
	p := m.PayloadsDe("c1")
	if p["areas.reservar"]["do_backdoor"] != true || p["areas.reservar"]["antecedencia_min_dias"] != float64(2) || p["areas.reservar"]["max_dias"] != nil || p["unidades.listar"]["company_id"] != "5982" {
		t.Fatalf("ajustes mesclam chave a chave e vazio apaga: %+v", p)
	}
	ctxPrompt := ContextoDoCondominio(Condominio{Nome: "C1"}, p, o.Geral)
	if !strings.Contains(ctxPrompt, "## Orientações da administradora") || !strings.Contains(ctxPrompt, "22h") || !strings.Contains(ctxPrompt, "areas.reservar") {
		t.Fatalf("contexto: %s", ctxPrompt)
	}
	ok(r.SalvarAjustes(ctx, "c1", map[string]map[string]any{"unidades.listar": {"company_id": nil}}))
	if _, tem := ok(r.Ler(ctx)).Payloads["c1"]["unidades.listar"]; tem {
		t.Fatal("recurso sem chaves sai do payload")
	}
	ok(r.SalvarOrientacao(ctx, Padrao, Orientacao{Geral: "  "}))
	if _, tem := ok(r.Ler(ctx)).Orientacoes[Padrao]; tem {
		t.Fatal("orientação vazia apaga")
	}
	ok(r.RemoverCondominio(ctx, "c1"))
	if _, tem := ok(r.Ler(ctx)).Orientacoes["c1"]; tem {
		t.Fatal("remover o condomínio leva a orientação junto")
	}
}

// cenarioPadraoRegravado: o padrão ("*") regravado substitui a entrada, sem duplicar.
func cenarioPadraoRegravado(t *testing.T, r Repositorio) {
	ctx := t.Context()
	ok := exigir(t)
	for _, habilitado := range []bool{false, true} {
		ok(r.SalvarHabilitacao(ctx, Padrao, "faq", Habilitacao{Habilitado: habilitado, Papeis: []string{"sindico", "morador"}}, false))
		ok(r.SalvarPayload(ctx, Padrao, "areas.reservar", map[string]any{"max_dias": 10}))
		ok(r.SalvarOrientacao(ctx, Padrao, Orientacao{Geral: "Olá."}))
	}
	ok(r.SalvarAjustes(ctx, Padrao, map[string]map[string]any{"areas.reservar": {"max_dias": 20}}))
	m := ok(r.Ler(ctx))
	h := m.Habilitacoes[Padrao]["faq"]
	if !h.Habilitado || !slices.Equal(h.Papeis, []string{"sindico", "morador"}) {
		t.Fatalf("habilitação do padrão: %+v", h)
	}
	if m.Payloads[Padrao]["areas.reservar"]["max_dias"] != float64(20) || m.OrientacaoDe("qualquer").Geral != "Olá." {
		t.Fatalf("padrão regravado: %+v %+v", m.Payloads, m.Orientacoes)
	}
}
