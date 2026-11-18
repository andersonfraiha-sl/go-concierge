package atende

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/escopo"
	"go-concierge/internal/pedidos"
)

func canalSessaoV2(t *testing.T, b banco.Banco, company string, condo *int) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := b.Pool.Exec(t.Context(), `INSERT INTO channels(id,company_id,condo_id,channel_type,provider,endpoint_identifier,feature_mode,enabled,root_agent,updated_at) VALUES($1,$2,$3,'WEB','test',$4,'INHERIT',true,'root',now())`, id, company, condo, id.String())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func registroSessaoV2(t *testing.T, id, company string, channel uuid.UUID, condo *int) RegistroSessao {
	t.Helper()
	r := RegistroSessao{ID: id, CompanyID: company, ChannelID: channel, Connector: "test", Protocolo: "atendimento", UserPhone: "5511999998888", CondoID: condo, AtualizadaEm: time.Now().UTC()}
	r.Dados = serializarRegistroV2(t, r, "original")
	return r
}

func serializarRegistroV2(t *testing.T, r RegistroSessao, name string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{"id": r.ID, "company_id": r.CompanyID, "channel_id": r.ChannelID, "condo_id": r.CondoID, "protocolo": r.Protocolo, "atualizada_em": r.AtualizadaEm, "origem": map[string]string{"conector": r.Connector}, "contato": map[string]string{"phone": r.UserPhone, "name": name}, "escopo": escopo.Contato{CompanyID: r.CompanyID, CondoID: r.CondoID, Telefone: r.UserPhone}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestV2SessaoAutorizaEscopoEValidaDados(t *testing.T) {
	b := bancotest.Novo(t)
	store := NovoSessaoPostgres(b)
	alpha := 101
	channel := canalSessaoV2(t, b, "A", &alpha)
	adm := canalSessaoV2(t, b, "A", nil)
	other := canalSessaoV2(t, b, "B", nil)
	ctx := ComEscopoSessao(t.Context(), "A", channel)
	r := registroSessaoV2(t, "sessao-alpha", "A", channel, &alpha)
	if err := store.Salvar(ctx, r); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.Carregar(ctx, r.ID); err != nil || !ok {
		t.Fatalf("Carregar autorizado=%v (%v)", ok, err)
	}
	if _, ok, err := store.Carregar(ComEscopoSessao(t.Context(), "A", uuid.Nil), r.ID); err != nil || !ok {
		t.Fatalf("ADM=%v (%v)", ok, err)
	}
	for _, unauthorized := range []struct {
		company string
		channel uuid.UUID
	}{{"B", other}, {"A", adm}} {
		denied := ComEscopoSessao(t.Context(), unauthorized.company, unauthorized.channel)
		if _, ok, err := store.Carregar(denied, r.ID); err != nil || ok {
			t.Fatalf("Carregar estrangeiro=%v (%v)", ok, err)
		}
		if err := store.Remover(denied, r.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := store.Carregar(t.Context(), r.ID); !errors.Is(err, ErrEscopoSessao) {
		t.Fatalf("sem autorização=%v", err)
	}
	if err := store.Remover(t.Context(), r.ID); !errors.Is(err, ErrEscopoSessao) {
		t.Fatalf("excluir sem autorização=%v", err)
	}
	if _, ok, err := store.Carregar(ctx, r.ID); err != nil || !ok {
		t.Fatalf("sessão original perdida=%v (%v)", ok, err)
	}
	changed := registroSessaoV2(t, r.ID, "B", other, nil)
	if err := store.Salvar(ComEscopoSessao(t.Context(), "B", other), changed); err == nil {
		t.Fatal("reatribuição empresarial aceita")
	}
	changed = registroSessaoV2(t, r.ID, "A", adm, nil)
	if err := store.Salvar(ComEscopoSessao(t.Context(), "A", adm), changed); err == nil {
		t.Fatal("reatribuição de canal aceita")
	}
	for name, invalidScope := range map[string]escopo.Contato{
		"sem empresa":  {CondoID: &alpha, Telefone: r.UserPhone},
		"sem telefone": {CompanyID: r.CompanyID, CondoID: &alpha},
		"sem escopo":   {},
		"amplo":        {CompanyID: r.CompanyID, Telefone: r.UserPhone},
	} {
		t.Run(name, func(t *testing.T) {
			invalid := r
			var data map[string]any
			if err := json.Unmarshal(invalid.Dados, &data); err != nil {
				t.Fatal(err)
			}
			data["escopo"] = invalidScope
			invalid.Dados, _ = json.Marshal(data)
			if err := store.Salvar(ctx, invalid); !errors.Is(err, ErrRegistroSessao) {
				t.Fatalf("escopo privado inválido aceito: %v", err)
			}
		})
	}
	wrong := registroSessaoV2(t, "null-em-alpha", "A", channel, nil)
	if err := store.Salvar(ctx, wrong); err == nil {
		t.Fatal("NULL aceito no canal estrito")
	}
	wrong = r
	wrong.UserPhone = "5511000000000"
	if err := store.Salvar(ctx, wrong); !errors.Is(err, ErrRegistroSessao) {
		t.Fatalf("JSON incompatível=%v", err)
	}
	if _, err := b.Pool.Exec(t.Context(), `UPDATE sessions SET data=jsonb_set(data,'{company_id}','"B"') WHERE id=$1`, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Carregar(ctx, r.ID); !errors.Is(err, ErrRegistroSessao) {
		t.Fatalf("JSON corrompido aceito=%v", err)
	}
}

func TestV2ExcluirSessaoPreservaPedidoEOrigemParcial(t *testing.T) {
	b := bancotest.Novo(t)
	store := NovoSessaoPostgres(b)
	alpha := 101
	channel := canalSessaoV2(t, b, "A", &alpha)
	ctx := ComEscopoSessao(t.Context(), "A", channel)
	r := registroSessaoV2(t, "sessao-fonte", "A", channel, &alpha)
	if err := store.Salvar(ctx, r); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO catalog_files(path,content,sha256,updated_at) VALUES('agentes/test/caso.yaml','','test',now()); INSERT INTO use_cases(id,file_path,agent_path,name,summary,visibility,source_sha256,updated_at) VALUES('reserva','agentes/test/caso.yaml','agentes/test','Reserva','','PUBLIC','test',now())`); err != nil {
		t.Fatal(err)
	}
	caseID := uuid.New()
	if _, err := b.Pool.Exec(t.Context(), `INSERT INTO company_use_cases(id,company_id,use_case_id,enabled,default_available,updated_at) VALUES($1,'A','reserva',true,true,now())`, caseID); err != nil {
		t.Fatal(err)
	}
	contact, err := escopo.NovoContato("A", &alpha, r.UserPhone)
	if err != nil {
		t.Fatal(err)
	}
	requests := pedidos.NovoPostgres(b)
	request := &pedidos.Pedido{Protocolo: "P-PRESERVAR", CompanyID: "A", CondoID: &alpha, Telefone: r.UserPhone, Caso: "reserva", Estado: pedidos.Coletando, SourceSessionID: r.ID, CriadoEm: time.Now(), AtualizadoEm: time.Now()}
	if err := requests.Gravar(ctx, contact, pedidos.Carteira{Pedidos: []*pedidos.Pedido{request}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Remover(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	cart, err := requests.Ler(ctx, contact)
	if err != nil || len(cart.Pedidos) != 1 {
		t.Fatalf("pedido perdido após excluir sessão: %+v (%v)", cart, err)
	}
	if cart.Pedidos[0].SourceSessionID != "" || cart.Pedidos[0].CompanyID != "A" {
		t.Fatalf("origem parcial inválida=%+v", cart.Pedidos[0])
	}
}

func TestV2LoteReverteSessaoSePedidoInvalido(t *testing.T) {
	b := bancotest.Novo(t)
	store := NovoSessaoPostgres(b)
	channel := canalSessaoV2(t, b, "A", nil)
	ctx := ComEscopoSessao(t.Context(), "A", channel)
	r := registroSessaoV2(t, "sessao-atomica", "A", channel, nil)
	if err := store.Salvar(ctx, r); err != nil {
		t.Fatal(err)
	}
	changed := r
	changed.Dados = serializarRegistroV2(t, changed, "não pode persistir")
	lote := banco.NovoLote()
	store.SalvarNoLote(ctx, lote, changed)
	contact, err := escopo.NovoContato("A", nil, r.UserPhone)
	if err != nil {
		t.Fatal(err)
	}
	invalid := &pedidos.Pedido{Protocolo: "P-INVALIDO", Caso: "nao-ativado", Estado: pedidos.Coletando, CriadoEm: time.Now(), AtualizadoEm: time.Now()}
	if err := pedidos.NovoPostgres(b).GravarNoLote(ctx, lote, contact, pedidos.Carteira{Pedidos: []*pedidos.Pedido{invalid}}); err != nil {
		t.Fatal(err)
	}
	if err := lote.Enviar(ctx, b.Pool); err == nil {
		t.Fatal("lote inválido aceito")
	}
	persisted, ok, err := store.Carregar(ctx, r.ID)
	if err != nil || !ok || string(persisted.Dados) != string(r.Dados) {
		var before, after any
		json.Unmarshal(r.Dados, &before)
		json.Unmarshal(persisted.Dados, &after)
		beforeJSON, _ := json.Marshal(before)
		afterJSON, _ := json.Marshal(after)
		if err != nil || !ok || string(beforeJSON) != string(afterJSON) {
			t.Fatalf("sessão alterada após rollback: %s (%v)", persisted.Dados, err)
		}
	}
}
