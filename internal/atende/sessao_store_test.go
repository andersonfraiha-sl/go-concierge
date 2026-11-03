package atende

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/okf"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func TestContratoSessaoStoreEmMemoria(t *testing.T) {
	testarSessaoStore(t, func(*testing.T) SessaoStore { return NovaSessaoEmMemoria() })
}

func TestContratoSessaoStorePostgres(t *testing.T) {
	testarSessaoStore(t, func(t *testing.T) SessaoStore { return NovoSessaoPostgres(bancotest.Novo(t).Ent) })
}

// testarSessaoStore é o contrato que toda implementação de SessaoStore cumpre.
func testarSessaoStore(t *testing.T, novo func(t *testing.T) SessaoStore) {
	base := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	registro := func(id, adm string, minutos int, dados string) RegistroSessao {
		return RegistroSessao{ID: id, AdministradoraID: adm, CondominioID: "cond-" + id, Protocolo: "P-" + id,
			AtualizadaEm: base.Add(time.Duration(minutos) * time.Minute), Dados: []byte(dados)}
	}

	t.Run("salva, carrega e sobrescreve", func(t *testing.T) {
		st, ctx := novo(t), t.Context()
		if _, ok, err := st.Carregar(ctx, "s1"); err != nil || ok {
			t.Fatalf("sessão inexistente: ok=%v err=%v", ok, err)
		}
		if err := st.Salvar(ctx, registro("s1", "adm-a", 0, `{"v":1}`)); err != nil {
			t.Fatal(err)
		}
		novoRegistro := registro("s1", "adm-b", 5, `{"v":2}`)
		if err := st.Salvar(ctx, novoRegistro); err != nil {
			t.Fatal(err)
		}
		got, ok, err := st.Carregar(ctx, "s1")
		if err != nil || !ok {
			t.Fatalf("carregar: ok=%v err=%v", ok, err)
		}
		if !mesmoRegistro(got, novoRegistro) {
			t.Fatalf("registro carregado = %+v, esperado %+v", got, novoRegistro)
		}
	})

	t.Run("lista por administradora, mais recentes primeiro, com limite", func(t *testing.T) {
		st, ctx := novo(t), t.Context()
		for _, r := range []RegistroSessao{
			registro("a1", "adm-a", 1, `{}`), registro("a2", "adm-a", 3, `{}`),
			registro("a3", "adm-a", 2, `{}`), registro("b1", "adm-b", 9, `{}`),
		} {
			if err := st.Salvar(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
		casos := []struct {
			nome   string
			adm    string
			limite int
			ids    []string
		}{
			{"uma administradora", "adm-a", 10, []string{"a2", "a3", "a1"}},
			{"limite", "adm-a", 2, []string{"a2", "a3"}},
			{"sem administradora lista todas", "", 10, []string{"b1", "a2", "a3", "a1"}},
			{"administradora sem sessões", "adm-z", 10, nil},
		}
		for _, c := range casos {
			t.Run(c.nome, func(t *testing.T) {
				regs, err := st.Listar(ctx, c.adm, c.limite)
				if err != nil {
					t.Fatal(err)
				}
				if got := idsDe(regs); strings.Join(got, ",") != strings.Join(c.ids, ",") {
					t.Fatalf("ids = %v, esperado %v", got, c.ids)
				}
			})
		}
	})

	t.Run("remove de forma idempotente", func(t *testing.T) {
		st, ctx := novo(t), t.Context()
		if err := st.Salvar(ctx, registro("s1", "adm-a", 0, `{}`)); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := st.Remover(ctx, "s1"); err != nil {
				t.Fatal(err)
			}
		}
		if _, ok, err := st.Carregar(ctx, "s1"); err != nil || ok {
			t.Fatalf("sessão removida ainda carrega: ok=%v err=%v", ok, err)
		}
	})
}

func mesmoRegistro(a, b RegistroSessao) bool {
	return a.ID == b.ID && a.AdministradoraID == b.AdministradoraID && a.CondominioID == b.CondominioID &&
		a.Protocolo == b.Protocolo && a.AtualizadaEm.Equal(b.AtualizadaEm) && mesmoJSON(a.Dados, b.Dados)
}

// mesmoJSON compara pelo conteúdo: o jsonb do Postgres reformata o texto.
func mesmoJSON(a, b []byte) bool {
	var va, vb any
	return json.Unmarshal(a, &va) == nil && json.Unmarshal(b, &vb) == nil && reflect.DeepEqual(va, vb)
}

func idsDe(regs []RegistroSessao) []string {
	var ids []string
	for _, r := range regs {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestRestaurarDestrava(t *testing.T) {
	sess, err := NewSessoes(NovaSessaoEmMemoria()).Abrir(t.Context(), entrada.Mensagem{ConversaID: "P-9", Tenant: "t", Contato: entrada.Contato{Phone: "5511999"}})
	if err != nil {
		t.Fatal(err)
	}
	sess.fixarContato(contatoDeTeste(t, "adm-a"), "company-1")
	sess.Processando = true
	r, err := sess.Registro()
	if err != nil {
		t.Fatal(err)
	}
	restaurada, err := RestaurarSessao(r)
	if err != nil {
		t.Fatal(err)
	}
	if restaurada.Processando {
		t.Fatal("sessão restaurada não pode continuar travada em processamento")
	}
	if restaurada.AdministradoraID != "adm-a" || restaurada.CompanyID != "company-1" || restaurada.contato() != sess.contato() {
		t.Fatalf("restaurada sem o condomínio resolvido: adm=%q company=%q contato=%+v", restaurada.AdministradoraID, restaurada.CompanyID, restaurada.contato())
	}
	if r.AdministradoraID != "adm-a" || r.CondominioID != sess.Condominio || r.Protocolo != "P-9" || r.ID != sess.ID {
		t.Fatalf("colunas do registro: %+v", r)
	}
}

func TestSessaoSobreviveRestart(t *testing.T) {
	stores := []struct {
		nome string
		novo func(t *testing.T) SessaoStore
	}{
		{"em memória", func(*testing.T) SessaoStore { return NovaSessaoEmMemoria() }},
		{"postgres", func(t *testing.T) SessaoStore { return NovoSessaoPostgres(bancotest.Novo(t).Ent) }},
	}
	for _, c := range stores {
		t.Run(c.nome, func(t *testing.T) { sessaoSobreviveRestart(t, c.novo(t)) })
	}
}

func sessaoSobreviveRestart(t *testing.T, st SessaoStore) {
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	fix, err := okf.LerRecursosFixture(filepath.Join(cat.Root, "nexus", "cond-001.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cat.Recursos = func() []okf.Recurso { return fix }
	adicionarToolsMCP(cat)
	novoServico := func() *Servico {
		return &Servico{
			Cat: cat, Agente: "agentes/concierge", Mapeamento: mapeamentoDeTeste(t), Sessoes: NewSessoes(st), Sender: &Outbox{},
			Config:      harness.Config{Mock: true, Env: func(string) string { return "" }, MCP: &mcpServicoFalso{}},
			TelefoneB2C: "5519981621916", Gruvi: &gruvi.Client{CompanyID: "5982"},
		}
	}
	svc := novoServico()
	ev := parseEvento(t, eventoExemplo)
	if _, err := svc.Processar(t.Context(), ev.Normalizar()); err != nil {
		t.Fatal(err)
	}
	ev2 := parseEvento(t, strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "até que horas a piscina abre?", 1))
	ev2.ID = "evt_2"
	antes, err := svc.Processar(t.Context(), ev2.Normalizar())
	if err != nil {
		t.Fatal(err)
	}

	depois, ok, err := novoServico().Sessoes.Get(t.Context(), antes.ID)
	if err != nil || !ok {
		t.Fatalf("sessão não sobreviveu ao restart: ok=%v err=%v", ok, err)
	}
	if len(depois.Turnos) != 4 || len(depois.Turnos) != len(antes.Turnos) {
		t.Fatalf("turnos depois do restart = %d, antes = %d", len(depois.Turnos), len(antes.Turnos))
	}
	if n := len(depois.historicoSnapshot()); n == 0 || n != len(antes.historicoSnapshot()) {
		t.Fatalf("histórico depois do restart = %d mensagens, antes = %d", n, len(antes.historicoSnapshot()))
	}
	if depois.contato() != antes.contato() || depois.CompanyID != antes.CompanyID {
		t.Fatalf("condomínio resolvido perdido: %+v / %q", depois.contato(), depois.CompanyID)
	}

	// a próxima mensagem depois do restart continua a mesma conversa
	svc3 := novoServico()
	ev3 := parseEvento(t, strings.Replace(eventoExemplo, "posso fazer barulho depois das 22h?", "e a academia?", 1))
	ev3.ID = "evt_3"
	continuada, err := svc3.Processar(context.Background(), ev3.Normalizar())
	if err != nil {
		t.Fatal(err)
	}
	if len(continuada.Turnos) != 6 {
		t.Fatalf("conversa recomeçou depois do restart: %d turnos", len(continuada.Turnos))
	}
}

// Ajudantes dos testes: a operação não pode falhar com o store em memória. Usam
// t.Errorf (e não Fatal) porque alguns testes chamam de outras goroutines.

func listar(t testing.TB, s *Sessoes) []*Sessao {
	t.Helper()
	sessoes, err := s.Lista(context.Background(), "")
	if err != nil {
		t.Errorf("listar sessões: %v", err)
	}
	return sessoes
}

func abrir(t testing.TB, s *Sessoes, msg entrada.Mensagem) *Sessao {
	t.Helper()
	sess, err := s.Abrir(context.Background(), msg)
	if err != nil {
		t.Errorf("abrir sessão: %v", err)
	}
	return sess
}

func preparar(t testing.TB, svc *Servico, msg entrada.Mensagem) (entrada.Mensagem, *Sessao) {
	t.Helper()
	msg, sess, err := svc.PrepararMensagem(context.Background(), msg)
	if err != nil {
		t.Errorf("preparar mensagem: %v", err)
	}
	return msg, sess
}

func registrarHumano(t testing.TB, svc *Servico, msg entrada.Mensagem) bool {
	t.Helper()
	ok, err := svc.RegistrarHumano(context.Background(), msg)
	if err != nil {
		t.Errorf("registrar humano: %v", err)
	}
	return ok
}

func encerrar(t testing.TB, svc *Servico, msg entrada.Mensagem) bool {
	t.Helper()
	ok, err := svc.Encerrar(context.Background(), msg)
	if err != nil {
		t.Errorf("encerrar: %v", err)
	}
	return ok
}

func parseEvento(t *testing.T, raw string) *Evento {
	t.Helper()
	ev, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return ev
}

// storeFalho falha as primeiras `falhas` gravações; o resto vai ao store em memória.
type storeFalho struct {
	*SessaoEmMemoria
	mu     sync.Mutex
	falhas int
}

func (f *storeFalho) Salvar(ctx context.Context, r RegistroSessao) error {
	f.mu.Lock()
	falhar := f.falhas > 0
	f.falhas--
	f.mu.Unlock()
	if falhar {
		return errors.New("postgres fora")
	}
	return f.SessaoEmMemoria.Salvar(ctx, r)
}

func TestEventoReenviadoNaoDuplicaTurno(t *testing.T) {
	casos := []struct {
		nome   string
		aplica func(*Servico, context.Context, entrada.Mensagem) (bool, error)
	}{
		{"mensagem humana", (*Servico).RegistrarHumano},
		{"fechamento", (*Servico).Encerrar},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			st := &storeFalho{SessaoEmMemoria: NovaSessaoEmMemoria(), falhas: 1}
			svc := &Servico{Sessoes: NewSessoes(st)}
			msg := entrada.Mensagem{ID: "evt-humano", ConversaID: "P-3", Tenant: "tenant", Autor: "atendente", Texto: "oi", Contato: entrada.Contato{Phone: "5511999"}}
			sess := abrir(t, svc.Sessoes, msg)
			if _, err := c.aplica(svc, t.Context(), msg); !errors.Is(err, ErrSessaoNaoPersistida) {
				t.Fatalf("primeira entrega deveria falhar ao persistir: %v", err)
			}
			if ok, err := c.aplica(svc, t.Context(), msg); err != nil || !ok {
				t.Fatalf("reenvio deveria só refazer a gravação: ok=%v err=%v", ok, err)
			}
			if len(sess.Turnos) != 1 {
				t.Fatalf("reenvio duplicou o turno: %+v", sess.Turnos)
			}
			r, ok, err := st.Carregar(t.Context(), sess.ID)
			if err != nil || !ok {
				t.Fatalf("sessão não gravada no reenvio: ok=%v err=%v", ok, err)
			}
			gravada, err := RestaurarSessao(r)
			if err != nil || len(gravada.Turnos) != 1 {
				t.Fatalf("sessão gravada: %+v err=%v", gravada, err)
			}
		})
	}
}

func TestPersistirFalhoLogaEProcessarDevolve(t *testing.T) {
	var log bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })

	svc := &Servico{Sessoes: NewSessoes(&storeFalho{SessaoEmMemoria: NovaSessaoEmMemoria(), falhas: 1})}
	msg := entrada.Mensagem{ID: "evt-1", ConversaID: "P-4", Tenant: "tenant", Texto: "oi", Contato: entrada.Contato{Phone: "5511999"}}
	abrir(t, svc.Sessoes, msg).encerrar(entrada.Mensagem{ID: "evt-0"})
	if _, err := svc.Processar(t.Context(), msg); !errors.Is(err, ErrSessaoNaoPersistida) {
		t.Fatalf("Processar deveria devolver a falha de persistência: %v", err)
	}
	if !strings.Contains(log.String(), "level=ERROR") || !strings.Contains(log.String(), "persistir_sessao") {
		t.Fatalf("falha de persistência não foi logada como ERROR: %s", log.String())
	}
}

