package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/admin"
	"go-concierge/internal/escopo"
)

// ambienteDeMapeamento troca o repositório do ambiente de integração por um Postgres real com
// o cache do turno ligado a ele, como no main: adm-a e adm-b, cond-a (de adm-a) e cond-b (de
// adm-b). O cache sobe aquecido, então uma escrita que não o atinge deixa o turno com o valor
// velho.
func ambienteDeMapeamento(t *testing.T) (ambienteRotas, *admin.Postgres, *admin.Cache) {
	t.Helper()
	amb := ambienteDeIntegracao(t)
	pg := admin.NovoPostgres(amb.s.banco.Ent)
	cache := admin.NovoCache(pg)
	ctx := t.Context()
	for _, a := range []admin.Administradora{{ID: "adm-a", Nome: "A"}, {ID: "adm-b", Nome: "B"}} {
		if _, err := pg.SalvarAdministradora(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []admin.Condominio{{ID: "cond-a", Nome: "Cond A", Administradora: "adm-a"}, {ID: "cond-b", Nome: "Cond B", Administradora: "adm-b", CompanyID: "company-b"}} {
		if _, err := pg.SalvarCondominio(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	amb.s.admin, amb.s.atende.Mapeamento = pg, cache
	if _, err := cache.Ler(ctx); err != nil {
		t.Fatal(err)
	}
	return amb, pg, cache
}

func chamarIntegracao(amb ambienteRotas, metodo, url, corpo string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(metodo, url, strings.NewReader(corpo))
	r.Header.Set("Authorization", "Bearer "+chaveDeTeste)
	w := httptest.NewRecorder()
	amb.handler.ServeHTTP(w, r)
	return w
}

func putMapeamento(amb ambienteRotas, query, corpo string) *httptest.ResponseRecorder {
	return chamarIntegracao(amb, http.MethodPut, "/integracao/v1/mapeamento"+query, corpo)
}

func lerMapeamento(t *testing.T, repo interface {
	Ler(context.Context) (admin.Mapeamento, error)
}) admin.Mapeamento {
	t.Helper()
	m, err := repo.Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func igual(t *testing.T, a, b admin.Mapeamento) bool {
	t.Helper()
	a.AtualizadoEm, b.AtualizadoEm = time.Time{}, time.Time{}
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	if err := errors.Join(errA, errB); err != nil {
		t.Fatal(err)
	}
	return string(ja) == string(jb)
}

type respostaDoLote struct {
	Aplicados int    `json:"aplicados"`
	Falhou    string `json:"falhou"`
	Codigo    string `json:"codigo"`
}

func lerResposta(t *testing.T, w *httptest.ResponseRecorder) respostaDoLote {
	t.Helper()
	var r respostaDoLote
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("resposta %q: %v", w.Body.String(), err)
	}
	return r
}

const loteCompleto = `{
  "administradoras": [{"id": "adm-x", "nome": "X"}],
  "condominios": [{"id": "cond-x", "nome": "Um", "administradora_id": "adm-x", "account": "acc-x"}],
  "habilitacoes": {"cond-x": {"boleto": {"habilitado": false, "papeis": ["morador"]}}},
  "orientacoes": {"cond-x": {"geral": "Falar com carinho", "por_caso": {"boleto": "Só segunda via"}}},
  "payloads": {"cond-x": {"recurso.nome": {"chave": "valor"}}}
}`

func TestMapeamentoExigeChaveDeIntegracao(t *testing.T) {
	amb, _, _ := ambienteDeMapeamento(t)
	casos := []struct {
		nome, autorizacao string
		quer              int
	}{
		{"sem cabeçalho", "", http.StatusUnauthorized},
		{"chave errada", "Bearer outra-chave-qualquer-de-32-caracteres", http.StatusUnauthorized},
		{"sem o esquema Bearer", chaveDeTeste, http.StatusUnauthorized},
		{"chave certa", "Bearer " + chaveDeTeste, http.StatusOK},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPut, "/integracao/v1/mapeamento", strings.NewReader(loteCompleto))
			r.Header.Set("X-Dev-Usuario", "admin") // usuário válido não abre a rota sem a chave
			if c.autorizacao != "" {
				r.Header.Set("Authorization", c.autorizacao)
			}
			w := httptest.NewRecorder()
			amb.handler.ServeHTTP(w, r)
			if w.Code != c.quer {
				t.Fatalf("status %d, quer %d: %s", w.Code, c.quer, w.Body.String())
			}
		})
	}
}

func TestRotasDeEscritaNaoExistemSemChaveConfigurada(t *testing.T) {
	amb := novoAmbienteRotas(t)
	for _, rota := range []struct{ metodo, url string }{
		{http.MethodPut, "/integracao/v1/mapeamento"},
		{http.MethodGet, "/integracao/v1/memorias?administradora=adm-a"},
		{http.MethodGet, "/integracao/v1/memoria?administradora=adm-a&phone=5519981621916"},
		{http.MethodDelete, "/integracao/v1/memoria?administradora=adm-a&phone=5519981621916"},
		{http.MethodGet, "/integracao/v1/pedidos?administradora=adm-a&phone=5519981621916"},
	} {
		t.Run(rota.metodo+" "+rota.url, func(t *testing.T) {
			if w := chamarIntegracao(amb, rota.metodo, rota.url, "{}"); w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status %d: rota de integração aberta sem chave configurada", w.Code)
			}
		})
	}
}

