package atende

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go-concierge/internal/admin"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/canais"
	"go-concierge/internal/entrada"
	"go-concierge/internal/escopo"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
	"go-concierge/internal/okf"
)

func TestRuntimeV2RelêCanalConteudoECasosPorTurno(t *testing.T) {
	b := bancotest.Novo(t)
	ctx := t.Context()
	channel := uuid.New()
	alpha, beta := 101, 202
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := b.Pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO channels(id,company_id,condo_id,name,channel_type,provider,endpoint_identifier,external_channel_id,feature_mode,enabled,root_agent,api_key,webhook_secret,config_jsonb,updated_at) VALUES($1,'Nexus-A',$2,'Alpha','WHATSAPP','ATENDE','55119000','tenant','INHERIT',true,'agentes/concierge','segredo-api','segredo-webhook','{"nexus_account_id":"account-externa","gruvi_company_id":"gruvi-externa"}',now())`, channel, alpha)
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := cat.ResolveAgente("agentes/concierge")
	if err != nil {
		t.Fatal(err)
	}
	def, err := cat.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var caso *okf.Agent
	for _, sub := range def.Subagentes {
		if sub.Caso != nil {
			caso = sub
			break
		}
	}
	if caso == nil {
		t.Fatal("catálogo sem caso")
	}
	file := "agentes/teste/caso.yaml"
	exec(`INSERT INTO catalog_files(path,content,sha256,updated_at) VALUES($1,'x','hash',now())`, file)
	exec(`INSERT INTO use_cases(id,file_path,agent_path,name,summary,visibility,source_sha256,updated_at) VALUES($1,$2,$3,'Caso','Resumo','PUBLIC','hash',now())`, caso.Caso.ID, file, caso.Rel)
	feature := uuid.New()
	exec(`INSERT INTO company_use_cases(id,company_id,use_case_id,enabled,default_available,updated_at) VALUES($1,'Nexus-A',$2,true,true,now())`, feature, caso.Caso.ID)
	exec(`INSERT INTO guidelines(scope_type,company_id,condo_id,general_text) VALUES('GLOBAL',NULL,NULL,'base global'),('COMPANY','Nexus-A',NULL,'conteúdo empresa'),('CONDO','Nexus-A',$1,'conteúdo Alpha'),('CONDO','Nexus-A',$2,'SEGREDO BETA')`, alpha, beta)
	mem := memoria.NovoPostgres(b)
	for _, id := range []*int{nil, &alpha, &beta} {
		c, err := escopo.NovoContato("Nexus-A", id, telefoneMorador)
		if err != nil {
			t.Fatal(err)
		}
		texto := "SEGREDO NULL"
		if id != nil {
			if *id == alpha {
				texto = "fato Alpha"
			} else {
				texto = "SEGREDO BETA"
			}
		}
		if err := mem.Gravar(ctx, c, memoria.Memoria{CompanyID: "Nexus-A", Telefone: c.Telefone, Fatos: []memoria.Fato{{Chave: "contexto", Texto: texto, CondoID: id, Quando: time.Now()}}}); err != nil {
			t.Fatal(err)
		}
	}
	svc := &Servico{Canais: canais.NovoPostgres(b.Pool), Cat: cat, Agente: "agentes/concierge", Mapeamento: admin.NovoPostgres(b.Ent), Sessoes: NewSessoes(NovoSessaoPostgres(b)), Memorias: mem, Config: harness.Config{Mock: true, Env: func(string) string { return "" }}}
	msg := entrada.Mensagem{ID: "primeiro", ConversaID: "protocolo", Provider: "ATENDE", ChannelType: "WHATSAPP", Tenant: "tenant", Endpoint: "55119000", Contato: entrada.Contato{Phone: telefoneMorador}, Origem: entrada.Origem{Conector: "atende", Meio: "whatsapp"}, Destino: entrada.Destino{Conector: "atende"}, Texto: "bom dia"}
	ctx, msg, err = svc.autorizarMensagem(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}
	_, sess, err := svc.PrepararMensagem(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}
	ag, err := svc.agenteDaSessao(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	if ag.Subagentes[caso.Nome] == nil || contarCasos(ag) != 1 {
		t.Fatalf("casos iniciais: %+v", ag.Subagentes)
	}
	if !strings.Contains(ag.Instrucao, "fato Alpha") || strings.Contains(ag.Instrucao, "SEGREDO") || !strings.Contains(ag.Instrucao, "conteúdo empresa") {
		t.Fatalf("contexto incorreto: %s", ag.Instrucao)
	}
	c, err := svc.Canais.PorID(ctx, "Nexus-A", channel)
	if err != nil {
		t.Fatal(err)
	}
	m, err := svc.conteudoDoCanal(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, _, err := svc.montar(ctx, sess, m, condominioDoCanal(c))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Identidade.AccountID != "account-externa" {
		t.Fatal("account externa foi confundida com empresa Nexus")
	}
	exec(`UPDATE company_use_cases SET enabled=false WHERE id=$1`, feature)
	exec(`UPDATE guidelines SET general_text='Alpha atualizada' WHERE scope_type='CONDO' AND condo_id=$1`, alpha)
	ag, err = svc.agenteDaSessao(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	if contarCasos(ag) != 0 || !strings.Contains(ag.Instrucao, "Alpha atualizada") {
		t.Fatal("próximo turno não refletiu alteração SQL")
	}
	if err := svc.Sessoes.Persistir(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := svc.Sessoes.Get(ComEscopoSessao(ctx, "Outra", channel), sess.ID); !errors.Is(err, ErrEscopoSessao) || ok {
		t.Fatalf("cache vazou sessão: ok=%v err=%v", ok, err)
	}
	r, err := sess.Registro()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(r.Dados), "segredo-") {
		t.Fatal("credenciais vazaram na sessão")
	}
	if err := ValidarRegistroSessao(r); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE channels SET enabled=false WHERE id=$1`, channel)
	msg.ID = "apos-desativacao"
	if _, err := svc.Processar(ctx, msg); !errors.Is(err, canais.ErrDesabilitado) {
		t.Fatalf("turno em canal desabilitado: %v", err)
	}
	if _, err := svc.contextoDaSessao(ctx, sess); !errors.Is(err, canais.ErrDesabilitado) {
		t.Fatalf("retomada em canal desabilitado: %v", err)
	}
	if _, err := svc.IdentificarCanal(ctx, "Nexus-A", channel, telefoneMorador); !errors.Is(err, canais.ErrDesabilitado) {
		t.Fatalf("identidade em canal desabilitado: %v", err)
	}
	nova := entrada.Mensagem{ChannelID: channel, CompanyID: "Nexus-A", ConversaID: "nova-desabilitada", Origem: entrada.Origem{Conector: "atende-emulado"}, Contato: entrada.Contato{Phone: telefoneMorador}}
	if _, err := svc.Processar(ctx, nova); !errors.Is(err, canais.ErrDesabilitado) {
		t.Fatalf("emulador em canal desabilitado: %v", err)
	}
	lista, err := svc.Sessoes.Lista(ComEscopoSessao(ctx, "Nexus-A", uuid.Nil), "Nexus-A")
	if err != nil || len(lista) != 1 || lista[0].ID != sess.ID {
		t.Fatalf("canal desabilitado abriu sessão: %d (%v)", len(lista), err)
	}
}

