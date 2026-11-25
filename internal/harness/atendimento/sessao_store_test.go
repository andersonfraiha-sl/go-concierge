package atendimento

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/harness/canais"
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
	testarSessaoStore(t, func(t *testing.T) SessaoStore { return novoSessaoStorePostgres(t) })
}

// testarSessaoStore é o contrato que toda implementação de SessaoStore cumpre.
var channelStoreA = uuid.MustParse("a1000000-0000-0000-0000-000000000001")
var channelStoreB = uuid.MustParse("b1000000-0000-0000-0000-000000000001")

func novoSessaoStorePostgres(t *testing.T) SessaoStore {
	t.Helper()
	b := bancotest.Novo(t)
	for company, id := range map[string]uuid.UUID{"adm-a": channelStoreA, "adm-b": channelStoreB} {
		if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,'WEB','test',$3,'INHERIT',true,'root',now())`, id, company, id.String()); err != nil {
			t.Fatal(err)
		}
	}
	return NovoSessaoPostgres(b)
}

func testarSessaoStore(t *testing.T, novo func(*testing.T) SessaoStore) {
	base := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	registro := func(t *testing.T, id, company string, minutos int) RegistroSessao {
		channel := channelStoreA
		if company == "adm-b" {
			channel = channelStoreB
		}
		r := registroSessaoV2(t, id, company, channel, nil)
		r.Protocolo = "P-" + id
		r.AtualizadaEm = base.Add(time.Duration(minutos) * time.Minute)
		r.Dados = serializarRegistroV2(t, r, "nome")
		return r
	}
	contextFor := func(t *testing.T, company string) context.Context {
		return ComEscopoSessao(t.Context(), company, uuid.Nil)
	}
	t.Run("salva carrega e atualiza estado mantendo identidade", func(t *testing.T) {
		store := novo(t)
		ctx := contextFor(t, "adm-a")
		if _, ok, err := store.Carregar(ctx, "s1"); err != nil || ok {
			t.Fatalf("inexistente=%v (%v)", ok, err)
		}
		r := registro(t, "s1", "adm-a", 0)
		if err := store.Salvar(ctx, r); err != nil {
			t.Fatal(err)
		}
		updated := registro(t, "s1", "adm-a", 5)
		updated.Dados = serializarRegistroV2(t, updated, "atualizado")
		if err := store.Salvar(ctx, updated); err != nil {
			t.Fatal(err)
		}
		got, ok, err := store.Carregar(ctx, "s1")
		if err != nil || !ok || !mesmoRegistro(got, updated) {
			t.Fatalf("registro=%+v (%v)", got, err)
		}
		if err := store.Salvar(contextFor(t, "adm-b"), registro(t, "s1", "adm-b", 6)); err == nil {
			t.Fatal("reatribuição empresarial aceita")
		}
		got, ok, err = store.Carregar(ctx, "s1")
		if err != nil || !ok || !mesmoRegistro(got, updated) {
			t.Fatalf("original alterado=%+v (%v)", got, err)
		}
	})
	t.Run("lista empresa autorizada ordenada e limitada", func(t *testing.T) {
		store := novo(t)
		for _, r := range []RegistroSessao{registro(t, "a1", "adm-a", 1), registro(t, "a2", "adm-a", 3), registro(t, "a3", "adm-a", 2), registro(t, "b1", "adm-b", 9)} {
			if err := store.Salvar(contextFor(t, r.CompanyID), r); err != nil {
				t.Fatal(err)
			}
		}
		for _, tt := range []struct {
			company string
			limit   int
			ids     []string
		}{{"adm-a", 10, []string{"a2", "a3", "a1"}}, {"adm-a", 2, []string{"a2", "a3"}}, {"adm-b", 10, []string{"b1"}}, {"adm-z", 10, nil}} {
			rows, err := store.Listar(contextFor(t, tt.company), tt.company, tt.limit)
			if err != nil || !slices.Equal(idsDe(rows), tt.ids) {
				t.Fatalf("lista %s=%v (%v)", tt.company, idsDe(rows), err)
			}
		}
		if _, err := store.Listar(contextFor(t, ""), "", 10); !errors.Is(err, ErrEscopoSessao) {
			t.Fatalf("listagem sem empresa=%v", err)
		}
	})
	t.Run("remove idempotente preservando outra empresa", func(t *testing.T) {
		store := novo(t)
		ctx := contextFor(t, "adm-a")
		r := registro(t, "s1", "adm-a", 0)
		if err := store.Salvar(ctx, r); err != nil {
			t.Fatal(err)
		}
		if err := store.Remover(contextFor(t, "adm-b"), "s1"); err != nil && !errors.Is(err, ErrEscopoSessao) {
			t.Fatal(err)
		}
		if _, ok, err := store.Carregar(ctx, "s1"); err != nil || !ok {
			t.Fatalf("sessão alheia removida=%v (%v)", ok, err)
		}
		for range 2 {
			if err := store.Remover(ctx, "s1"); err != nil {
				t.Fatal(err)
			}
		}
		if _, ok, err := store.Carregar(ctx, "s1"); err != nil || ok {
			t.Fatalf("removida=%v (%v)", ok, err)
		}
	})
}

func mesmoRegistro(a, b RegistroSessao) bool {
	return a.ID == b.ID && a.CompanyID == b.CompanyID && a.ChannelID == b.ChannelID && a.Connector == b.Connector && a.UserPhone == b.UserPhone && escopo.MesmoCondominio(a.CondoID, b.CondoID) && a.Protocolo == b.Protocolo && a.AtualizadaEm.Equal(b.AtualizadaEm) && mesmoJSON(a.Dados, b.Dados)
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
	sess.fixarContato(contatoDeTeste(t, "company-1"), "company-1")
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
	if restaurada.AdministradoraID != "company-1" || restaurada.CompanyID != "company-1" || restaurada.contato().Canonico() != sess.contato().Canonico() {
		t.Fatalf("restaurada sem o condomínio resolvido: adm=%q company=%q contato=%+v", restaurada.AdministradoraID, restaurada.CompanyID, restaurada.contato())
	}
	if r.AdministradoraID != "company-1" || r.CondominioID != sess.Condominio || r.Protocolo != "P-9" || r.ID != sess.ID {
		t.Fatalf("colunas do registro: %+v", r)
	}
}

func TestSessaoSobreviveRestart(t *testing.T) {
	stores := []struct {
		nome string
		novo func(t *testing.T) SessaoStore
	}{
		{"em memória", func(*testing.T) SessaoStore { return NovaSessaoEmMemoria() }},
		{"postgres", func(t *testing.T) SessaoStore { return novoSessaoStorePostgres(t) }},
	}
	for _, c := range stores {
		t.Run(c.nome, func(t *testing.T) { sessaoSobreviveRestart(t, c.novo(t)) })
	}
}

func sessaoSobreviveRestart(t *testing.T, store SessaoStore) {
	ctx := ComEscopoSessao(t.Context(), "adm-a", channelStoreA)
	msg := entrada.Mensagem{CompanyID: "adm-a", ChannelID: channelStoreA, ConversaID: "P-RESTART", Contato: entrada.Contato{Phone: "5511999998888"}, Origem: entrada.Origem{Conector: "test", Meio: "web", ConexaoID: channelStoreA.String()}}
	manager := NewSessoes(store)
	session, err := manager.Abrir(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := session.fixarCanal(canais.Canal{ID: channelStoreA, CompanyID: "adm-a"}, msg.Contato.Phone); err != nil {
		t.Fatal(err)
	}
	session.registrar(Turno{Quem: "morador", Texto: "primeira mensagem"})
	session.registrar(Turno{Quem: "concierge", Texto: "primeira resposta"})
	session.historico = []*schema.Message{{Role: schema.User, Content: "primeira mensagem"}, {Role: schema.Assistant, Content: "primeira resposta"}}
	session.mcpResultados = map[string]any{"recurso": map[string]any{"resultado": "preservado"}}
	session.Processando = true
	if err := manager.Persistir(ctx, session); err != nil {
		t.Fatal(err)
	}
	restored, ok, err := NewSessoes(store).Get(ctx, session.ID)
	if err != nil || !ok {
		t.Fatalf("restart=%v (%v)", ok, err)
	}
	if restored.Processando || len(restored.Turnos) != 2 || !reflect.DeepEqual(restored.historicoSnapshot(), session.historicoSnapshot()) || !reflect.DeepEqual(restored.resultadosMCP(), session.resultadosMCP()) {
		t.Fatalf("estado perdido no restart: %+v", restored)
	}
	manager = NewSessoes(store)
	continued, err := manager.Abrir(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}
	continued.registrar(Turno{Quem: "morador", Texto: "continuação"})
	if err := manager.Persistir(ctx, continued); err != nil {
		t.Fatal(err)
	}
	final, ok, err := NewSessoes(store).Get(ctx, session.ID)
	if err != nil || !ok || len(final.Turnos) != 3 {
		t.Fatalf("conversa reiniciada=%+v (%v)", final, err)
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
}
