package okf

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func blocosDeTipos(tipos ...TipoBloco) []Bloco {
	out := make([]Bloco, len(tipos))
	for i, t := range tipos {
		b := Bloco{ID: "b" + string(rune('1'+i)), Tipo: t}
		switch t {
		case BlocoBuscar, BlocoGravar:
			b.Recurso = "x.y"
		case BlocoEscolha:
			b.Param = "p"
		case BlocoPerguntar, BlocoReaproveitar:
			b.Campos = []string{"c"}
		case BlocoFormulario:
			b.De = "d"
		default:
			b.Texto = "t"
		}
		out[i] = b
	}
	return out
}

func prob(id, codigo, msg string) ProblemaFluxo {
	return ProblemaFluxo{BlocoID: id, Codigo: codigo, Nivel: "erro", Mensagem: msg}
}

const (
	msgConfirmar = "Coloque um Confirmar antes de gravar no sistema."
	msgFim       = "O Responder precisa ser o último bloco."
)

func TestValidarFluxo(t *testing.T) {
	buscarVazio := blocosDeTipos(BlocoBuscar, BlocoResponder)
	buscarVazio[0].Recurso = ""
	extraDuplicado := blocosDeTipos(BlocoBuscar, BlocoResponder)
	extraDuplicado[0].GuardarEm = "x"
	extraDuplicado[0].Extra = map[string]any{"save_as": "x", "timeout": 3}
	extraEmEscolha := blocosDeTipos(BlocoEscolha, BlocoResponder)
	extraEmEscolha[0].Titulo = "T"
	extraEmEscolha[0].Extra = map[string]any{"title": "t"}

	tests := []struct {
		name string
		f    Fluxo
		want []ProblemaFluxo
	}{
		{"fluxo completo", Fluxo{Blocos: blocosDeTipos(BlocoBuscar, BlocoEscolha, BlocoPerguntar, BlocoConfirmar, BlocoGravar, BlocoResponder)}, []ProblemaFluxo{}},
		{"gravar sem confirmar", Fluxo{Blocos: blocosDeTipos(BlocoBuscar, BlocoGravar, BlocoResponder)}, []ProblemaFluxo{prob("b2", "GRAVAR_SEM_CONFIRMAR", msgConfirmar)}},
		{"cada confirmar vale uma gravacao", Fluxo{Blocos: blocosDeTipos(BlocoConfirmar, BlocoGravar, BlocoGravar, BlocoResponder)}, []ProblemaFluxo{prob("b3", "GRAVAR_SEM_CONFIRMAR", msgConfirmar)}},
		{"um confirmar nao vale para duas gravacoes", Fluxo{Blocos: blocosDeTipos(BlocoConfirmar, BlocoConfirmar, BlocoGravar, BlocoGravar, BlocoResponder)}, []ProblemaFluxo{prob("b4", "GRAVAR_SEM_CONFIRMAR", msgConfirmar)}},
		{"confirmar depois de gravar nao vale", Fluxo{Blocos: blocosDeTipos(BlocoGravar, BlocoConfirmar, BlocoResponder)}, []ProblemaFluxo{prob("b1", "GRAVAR_SEM_CONFIRMAR", msgConfirmar)}},
		{"sem responder", Fluxo{Blocos: blocosDeTipos(BlocoBuscar)}, []ProblemaFluxo{prob("", "SEM_RESPONDER", "O fluxo precisa terminar com um Responder.")}},
		{"responder fora do fim", Fluxo{Blocos: blocosDeTipos(BlocoResponder, BlocoBuscar)}, []ProblemaFluxo{prob("b1", "RESPONDER_FORA_DO_FIM", msgFim)}},
		{"dois responders", Fluxo{Blocos: blocosDeTipos(BlocoResponder, BlocoResponder)}, []ProblemaFluxo{prob("b1", "RESPONDER_FORA_DO_FIM", msgFim)}},
		{"so responder", Fluxo{Blocos: blocosDeTipos(BlocoResponder)}, []ProblemaFluxo{prob("", "FLUXO_VAZIO", "Adicione ao menos um bloco antes do Responder.")}},
		{"recurso vazio", Fluxo{Blocos: buscarVazio}, []ProblemaFluxo{prob("b1", "BLOCO_INCOMPLETO", "Preencha a ferramenta.")}},
		{"extra repete campo do bloco", Fluxo{Blocos: extraDuplicado}, []ProblemaFluxo{prob("b1", "EXTRA_REPETE_CAMPO", "Este bloco tem um campo extra repetido: save_as.")}},
		{"extra da informacao repete campo", Fluxo{
			Blocos:      blocosDeTipos(BlocoBuscar, BlocoResponder),
			Informacoes: []Informacao{{Nome: "cor", Tipo: "string", Extra: map[string]any{"type": "x", "help": "ok"}}},
		}, []ProblemaFluxo{prob("", "EXTRA_REPETE_CAMPO", "A informação cor tem um campo extra repetido: type.")}},
		{"extra repete campo de escolha", Fluxo{Blocos: extraEmEscolha}, []ProblemaFluxo{prob("b1", "EXTRA_REPETE_CAMPO", "Este bloco tem um campo extra repetido: title.")}},
		{"opcoes simples com rotulo diferente", Fluxo{
			Blocos:      blocosDeTipos(BlocoBuscar, BlocoResponder),
			Informacoes: []Informacao{{Nome: "cor", OpcoesSimples: true, Opcoes: []Opcao{{"a", "a"}, {"b", "B"}}}},
		}, []ProblemaFluxo{prob("", "OPCAO_ROTULO_PERDIDO", "A informação cor tem opções simples com rótulo diferente do valor; o rótulo se perderia.")}},
		{"opcoes simples iguais passam", Fluxo{
			Blocos:      blocosDeTipos(BlocoBuscar, BlocoResponder),
			Informacoes: []Informacao{{Nome: "cor", OpcoesSimples: true, Opcoes: []Opcao{{"a", "a"}}}},
		}, []ProblemaFluxo{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidarFluxo(tt.f)
			if got == nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidarFluxoExigeCampoDeCadaTipo(t *testing.T) {
	tipos := []TipoBloco{BlocoBuscar, BlocoGravar, BlocoEscolha, BlocoPerguntar, BlocoFormulario, BlocoConfirmar, BlocoMontarTexto, BlocoReaproveitar, BlocoResponder}
	for _, tipo := range tipos {
		t.Run(string(tipo), func(t *testing.T) {
			vazio := Bloco{ID: "b1", Tipo: tipo}
			got := problemasDeBlocos([]Bloco{vazio})
			if !slices.ContainsFunc(got, func(pr ProblemaFluxo) bool { return pr.Codigo == "BLOCO_INCOMPLETO" && pr.BlocoID == "b1" }) {
				t.Fatalf("sem BLOCO_INCOMPLETO: %+v", got)
			}
		})
	}
}

func TestValidarFluxoIgnoraExtraQueNaoRepete(t *testing.T) {
	semCom := blocosDeTipos(BlocoBuscar, BlocoResponder)
	semCom[0].Extra = map[string]any{"with": "texto solto"}
	f := Fluxo{
		Blocos:      semCom,
		Informacoes: []Informacao{{Nome: "cor", Extra: map[string]any{"options": []any{}}}},
	}
	if got := ValidarFluxo(f); len(got) != 0 {
		t.Fatalf("extra sem campo equivalente não repete chave: %+v", got)
	}
}

func TestSkillsRealmenteExistentesSaoGravaveis(t *testing.T) {
	catalogo, err := LerRecursosFixture("../../okf/nexus/cond-001.yaml")
	if err != nil {
		t.Fatalf("ler catálogo: %v", err)
	}
	escritaDoCatalogo := func(recurso string) bool {
		i := slices.IndexFunc(catalogo, func(r Recurso) bool { return r.Name == recurso })
		if i < 0 {
			return EscritaPeloNome(recurso)
		}
		return catalogo[i].Write
	}
	arquivos, err := filepath.Glob("../../okf/agentes/*/agentes/*/caso.yaml")
	if err != nil || len(arquivos) == 0 {
		t.Fatalf("nenhum caso.yaml encontrado: %v", err)
	}
	for _, arq := range arquivos {
		t.Run(filepath.Base(filepath.Dir(arq)), func(t *testing.T) {
			raw, err := os.ReadFile(arq)
			if err != nil {
				t.Fatal(err)
			}
			f, err := FluxoDeYAML(raw, escritaDoCatalogo)
			if err != nil {
				t.Fatalf("ler fluxo: %v", err)
			}
			for _, pr := range ValidarFluxo(Reclassificar(f, escritaDoCatalogo)) {
				if pr.Nivel == nivelErro {
					t.Errorf("skill sem alteração não pode ter erro: %+v", pr)
				}
			}
		})
	}
}

func TestReclassificarMantemRecursoVazio(t *testing.T) {
	f := Fluxo{Blocos: []Bloco{{ID: "b1", Tipo: BlocoBuscar}, {ID: "b2", Tipo: BlocoGravar}}}
	got := Reclassificar(f, func(string) bool { return true })
	if got.Blocos[0].Tipo != BlocoBuscar || got.Blocos[1].Tipo != BlocoGravar {
		t.Fatalf("recurso vazio não deve mudar de tipo: %+v", got.Blocos)
	}
}

func TestReclassificar(t *testing.T) {
	entrada := Fluxo{Blocos: []Bloco{
		{ID: "b1", Tipo: BlocoBuscar, Recurso: "mudancas.agendar"},
		{ID: "b2", Tipo: BlocoGravar, Recurso: "x.get-a"},
		{ID: "b3", Tipo: BlocoConfirmar, Texto: "ok?"},
	}}
	copia := Fluxo{Blocos: append([]Bloco(nil), entrada.Blocos...)}
	got := Reclassificar(entrada, EscritaPeloNome)
	if got.Blocos[0].Tipo != BlocoGravar || got.Blocos[1].Tipo != BlocoBuscar || !reflect.DeepEqual(got.Blocos[2], entrada.Blocos[2]) {
		t.Fatalf("classificação errada: %+v", got.Blocos)
	}
	if !reflect.DeepEqual(entrada, copia) {
		t.Fatalf("entrada alterada: %+v", entrada)
	}
}

func TestClassificarFerramenta(t *testing.T) {
	tests := map[string]bool{
		"gruvi.bff.building-operation.get-company-by-document-controller-handle": false,
		"gruvi.bff.building-operation.fetch-unit-packages-controller-handle":     false,
		"gruvi.bff.building-operation.create-visit-controller-handle":            true,
		"mudancas.agendar": true,
		"health":           false,
	}
	for nome, want := range tests {
		if got := EscritaPeloNome(nome); got != want {
			t.Errorf("EscritaPeloNome(%q) = %v, want %v", nome, got, want)
		}
	}
}

func TestFerramentasDesconhecidas(t *testing.T) {
	f := Fluxo{Blocos: []Bloco{
		{ID: "b1", Tipo: BlocoBuscar, Recurso: "a.get"},
		{ID: "b2", Tipo: BlocoGravar, Recurso: "b.nova"},
		{ID: "b3", Tipo: BlocoBuscar},
	}}
	got := FerramentasDesconhecidas(f, func(r string) bool { return r == "a.get" })
	want := []ProblemaFluxo{{BlocoID: "b2", Codigo: "FERRAMENTA_DESCONHECIDA", Nivel: "aviso",
		Mensagem: "Esta ferramenta não está no catálogo agora; o Studio deduziu pelo nome se ela grava no sistema."}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