func contarCasos(ag *harness.Agente) int {
	n := 0
	for _, sub := range ag.Subagentes {
		if sub.Def.Caso != nil {
			n++
		}
	}
	return n
}

func TestCanalEstritoFiltraIdentidadeERecusaFerramentaBeta(t *testing.T) {
	alpha := 101
	c := canais.Canal{ID: uuid.New(), CompanyID: "Nexus", CondoID: &alpha, Integracoes: canais.Integracoes{GruviCompanyID: "gruvi", GruviCondominiumID: "external-alpha"}}
	ctx := context.WithValue(t.Context(), chaveCanal{}, c)
	var dados any
	if err := json.Unmarshal([]byte(`[{"condominium":{"id":"external-alpha","name":"Alpha"},"units":[{"id":"a","resident":{"mobile_number":"5519981621916"}}]},{"condominium":{"id":"external-beta","name":"SEGREDO BETA"},"units":[{"id":"b","resident":{"mobile_number":"5519981621916"}}]}]`), &dados); err != nil {
		t.Fatal(err)
	}
	filtrado, err := restringirIdentidadeGruvi(ctx, dados)
	if err != nil {
		t.Fatal(err)
	}
	id, cache := gruvi.IdentidadeComDadosVinculadosMCP(telefoneMorador, "gruvi", filtrado)
	raw, _ := json.Marshal(id)
	if strings.Contains(string(raw), "BETA") || len(id.Units) != 1 || id.Units[0].ID != "a" {
		t.Fatalf("identidade vazou: %s", raw)
	}
	validar := validarFerramentaDoCanal(c, id, map[string]any{ferramentaEmpresa: cache})
	if err := validar("gruvi.bff.building-operation.fetch-unit-packages-controller-handle", map[string]any{"unitId": "b", "companyId": "gruvi"}); err == nil {
		t.Fatal("unidade Beta autorizada")
	}
	if err := validar("gruvi.bff.building-operation.fetch-unit-packages-controller-handle", map[string]any{"unitId": "a", "companyId": "gruvi"}); err != nil {
		t.Fatal(err)
	}
	if err := validar("nexus.tool", nil); err == nil {
		t.Fatal("tool sem comprovação de recorte autorizada")
	}
	if err := validar("gruvi.bff.operacao-sem-contrato", map[string]any{"unitId": "a", "companyId": "gruvi"}); err == nil {
		t.Fatal("operação sem contrato aceita")
	}
	if err := validar("gruvi.bff.building-operation.fetch-unit-packages-controller-handle", map[string]any{"unitId": "a", "companyId": "outra"}); err == nil {
		t.Fatal("empresa divergente aceita")
	}
	c.Integracoes.GruviCondominiumID = ""
	if _, err := restringirIdentidadeGruvi(context.WithValue(ctx, chaveCanal{}, c), dados); err == nil {
		t.Fatal("contexto sem vínculo aceito")
	}
}