// A escrita da integração tem de chegar ao cache que o turno lê, sem reiniciar: o Cache sobe
// aquecido, então se a escrita não o atingisse ele devolveria o mapeamento de antes.
func TestMapeamentoGravaEChegaAoCacheDoTurnoEAoPacote(t *testing.T) {
	amb, pg, cache := ambienteDeMapeamento(t)
	if w := putMapeamento(amb, "", loteCompleto); w.Code != http.StatusOK || lerResposta(t, w).Aplicados != 5 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	doTurno, err := cache.Ler(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	doBanco := lerMapeamento(t, pg)
	if !igual(t, doTurno, doBanco) {
		t.Fatalf("cache do turno difere do banco:\ncache %+v\nbanco %+v", doTurno, doBanco)
	}
	condo, ok := doTurno.Condominio("cond-x")
	if !ok || condo.Administradora != "adm-x" || condo.Account != "acc-x" {
		t.Fatalf("condomínio no cache do turno: %+v", condo)
	}
	if h := doTurno.Habilitacoes["cond-x"]["boleto"]; h.Habilitado || len(h.Papeis) != 1 {
		t.Errorf("habilitação no cache do turno: %+v", h)
	}
	if o := doTurno.OrientacaoDe("cond-x"); o.Geral != "Falar com carinho" || o.PorCaso["boleto"] != "Só segunda via" {
		t.Errorf("orientação no cache do turno: %+v", o)
	}
	if p := doTurno.PayloadsDe("cond-x")["recurso.nome"]["chave"]; p != "valor" {
		t.Errorf("payload no cache do turno: %v", p)
	}

	var p pacote
	w := pedirPacote(amb, "/integracao/v1/pacote?administradora=adm-x", "Bearer "+chaveDeTeste)
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Mapeamento.Condominios) != 1 || p.Mapeamento.Condominios[0].ID != "cond-x" {
		t.Errorf("a leitura seguinte do pacote não traz a escrita: %+v", p.Mapeamento.Condominios)
	}
}

