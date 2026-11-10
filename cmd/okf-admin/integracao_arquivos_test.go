package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-concierge/internal/catalogo"
	"go-concierge/internal/escopo"
)

const (
	arquivoNovo    = "agentes/concierge/knowledge/novo.md"
	arquivoPersona = "agentes/concierge/persona.md"
	arquivoDoCaso  = "agentes/concierge/agentes/agendar-mudanca/caso.yaml"
)

// ambienteDeArquivos liga o catálogo do servidor ao Postgres do ambiente, como no main: a
// pasta é a cópia de trabalho e todo lote aplicado tem de chegar à tabela.
func ambienteDeArquivos(t *testing.T) (ambienteRotas, *catalogo.Postgres) {
	t.Helper()
	amb := ambienteDeIntegracao(t)
	amb.s.cat = abrirCatalogoDeTeste(t, sementeDoRepositorio(t), amb.s.banco)
	return amb, catalogo.NovoPostgres(amb.s.banco.Ent)
}

func putArquivos(amb ambienteRotas, query, corpo string) *httptest.ResponseRecorder {
	return chamarIntegracao(amb, http.MethodPut, "/integracao/v1/arquivos"+query, corpo)
}

func loteJSON(t *testing.T, gravar []arquivoDoPacote, remover []string) string {
	t.Helper()
	b, err := json.Marshal(loteDeArquivos{Gravar: gravar, Remover: remover})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func arquivoTexto(caminho, conteudo string) arquivoDoPacote {
	return arquivoDoPacote{Caminho: caminho, Conteudo: conteudo}
}

// estadoDoCatalogo é o que o disco e o banco guardam agora, para comparar antes e depois.
type estadoDoCatalogo struct{ disco, banco map[string]string }

func fotografarCatalogo(t *testing.T, pg *catalogo.Postgres, raiz string) estadoDoCatalogo {
	t.Helper()
	noDisco, err := catalogo.LerDisco(raiz)
	if err != nil {
		t.Fatal(err)
	}
	noBanco, err := pg.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	conteudos := func(arquivos []catalogo.Arquivo) map[string]string {
		m := make(map[string]string, len(arquivos))
		for _, a := range arquivos {
			m[a.Caminho] = string(a.Conteudo)
		}
		return m
	}
	return estadoDoCatalogo{disco: conteudos(noDisco), banco: conteudos(noBanco)}
}

func (e estadoDoCatalogo) igual(o estadoDoCatalogo) bool {
	return maps.Equal(e.disco, o.disco) && maps.Equal(e.banco, o.banco)
}

func respostaEmMapa(t *testing.T, corpo []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(corpo, &m); err != nil {
		t.Fatalf("resposta não é JSON: %v: %s", err, corpo)
	}
	return m
}

func TestArquivosGravaAlteraERemoveNoDiscoENoBancoEOPacoteRefleteAEscrita(t *testing.T) {
	amb, pg := ambienteDeArquivos(t)
	binario := []byte{0x89, 'P', 'N', 'G', 0xff, 0x00, 0x1a}

	w := putArquivos(amb, "", loteJSON(t,
		[]arquivoDoPacote{
			arquivoTexto(arquivoNovo, "# novo\n"),
			arquivoTexto(arquivoPersona, "# persona nova\n"),
			{Caminho: "agentes/concierge/assets/logo.png", Conteudo: base64.StdEncoding.EncodeToString(binario), Codificacao: codificacaoBase64},
		},
		[]string{arquivoDoCaso, "agentes/concierge/knowledge/nunca-existiu.md"},
	))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	resp := respostaEmMapa(t, w.Body.Bytes())
	if resp["gravados"] != float64(3) || resp["removidos"] != float64(1) {
		t.Errorf("resposta = %v, quer gravados 3 e removidos 1", resp)
	}

	bancoIgualAoDisco(t, pg, amb.s.cat.Root)
	noBanco, err := pg.Listar(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	porCaminho := map[string][]byte{}
	for _, a := range noBanco {
		porCaminho[a.Caminho] = a.Conteudo
	}
	if string(porCaminho[arquivoNovo]) != "# novo\n" || string(porCaminho[arquivoPersona]) != "# persona nova\n" {
		t.Error("o banco não tem os arquivos gravados")
	}
	if string(porCaminho["agentes/concierge/assets/logo.png"]) != string(binario) {
		t.Error("o banco não preservou os bytes do binário")
	}
	if _, existe := porCaminho[arquivoDoCaso]; existe {
		t.Error("o banco ainda tem o arquivo removido")
	}

	var p pacote
	if err := json.Unmarshal(pedirPacote(amb, "/integracao/v1/pacote", "Bearer "+chaveDeTeste).Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if conteudoDe(p.Arquivos, arquivoNovo) != "# novo\n" || conteudoDe(p.Arquivos, arquivoPersona) != "# persona nova\n" {
		t.Error("o pacote não reflete os arquivos gravados")
	}
	if conteudoDe(p.Arquivos, arquivoDoCaso) != "" {
		t.Error("o pacote ainda traz o arquivo removido")
	}
	var logo *arquivoDoPacote
	for i := range p.Arquivos {
		if p.Arquivos[i].Caminho == "agentes/concierge/assets/logo.png" {
			logo = &p.Arquivos[i]
		}
	}
	if logo == nil || logo.Codificacao != codificacaoBase64 || logo.Conteudo != base64.StdEncoding.EncodeToString(binario) {
		t.Errorf("binário no pacote = %+v", logo)
	}
}

func TestArquivosReenviarOLoteNaoMudaNadaESoRemoveOQueExistia(t *testing.T) {
	amb, pg := ambienteDeArquivos(t)
	lote := loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoNovo, "# novo\n")}, []string{arquivoDoCaso})

	primeiro := putArquivos(amb, "", lote)
	depoisDoPrimeiro := fotografarCatalogo(t, pg, amb.s.cat.Root)
	segundo := putArquivos(amb, "", lote)

	if primeiro.Code != http.StatusOK || segundo.Code != http.StatusOK {
		t.Fatalf("status %d e %d: %s", primeiro.Code, segundo.Code, segundo.Body.String())
	}
	if got := respostaEmMapa(t, primeiro.Body.Bytes()); got["removidos"] != float64(1) {
		t.Errorf("primeiro envio = %v, quer removidos 1", got)
	}
	if got := respostaEmMapa(t, segundo.Body.Bytes()); got["gravados"] != float64(1) || got["removidos"] != float64(0) {
		t.Errorf("reenvio = %v, quer gravados 1 e removidos 0", got)
	}
	if !depoisDoPrimeiro.igual(fotografarCatalogo(t, pg, amb.s.cat.Root)) {
		t.Error("o reenvio mudou o catálogo")
	}
}

