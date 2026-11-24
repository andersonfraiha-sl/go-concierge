package pedidos

import (
	"testing"
	"time"

	"go-concierge/internal/escopo"
)

func TestV2PedidosPreservamRecortes(t *testing.T) {
	for _, implementation := range []string{"fake", "postgres", "cache"} {
		t.Run(implementation, func(t *testing.T) {
			var store Store = NovoFake()
			if implementation == "postgres" {
				store = NovoPostgres(bancoDeTeste(t))
			}
			if implementation == "cache" {
				store = NovoCache(NovoPostgres(bancoDeTeste(t)))
			}
			alpha, beta := 101, 202
			contact := func(company, phone string, condo *int) escopo.Contato {
				c, err := escopo.NovoContato(company, condo, phone)
				if err != nil {
					t.Fatal(err)
				}
				return c
			}
			amplo := contact("adm-a", "11999998888", nil)
			restrito := contact(amplo.CompanyID, amplo.Telefone, &alpha)
			terceiro := contact(amplo.CompanyID, "11888887777", nil)
			outra := contact("adm-b", amplo.Telefone, nil)
			request := func(c escopo.Contato, protocol string, condo *int) *Pedido {
				p := pedidoDeTeste(c, protocol, time.Now().UTC())
				p.CondoID = condo
				p.CondominioID = ""
				return p
			}
			n, a, b := request(amplo, "P-NULL", nil), request(amplo, "P-ALPHA", &alpha), request(amplo, "P-BETA", &beta)
			gravarCarteira(t, store, amplo, n, a, b)
			gravarCarteira(t, store, terceiro, request(terceiro, "P-USER", nil))
			gravarCarteira(t, store, outra, request(outra, "P-COMPANY", nil))
			assert := func(c escopo.Contato, want ...string) {
				t.Helper()
				cart := lerCarteira(t, store, c)
				if len(cart.Pedidos) != len(want) {
					t.Fatalf("recorte %v: protocolos=%v, want %v", c.CondoID, protocolos(cart), want)
				}
				for _, id := range want {
					if cart.Por(id) == nil {
						t.Fatalf("%s ausente", id)
					}
				}
			}
			assert(restrito, "P-ALPHA")
			assert(amplo, "P-NULL", "P-ALPHA", "P-BETA")
			a.Resumo = "atualizado"
			gravarCarteira(t, store, amplo, a)
			assert(amplo, "P-ALPHA", "P-BETA")
			for _, forbidden := range []*Pedido{n, b} {
				if err := store.Gravar(t.Context(), restrito, Carteira{Pedidos: []*Pedido{forbidden}}); err == nil {
					t.Fatal("pedido fora do alvo aceito")
				}
			}
			gravarCarteira(t, store, restrito)
			assert(restrito)
			assert(amplo, "P-BETA")
			gravarCarteira(t, store, amplo)
			assert(amplo, "P-BETA")
			assert(terceiro, "P-USER")
			assert(outra, "P-COMPANY")
			gravarCarteira(t, store, restrito, request(restrito, "P-NOVO", &alpha))
			assert(amplo, "P-BETA", "P-NOVO")
		})
	}
}

func TestV2PedidoNaoCriaAtivacaoEmpresarial(t *testing.T) {
	b := bancoDeTeste(t)
	store := NovoPostgres(b)
	c := contatoDeTeste("nao-ativada", "5511999999999")
	p := pedidoDeTeste(c, "P-NAO-ATIVADO", diaDeTeste)
	if err := store.Gravar(t.Context(), c, Carteira{Pedidos: []*Pedido{p}}); err == nil {
		t.Fatal("caso sem ativação aceito")
	}
	var count int
	if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM company_use_cases WHERE company_id='nao-ativada'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("ativações=%d (%v)", count, err)
	}
	if got := lerCarteira(t, store, c); len(got.Pedidos) != 0 {
		t.Fatal("pedido inválido foi gravado")
	}
}

func TestV2PedidoRecusaSnapshotIncompativel(t *testing.T) {
	b := bancoDeTeste(t)
	store := NovoPostgres(b)
	contact := contatoDeTeste("adm-a", "5511999999999")
	p := pedidoDeTeste(contact, "P-SNAPSHOT", diaDeTeste)
	gravarCarteira(t, store, contact, p)
	var original []byte
	if err := b.Pool.QueryRow(t.Context(), `SELECT data FROM requests WHERE protocol='P-SNAPSHOT'`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		path  []string
		value string
	}{
		{[]string{"schema_version"}, `1`}, {[]string{"company_id"}, `"outra"`}, {[]string{"condo_id"}, `101`},
		{[]string{"use_case_id"}, `"mudanca"`}, {[]string{"status"}, `"READY"`},
		{[]string{"state", "company_id"}, `"outra"`}, {[]string{"state", "telefone"}, `"5511888888888"`},
		{[]string{"state", "condo_id"}, `101`}, {[]string{"state", "caso"}, `"mudanca"`}, {[]string{"state", "estado"}, `"pronto"`},
	} {
		if _, err := b.Pool.Exec(t.Context(), `UPDATE requests SET data=jsonb_set($1::jsonb,$2::text[],$3::jsonb) WHERE protocol='P-SNAPSHOT'`, original, entry.path, entry.value); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Ler(t.Context(), contact); err == nil {
			t.Fatalf("snapshot incompatível aceito: %v", entry.path)
		}
	}
	if _, err := b.Pool.Exec(t.Context(), `UPDATE requests SET data=$1 WHERE protocol='P-SNAPSHOT'`, original); err != nil {
		t.Fatal(err)
	}
	if cart := lerCarteira(t, store, contact); len(cart.Pedidos) != 1 {
		t.Fatal("snapshot válido não foi restaurado")
	}
}