func TestMapeamentoReenviarOLoteNaoMudaNada(t *testing.T) {
	amb, pg, cache := ambienteDeMapeamento(t)
	lote := `{"administradoras": [{"id": "adm-a", "nome": "A renomeada"}],
	  "condominios": [{"id": "cond-a", "nome": "Cond A", "administradora_id": "adm-a", "account": "acc-a"}, {"id": "cond-novo", "nome": "Novo", "administradora_id": "adm-a"}],
	  "habilitacoes": {"cond-a": {"boleto": {"habilitado": true}}, "*": {"reserva": {"habilitado": false}}},
	  "orientacoes": {"cond-a": {"geral": "oi"}},
	  "payloads": {"cond-a": {"r": {"a": 1}}},
	  "remover": {"condominios": ["cond-b"]}}`
	primeira := putMapeamento(amb, "", lote)
	if primeira.Code != http.StatusOK {
		t.Fatalf("primeiro envio: status %d: %s", primeira.Code, primeira.Body.String())
	}
	depoisDoPrimeiro := lerMapeamento(t, pg)

	segunda := putMapeamento(amb, "", lote)
	if segunda.Code != http.StatusOK || lerResposta(t, segunda).Aplicados != lerResposta(t, primeira).Aplicados {
		t.Fatalf("reenvio: status %d: %s", segunda.Code, segunda.Body.String())
	}
	if depoisDoSegundo := lerMapeamento(t, pg); !igual(t, depoisDoPrimeiro, depoisDoSegundo) {
		t.Fatalf("reenviar mudou o mapeamento:\n%+v\n%+v", depoisDoPrimeiro, depoisDoSegundo)
	}
	if noTurno, err := cache.Ler(t.Context()); err != nil || !igual(t, noTurno, depoisDoPrimeiro) {
		t.Fatalf("cache do turno após o reenvio: %v %+v", err, noTurno)
	}
	if _, existe := depoisDoPrimeiro.Condominio("cond-b"); existe {
		t.Error("cond-b devia ter sido removido")
	}
}

func TestMapeamentoRecusaForaDoRecorteSemGravarNada(t *testing.T) {
	casos := []struct {
		nome, corpo string
	}{
		{"outra administradora na lista", `{"administradoras": [{"id": "adm-b", "nome": "B"}]}`},
		{"condomínio de outra administradora", `{"condominios": [{"id": "cond-novo", "nome": "N", "administradora_id": "adm-b"}]}`},
		{"condomínio sem dona", `{"condominios": [{"id": "cond-novo", "nome": "N"}]}`},
		{"tomar condomínio existente de outra", `{"condominios": [{"id": "cond-b", "nome": "Tomado", "administradora_id": "adm-a", "company_id": "company-b"}]}`},
		{"chave de roteamento nova", `{"condominios": [{"id": "cond-novo", "nome": "N", "administradora_id": "adm-a", "company_id": "outra"}]}`},
		{"habilitação do padrão", `{"habilitacoes": {"*": {"boleto": {"habilitado": false}}}}`},
		{"habilitação de condomínio de outra", `{"habilitacoes": {"cond-b": {"boleto": {"habilitado": false}}}}`},
		{"orientação de condomínio de outra", `{"orientacoes": {"cond-b": {"geral": "x"}}}`},
		{"payload de condomínio de outra", `{"payloads": {"cond-b": {"r": {"a": 1}}}}`},
		{"payload de condomínio desconhecido", `{"payloads": {"nao-existe": {"r": {"a": 1}}}}`},
		{"remover condomínio de outra", `{"remover": {"condominios": ["cond-b"]}}`},
		{"remover administradora de outra", `{"remover": {"administradoras": ["adm-b"]}}`},
		{"remover habilitação do padrão", `{"remover": {"habilitacoes": {"*": ["boleto"]}}}`},
		{"item bom seguido de ruim", `{"condominios": [{"id": "cond-novo", "nome": "N", "administradora_id": "adm-a"}], "habilitacoes": {"cond-b": {"boleto": {}}}}`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			amb, pg, _ := ambienteDeMapeamento(t)
			antes := lerMapeamento(t, pg)
			w := putMapeamento(amb, "?administradora=adm-a", c.corpo)
			if w.Code != http.StatusForbidden || lerResposta(t, w).Codigo != "FORA_DO_RECORTE" {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if depois := lerMapeamento(t, pg); !igual(t, antes, depois) {
				t.Fatalf("a recusa gravou algo:\n%+v\n%+v", antes, depois)
			}
		})
	}
}

