package admin

import (
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
