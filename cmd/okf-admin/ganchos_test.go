package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/acesso"
	"go-concierge/internal/banco"
	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/escopo"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/auditoria"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/regras"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

const segredoGanchos = "segredo-da-superlogica"

type ambienteGanchos struct {
	ambienteRotas
	banco banco.Banco
}

// novoAmbienteGanchos é o servidor inteiro com as regras num Postgres próprio. fixa é o
// ADMINISTRADORA_ID da implantação (vazio = todas).
func novoAmbienteGanchos(t *testing.T, fixa, segredo string) ambienteGanchos {
	t.Helper()
	amb := novoAmbienteRotas(t)
	b := bancotest.Novo(t)
	amb.s.banco, amb.s.regras, amb.s.segredoExportacao = b, regras.NovoPostgres(b.Ent), []byte(segredo)
	amb.s.implantacao = escopo.Implantacao{AdministradoraFixa: fixa}
	usuarios := usuariosTeste()
	usuarios["gestor-a-avancado"] = naAdministradora("gestor_administradora", "adm-a")
	amb.s.acessos = acessosFake{atribuicoes: usuarios, mapeamento: amb.repo, concessoes: map[string][]acesso.Concessao{
		"gestor-a-avancado": {{AdministradoraID: "adm-a", PapelID: "gestor_administradora", Permissao: acesso.StudioAvancado}},
	}}
	amb.handler = amb.s.rotear(nil)
	return ambienteGanchos{amb, b}
}

// regraJSON é uma regra válida de antes_da_ferramenta; extra entra no fim do objeto.
func regraJSON(nome, adm, extra string) string {
	return fmt.Sprintf(`{"nome":%q,"evento":"antes_da_ferramenta","alvo":{"administradora":%q},`+
		`"condicoes":[{"tipo":"ferramenta","nomes":["areas.reservar"]}],`+
		`"decisao":{"tipo":"bloquear","mensagem":"Reserva fechada agora."},"ativa":true%s}`, nome, adm, extra)
}

func corpoDe[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("corpo %q: %v", w.Body.String(), err)
	}
	return v
}

func (a ambienteGanchos) criar(t *testing.T, corpo string) regraVisao {
	t.Helper()
	w := a.pedir(t, "admin", http.MethodPost, "/api/ganchos/regras", corpo)
	if w.Code != http.StatusCreated {
		t.Fatalf("criar regra = %d %s", w.Code, w.Body)
	}
	return corpoDe[regraVisao](t, w)
}

func (a ambienteGanchos) alterar(t *testing.T, id, corpo string) *httptest.ResponseRecorder {
	t.Helper()
	return a.pedir(t, "admin", http.MethodPut, "/api/ganchos/regras/"+id, corpo)
}

func (a ambienteGanchos) gravadas(t *testing.T) []regras.RegraGravada {
	t.Helper()
	todas, err := a.s.regras.Listar(t.Context(), "", 200)
	if err != nil {
		t.Fatal(err)
	}
	return todas
}

type respostaDeErro struct {
	Codigo    string         `json:"codigo"`
	Problemas []problemaJSON `json:"problemas"`
}

func (r respostaDeErro) temProblema(codigo string) bool {
	return slices.ContainsFunc(r.Problemas, func(p problemaJSON) bool { return p.Codigo == codigo })
}