func TestMapeamentoAceitaODentroDoRecorte(t *testing.T) {
	amb, pg, cache := ambienteDeMapeamento(t)
	lote := `{"administradoras": [{"id": "adm-a", "nome": "A nova"}],
	  "condominios": [{"id": "cond-a", "nome": "Cond A", "administradora_id": "adm-a", "account": "acc"}, {"id": "cond-novo", "nome": "Novo", "administradora_id": "adm-a"}],
	  "habilitacoes": {"cond-novo": {"boleto": {"habilitado": true}}},
	  "orientacoes": {"cond-a": {"geral": "oi"}},
	  "payloads": {"cond-a": {"r": {"a": 1}}},
	  "remover": {"habilitacoes": {"cond-novo": ["inexistente"]}}}`
	w := putMapeamento(amb, "?administradora=adm-a", lote)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	m := lerMapeamento(t, pg)
	if a, _ := m.Administradora("adm-a"); a.Nome != "A nova" {
		t.Errorf("administradora: %+v", a)
	}
	if b, _ := m.Administradora("adm-b"); b.Nome != "B" {
		t.Errorf("a administradora de fora mudou: %+v", b)
	}
	if noTurno, _ := cache.Ler(t.Context()); !igual(t, noTurno, m) {
		t.Error("cache do turno difere do banco")
	}
	if w := putMapeamento(amb, "?administradora=adm-a", lote); w.Code != http.StatusOK {
		t.Fatalf("reenvio dentro do recorte: status %d: %s", w.Code, w.Body.String())
	}
}

func TestMapeamentoReenviarRemocaoDentroDoRecorteContinuaValendo(t *testing.T) {
	amb, _, _ := ambienteDeMapeamento(t)
	lote := `{"remover": {"habilitacoes": {"cond-a": ["boleto"]}, "condominios": ["cond-a"]}}`
	for i := range 2 {
		if w := putMapeamento(amb, "?administradora=adm-a", lote); w.Code != http.StatusOK {
			t.Fatalf("envio %d: status %d: %s", i+1, w.Code, w.Body.String())
		}
	}
}

func TestMapeamentoRecorteDaImplantacaoFixaVale(t *testing.T) {
	amb, pg, _ := ambienteDeMapeamento(t)
	amb.s.implantacao = escopo.Implantacao{AdministradoraFixa: "adm-a"}
	proibido := `{"habilitacoes": {"cond-b": {"boleto": {}}}}`
	for _, query := range []string{"", "?administradora=adm-b"} { // a fixa vence o parâmetro
		t.Run("query "+query, func(t *testing.T) {
			if w := putMapeamento(amb, query, proibido); w.Code != http.StatusForbidden {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
		})
	}
	if w := putMapeamento(amb, "", `{"orientacoes": {"cond-a": {"geral": "oi"}}}`); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if lerMapeamento(t, pg).Orientacoes["cond-a"].Geral != "oi" {
		t.Error("orientação de cond-a não gravada")
	}
}

func TestMapeamentoRecusaLoteInvalidoSemGravarNada(t *testing.T) {
	grande := make([]string, maximoDeItensDoLote+1)
	for i := range grande {
		grande[i] = fmt.Sprintf(`{"id": "c%d", "nome": "N", "administradora_id": "adm-a"}`, i)
	}
	casos := []struct {
		nome, corpo string
		quer        int
		codigo      string
	}{
		{"JSON quebrado", `{`, http.StatusBadRequest, ""},
		{"corpo vazio", ``, http.StatusBadRequest, ""},
		{"campo desconhecido", `{"administradora": []}`, http.StatusBadRequest, ""},
		{"corpo acima do teto", `{"administradoras": [{"id": "x", "nome": "` + strings.Repeat("a", tetoDoLoteDeMapeamento) + `"}]}`, http.StatusRequestEntityTooLarge, ""},
		{"id inválido depois de um item bom", `{"administradoras": [{"id": "adm-ok", "nome": "Ok"}, {"id": "Inválido!", "nome": "X"}]}`, http.StatusBadRequest, "LOTE_INVALIDO"},
		{"nome vazio", `{"condominios": [{"id": "cond-novo", "nome": " ", "administradora_id": "adm-a"}]}`, http.StatusBadRequest, "LOTE_INVALIDO"},
		{"administradora desconhecida", `{"condominios": [{"id": "cond-novo", "nome": "N", "administradora_id": "adm-nao-existe"}]}`, http.StatusBadRequest, "LOTE_INVALIDO"},
		{"habilitação de condomínio desconhecido", `{"habilitacoes": {"nao-existe": {"boleto": {}}}}`, http.StatusBadRequest, "LOTE_INVALIDO"},
		{"id repetido", `{"administradoras": [{"id": "adm-x", "nome": "X"}, {"id": "ADM-X", "nome": "Y"}]}`, http.StatusBadRequest, "LOTE_INVALIDO"},
		{"lote grande demais", `{"condominios": [` + strings.Join(grande, ",") + `]}`, http.StatusBadRequest, "LOTE_GRANDE_DEMAIS"},
		{"administradora com formato inválido no recorte", ``, http.StatusBadRequest, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			amb, pg, _ := ambienteDeMapeamento(t)
			antes := lerMapeamento(t, pg)
			query := ""
			if strings.Contains(c.nome, "formato inválido") {
				query = "?administradora=../x"
			}
			w := putMapeamento(amb, query, c.corpo)
			if w.Code != c.quer || (c.codigo != "" && lerResposta(t, w).Codigo != c.codigo) {
				t.Fatalf("status %d, quer %d %s: %.200s", w.Code, c.quer, c.codigo, w.Body.String())
			}
			if depois := lerMapeamento(t, pg); !igual(t, antes, depois) {
				t.Fatalf("a recusa gravou algo")
			}
		})
	}
}