func TestEnvioETransferenciaV2ReleemCredenciaisDoCanal(t *testing.T) {
	b := bancotest.Novo(t)
	ctx := t.Context()
	var mu sync.Mutex
	var recebidas []string
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		recebidas = append(recebidas, r.URL.Path+":"+r.Header.Get("x-api-key"))
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))
	defer servidor.Close()
	a, outro := uuid.New(), uuid.New()
	for i, id := range []uuid.UUID{a, outro} {
		chave := []string{"chave-a", "chave-b"}[i]
		if _, err := b.Pool.Exec(ctx, `INSERT INTO channels(id,company_id,name,channel_type,provider,endpoint_identifier,external_channel_id,feature_mode,enabled,root_agent,api_base_url,api_key,updated_at) VALUES($1,'Nexus','Canal','WHATSAPP','ATENDE',$2,'tenant','INHERIT',true,'agentes/concierge',$3,$4,now())`, id, id.String(), servidor.URL, chave); err != nil {
			t.Fatal(err)
		}
	}
	repo := canais.NovoPostgres(b.Pool)
	sender, transferidor := SenderPorCanal{Canais: repo}, TransferidorPorCanal{Canais: repo}
	destino := entrada.Destino{CompanyID: "Nexus", ChannelID: a, Telefone: "5511999999999", Protocolo: "P-1"}
	if err := sender.Enviar(ctx, destino, "primeira"); err != nil {
		t.Fatal(err)
	}
	destino.ChannelID = outro
	if err := transferidor.Transferir(ctx, destino, "atendente"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Pool.Exec(ctx, `UPDATE channels SET api_key='chave-atualizada' WHERE id=$1`, a); err != nil {
		t.Fatal(err)
	}
	destino.ChannelID = a
	if err := sender.Enviar(ctx, destino, "segunda"); err != nil {
		t.Fatal(err)
	}
	if err := transferidor.Transferir(ctx, destino, "atendente"); err != nil {
		t.Fatal(err)
	}
	destino.CompanyID = "Outra"
	if err := sender.Enviar(ctx, destino, "não deve sair"); err == nil {
		t.Fatal("envio de outra empresa aceito")
	}
	if err := transferidor.Transferir(ctx, destino, "não deve sair"); err == nil {
		t.Fatal("transferência de outra empresa aceita")
	}
	if _, err := b.Pool.Exec(ctx, `UPDATE channels SET enabled=false WHERE id=$1`, a); err != nil {
		t.Fatal(err)
	}
	destino.CompanyID = "Nexus"
	if err := sender.Enviar(ctx, destino, "não deve sair"); err == nil {
		t.Fatal("canal desabilitado aceito")
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"/integrations/messages:chave-a", "/protocols/P-1/transfer:chave-b", "/integrations/messages:chave-atualizada", "/protocols/P-1/transfer:chave-atualizada"}
	if strings.Join(recebidas, "|") != strings.Join(want, "|") {
		t.Fatalf("requisições = %v, want %v", recebidas, want)
	}
}

func TestAliasDeSessaoV2ResolveDentroDaEmpresa(t *testing.T) {
	sessoes := NewSessoes(NovaSessaoEmMemoria())
	for _, company := range []string{"A", "B"} {
		sess := &Sessao{ID: company + "-id", CompanyID: company, ChannelID: uuid.New(), Protocolo: "mesmo-protocolo"}
		sessoes.adotar(sess)
	}
	for _, company := range []string{"A", "B"} {
		sess, ok, err := sessoes.Get(ComEscopoSessao(t.Context(), company, uuid.Nil), "mesmo-protocolo")
		if err != nil || !ok || sess.CompanyID != company {
			t.Fatalf("empresa %s: sess=%+v ok=%v err=%v", company, sess, ok, err)
		}
	}
}