func TestGanchosRotasExigemPermissao(t *testing.T) {
	amb := novoAmbienteGanchos(t, "", segredoGanchos)
	regra := amb.criar(t, regraJSON("Reserva até 22h", "adm-a", ""))
	alterar := regraJSON("Reserva até 22h", "adm-a", `,"versao":1`)
	versao := "/api/ganchos/regras/" + regra.ID + "/versoes/1"
	tests := []struct {
		usuario, metodo, url, corpo string
		quer                        int
	}{
		{"sem-papel", "GET", "/api/ganchos/fixos", "", 403},
		{"gestor-a", "GET", "/api/ganchos/fixos", "", 403},
		{"gestor-a", "GET", "/api/ganchos/regras", "", 403},
		{"gestor-a", "GET", "/api/ganchos/regras/" + regra.ID, "", 403},
		{"gestor-a", "POST", "/api/ganchos/regras", regraJSON("Outra", "adm-a", ""), 403},
		{"gestor-a", "PUT", "/api/ganchos/regras/" + regra.ID, alterar, 403},
		{"gestor-a", "GET", "/api/ganchos/regras/exportar", "", 403},
		{"gestor-a", "POST", "/api/ganchos/regras/importar", "{}", 403},
		{"gestor-a-avancado", "POST", "/api/ganchos/regras", regraJSON("Outra", "adm-a", ""), 403},
		{"gestor-a-avancado", "PUT", "/api/ganchos/regras/" + regra.ID, alterar, 403},
		{"gestor-a-avancado", "POST", "/api/ganchos/regras/importar", "{}", 403},
		{"sem-papel", "GET", versao, "", 403},
		{"gestor-b", "GET", versao, "", 403},
		{"gestor-a", "GET", versao, "", 200},
		{"admin", "GET", versao, "", 200},
		{"admin", "GET", "/api/ganchos/fixos", "", 200},
		{"admin", "GET", "/api/ganchos/regras/" + regra.ID, "", 200},
	}
	for _, tt := range tests {
		t.Run(tt.usuario+" "+tt.metodo+" "+tt.url, func(t *testing.T) {
			if w := amb.pedir(t, tt.usuario, tt.metodo, tt.url, tt.corpo); w.Code != tt.quer {
				t.Fatalf("status = %d, quer %d: %s", w.Code, tt.quer, w.Body)
			}
		})
	}
}

func TestGanchosFixosListaOsNomesPorEvento(t *testing.T) {
	amb := novoAmbienteGanchos(t, "", segredoGanchos)
	w := amb.pedir(t, "admin", "GET", "/api/ganchos/fixos", "")
	got := corpoDe[map[string]map[ganchos.NomeDoEvento][]string](t, w)["fixos"]
	if !slices.Contains(got[ganchos.EventoMensagemRecebida], fixos.EscopoDaConversa) || len(got[ganchos.EventoAntesDeResponder]) == 0 {
		t.Fatalf("fixos = %v", got)
	}
}

func TestGanchosGravacaoRecusada(t *testing.T) {
	amb := novoAmbienteGanchos(t, "", segredoGanchos)
	existente := amb.criar(t, regraJSON("Reserva até 22h", "adm-a", ""))
	if w := amb.alterar(t, existente.ID, regraJSON("Reserva até 23h", "adm-a", `,"versao":1`)); w.Code != http.StatusOK {
		t.Fatalf("primeira alteração = %d %s", w.Code, w.Body)
	}
	semCondicao := strings.Replace(regraJSON("Sem condição", "adm-a", ""), `{"tipo":"ferramenta","nomes":["areas.reservar"]}`, "", 1)
	tests := []struct {
		nome, metodo, url, corpo string
		status                   int
		codigo, problema         string
	}{
		{"nome de fixo", "POST", "/api/ganchos/regras", regraJSON(fixos.EscopoDaConversa, "adm-a", ""), 422, "REGRA_INVALIDA", "NOME_DE_FIXO"},
		{"regra inválida", "POST", "/api/ganchos/regras", semCondicao, 422, "REGRA_INVALIDA", "CONDICOES_QUANTIDADE"},
		{"ferramenta fora do catálogo", "POST", "/api/ganchos/regras", strings.Replace(regraJSON("Outra", "adm-a", ""), "areas.reservar", "nao.existe", 1), 422, "REGRA_INVALIDA", "FERRAMENTA_DESCONHECIDA"},
		{"alterada_por no corpo", "POST", "/api/ganchos/regras", regraJSON("Outra", "adm-a", `,"alterada_por":"intruso"`), 422, "REGRA_INVALIDA", "JSON_INVALIDO"},
		{"put sem versão", "PUT", "/api/ganchos/regras/" + existente.ID, regraJSON("Reserva", "adm-a", ""), 422, "VERSAO_OBRIGATORIA", ""},
		{"put com versão velha", "PUT", "/api/ganchos/regras/" + existente.ID, regraJSON("Reserva", "adm-a", `,"versao":1`), 409, "VERSAO_VELHA", ""},
		{"put de regra que não existe", "PUT", "/api/ganchos/regras/00000000-0000-0000-0000-000000000001", regraJSON("Reserva", "adm-a", `,"versao":1`), 404, "REGRA_DESCONHECIDA", ""},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			w := amb.pedir(t, "admin", tt.metodo, tt.url, tt.corpo)
			resp := corpoDe[respostaDeErro](t, w)
			if w.Code != tt.status || resp.Codigo != tt.codigo || (tt.problema != "" && !resp.temProblema(tt.problema)) {
				t.Fatalf("resposta = %d %s, quer %d %s com %s", w.Code, w.Body, tt.status, tt.codigo, tt.problema)
			}
		})
	}
	if n := len(amb.gravadas(t)); n != 1 {
		t.Fatalf("regras gravadas = %d, quer só a existente", n)
	}
}

