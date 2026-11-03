package memoria

import (
	"os"
	"slices"
	"testing"
	"time"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/escopo"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func TestContratoFake(t *testing.T) {
	testarStore(t, func(*testing.T) Store { return NovoFake() })
}

func TestContratoPostgres(t *testing.T) {
	testarStore(t, func(t *testing.T) Store { return NovoPostgres(bancotest.Novo(t)) })
}

// testarStore é o contrato que toda implementação de Store cumpre.
func testarStore(t *testing.T, novo func(t *testing.T) Store) {
	t.Run("gravar e ler", func(t *testing.T) { cenarioGravarLer(t, novo(t)) })
	t.Run("esquecer uma chave", func(t *testing.T) { cenarioEsquecer(t, novo(t)) })
	t.Run("listar por administradora", func(t *testing.T) { cenarioListar(t, novo(t)) })
	t.Run("contato invalido", func(t *testing.T) { cenarioContatoInvalido(t, novo(t)) })
	t.Run("nome sem fatos", func(t *testing.T) { cenarioNomeSemFatos(t, novo(t)) })
	t.Run("chave repetida fica com a ultima", func(t *testing.T) { cenarioChaveRepetida(t, novo(t)) })
}

func fatosDeTeste() []Fato {
	return []Fato{
		{Chave: "unidades", Texto: "tem só uma unidade", Fonte: "P-1", CondominioID: "cond-1", Quando: dia, Confianca: 0.8},
		{Chave: "pets", Texto: "tem um cachorro", Fonte: "P-2", CondominioID: "cond-2", Quando: dia.Add(time.Hour)},
	}
}

func gravar(t *testing.T, st Store, c escopo.Contato, fatos []Fato) {
	t.Helper()
	gravarMemoria(t, st, c, Memoria{Fatos: fatos, AtualizadaEm: dia.Add(3 * time.Hour)})
}

func gravarMemoria(t *testing.T, st Store, c escopo.Contato, m Memoria) {
	t.Helper()
	if err := st.Gravar(t.Context(), c, m); err != nil {
		t.Fatalf("Gravar: %v", err)
	}
}

func ler(t *testing.T, st Store, c escopo.Contato) Memoria {
	t.Helper()
	m, err := st.Ler(t.Context(), c)
	if err != nil {
		t.Fatalf("Ler: %v", err)
	}
	return m
}

func mesmosFatos(a, b []Fato) bool {
	return slices.EqualFunc(a, b, func(x, y Fato) bool {
		return x.Chave == y.Chave && x.Texto == y.Texto && x.Fonte == y.Fonte &&
			x.CondominioID == y.CondominioID && x.Quando.Equal(y.Quando) && x.Confianca == y.Confianca
	})
}

func cenarioGravarLer(t *testing.T, st Store) {
	c := contato(t, "adm-a", "5519981621916")
	vazia := ler(t, st, c)
	if len(vazia.Fatos) != 0 || vazia.Fatos == nil || vazia.AdministradoraID != "adm-a" || vazia.Telefone != c.Telefone ||
		vazia.Nome != "" || !vazia.AtualizadaEm.IsZero() {
		t.Fatalf("memória nova: %+v", vazia)
	}

	gravarMemoria(t, st, c, Memoria{Nome: "Vanessa", Fatos: fatosDeTeste(), AtualizadaEm: dia.Add(3 * time.Hour)})

	m := ler(t, st, c)
	if !mesmosFatos(m.Fatos, fatosDeTeste()) {
		t.Fatalf("Ler = %+v, want %+v", m.Fatos, fatosDeTeste())
	}
	if m.Nome != "Vanessa" || !m.AtualizadaEm.Equal(dia.Add(3*time.Hour)) || m.AdministradoraID != "adm-a" || m.Telefone != c.Telefone {
		t.Fatalf("cabeçalho: %+v", m)
	}
	if outra := ler(t, st, contato(t, "adm-b", c.Telefone)); len(outra.Fatos) != 0 || outra.Nome != "" {
		t.Fatalf("outra administradora vê a memória: %+v", outra)
	}
	gravarMemoria(t, st, c, Memoria{Fatos: fatosDeTeste()})
	if m := ler(t, st, c); m.AtualizadaEm.IsZero() {
		t.Fatalf("gravar sem AtualizadaEm deveria marcar o instante: %+v", m)
	}
}