func TestArquivosRecusaPedidoInvalidoComoErroDeCliente(t *testing.T) {
	grande := strings.Repeat("a", tetoDoArquivo+1)
	grandeEmBase64 := base64.StdEncoding.EncodeToString([]byte(grande))
	casos := []struct {
		nome  string
		corpo string
	}{
		{"JSON quebrado", `{"gravar": [`},
		{"campo desconhecido", `{"gravar": [], "remove": ["agentes/a/x.md"]}`},
		{"campo desconhecido no arquivo", `{"gravar": [{"caminho": "agentes/concierge/x.md", "conteudo": "x", "encoding": "utf8"}]}`},
		{"lote vazio", `{}`},
		{"listas vazias", `{"gravar": [], "remover": []}`},
		{"codificação desconhecida", loteJSON(t, []arquivoDoPacote{{Caminho: arquivoNovo, Conteudo: "x", Codificacao: "gzip"}}, nil)},
		{"base64 inválido", loteJSON(t, []arquivoDoPacote{{Caminho: arquivoNovo, Conteudo: "isto não é base64!", Codificacao: codificacaoBase64}}, nil)},
		{"arquivo de texto maior que 1 MiB", loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoNovo, grande)}, nil)},
		{"binário maior que 1 MiB depois de decodificar", loteJSON(t, []arquivoDoPacote{{Caminho: arquivoNovo, Conteudo: grandeEmBase64, Codificacao: codificacaoBase64}}, nil)},
		{"arquivo omitido do pacote devolvido sem conteúdo", loteJSON(t, []arquivoDoPacote{{Caminho: arquivoNovo, Omitido: motivoArquivoGrande}}, nil)},
		{"repetido em gravar", loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoNovo, "a"), arquivoTexto(arquivoNovo, "b")}, nil)},
		{"repetido em remover", loteJSON(t, nil, []string{arquivoDoCaso, arquivoDoCaso})},
		{"repetido entre gravar e remover", loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoNovo, "a")}, []string{arquivoNovo})},
		{"arquivo e pasta no mesmo lote", loteJSON(t, []arquivoDoPacote{arquivoTexto("agentes/concierge/x", "a"), arquivoTexto("agentes/concierge/x/y.md", "b")}, nil)},
		{"ponto-ponto", loteJSON(t, []arquivoDoPacote{arquivoTexto("agentes/concierge/../../fora.md", "x")}, nil)},
		{"ponto-ponto para o nexus", loteJSON(t, []arquivoDoPacote{arquivoTexto("agentes/../nexus/cond-001.yaml", "x")}, nil)},
		{"caminho absoluto", loteJSON(t, []arquivoDoPacote{arquivoTexto("/etc/passwd", "x")}, nil)},
		{"barra invertida", loteJSON(t, []arquivoDoPacote{arquivoTexto(`agentes\concierge\x.md`, "x")}, nil)},
		{"barra invertida com ponto-ponto", loteJSON(t, []arquivoDoPacote{arquivoTexto(`agentes/..\..\x.md`, "x")}, nil)},
		{"unidade do windows", loteJSON(t, []arquivoDoPacote{arquivoTexto("C:/x.md", "x")}, nil)},
		{"segmento oculto", loteJSON(t, []arquivoDoPacote{arquivoTexto("agentes/concierge/.env", "x")}, nil)},
		{"pasta oculta", loteJSON(t, []arquivoDoPacote{arquivoTexto("agentes/concierge/.git/config", "x")}, nil)},
		{"nexus", loteJSON(t, []arquivoDoPacote{arquivoTexto("nexus/cond-001.yaml", "x")}, nil)},
		{"nexus em remover", loteJSON(t, nil, []string{"nexus/cond-001.yaml"})},
		{"fora das pastas do catálogo", loteJSON(t, []arquivoDoPacote{arquivoTexto("README.md", "x")}, nil)},
		{"caminho vazio", loteJSON(t, []arquivoDoPacote{arquivoTexto("", "x")}, nil)},
		{"um caminho ruim estraga o lote inteiro", loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoNovo, "bom"), arquivoTexto("nexus/x.yaml", "ruim")}, nil)},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			amb, pg := ambienteDeArquivos(t)
			antes := fotografarCatalogo(t, pg, amb.s.cat.Root)

			w := putArquivos(amb, "", c.corpo)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if !antes.igual(fotografarCatalogo(t, pg, amb.s.cat.Root)) {
				t.Error("um pedido inválido mudou o catálogo")
			}
		})
	}
}