func TestGanchosGravaComOUsuarioAutenticado(t *testing.T) {
	amb := novoAmbienteGanchos(t, "", segredoGanchos)
	criada := amb.criar(t, regraJSON("Reserva até 22h", "adm-a", ""))
	if criada.AlteradaPor != "admin" || criada.Versao != 1 {
		t.Fatalf("criada = %+v, quer versão 1 por admin", criada)
	}
	var definicao map[string]any
	if err := json.Unmarshal(criada.Definicao, &definicao); err != nil {
		t.Fatal(err)
	}
	if _, temID := definicao["id"]; temID || definicao["nome"] != "Reserva até 22h" {
		t.Fatalf("definição = %v, quer a regra sem id", definicao)
	}
}

func TestGanchosListaPaginadaComDisparos(t *testing.T) {
	amb := novoAmbienteGanchos(t, "", segredoGanchos)
	for _, nome := range []string{"Regra um", "Regra dois", "Regra três"} {
		amb.criar(t, regraJSON(nome, "adm-a", ""))
	}
	todas := amb.gravadas(t)
	disparo := auditoria.Linha{CriadoEm: time.Now(), Origem: ganchos.OrigemConcierge, Evento: "antes_da_ferramenta",
		Gancho: "regra", RegraID: todas[0].ID, RegraVersao: 1, Decisao: "bloquear", Motivo: "Reserva fechada agora."}
	if err := auditoria.Gravar(t.Context(), amb.banco, []auditoria.Linha{disparo, disparo}, nil); err != nil {
		t.Fatal(err)
	}

	primeira := corpoDe[paginaDeRegras](t, amb.pedir(t, "admin", "GET", "/api/ganchos/regras?limite=2", ""))
	if len(primeira.Regras) != 2 || primeira.Proximo != todas[1].ID || primeira.Regras[0].Disparos7d != 2 || primeira.Regras[1].Disparos7d != 0 {
		t.Fatalf("primeira página = %+v", primeira)
	}
	segunda := corpoDe[paginaDeRegras](t, amb.pedir(t, "admin", "GET", "/api/ganchos/regras?limite=2&depois="+primeira.Proximo, ""))
	if len(segunda.Regras) != 1 || segunda.Regras[0].ID != todas[2].ID || segunda.Proximo != "" {
		t.Fatalf("segunda página = %+v", segunda)
	}
	for _, consulta := range []string{"limite=0", "limite=201", "limite=x", "depois=nao-e-id"} {
		if w := amb.pedir(t, "admin", "GET", "/api/ganchos/regras?"+consulta, ""); w.Code != http.StatusBadRequest {
			t.Errorf("?%s = %d, quer 400", consulta, w.Code)
		}
	}
}

