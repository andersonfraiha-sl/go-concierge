package camadas

import (
	"encoding/json"
	"reflect"
	"testing"

	"go-concierge/internal/okf"
)

func TestPersistenciaDocumentoV2UsaInglesEPreservaDadosArbitrarios(t *testing.T) {
	p, cli := novoBanco(t)
	payload := map[string]any{"tipo": "externo", "nome": "João", "valor": map[string]any{"pergunta": "livre"}}
	documento := Documento{Textos: okf.Textos{Titulo: "Título em português", QuandoUsar: "Quando precisar"}, Orientacao: "Orientação livre", Ferramentas: []string{"recurso.portugues"}, Fluxo: okf.Fluxo{
		Blocos:      []okf.Bloco{{ID: "etapa-estavel", Tipo: okf.BlocoBuscar, Recurso: "recurso.portugues", Com: []okf.CampoValor{{Campo: "parametro_externo", Valor: payload}}, Extra: payload}},
		Informacoes: []okf.Informacao{{Nome: "parametro_externo", Tipo: "text", Titulo: "Informação", Opcoes: []okf.Opcao{{Valor: "sim", Rotulo: "Sim"}}, Extra: payload}},
	}}
	versao := inserir(t, p.Repo(), camadaBase, 0, documento)
	if _, err := p.Repo().SalvarRascunho(t.Context(), camadaBase, "", documento); err != nil {
		t.Fatal(err)
	}
	rows, err := cli.QueryContext(t.Context(), `SELECT v.document,l.draft FROM skill_versions v JOIN skill_layers l ON l.id=v.layer_id WHERE v.id=$1`, versao.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("documento ausente")
	}
	var raw, rascunho []byte
	if err := rows.Scan(&raw, &rascunho); err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{raw, rascunho} {
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatal(err)
		}
		if obj["texts"] == nil || obj["flow"] == nil || obj["guidance"] != "Orientação livre" || obj["textos"] != nil {
			t.Fatalf("contrato persistido: %s", raw)
		}
		bloco := obj["flow"].(map[string]any)["blocks"].([]any)[0].(map[string]any)
		if bloco["type"] != "FETCH_DATA" || !reflect.DeepEqual(bloco["extra"], payload) || !reflect.DeepEqual(bloco["with"].([]any)[0].(map[string]any)["value"], payload) {
			t.Fatalf("payload arbitrário alterado: %#v", bloco)
		}
	}
	lida, err := p.Repo().Versao(t.Context(), versao.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lida.Documento, documento) {
		t.Fatalf("documento alterado no roundtrip: %#v", lida.Documento)
	}
}

func TestPersistenciaDecisoesV2UsaChavesEValoresTecnicosEmIngles(t *testing.T) {
	ds := []Decisao{{Trecho: "texto:titulo", Escolha: Manter}, {Trecho: "orientacao", Escolha: Misturar, Mistura: "Texto livre em português"}}
	raw, err := codificarDecisoes(ds)
	if err != nil {
		t.Fatal(err)
	}
	var banco []map[string]any
	if err := json.Unmarshal(raw, &banco); err != nil {
		t.Fatal(err)
	}
	if banco[0]["choice"] != "KEEP" || banco[0]["section"] != "texto:titulo" || banco[1]["blend"] != "Texto livre em português" || banco[0]["escolha"] != nil {
		t.Fatalf("decisões persistidas: %s", raw)
	}
	var lidas []Decisao
	if err := decodificarDecisoes(raw, &lidas); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lidas, ds) {
		t.Fatalf("decisões alteradas: %#v", lidas)
	}
}