func cenarioNomeSemFatos(t *testing.T, st Store) {
	c := contato(t, "adm-a", "5519981621916")
	gravarMemoria(t, st, c, Memoria{Nome: "Van", Fatos: []Fato{}, AtualizadaEm: dia})

	if m := ler(t, st, c); m.Nome != "Van" || len(m.Fatos) != 0 || m.Fatos == nil || !m.AtualizadaEm.Equal(dia) {
		t.Fatalf("Ler = %+v", m)
	}
	l, err := st.Listar(t.Context(), "adm-a")
	if err != nil || len(l) != 1 || l[0].Nome != "Van" || l[0].Telefone != c.Telefone {
		t.Fatalf("Listar = %+v %v", l, err)
	}
}

func cenarioChaveRepetida(t *testing.T, st Store) {
	c := contato(t, "adm-a", "5519981621916")
	gravar(t, st, c, []Fato{
		{Chave: "pets", Texto: "tem um gato", Quando: dia},
		{Chave: "unidades", Texto: "uma", Quando: dia},
		{Chave: "pets", Texto: "tem um cachorro", Quando: dia.Add(time.Hour)},
	})

	want := []Fato{{Chave: "pets", Texto: "tem um cachorro", Quando: dia.Add(time.Hour)}, {Chave: "unidades", Texto: "uma", Quando: dia}}
	if m := ler(t, st, c); !mesmosFatos(m.Fatos, want) {
		t.Fatalf("Ler = %+v, want %+v", m.Fatos, want)
	}
}

func cenarioEsquecer(t *testing.T, st Store) {
	c := contato(t, "adm-a", "5519981621916")
	gravar(t, st, c, fatosDeTeste())

	m, _ := ler(t, st, c).Esquecer("unidades")
	m, _ = m.Upsert(Fato{Chave: "pets", Texto: "tem dois cachorros", Quando: dia.Add(2 * time.Hour)})
	gravar(t, st, c, m.Fatos)

	got := ler(t, st, c)
	if len(got.Fatos) != 1 || got.Fatos[0].Chave != "pets" || got.Fatos[0].Texto != "tem dois cachorros" {
		t.Fatalf("depois de esquecer: %+v", got.Fatos)
	}
	gravarMemoria(t, st, c, Memoria{Nome: "Vanessa", Fatos: []Fato{}, AtualizadaEm: dia})
	if got := ler(t, st, c); len(got.Fatos) != 0 || got.Nome != "Vanessa" {
		t.Fatalf("esquecer tudo: %+v", got)
	}
}

func cenarioListar(t *testing.T, st Store) {
	antigo, recente := contato(t, "adm-a", "5511911111111"), contato(t, "adm-a", "5511922222222")
	gravarMemoria(t, st, antigo, Memoria{Fatos: fatosDeTeste()[:1], AtualizadaEm: dia})
	gravarMemoria(t, st, recente, Memoria{Nome: "Rê", Fatos: fatosDeTeste(), AtualizadaEm: dia.Add(time.Hour)})
	gravar(t, st, contato(t, "adm-b", "5511933333333"), fatosDeTeste())

	l, err := st.Listar(t.Context(), "adm-a")
	if err != nil {
		t.Fatalf("Listar: %v", err)
	}
	telefones := []string{}
	for _, m := range l {
		telefones = append(telefones, m.AdministradoraID+":"+m.Telefone)
	}
	want := []string{"adm-a:" + recente.Telefone, "adm-a:" + antigo.Telefone}
	if !slices.Equal(telefones, want) {
		t.Fatalf("Listar = %v, want %v (mais recente primeiro, só da administradora)", telefones, want)
	}
	if !mesmosFatos(l[0].Fatos, fatosDeTeste()) || l[0].Nome != "Rê" {
		t.Fatalf("fatos listados: %+v", l[0].Fatos)
	}
	if vazia, err := st.Listar(t.Context(), "adm-c"); err != nil || len(vazia) != 0 || vazia == nil {
		t.Fatalf("Listar sem memórias = %v %v", vazia, err)
	}
}

func cenarioContatoInvalido(t *testing.T, st Store) {
	if _, err := st.Ler(t.Context(), escopo.Contato{}); err == nil {
		t.Fatal("Ler de contato vazio deveria falhar")
	}
	if err := st.Gravar(t.Context(), escopo.Contato{AdministradoraID: "adm-a"}, Memoria{}); err == nil {
		t.Fatal("Gravar sem telefone deveria falhar")
	}
	if _, err := st.Listar(t.Context(), "../x"); err == nil {
		t.Fatal("Listar de administradora inválida deveria falhar")
	}
}
