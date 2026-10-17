package admin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapeamento(t *testing.T) {
	st := NewStore(filepath.Join(t.TempDir(), "mapeamento.json"))
	if _, err := st.SalvarCondominio(Condominio{ID: "cond-x", Nome: "Residencial X", Account: "cond-001", CompanyID: "5982", Tenant: "t1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SalvarCondominio(Condominio{ID: "Cond Y", Nome: "Y"}); err == nil {
		t.Fatal("id inválido deveria falhar")
	}
	// padrão: reserva desabilitada para todos; X reabilita só para proprietário
	if _, err := st.SalvarHabilitacao(Padrao, "reserva_area_comum", Habilitacao{Habilitado: false}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SalvarHabilitacao("cond-x", "reserva_area_comum", Habilitacao{Habilitado: true, Papeis: []string{"proprietario"}}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SalvarHabilitacao("cond-z", "x", Habilitacao{}, false); err == nil {
		t.Fatal("condomínio desconhecido deveria falhar")
	}
	// payloads: padrão e sobreposição do condomínio
	st.SalvarPayload(Padrao, "areas.reservar", map[string]any{"antecedencia_min_dias": 2, "max_dias": 60})
	st.SalvarPayload("cond-x", "areas.reservar", map[string]any{"max_dias": 30, "areas": []any{"salão", "churrasqueira"}})
	st.SalvarPayload("cond-x", "unidades.listar", map[string]any{"company_id": "5982"})

	m, err := st.Ler()
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := m.Resolver("5982", "", ""); !ok || c.ID != "cond-x" {
		t.Fatalf("resolver por company: %+v %v", c, ok)
	}
	if c, ok := m.Resolver("", "t1", ""); !ok || c.ID != "cond-x" {
		t.Fatal("resolver por tenant")
	}
	if _, ok := m.Resolver("999", "nao", "nao"); ok {
		t.Fatal("sem casar deveria ser false")
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
	if h.Habilitado || de != Padrao {
		t.Fatalf("efetiva: %+v %s", h, de)
	}
	p := m.PayloadsDe("cond-x")
	if p["areas.reservar"]["antecedencia_min_dias"] != float64(2) || p["areas.reservar"]["max_dias"] != float64(30) || p["unidades.listar"]["company_id"] != "5982" {
		t.Fatalf("payload remontado: %+v", p)
	}
	ctx := Contexto(Condominio{Nome: "Residencial X"}, p)
	if !strings.Contains(ctx, "Contexto do condomínio (Residencial X)") || !strings.Contains(ctx, "`areas.reservar`") {
		t.Fatalf("contexto:\n%s", ctx)
	}
	if _, err := st.RemoverCondominio("cond-x"); err != nil {
		t.Fatal(err)
	}
	m, _ = st.Ler()
	if len(m.Condominios) != 0 || m.Payloads["cond-x"] != nil || m.Habilitacoes["cond-x"] != nil {
		t.Fatal("remover deveria limpar tudo do condomínio")
	}
	if !m.Habilitado("cond-x", "liberacao_visitante", "") {
		t.Fatal("padrão continua")
	}
}

func TestAdministradorasEVisao(t *testing.T) {
	st := NewStore(filepath.Join(t.TempDir(), "mapeamento.json"))
	if _, err := st.SalvarAdministradora(Administradora{ID: "adm-a", Nome: "Alfa"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SalvarAdministradora(Administradora{ID: "adm-b", Nome: "Beta"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SalvarCondominio(Condominio{ID: "c1", Nome: "C1", Administradora: "adm-a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SalvarCondominio(Condominio{ID: "c2", Nome: "C2", Administradora: "adm-b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SalvarCondominio(Condominio{ID: "c3", Nome: "C3", Administradora: "adm-zzz"}); err == nil {
		t.Fatal("administradora desconhecida deveria falhar")
	}
	st.SalvarHabilitacao(Padrao, "faq", Habilitacao{Habilitado: false}, false)
	st.SalvarHabilitacao("c2", "faq", Habilitacao{Habilitado: true}, false)
	st.SalvarPayload("c1", "unidades.listar", map[string]any{"x": 1})
	m, _ := st.Ler()
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
	if _, err := st.RemoverAdministradora("adm-a"); err == nil {
		t.Fatal("remover administradora com condomínio deveria falhar")
	}
	st.SalvarCondominio(Condominio{ID: "c1", Nome: "C1", Administradora: "adm-b"})
	if _, err := st.RemoverAdministradora("adm-a"); err != nil {
		t.Fatal(err)
	}
	// legado: arquivo com "administradora" string vira a primeira da lista
	legado := NewStore(filepath.Join(t.TempDir(), "m.json"))
	os.WriteFile(legado.Path, []byte(`{"administradora":"Antiga","condominios":[{"id":"c9","nome":"C9"}]}`), 0o644)
	ml, err := legado.Ler()
	if err != nil {
		t.Fatal(err)
	}
	if len(ml.Administradoras) != 1 || ml.Administradoras[0].Nome != "Antiga" || ml.Condominios[0].Administradora != "adm-1" {
		t.Fatalf("migração do legado: %+v", ml)
	}
}

func TestOrientacoesEAjustes(t *testing.T) {
	st := NewStore(filepath.Join(t.TempDir(), "m.json"))
	st.SalvarCondominio(Condominio{ID: "c1", Nome: "C1"})
	if _, err := st.SalvarOrientacao(Padrao, Orientacao{Geral: "Trate todos por você."}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SalvarOrientacao("c1", Orientacao{Geral: "A piscina fecha às 22h.", PorCaso: map[string]string{"reserva-area-comum": "Churrasqueira só aos fins de semana.", "faq": "  "}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SalvarOrientacao("zz", Orientacao{Geral: "x"}); err == nil {
		t.Fatal("condomínio desconhecido deveria falhar")
	}
	st.SalvarPayload("c1", "areas.reservar", map[string]any{"do_backdoor": true, "max_dias": 60})
	if _, err := st.SalvarAjustes("c1", map[string]map[string]any{"areas.reservar": {"antecedencia_min_dias": 2, "max_dias": ""}, "unidades.listar": {"company_id": "5982"}}); err != nil {
		t.Fatal(err)
	}
	m, _ := st.Ler()
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
	ctx := ContextoDoCondominio(Condominio{Nome: "C1"}, p, o.Geral)
	if !strings.Contains(ctx, "## Orientações da administradora") || !strings.Contains(ctx, "22h") || !strings.Contains(ctx, "areas.reservar") {
		t.Fatalf("contexto: %s", ctx)
	}
	if _, err := st.RemoverCondominio("c1"); err != nil {
		t.Fatal(err)
	}
	m, _ = st.Ler()
	if _, tem := m.Orientacoes["c1"]; tem {
		t.Fatal("remover o condomínio leva a orientação junto")
	}
}
