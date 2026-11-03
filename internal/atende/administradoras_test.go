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
	tenantExemplo     = "69a5a131e67bd8e2f54e5981" // o tenant de eventoExemplo
	telefoneMorador   = "5519981621916"
	ferramentaEmpresa = "gruvi.bff.building-operation.get-company-by-document-controller-handle"
)

// mapeamentoDeTeste: a administradora A dona do condomínio do eventoExemplo
// (cond-001, pelo tenant) e a B dona de cond-b (company 7777, tenant-b). Os outros
// condomínios de B são os que não se atendem: sem account, sem company, e um tenant
// que as duas administradoras usam.
func mapeamentoDeTeste(t testing.TB) *admin.EmMemoria {
	t.Helper()
	ctx := context.Background()
	repo := admin.NovaEmMemoria()
	for _, a := range []admin.Administradora{{ID: "adm-a", Nome: "A"}, {ID: "adm-b", Nome: "B"}} {
		if _, err := repo.SalvarAdministradora(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	condominios := []admin.Condominio{
		{ID: "cond-001", Nome: "Exemplo", Administradora: "adm-a", Account: "cond-001", CompanyID: "5982", Tenant: tenantExemplo},
		{ID: "cond-b", Nome: "Condomínio B", Administradora: "adm-b", Account: "acc-b", CompanyID: "7777", Tenant: "tenant-b"},
		{ID: "cond-b-sem-account", Nome: "B sem account", Administradora: "adm-b", CompanyID: "8888", Tenant: "tenant-b-sem-account"},
		{ID: "cond-b-sem-company", Nome: "B sem company", Administradora: "adm-b", Account: "acc-b2", Tenant: "tenant-b-sem-company"},
		{ID: "cond-a-dividido", Nome: "A dividido", Administradora: "adm-a", Account: "acc-ad", CompanyID: "9001", Tenant: "tenant-dividido"},
		{ID: "cond-b-dividido", Nome: "B dividido", Administradora: "adm-b", Account: "acc-bd", CompanyID: "9002", Tenant: "tenant-dividido"},
	}
	for _, c := range condominios {
		if _, err := repo.SalvarCondominio(ctx, c); err != nil {
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

func servicoMultiAdministradora(t *testing.T, imp escopo.Implantacao) (*Servico, *memoria.Fake, *mcpServicoFalso) {
	t.Helper()
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	fix, _ := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	cat.Recursos = func() []okf.Recurso { return fix }
	adicionarToolsMCP(cat)
	st := memoria.NovoFake()
	mcpFalso := &mcpServicoFalso{}
	svc := &Servico{
		Cat: cat, Agente: "agentes/concierge", Sessoes: NewSessoes(NovaSessaoEmMemoria()), Sender: &Outbox{},
		Config:     harness.Config{Mock: true, Env: func(string) string { return "" }, MCP: mcpFalso},
		Mapeamento: mapeamentoDeTeste(t), Implantacao: imp, Memorias: st,
	}
	return svc, st, mcpFalso
}

// mensagemDe: o eventoExemplo vindo de outro protocolo, tenant e company.
func mensagemDe(t *testing.T, protocolo, tenant, company, texto string) entrada.Mensagem {
	t.Helper()
	ev, err := Parse([]byte(strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", texto, 1)))
	if err != nil {
		t.Fatal(err)
	}
	ev.ID = "evt-" + protocolo + "-" + texto
	ev.Tenant = tenant
	ev.Data.Protocol.Number = protocolo
	msg := ev.Normalizar()
	msg.CompanyID = company
	return msg
}

func TestServicoAtendeDuasAdministradoras(t *testing.T) {
	svc, st, _ := servicoMultiAdministradora(t, escopo.Implantacao{})
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
		company        string
		fato           string
	}{
		{sessA, "adm-a", "cond-001", "cond-001", "5982", "tem um cachorro"},
		{sessB, "adm-b", "cond-b", "acc-b", "7777", "tem dois gatos"},
	}
	for _, c := range casos {
		t.Run(c.administradora, func(t *testing.T) {
			if c.sess.AdministradoraID != c.administradora || c.sess.Condominio != c.condominio || c.sess.CompanyID != c.company {
				t.Fatalf("sessão resolvida em %s/%s company %s", c.sess.AdministradoraID, c.sess.Condominio, c.sess.CompanyID)
			}
			mem, err := st.Ler(ctx, contatoDeTeste(t, c.administradora))
			if err != nil {
				t.Fatal(err)
			}
			if len(mem.Fatos) != 1 || mem.Fatos[0].Texto != c.fato || mem.Fatos[0].CondominioID != c.condominio {
				t.Fatalf("memória de %s: %+v", c.administradora, mem.Fatos)
			}
			m, condo, err := svc.condominioDaSessao(ctx, c.sess, c.sess.contato())
			if err != nil {
				t.Fatal(err)
			}
			cfg, _, _, err := svc.montar(ctx, c.sess, m, condo)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Identidade.AccountID != c.account {
				t.Fatalf("account das tools = %q, want %q", cfg.Identidade.AccountID, c.account)
			}
			if got := cfg.Payloads[ferramentaEmpresa]["companyId"]; got != c.company {
				t.Fatalf("companyId das tools MCP = %v, want %s", got, c.company)
			}
		})
	}
}

// A identidade no Gruvi consulta a company do condomínio da mensagem, não a do processo.
func TestIdentidadeUsaCompanyDoCondominio(t *testing.T) {
	svc, _, mcpFalso := servicoMultiAdministradora(t, escopo.Implantacao{})
	svc.TelefoneB2C = telefoneMorador
	sess, err := svc.Processar(context.Background(), mensagemDe(t, "P-B", "tenant-b", "7777", "oi"))
	if err != nil {
		t.Fatal(err)
	}
	if sess.Identidade.CompanyID != "7777" || !strings.Contains(strings.Join(mcpFalso.chamadas, ","), ferramentaEmpresa) {
		t.Fatalf("identidade pela company do condomínio: %+v chamadas %v", sess.Identidade, mcpFalso.chamadas)
	}
}

func TestServicoTransbordaCondominioNaoAtendido(t *testing.T) {
	tests := []struct {
		name                  string
		imp                   escopo.Implantacao
		tenant, company, prot string
	}{
		{"company desconhecida", escopo.Implantacao{}, "tenant-x", "999", "P-X"},
		{"condominio de outra administradora", escopo.Implantacao{AdministradoraFixa: "adm-a"}, "tenant-b", "7777", "P-B"},
		{"condominio sem account", escopo.Implantacao{}, "tenant-b-sem-account", "", "P-SA"},
		{"condominio sem company", escopo.Implantacao{}, "tenant-b-sem-company", "", "P-SC"},
		{"tenant de duas administradoras", escopo.Implantacao{}, "tenant-dividido", "", "P-D"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, mcpFalso := servicoMultiAdministradora(t, tt.imp)
			sess, err := svc.Processar(context.Background(), mensagemDe(t, tt.prot, tt.tenant, tt.company, "oi"))
			if err != nil {
				t.Fatal(err)
			}
			exigirTransbordoSemConcierge(t, sess)
			if len(mcpFalso.chamadas) != 0 {
				t.Fatalf("nenhuma tool deveria ser chamada: %v", mcpFalso.chamadas)
			}
		})
	}
}

// Uma conversa não troca de condomínio no meio: a segunda resolução diferente transborda.
func TestServicoTransbordaQuandoOCondominioDaConversaMuda(t *testing.T) {
	svc, _, _ := servicoMultiAdministradora(t, escopo.Implantacao{})
	ctx := context.Background()
	if _, err := svc.Processar(ctx, mensagemDe(t, "P-1", tenantExemplo, "", "oi")); err != nil {
		t.Fatal(err)
	}
	sess, err := svc.Processar(ctx, mensagemDe(t, "P-1", tenantExemplo, "7777", "e agora?"))
	if err != nil {
		t.Fatal(err)
	}
	if sess.Transbordo == nil || !sess.Transbordo.Ativo || sess.AdministradoraID != "adm-a" {
		t.Fatalf("deveria transbordar mantendo adm-a: %+v %s", sess.Transbordo, sess.AdministradoraID)
	}
}

func exigirTransbordoSemConcierge(t *testing.T, sess *Sessao) {
	t.Helper()
	if sess.Transbordo == nil || !sess.Transbordo.Ativo {
		t.Fatalf("deveria transbordar: %+v", sess.Transbordo)
	}
	for _, turno := range sess.Turnos {
		if turno.Quem == "concierge" {
			t.Fatalf("o concierge não deveria responder: %+v", sess.Turnos)
		}
	}
}

// mapeamentoContado conta as leituras: o turno lê o mapeamento uma vez só, na resolução.
type mapeamentoContado struct {
	admin.Repositorio
	leituras int
}

func (m *mapeamentoContado) Ler(ctx context.Context) (admin.Mapeamento, error) {
	m.leituras++
	return m.Repositorio.Ler(ctx)
}

func TestTurnoLeOMapeamentoUmaVez(t *testing.T) {
	svc, _, _ := servicoMultiAdministradora(t, escopo.Implantacao{})
	contado := &mapeamentoContado{Repositorio: svc.Mapeamento}
	svc.Mapeamento = contado
	if _, err := svc.Processar(context.Background(), mensagemDe(t, "P-1", tenantExemplo, "", "oi")); err != nil {
		t.Fatal(err)
	}
	if contado.leituras != 1 {
		t.Fatalf("leituras do mapeamento no turno = %d, want 1", contado.leituras)
	}
}

// Falha ao montar o turno (aqui, o agente não existe no catálogo): o morador recebe a
// frase de falha, a troca entra no histórico e nenhum agente ou tool roda.
func TestFalhaAoMontarOTurnoRespondeAoMorador(t *testing.T) {
	svc, _, mcpFalso := servicoMultiAdministradora(t, escopo.Implantacao{})
	svc.Agente = "agentes/nao-existe"
	out := svc.Sender.(*Outbox)
	msg := mensagemDe(t, "P-1", tenantExemplo, "", "oi")
	sess, err := svc.Processar(context.Background(), msg)
	if err == nil {
		t.Fatal("o erro da montagem deveria voltar")
	}
	ultimo := sess.Turnos[len(sess.Turnos)-1]
	if ultimo.Quem != "concierge" || ultimo.Texto != FraseFalha || ultimo.Erro == "" {
		t.Fatalf("resposta de falha: %+v", ultimo)
	}
	if saidas := out.Itens(msg.ConversaID); len(saidas) != 1 || saidas[0].Texto != FraseFalha {
		t.Fatalf("o morador deveria receber a frase de falha: %+v", saidas)
	}
	if h := sess.historicoSnapshot(); len(h) != 2 {
		t.Fatalf("a troca deveria entrar no histórico: %d mensagens", len(h))
	}
	if len(mcpFalso.chamadas) != 0 {
		t.Fatalf("nenhuma tool deveria rodar: %v", mcpFalso.chamadas)
	}
}
