package regras

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/ganchos"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func novaRegra(nome string, ativa bool) RegraGravada {
	return RegraGravada{
		Nome:             nome,
		Evento:           ganchos.EventoMensagemRecebida,
		AdministradoraID: "adm-1",
		Condominios:      []string{"cond-1"},
		Ativa:            ativa,
		Definicao:        json.RawMessage(`{"nome":"` + nome + `"}`),
		AlteradaPor:      "ana",
	}
}

func salvar(t *testing.T, p *Postgres, r RegraGravada, versaoLida int) RegraGravada {
	t.Helper()
	salva, err := p.Salvar(t.Context(), r, versaoLida)
	if err != nil {
		t.Fatalf("Salvar %q (versão lida %d): %v", r.Nome, versaoLida, err)
	}
	return salva
}

func TestSalvarCriaRegraNaVersaoUm(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	salva := salvar(t, p, novaRegra("horario", true), 0)
	if salva.ID == "" || salva.Versao != 1 || salva.AlteradaEm.IsZero() {
		t.Fatalf("regra salva = %+v, quer id, versão 1 e data", salva)
	}
	lida, err := p.Ler(t.Context(), salva.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lida.Nome != "horario" || lida.Versao != 1 || !jsonIgual(t, lida.Definicao, salva.Definicao) {
		t.Fatalf("Ler = %+v", lida)
	}
	if len(lida.Condominios) != 1 || lida.Condominios[0] != "cond-1" || !lida.AlteradaEm.Equal(salva.AlteradaEm) {
		t.Fatalf("Ler = %+v, salva = %+v", lida, salva)
	}
}

func TestSalvarComVersaoCertaSobeAVersaoEGuardaAAnterior(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	v1 := salvar(t, p, novaRegra("horario", true), 0)
	alterada := v1
	alterada.Definicao = json.RawMessage(`{"nome":"outra"}`)
	v2 := salvar(t, p, alterada, v1.Versao)
	if v2.Versao != 2 || v2.ID != v1.ID {
		t.Fatalf("segunda gravação = %+v, quer versão 2 do mesmo id", v2)
	}
	antiga, err := p.LerVersao(t.Context(), v1.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonIgual(t, antiga.Definicao, v1.Definicao) || antiga.Versao != 1 {
		t.Fatalf("versão 1 = %+v, quer a definição original", antiga)
	}
	nova, err := p.LerVersao(t.Context(), v1.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonIgual(t, nova.Definicao, alterada.Definicao) {
		t.Fatalf("versão 2 = %s, quer %s", nova.Definicao, alterada.Definicao)
	}
}

func TestSalvarComVersaoVelhaDevolveErrVersaoVelhaSemAvisar(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	v1 := salvar(t, p, novaRegra("horario", true), 0)
	salvar(t, p, v1, v1.Versao)
	avisos := 0
	p.aoAlterar = func([]RegraGravada) { avisos++ }
	_, err := p.Salvar(t.Context(), v1, v1.Versao)
	if !errors.Is(err, ErrVersaoVelha) {
		t.Fatalf("Salvar com versão velha = %v, quer ErrVersaoVelha", err)
	}
	if avisos != 0 {
		t.Fatalf("aoAlterar chamado %d vezes numa gravação desfeita", avisos)
	}
	if atual, _ := p.Ler(t.Context(), v1.ID); atual.Versao != 2 {
		t.Fatalf("versão no banco = %d, quer 2", atual.Versao)
	}
}

func TestAtivasNaoTrazDesativada(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	ativa := salvar(t, p, novaRegra("ativa", true), 0)
	salvar(t, p, novaRegra("desligada", false), 0)
	ativas, err := p.Ativas(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(ativas) != 1 || ativas[0].ID != ativa.ID {
		t.Fatalf("Ativas = %+v, quer só %q", ativas, ativa.ID)
	}
	todas, err := p.Listar(t.Context(), "", limiteMaximo)
	if err != nil {
		t.Fatal(err)
	}
	if len(todas) != 2 {
		t.Fatalf("Listar trouxe %d regras, quer 2", len(todas))
	}
}

func TestLeituraDeRegraInexistenteDevolveErrRegraDesconhecida(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	existente := salvar(t, p, novaRegra("horario", true), 0)
	tests := []struct {
		name string
		ler  func() error
	}{
		{"id sem regra", func() error { _, err := p.Ler(t.Context(), "6f1c2a3b-0000-4000-8000-000000000000"); return err }},
		{"id que não é uuid", func() error { _, err := p.Ler(t.Context(), "abc"); return err }},
		{"versão que não existe", func() error { _, err := p.LerVersao(t.Context(), existente.ID, 9); return err }},
		{"alterar id que não é uuid", func() error { _, err := p.Salvar(t.Context(), novaRegra("x", true), 1); return err }},
		{"alterar id sem regra", func() error {
			r := novaRegra("x", true)
			r.ID = "6f1c2a3b-0000-4000-8000-000000000000"
			_, err := p.Salvar(t.Context(), r, 1)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ler(); !errors.Is(err, ErrRegraDesconhecida) {
				t.Fatalf("erro = %v, quer ErrRegraDesconhecida", err)
			}
		})
	}
}

func TestListarPaginaPorId(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	for i := range 3 {
		salvar(t, p, novaRegra(fmt.Sprintf("regra %d", i), i%2 == 0), 0)
	}
	todas, err := p.Listar(t.Context(), "", limiteMaximo)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		depois string
		limite int
		quer   []RegraGravada
	}{
		{"primeira página", "", 2, todas[:2]},
		{"página seguinte", todas[1].ID, 2, todas[2:]},
		{"depois da última", todas[2].ID, 2, nil},
		{"limite abaixo de 1 vira 1", "", 0, todas[:1]},
		{"limite acima de 200 vira 200", "", 500, todas},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pagina, err := p.Listar(t.Context(), tt.depois, tt.limite)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ids(pagina), ids(tt.quer)) {
				t.Fatalf("página = %v, quer %v", ids(pagina), ids(tt.quer))
			}
		})
	}
}

