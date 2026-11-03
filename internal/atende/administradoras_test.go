package atende

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
	"go-concierge/internal/okf"
)

const (
	tenantExemplo   = "69a5a131e67bd8e2f54e5981" // o tenant de eventoExemplo
	telefoneMorador = "5519981621916"
)

// mapeamentoDeTeste: a administradora A dona do condomínio do eventoExemplo
// (cond-001, pelo tenant) e a B dona de cond-b (company 7777, tenant-b).
func mapeamentoDeTeste(t testing.TB) *admin.EmMemoria {
	t.Helper()
	ctx := context.Background()
	repo := admin.NovaEmMemoria()
	passos := []func() (admin.Mapeamento, error){
		func() (admin.Mapeamento, error) {
			return repo.SalvarAdministradora(ctx, admin.Administradora{ID: "adm-a", Nome: "A"})
		},
		func() (admin.Mapeamento, error) {
			return repo.SalvarAdministradora(ctx, admin.Administradora{ID: "adm-b", Nome: "B"})
		},
		func() (admin.Mapeamento, error) {
			return repo.SalvarCondominio(ctx, admin.Condominio{ID: "cond-001", Nome: "Exemplo", Administradora: "adm-a", Account: "cond-001", CompanyID: "5982", Tenant: tenantExemplo})
		},
		func() (admin.Mapeamento, error) {
			return repo.SalvarCondominio(ctx, admin.Condominio{ID: "cond-b", Nome: "Condomínio B", Administradora: "adm-b", Account: "acc-b", CompanyID: "7777", Tenant: "tenant-b"})
		},
	}
	for _, passo := range passos {
		if _, err := passo(); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

// contatoDeTeste: o telefone do eventoExemplo numa administradora.
func contatoDeTeste(t testing.TB, administradoraID string) escopo.Contato {
	t.Helper()
	c, err := escopo.NovoContato(administradoraID, "cond-001", telefoneMorador)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func servicoMultiAdministradora(t *testing.T, imp escopo.Implantacao) (*Servico, *memoria.Arquivos) {
	t.Helper()
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	fix, _ := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	cat.Recursos = func() []okf.Recurso { return fix }
	adicionarToolsMCP(cat)
	st := memoria.NewArquivos(t.TempDir())
	svc := &Servico{
		Cat: cat, Agente: "agentes/concierge", Sessoes: NewSessoes(), Sender: &Outbox{},
		Config:     harness.Config{Mock: true, Env: func(string) string { return "" }, MCP: &mcpServicoFalso{}},
		Mapeamento: mapeamentoDeTeste(t), Implantacao: imp, Memorias: st,
	}
	return svc, st
}

// mensagemDe: o eventoExemplo vindo de outro protocolo, tenant e company.
func mensagemDe(t *testing.T, protocolo, tenant, company, texto string) entrada.Mensagem {
	t.Helper()
	ev, err := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", texto, 1)))
	if err != nil {
		t.Fatal(err)
	}
	ev.ID = "evt-" + protocolo
	ev.Tenant = tenant
	ev.Data.Protocol.Number = protocolo
	msg := ev.Normalizar()
	msg.CompanyID = company
	return msg
}

func TestServicoAtendeDuasAdministradoras(t *testing.T) {
	svc, st := servicoMultiAdministradora(t, escopo.Implantacao{})
	ctx := context.Background()
	sessA, err := svc.Processar(ctx, mensagemDe(t, "P-A", tenantExemplo, "", "tenho um cachorro"))
	if err != nil {
		t.Fatal(err)
	}
	sessB, err := svc.Processar(ctx, mensagemDe(t, "P-B", "tenant-b", "7777", "tenho dois gatos"))
	if err != nil {
		t.Fatal(err)
	}
	svc.Aguardar()

	casos := []struct {
		sess           *Sessao
		administradora string
		condominio     string
		account        string
		fato           string
	}{
		{sessA, "adm-a", "cond-001", "cond-001", "tem um cachorro"},
		{sessB, "adm-b", "cond-b", "acc-b", "tem dois gatos"},
	}
	for _, c := range casos {
		t.Run(c.administradora, func(t *testing.T) {
			if c.sess.AdministradoraID != c.administradora || c.sess.Condominio != c.condominio {
				t.Fatalf("sessão resolvida em %s/%s", c.sess.AdministradoraID, c.sess.Condominio)
			}
			mem, err := st.Ler(ctx, contatoDeTeste(t, c.administradora))
			if err != nil {
				t.Fatal(err)
			}
			if len(mem.Fatos) != 1 || mem.Fatos[0].Texto != c.fato || mem.Fatos[0].CondominioID != c.condominio {
				t.Fatalf("memória de %s: %+v", c.administradora, mem.Fatos)
			}
			cfg, _, _, err := svc.montar(ctx, c.sess)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Identidade.AccountID != c.account {
				t.Fatalf("account das tools = %q, want %q", cfg.Identidade.AccountID, c.account)
			}
		})
	}
}

func TestServicoTransbordaCondominioNaoAtendido(t *testing.T) {
	tests := []struct {
		name string
		imp  escopo.Implantacao
		msg  func(t *testing.T) entrada.Mensagem
	}{
		{"company desconhecida", escopo.Implantacao{}, func(t *testing.T) entrada.Mensagem {
			return mensagemDe(t, "P-X", "tenant-x", "999", "oi")
		}},
		{"condominio de outra administradora", escopo.Implantacao{AdministradoraFixa: "adm-a"}, func(t *testing.T) entrada.Mensagem {
			return mensagemDe(t, "P-B", "tenant-b", "7777", "oi")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := servicoMultiAdministradora(t, tt.imp)
			sess, err := svc.Processar(context.Background(), tt.msg(t))
			if err != nil {
				t.Fatal(err)
			}
			if sess.Transbordo == nil || !sess.Transbordo.Ativo {
				t.Fatalf("deveria transbordar: %+v", sess.Transbordo)
			}
			for _, turno := range sess.Turnos {
				if turno.Quem == "concierge" {
					t.Fatalf("o concierge não deveria responder: %+v", sess.Turnos)
				}
			}
		})
	}
}