func TestSessaoDeOutraAdministradoraNaoAparece(t *testing.T) {
	st := NovaSessaoEmMemoria()
	alheia := &Sessao{ID: "s-b", AdministradoraID: "adm-b", Protocolo: "P-b", Destino: entrada.Destino{Conector: "atende-emulado"}}
	r, err := alheia.Registro()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Salvar(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	svc := &Servico{Sessoes: NewSessoes(st), Implantacao: escopo.Implantacao{AdministradoraFixa: "adm-a"}}
	if _, ok, err := svc.Sessao(t.Context(), "s-b"); err != nil || ok {
		t.Fatalf("sessão de outra administradora visível: ok=%v err=%v", ok, err)
	}
	operacoes := map[string]func() error{
		"assumir":  func() error { _, err := svc.Assumir(t.Context(), "s-b", "Ana", "teste"); return err },
		"devolver": func() error { _, err := svc.Devolver(t.Context(), "s-b"); return err },
		"responder": func() error {
			_, err := svc.Responder(t.Context(), "s-b", "Ana", "oi")
			return err
		},
	}
	for nome, op := range operacoes {
		if err := op(); !errors.Is(err, ErrProtocoloDesconhecido) {
			t.Fatalf("%s numa sessão alheia: %v", nome, err)
		}
	}
}
