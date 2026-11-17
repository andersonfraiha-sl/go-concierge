package camadas

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"go-concierge/internal/okf"
)

// Ids dos trechos que não dependem de um bloco ou de uma informação.
const (
	IDTitulo          TrechoID = "texto:titulo"
	IDResumo          TrechoID = "texto:resumo"
	IDQuandoUsar      TrechoID = "texto:quandoUsar"
	IDFrasesDeExemplo TrechoID = "texto:frasesDeExemplo"
	IDPalavrasChave   TrechoID = "texto:palavrasChave"
	IDOrientacao      TrechoID = "orientacao"
	IDOrdem           TrechoID = "fluxo:ordem"
)

const (
	prefixoEtapa      = "etapa:"
	prefixoInformacao = "informacao:"
)

// IDDaEtapa é o id do trecho de uma etapa; blocoID é o id estável do bloco (`responder` na resposta).
func IDDaEtapa(blocoID string) TrechoID { return TrechoID(prefixoEtapa + blocoID) }

// IDDaInformacao é o id do trecho de uma informação; o nome do parâmetro é a identidade dela.
func IDDaInformacao(nome string) TrechoID { return TrechoID(prefixoInformacao + nome) }

// BlocoDoTrecho devolve o id do bloco de um trecho de etapa.
func BlocoDoTrecho(id TrechoID) (string, bool) {
	return strings.CutPrefix(string(id), prefixoEtapa)
}

// InformacaoDoTrecho devolve o nome do parâmetro de um trecho de informação.
func InformacaoDoTrecho(id TrechoID) (string, bool) {
	return strings.CutPrefix(string(id), prefixoInformacao)
}

var rotulosDeTipoDeBloco = map[okf.TipoBloco]string{
	okf.BlocoBuscar:       "Buscar dados",
	okf.BlocoGravar:       "Gravar no sistema",
	okf.BlocoEscolha:      "Pedir escolha",
	okf.BlocoPerguntar:    "Perguntar",
	okf.BlocoFormulario:   "Formulário",
	okf.BlocoConfirmar:    "Confirmar",
	okf.BlocoMontarTexto:  "Montar texto",
	okf.BlocoReaproveitar: "Reaproveitar resposta",
	okf.BlocoResponder:    "Responder",
}

// Trechos decompõe o documento, na ordem: textos, orientação, ordem das etapas, etapas na
// ordem do fluxo e informações. Campo vazio é trecho de conteúdo vazio; etapa ou informação
// que não existe simplesmente não aparece.
func Trechos(d Documento) []Trecho {
	trechos := trechosDeTexto(d.Textos)
	trechos = append(trechos,
		Trecho{ID: IDOrientacao, Tipo: TrechoOrientacao, Rotulo: "Orientação", Conteudo: d.Orientacao},
		Trecho{ID: IDOrdem, Tipo: TrechoOrdem, Rotulo: "Ordem das etapas", Conteudo: canonico(idsDosBlocos(d.Fluxo.Blocos))},
	)
	trechos = append(trechos, trechosDasEtapas(d.Fluxo.Blocos)...)
	return append(trechos, trechosDasInformacoes(d.Fluxo.Informacoes)...)
}

func trechosDeTexto(t okf.Textos) []Trecho {
	texto := func(id TrechoID, rotulo, conteudo string) Trecho {
		return Trecho{ID: id, Tipo: TrechoTexto, Rotulo: rotulo, Conteudo: conteudo}
	}
	return []Trecho{
		texto(IDTitulo, "Título", t.Titulo),
		texto(IDResumo, "Resumo", t.Resumo),
		texto(IDQuandoUsar, "Quando usar", t.QuandoUsar),
		texto(IDFrasesDeExemplo, "Frases de exemplo", strings.Join(t.FrasesDeExemplo, "\n")),
		texto(IDPalavrasChave, "Palavras-chave", strings.Join(t.PalavrasChave, "\n")),
	}
}

func idsDosBlocos(blocos []okf.Bloco) []string {
	ids := make([]string, len(blocos))
	for i, b := range blocos {
		ids[i] = b.ID
	}
	return ids
}

func trechosDasEtapas(blocos []okf.Bloco) []Trecho {
	trechos := make([]Trecho, len(blocos))
	ocorrencias := map[okf.TipoBloco]int{}
	for i, b := range blocos {
		ocorrencias[b.Tipo]++
		trechos[i] = Trecho{
			ID:       IDDaEtapa(b.ID),
			Tipo:     TrechoEtapa,
			Rotulo:   fmt.Sprintf("Etapa: %s %d", rotuloDoTipo(b.Tipo), ocorrencias[b.Tipo]),
			Conteudo: conteudoDaEtapa(b),
		}
	}
	return trechos
}

// conteudoDaEtapa é o JSON do bloco sem o id (a identidade já está no id do trecho), com as
// chaves em ordem alfabética.
func conteudoDaEtapa(b okf.Bloco) string {
	b.ID = ""
	raw, err := json.Marshal(b)
	if err != nil {
		return canonico(b)
	}
	var campos map[string]json.RawMessage
	if err := json.Unmarshal(raw, &campos); err != nil {
		return string(raw)
	}
	delete(campos, "id")
	return canonico(campos)
}

func rotuloDoTipo(tipo okf.TipoBloco) string {
	if rotulo, ok := rotulosDeTipoDeBloco[tipo]; ok {
		return rotulo
	}
	return string(tipo)
}

func trechosDasInformacoes(informacoes []okf.Informacao) []Trecho {
	trechos := make([]Trecho, len(informacoes))
	for i, info := range informacoes {
		nome := info.Titulo
		if nome == "" {
			nome = info.Nome
		}
		trechos[i] = Trecho{
			ID:       IDDaInformacao(info.Nome),
			Tipo:     TrechoInformacao,
			Rotulo:   "Informação: " + nome,
			Conteudo: canonico(info),
		}
	}
	return trechos
}

// canonico é a forma textual estável de um valor. O JSON ordena as chaves dos mapas; se um
// valor vindo do YAML não couber em JSON (chave que não é texto, NaN), %#v também ordena e
// mantém a função total.
func canonico(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%#v", v)
	}
	return string(raw)
}

// mesmoConteudo compara só os trechos que o filtro aceita, por id e conteúdo.
func mesmoConteudo(a, b Documento, aceita func(Trecho) bool) bool {
	return maps.Equal(conteudos(a, aceita), conteudos(b, aceita))
}

func conteudos(d Documento, aceita func(Trecho) bool) map[TrechoID]string {
	porID := map[TrechoID]string{}
	for _, t := range Trechos(d) {
		if aceita(t) {
			porID[t.ID] = t.Conteudo
		}
	}
	return porID
}
