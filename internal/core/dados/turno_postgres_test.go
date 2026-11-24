package dados

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/core/banco/bancotest"
	sessoespg "go-concierge/internal/core/dados/postgres/sessoes"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/pedidos"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

func TestGravacaoTurnoHTTPAtomicaEEscopoDoCanal(t *testing.T) {
	b := bancotest.Novo(t)
	r, contato, cart := registroValido(t)
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, r.ChannelID, r.CompanyID, r.CondoID, r.ChannelID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO catalog_files(path,content,sha256,updated_at) VALUES('agentes/test/caso.yaml','','test',now()); INSERT INTO use_cases(id,file_path,agent_path,name,summary,visibility,source_sha256,updated_at) VALUES('reserva','agentes/test/caso.yaml','agentes/test','Reserva','','PUBLIC','test',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO company_use_cases(id,company_id,use_case_id,enabled,default_available,updated_at) VALUES($1,$2,'reserva',true,true,now())`, uuid.New(), r.CompanyID); err != nil {
		t.Fatal(err)
	}
	cart.Pedidos = []*pedidos.Pedido{{Protocolo: "P-VALIDO", CompanyID: contato.CompanyID, Telefone: contato.Telefone, CondoID: contato.CondoID, Caso: "reserva", Estado: pedidos.Coletando, CriadoEm: time.Now(), AtualizadoEm: time.Now()}}
	servidor, cliente, err := Iniciar(NovoGravadorPostgres(b))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	linha := auditoria.Linha{CriadoEm: time.Now(), Origem: "concierge", Evento: "antes_de_responder", Gancho: "test", Decisao: "bloquear", AdministradoraID: r.CompanyID, CondominioID: "42", ChannelID: r.ChannelID.String(), SessaoID: r.ID, Protocolo: r.Protocolo}
	if err := cliente.Gravar(t.Context(), r, contato, cart, []auditoria.Linha{linha}); err != nil {
		t.Fatalf("gravação inicial: %v", err)
	}
	store := sessoespg.NovoSessaoPostgres(b)
	ctx := atende.ComEscopoSessao(t.Context(), r.CompanyID, r.ChannelID)
	antes, ok, err := store.Carregar(ctx, r.ID)
	if err != nil || !ok {
		t.Fatalf("sessão inicial: encontrada=%v erro=%v", ok, err)
	}
	var auditAntes int
	if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM hook_audit_logs`).Scan(&auditAntes); err != nil || auditAntes != 1 {
		t.Fatalf("auditoria inicial: %d (%v)", auditAntes, err)
	}
	var pedidosAntes int
	if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM requests`).Scan(&pedidosAntes); err != nil || pedidosAntes != 1 {
		t.Fatalf("pedidos iniciais: %d (%v)", pedidosAntes, err)
	}
	var estado map[string]any
	if err := json.Unmarshal(r.Dados, &estado); err != nil {
		t.Fatal(err)
	}
	estado["teste"] = "alteração que deve reverter"
	r.Dados, err = json.Marshal(estado)
	if err != nil {
		t.Fatal(err)
	}
	cart.Pedidos = []*pedidos.Pedido{{Protocolo: "P-INVALIDO", CompanyID: contato.CompanyID, Telefone: contato.Telefone, CondoID: contato.CondoID, Caso: "nao-ativado", Estado: pedidos.Coletando, CriadoEm: time.Now(), AtualizadoEm: time.Now()}}
	if err := cliente.Gravar(t.Context(), r, contato, cart, []auditoria.Linha{linha}); !errors.Is(err, atende.ErrGravacaoNaoAplicada) {
		t.Fatalf("pedido inválido: %v", err)
	}
	depois, ok, err := store.Carregar(ctx, r.ID)
	if err != nil || !ok || string(depois.Dados) != string(antes.Dados) {
		t.Fatalf("sessão alterada após rollback: encontrada=%v erro=%v", ok, err)
	}
	var auditDepois int
	if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM hook_audit_logs`).Scan(&auditDepois); err != nil || auditDepois != auditAntes {
		t.Fatalf("auditoria após rollback: %d (%v)", auditDepois, err)
	}
	var pedidosDepois int
	if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM requests`).Scan(&pedidosDepois); err != nil || pedidosDepois != pedidosAntes {
		t.Fatalf("pedidos após rollback: %d (%v)", pedidosDepois, err)
	}
	outroCanal := uuid.New()
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,'outra',$2,'WEB','test',$3,'INHERIT',true,'root',now())`, outroCanal, r.CondoID, outroCanal.String()); err != nil {
		t.Fatal(err)
	}
	r.ChannelID = outroCanal
	estado["channel_id"] = outroCanal
	r.Dados, err = json.Marshal(estado)
	if err != nil {
		t.Fatal(err)
	}
	cart.Pedidos = nil
	if err := cliente.Gravar(t.Context(), r, contato, cart, nil); !errors.Is(err, atende.ErrGravacaoRecusada) || !errors.Is(err, atende.ErrGravacaoNaoAplicada) {
		t.Fatalf("canal de outra empresa: %v", err)
	}
}