func TestArquivosRecusaCorpoAcimaDe8MiB(t *testing.T) {
	amb, pg := ambienteDeArquivos(t)
	antes := fotografarCatalogo(t, pg, amb.s.cat.Root)
	corpo := loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoNovo, strings.Repeat("a", tetoDoCorpoDeArquivos))}, nil)

	if w := putArquivos(amb, "", corpo); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, quer 413", w.Code)
	}
	if !antes.igual(fotografarCatalogo(t, pg, amb.s.cat.Root)) {
		t.Error("o corpo grande mudou o catálogo")
	}
}

func TestArquivosComRecorteDeAdministradoraResponde403SemTocarNoCatalogo(t *testing.T) {
	valido := loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoNovo, "# novo\n")}, nil)
	casos := []struct {
		nome, query string
		fixa        string
		corpo       string
	}{
		{"parâmetro", "?administradora=adm-a", "", valido},
		{"parâmetro com corpo inválido: o 403 vem antes", "?administradora=adm-a", "", `{"lixo": 1}`},
		{"administradora fixa da implantação", "", "adm-a", valido},
		{"fixa vence o parâmetro", "?administradora=adm-b", "adm-a", valido},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			amb, pg := ambienteDeArquivos(t)
			amb.s.implantacao = escopo.Implantacao{AdministradoraFixa: c.fixa}
			antes := fotografarCatalogo(t, pg, amb.s.cat.Root)

			w := putArquivos(amb, c.query, c.corpo)

			if w.Code != http.StatusForbidden {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if !antes.igual(fotografarCatalogo(t, pg, amb.s.cat.Root)) {
				t.Error("a chamada recusada mudou o catálogo")
			}
		})
	}
}