func ids(rs []RegraGravada) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

func jsonIgual(t *testing.T, a, b json.RawMessage) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	ja, _ := json.Marshal(x)
	jb, _ := json.Marshal(y)
	return string(ja) == string(jb)
}

func TestSalvarVariasGravaTudoEAvisaUmaVez(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	existente := salvar(t, p, novaRegra("existente", true), 0)
	var avisos [][]RegraGravada
	p.aoAlterar = func(rs []RegraGravada) { avisos = append(avisos, rs) }
	importada := novaRegra("importada", true)
	importada.ID = "7d3c3f9e-0f5b-4c7a-9a43-2f1c1d0e5b11"
	salvas, err := p.SalvarVarias(t.Context(), []RegraGravada{existente, importada}, []int{existente.Versao, 0})
	if err != nil {
		t.Fatal(err)
	}
	if salvas[0].Versao != 2 || salvas[1].Versao != 1 || salvas[1].ID != importada.ID {
		t.Fatalf("salvas = %+v, quer existente na versão 2 e importada na 1 com o id dela", salvas)
	}
	if len(avisos) != 1 || len(avisos[0]) != 2 {
		t.Fatalf("avisos = %d, quer 1 com as 2 regras", len(avisos))
	}
	versoes, err := p.VersoesAtuais(t.Context(), []string{existente.ID, importada.ID, "00000000-0000-0000-0000-000000000001"})
	if err != nil {
		t.Fatal(err)
	}
	if len(versoes) != 2 || versoes[existente.ID] != 2 || versoes[importada.ID] != 1 {
		t.Fatalf("VersoesAtuais = %v", versoes)
	}
}

func TestSalvarVariasComUmaVersaoVelhaNaoGravaNenhuma(t *testing.T) {
	p := NovoPostgres(bancotest.Novo(t).Ent)
	v1 := salvar(t, p, novaRegra("horario", true), 0)
	salvar(t, p, v1, v1.Versao)
	avisos := 0
	p.aoAlterar = func([]RegraGravada) { avisos++ }
	_, err := p.SalvarVarias(t.Context(), []RegraGravada{novaRegra("nova", true), v1}, []int{0, v1.Versao})
	if !errors.Is(err, ErrVersaoVelha) {
		t.Fatalf("SalvarVarias = %v, quer ErrVersaoVelha", err)
	}
	todas, err := p.Listar(t.Context(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(todas) != 1 || avisos != 0 {
		t.Fatalf("regras no banco = %d e avisos = %d, quer 1 e 0 (nada da transação)", len(todas), avisos)
	}
}