// A versão vale pelo alvo gravado nela: gestor de A vê a versão 1 (alvo A), não a 2 (alvo B).
func TestGanchosVersaoUsaOAlvoDaDefinicaoGravada(t *testing.T) {
	amb := novoAmbienteGanchos(t, "", segredoGanchos)
	regra := amb.criar(t, regraJSON("Reserva até 22h", "adm-a", ""))
	if w := amb.alterar(t, regra.ID, regraJSON("Reserva até 22h", "adm-b", `,"versao":1`)); w.Code != http.StatusOK {
		t.Fatalf("mudar alvo = %d %s", w.Code, w.Body)
	}
	base := "/api/ganchos/regras/" + regra.ID + "/versoes/"
	tests := []struct {
		usuario, versao string
		quer            int
	}{
		{"gestor-a", "1", 200},
		{"gestor-a", "2", 403},
		{"gestor-b", "1", 403},
		{"gestor-b", "2", 200},
		{"admin", "9", 404},
		{"admin", "zero", 400},
	}
	for _, tt := range tests {
		t.Run(tt.usuario+" v"+tt.versao, func(t *testing.T) {
			if w := amb.pedir(t, tt.usuario, "GET", base+tt.versao, ""); w.Code != tt.quer {
				t.Fatalf("status = %d, quer %d: %s", w.Code, tt.quer, w.Body)
			}
		})
	}
	v1 := corpoDe[map[string]any](t, amb.pedir(t, "gestor-a", "GET", base+"1", ""))
	if v1["alterada_por"] != "admin" || !strings.Contains(fmt.Sprint(v1["definicao"]), "adm-a") {
		t.Fatalf("versão 1 = %v", v1)
	}
}

func (a ambienteGanchos) exportar(t *testing.T) arquivoDeRegras {
	t.Helper()
	w := a.pedir(t, "admin", "GET", "/api/ganchos/regras/exportar", "")
	if w.Code != http.StatusOK {
		t.Fatalf("exportar = %d %s", w.Code, w.Body)
	}
	return corpoDe[arquivoDeRegras](t, w)
}

func (a ambienteGanchos) importar(t *testing.T, usuario string, arquivo any) *httptest.ResponseRecorder {
	t.Helper()
	corpo, ok := arquivo.(string)
	if !ok {
		bruto, err := json.Marshal(arquivo)
		if err != nil {
			t.Fatal(err)
		}
		corpo = string(bruto)
	}
	return a.pedir(t, usuario, "POST", "/api/ganchos/regras/importar", corpo)
}

func TestGanchosExportarEImportarNumaImplantacaoDedicada(t *testing.T) {
	origem := novoAmbienteGanchos(t, "", segredoGanchos)
	origem.criar(t, regraJSON("Reserva até 22h", "adm-a", ""))
	origem.criar(t, regraJSON("Regra de todos", "", `,"fuso":"America/Manaus"`))
	arquivo := origem.exportar(t)
	destino := novoAmbienteGanchos(t, "adm-a", segredoGanchos)

	for rodada := 1; rodada <= 2; rodada++ {
		if w := destino.importar(t, "construtor-a", arquivo); w.Code != http.StatusOK {
			t.Fatalf("importar rodada %d = %d %s", rodada, w.Code, w.Body)
		}
	}

	esperadas, importadas := origem.gravadas(t), destino.gravadas(t)
	if len(importadas) != len(esperadas) {
		t.Fatalf("importadas = %d, quer %d", len(importadas), len(esperadas))
	}
	for i, imp := range importadas {
		if imp.ID != esperadas[i].ID || imp.Versao != 2 || imp.AlteradaPor != "construtor-a" || !jsonIgual(t, imp.Definicao, esperadas[i].Definicao) {
			t.Errorf("regra %d = %+v, quer a mesma de origem na versão 2 (importada duas vezes)", i, imp)
		}
	}
}

// assinado monta um arquivo assinado com o segredo, como a exportação faria.
func assinado(t *testing.T, segredo string, itens ...string) arquivoDeRegras {
	t.Helper()
	return assinadoNoFormato(t, segredo, formatoDasRegras, itens...)
}

func assinadoNoFormato(t *testing.T, segredo, formato string, itens ...string) arquivoDeRegras {
	t.Helper()
	brutos := make([]json.RawMessage, len(itens))
	for i, item := range itens {
		brutos[i] = json.RawMessage(item)
	}
	mac, err := assinar([]byte(segredo), formato, brutos)
	if err != nil {
		t.Fatal(err)
	}
	return arquivoDeRegras{Formato: formato, Regras: brutos, Assinatura: hex.EncodeToString(mac)}
}