func TestMapeamentoRemoverAdministradoraComCondominioEConflito(t *testing.T) {
	amb, pg, _ := ambienteDeMapeamento(t)
	antes := lerMapeamento(t, pg)
	w := putMapeamento(amb, "", `{"remover": {"administradoras": ["adm-a"]}}`)
	r := lerResposta(t, w)
	if w.Code != http.StatusConflict || r.Codigo != "CONFLITO_DE_REGRA" || r.Falhou != "remover.administradoras[adm-a]" || r.Aplicados != 0 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if !igual(t, antes, lerMapeamento(t, pg)) {
		t.Error("a recusa gravou algo")
	}
}

// repoQueFalha deixa passar o ensaio (que usa a cópia em memória) e falha no passo real: é o
// banco caindo no meio do lote.
type repoQueFalha struct {
	admin.Repositorio
	erro error
	no   string // administradora cujo SalvarAdministradora falha; vazio = nunca
}

func (r *repoQueFalha) SalvarAdministradora(ctx context.Context, a admin.Administradora) (admin.Mapeamento, error) {
	if r.no != "" && a.ID == r.no {
		return admin.Mapeamento{}, r.erro
	}
	return r.Repositorio.SalvarAdministradora(ctx, a)
}

func TestMapeamentoFalhaNoMeioDizOQueFoiAplicadoEReenviarCompleta(t *testing.T) {
	lote := `{"administradoras": [{"id": "adm-1", "nome": "Um"}, {"id": "adm-2", "nome": "Dois"}, {"id": "adm-3", "nome": "Três"}]}`
	casos := []struct {
		nome       string
		erro       error
		quer       int
		codigo     string
		reenviavel bool
	}{
		{"banco indisponível", errors.New("conexão perdida"), http.StatusInternalServerError, "ERRO_INTERNO", true},
		{"conflito com outra escrita", fmt.Errorf("salvar: %w", admin.ErrConflitoConcorrente), http.StatusServiceUnavailable, codigoConflitoConcorrente, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			amb, pg, _ := ambienteDeMapeamento(t)
			falhando := &repoQueFalha{Repositorio: pg, erro: c.erro, no: "adm-2"}
			amb.s.admin = falhando

			w := putMapeamento(amb, "", lote)
			r := lerResposta(t, w)
			if w.Code != c.quer || r.Codigo != c.codigo || r.Aplicados != 1 || r.Falhou != "administradoras[adm-2]" {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), c.erro.Error()) {
				t.Errorf("a resposta expõe o erro interno: %s", w.Body.String())
			}
			m := lerMapeamento(t, pg)
			if _, ok := m.Administradora("adm-1"); !ok {
				t.Error("o item anterior à falha devia estar aplicado")
			}
			if _, ok := m.Administradora("adm-3"); ok {
				t.Error("o item depois da falha não devia ter sido aplicado")
			}

			falhando.no = ""
			if w := putMapeamento(amb, "", lote); w.Code != http.StatusOK || lerResposta(t, w).Aplicados != 3 {
				t.Fatalf("reenvio: status %d: %s", w.Code, w.Body.String())
			}
			if _, ok := lerMapeamento(t, pg).Administradora("adm-3"); !ok {
				t.Error("o reenvio devia completar o lote")
			}
		})
	}
}