func TestArquivosSemChaveNaoGrava(t *testing.T) {
	amb, pg := ambienteDeArquivos(t)
	antes := fotografarCatalogo(t, pg, amb.s.cat.Root)

	r := httptest.NewRequest(http.MethodPut, "/integracao/v1/arquivos", strings.NewReader(loteJSON(t, []arquivoDoPacote{arquivoTexto(arquivoNovo, "x")}, nil)))
	w := httptest.NewRecorder()
	amb.handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
	if !antes.igual(fotografarCatalogo(t, pg, amb.s.cat.Root)) {
		t.Error("chamada sem chave mudou o catálogo")
	}
}

func TestArquivosQueQuebramOCatalogoResponde422ENadaMuda(t *testing.T) {
	casos := []struct {
		nome   string
		gravar []arquivoDoPacote
		remove []string
	}{
		{
			"yaml inválido num agente existente junto de escritas boas",
			[]arquivoDoPacote{arquivoTexto(arquivoNovo, "# bom\n"), arquivoTexto("agentes/concierge/agentes/agendar-mudanca/agent.yaml", "nome: [\n")},
			[]string{arquivoDoCaso},
		},
		{
			"agente novo com yaml inválido",
			[]arquivoDoPacote{arquivoTexto("agentes/agente-novo/agent.yaml", "nome: [\n"), arquivoTexto("agentes/agente-novo/knowledge/x.md", "x")},
			nil,
		},
		{
			"caso.yaml inválido",
			[]arquivoDoPacote{arquivoTexto(arquivoDoCaso, "isto: [não, é, um, caso\n")},
			nil,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			amb, pg := ambienteDeArquivos(t)
			antes := fotografarCatalogo(t, pg, amb.s.cat.Root)

			w := putArquivos(amb, "", loteJSON(t, c.gravar, c.remove))

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			var resp struct {
				Problemas []struct{ Nivel, Onde, Texto string } `json:"problemas"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || len(resp.Problemas) == 0 {
				t.Fatalf("corpo sem problemas: %v: %s", err, w.Body.String())
			}
			for _, p := range resp.Problemas {
				if p.Nivel != "erro" || p.Onde == "" || p.Texto == "" {
					t.Errorf("problema incompleto: %+v", p)
				}
			}
			depois := fotografarCatalogo(t, pg, amb.s.cat.Root)
			if !antes.igual(depois) {
				t.Error("o lote recusado mudou o disco ou o banco")
			}
		})
	}
}

func TestArquivosComFalhaDeBancoResponde500GenericoERestauraDiscoEBanco(t *testing.T) {
	amb, pg := ambienteDeArquivos(t)
	gancho := persistirNoBanco(pg, amb.s.cat.Root)
	chamadas := 0
	amb.s.cat.Persistir = func() error {
		chamadas++
		if chamadas == 1 {
			return errors.New("conexão com 10.0.0.5 recusada")
		}
		return gancho()
	}
	antes := fotografarCatalogo(t, pg, amb.s.cat.Root)

	w := putArquivos(amb, "", loteJSON(t,
		[]arquivoDoPacote{arquivoTexto(arquivoNovo, "# novo\n"), arquivoTexto(arquivoPersona, "# outra\n"), arquivoTexto("agentes/pasta-nova/agent.yaml", "nome: pasta-nova\n")},
		[]string{arquivoDoCaso},
	))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "10.0.0.5") {
		t.Errorf("a resposta expôs o erro do banco: %s", w.Body.String())
	}
	depois := fotografarCatalogo(t, pg, amb.s.cat.Root)
	if !antes.igual(depois) {
		t.Error("depois da falha, disco e banco não voltaram ao estado anterior")
	}
	bancoIgualAoDisco(t, pg, amb.s.cat.Root)
}