func TestGanchosImportarRecusaSemGravarNada(t *testing.T) {
	valida := regraJSON("Reserva até 22h", "adm-a", "")
	invalida := regraJSON(fixos.EscopoDaConversa, "adm-a", "")
	alterada := assinado(t, segredoGanchos, valida)
	alterada.Regras[0] = json.RawMessage(strings.Replace(valida, "fechada", "aberta", 1))
	semAssinatura := assinado(t, segredoGanchos, valida)
	semAssinatura.Assinatura = ""
	mil := make([]string, limiteRegrasImportadas+1)
	for i := range mil {
		mil[i] = valida
	}
	tests := []struct {
		nome, segredoDestino string
		arquivo              any
		status               int
		codigo, problema     string
	}{
		{"regra alterada depois de assinar", segredoGanchos, alterada, 422, "ASSINATURA_INVALIDA", ""},
		{"sem assinatura", segredoGanchos, semAssinatura, 422, "ARQUIVO_NAO_ASSINADO", ""},
		{"assinado com outro segredo", "segredo-de-outra-instalacao", assinado(t, segredoGanchos, valida), 422, "ASSINATURA_INVALIDA", ""},
		{"regra de outra administradora", segredoGanchos, assinado(t, segredoGanchos, valida, regraJSON("Da B", "adm-b", "")), 422, "ALVO_FORA_DA_IMPLANTACAO", "ALVO_FORA_DA_IMPLANTACAO"},
		{"uma inválida cancela todas", segredoGanchos, assinado(t, segredoGanchos, valida, invalida), 422, "REGRA_INVALIDA", "NOME_DE_FIXO"},
		{"mais de mil regras", segredoGanchos, assinado(t, segredoGanchos, mil...), 422, "ARQUIVO_GRANDE_DEMAIS", ""},
		{"mais de 5 MB", segredoGanchos, `{"assinatura":"` + strings.Repeat("a", limiteArquivo) + `"}`, 413, "ARQUIVO_GRANDE_DEMAIS", ""},
		{"formato trocado depois de assinar", segredoGanchos, func() arquivoDeRegras {
			a := assinado(t, segredoGanchos, valida)
			a.Formato = "outro/1"
			return a
		}(), 422, "ASSINATURA_INVALIDA", ""},
		{"formato desconhecido", segredoGanchos, assinadoNoFormato(t, segredoGanchos, "outro/1", valida), 422, "FORMATO_DESCONHECIDO", ""},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			destino := novoAmbienteGanchos(t, "adm-a", tt.segredoDestino)
			w := destino.importar(t, "construtor-a", tt.arquivo)
			resp := corpoDe[respostaDeErro](t, w)
			if w.Code != tt.status || resp.Codigo != tt.codigo || (tt.problema != "" && !resp.temProblema(tt.problema)) {
				t.Fatalf("resposta = %d %.300s, quer %d %s com %s", w.Code, w.Body, tt.status, tt.codigo, tt.problema)
			}
			if n := len(destino.gravadas(t)); n != 0 {
				t.Fatalf("regras gravadas = %d, quer 0", n)
			}
		})
	}
}

func TestGanchosExportarEImportarSemSegredoRecusam(t *testing.T) {
	amb := novoAmbienteGanchos(t, "", "")
	for _, pedido := range []struct{ metodo, url string }{{"GET", "/api/ganchos/regras/exportar"}, {"POST", "/api/ganchos/regras/importar"}} {
		if w := amb.pedir(t, "admin", pedido.metodo, pedido.url, "{}"); w.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s = %d, quer 503", pedido.metodo, pedido.url, w.Code)
		}
	}
}

func jsonIgual(t *testing.T, a, b json.RawMessage) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	ja, _ := json.Marshal(x)
	jb, _ := json.Marshal(y)
	return string(ja) == string(jb)
}