func TestConferirRecorte(t *testing.T) {
	atual := admin.Mapeamento{
		Administradoras: []admin.Administradora{{ID: "adm-a"}, {ID: "adm-b"}},
		Condominios: []admin.Condominio{
			{ID: "cond-a", Administradora: "adm-a", CompanyID: "ca", Tenant: "ta"},
			{ID: "cond-b", Administradora: "adm-b"},
		},
	}
	casos := []struct {
		nome string
		lote loteDeMapeamento
		quer bool // dentro do recorte de adm-a
	}{
		{"lote vazio", loteDeMapeamento{}, true},
		{"edita o seu mantendo as chaves", loteDeMapeamento{Condominios: []admin.Condominio{{ID: "cond-a", Administradora: "adm-a", CompanyID: "ca", Tenant: "ta"}}}, true},
		{"id com caixa alta ainda é o seu", loteDeMapeamento{Condominios: []admin.Condominio{{ID: " COND-A ", Administradora: "adm-a", CompanyID: "ca", Tenant: "ta"}}}, true},
		{"muda a company do seu", loteDeMapeamento{Condominios: []admin.Condominio{{ID: "cond-a", Administradora: "adm-a", CompanyID: "outra", Tenant: "ta"}}}, false},
		{"apaga a company do seu", loteDeMapeamento{Condominios: []admin.Condominio{{ID: "cond-a", Administradora: "adm-a"}}}, false},
		{"toma o de outra", loteDeMapeamento{Condominios: []admin.Condominio{{ID: "cond-b", Administradora: "adm-a"}}}, false},
		{"entrega o seu a outra", loteDeMapeamento{Condominios: []admin.Condominio{{ID: "cond-a", Administradora: "adm-b", CompanyID: "ca", Tenant: "ta"}}}, false},
		{"habilita no condomínio criado no próprio lote", loteDeMapeamento{
			Condominios:  []admin.Condominio{{ID: "cond-n", Administradora: "adm-a"}},
			Habilitacoes: map[string]map[string]admin.Habilitacao{"cond-n": {"boleto": {}}},
		}, true},
		{"habilita no condomínio que o lote entrega a outra", loteDeMapeamento{
			Condominios:  []admin.Condominio{{ID: "cond-a", Administradora: "adm-b", CompanyID: "ca", Tenant: "ta"}},
			Habilitacoes: map[string]map[string]admin.Habilitacao{"cond-a": {"boleto": {}}},
		}, false},
		{"remove condomínio que não existe", loteDeMapeamento{Remover: remocoesDoLote{Condominios: []string{"cond-fantasma"}}}, true},
		{"remove a própria administradora", loteDeMapeamento{Remover: remocoesDoLote{Administradoras: []string{"adm-a"}}}, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			err := conferirRecorte(c.lote, atual, "adm-a")
			if dentro := err == nil; dentro != c.quer {
				t.Fatalf("dentro do recorte = %v, quer %v (%v)", dentro, c.quer, err)
			}
			if e, ok := errors.AsType[erroDeAlvo](err); err != nil && (!ok || e.status != http.StatusForbidden) {
				t.Errorf("erro %v, quer 403", err)
			}
		})
	}
}

func TestEnsaiarApontaOPassoQueAsRegrasRecusam(t *testing.T) {
	atual := admin.Mapeamento{}.Normalizado()
	lote := loteDeMapeamento{Administradoras: []admin.Administradora{{ID: "adm-ok", Nome: "Ok"}, {ID: "Ruim!", Nome: "X"}}}
	err := ensaiar(t.Context(), lote.passos(atual), atual)
	falha, ok := errors.AsType[*falhaNoLote](err)
	if !ok || falha.rotulo != "administradoras[ruim!]" || !errors.Is(err, admin.ErrInvalido) {
		t.Fatalf("ensaio: %v", err)
	}
	if len(atual.Administradoras) != 0 {
		t.Error("o ensaio alterou o mapeamento recebido")
	}
}
