package camadas

import (
	"encoding/json"
	"fmt"

	okf "go-concierge/internal/harness/conteudo"
)

type textosPersistidos struct {
	Titulo          string   `json:"title"`
	QuandoUsar      string   `json:"when_to_use"`
	FrasesDeExemplo []string `json:"examples"`
	PalavrasChave   []string `json:"keywords"`
	Resumo          string   `json:"summary"`
	Visibilidade    string   `json:"visibility"`
	TemFluxo        bool     `json:"has_flow"`
}

type campoValorPersistido struct {
	Campo string `json:"field"`
	Valor any    `json:"value"`
}

type blocoPersistido struct {
	ID           string                 `json:"id"`
	Tipo         string                 `json:"type"`
	Recurso      string                 `json:"resource,omitempty"`
	Com          []campoValorPersistido `json:"with,omitempty"`
	GuardarEm    string                 `json:"store_as,omitempty"`
	Param        string                 `json:"param,omitempty"`
	Titulo       string                 `json:"title,omitempty"`
	De           string                 `json:"from,omitempty"`
	Rotulo       string                 `json:"label,omitempty"`
	Valor        string                 `json:"value,omitempty"`
	Pergunta     string                 `json:"question,omitempty"`
	SeVazio      string                 `json:"if_empty,omitempty"`
	Reaproveitar string                 `json:"reuse,omitempty"`
	Campos       []string               `json:"fields,omitempty"`
	Confirmacao  string                 `json:"confirmation,omitempty"`
	Texto        string                 `json:"text,omitempty"`
	Extra        map[string]any         `json:"extra,omitempty"`
}

type opcaoPersistida struct {
	Valor  string `json:"value"`
	Rotulo string `json:"label"`
}

type informacaoPersistida struct {
	Nome          string            `json:"name"`
	Titulo        string            `json:"title,omitempty"`
	Pergunta      string            `json:"question,omitempty"`
	Tipo          string            `json:"type,omitempty"`
	Opcoes        []opcaoPersistida `json:"options,omitempty"`
	OpcoesSimples bool              `json:"simple_options,omitempty"`
	Extra         map[string]any    `json:"extra,omitempty"`
}

type fluxoPersistido struct {
	Blocos      []blocoPersistido      `json:"blocks"`
	Informacoes []informacaoPersistida `json:"information"`
}

type conteudoDocumentoPersistido struct {
	Textos      textosPersistidos `json:"texts"`
	Fluxo       fluxoPersistido   `json:"flow"`
	Ferramentas []string          `json:"tools"`
	Orientacao  string            `json:"guidance"`
}

type documentoPersistido struct {
	SchemaVersion int `json:"schema_version"`
	conteudoDocumentoPersistido
}

func textosParaBanco(v okf.Textos) textosPersistidos {
	return textosPersistidos(v)
}

func textosDoBanco(v textosPersistidos) okf.Textos {
	return okf.Textos(v)
}

func campoValorParaBanco(v okf.CampoValor) campoValorPersistido {
	return campoValorPersistido(v)
}

func campoValorDoBanco(v campoValorPersistido) okf.CampoValor {
	return okf.CampoValor(v)
}

func blocoParaBanco(v okf.Bloco) blocoPersistido {
	out := blocoPersistido{
		ID:           v.ID,
		Tipo:         tipoBlocoParaBanco(v.Tipo),
		Recurso:      v.Recurso,
		GuardarEm:    v.GuardarEm,
		Param:        v.Param,
		Titulo:       v.Titulo,
		De:           v.De,
		Rotulo:       v.Rotulo,
		Valor:        v.Valor,
		Pergunta:     v.Pergunta,
		SeVazio:      v.SeVazio,
		Reaproveitar: v.Reaproveitar,
		Campos:       v.Campos,
		Confirmacao:  v.Confirmacao,
		Texto:        v.Texto,
		Extra:        v.Extra,
	}
	if v.Com != nil {
		out.Com = make([]campoValorPersistido, len(v.Com))
		for i, x := range v.Com {
			out.Com[i] = campoValorParaBanco(x)
		}
	}
	return out
}

func blocoDoBanco(v blocoPersistido) okf.Bloco {
	out := okf.Bloco{
		ID:           v.ID,
		Tipo:         tipoBlocoDoBanco(v.Tipo),
		Recurso:      v.Recurso,
		GuardarEm:    v.GuardarEm,
		Param:        v.Param,
		Titulo:       v.Titulo,
		De:           v.De,
		Rotulo:       v.Rotulo,
		Valor:        v.Valor,
		Pergunta:     v.Pergunta,
		SeVazio:      v.SeVazio,
		Reaproveitar: v.Reaproveitar,
		Campos:       v.Campos,
		Confirmacao:  v.Confirmacao,
		Texto:        v.Texto,
		Extra:        v.Extra,
	}
	if v.Com != nil {
		out.Com = make([]okf.CampoValor, len(v.Com))
		for i, x := range v.Com {
			out.Com[i] = campoValorDoBanco(x)
		}
	}
	return out
}

func opcaoParaBanco(v okf.Opcao) opcaoPersistida {
	return opcaoPersistida(v)
}

func opcaoDoBanco(v opcaoPersistida) okf.Opcao {
	return okf.Opcao(v)
}

func informacaoParaBanco(v okf.Informacao) informacaoPersistida {
	out := informacaoPersistida{
		Nome:          v.Nome,
		Titulo:        v.Titulo,
		Pergunta:      v.Pergunta,
		Tipo:          v.Tipo,
		OpcoesSimples: v.OpcoesSimples,
		Extra:         v.Extra,
	}
	if v.Opcoes != nil {
		out.Opcoes = make([]opcaoPersistida, len(v.Opcoes))
		for i, x := range v.Opcoes {
			out.Opcoes[i] = opcaoParaBanco(x)
		}
	}
	return out
}

func informacaoDoBanco(v informacaoPersistida) okf.Informacao {
	out := okf.Informacao{
		Nome:          v.Nome,
		Titulo:        v.Titulo,
		Pergunta:      v.Pergunta,
		Tipo:          v.Tipo,
		OpcoesSimples: v.OpcoesSimples,
		Extra:         v.Extra,
	}
	if v.Opcoes != nil {
		out.Opcoes = make([]okf.Opcao, len(v.Opcoes))
		for i, x := range v.Opcoes {
			out.Opcoes[i] = opcaoDoBanco(x)
		}
	}
	return out
}

func fluxoParaBanco(v okf.Fluxo) fluxoPersistido {
	out := fluxoPersistido{}
	if v.Blocos != nil {
		out.Blocos = make([]blocoPersistido, len(v.Blocos))
		for i, x := range v.Blocos {
			out.Blocos[i] = blocoParaBanco(x)
		}
	}
	if v.Informacoes != nil {
		out.Informacoes = make([]informacaoPersistida, len(v.Informacoes))
		for i, x := range v.Informacoes {
			out.Informacoes[i] = informacaoParaBanco(x)
		}
	}
	return out
}

func fluxoDoBanco(v fluxoPersistido) okf.Fluxo {
	out := okf.Fluxo{}
	if v.Blocos != nil {
		out.Blocos = make([]okf.Bloco, len(v.Blocos))
		for i, x := range v.Blocos {
			out.Blocos[i] = blocoDoBanco(x)
		}
	}
	if v.Informacoes != nil {
		out.Informacoes = make([]okf.Informacao, len(v.Informacoes))
		for i, x := range v.Informacoes {
			out.Informacoes[i] = informacaoDoBanco(x)
		}
	}
	return out
}

func documentoParaBanco(v Documento) conteudoDocumentoPersistido {
	out := conteudoDocumentoPersistido{
		Ferramentas: v.Ferramentas,
		Orientacao:  v.Orientacao,
	}
	out.Textos = textosParaBanco(v.Textos)
	out.Fluxo = fluxoParaBanco(v.Fluxo)
	return out
}

func documentoDoBanco(v conteudoDocumentoPersistido) Documento {
	out := Documento{
		Ferramentas: v.Ferramentas,
		Orientacao:  v.Orientacao,
	}
	out.Textos = textosDoBanco(v.Textos)
	out.Fluxo = fluxoDoBanco(v.Fluxo)
	return out
}

var tiposDeblocoPersistidos = map[okf.TipoBloco]string{
	okf.BlocoBuscar: "FETCH_DATA", okf.BlocoGravar: "WRITE_DATA", okf.BlocoEscolha: "ASK_CHOICE", okf.BlocoPerguntar: "ASK", okf.BlocoFormulario: "FORM", okf.BlocoConfirmar: "CONFIRM", okf.BlocoMontarTexto: "COMPOSE_TEXT", okf.BlocoReaproveitar: "REUSE", okf.BlocoResponder: "RESPOND",
}

func tipoBlocoParaBanco(v okf.TipoBloco) string {
	if x, ok := tiposDeblocoPersistidos[v]; ok {
		return x
	}
	return string(v)
}
func tipoBlocoDoBanco(v string) okf.TipoBloco {
	for k, x := range tiposDeblocoPersistidos {
		if x == v {
			return k
		}
	}
	return okf.TipoBloco(v)
}
func codificarDocumento(d Documento) ([]byte, error) {
	return json.Marshal(documentoPersistido{1, documentoParaBanco(d)})
}
func decodificarDocumento(raw []byte, destino *Documento) error {
	var p documentoPersistido
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if p.SchemaVersion != 1 {
		return fmt.Errorf("versão do documento persistido inválida: %d", p.SchemaVersion)
	}
	*destino = documentoDoBanco(p.conteudoDocumentoPersistido)
	return nil
}

type decisaoPersistida struct {
	Trecho  TrechoID `json:"section"`
	Escolha string   `json:"choice"`
	Mistura string   `json:"blend,omitempty"`
}

func codificarDecisoes(ds []Decisao) ([]byte, error) {
	p := make([]decisaoPersistida, len(ds))
	for i, d := range ds {
		choice, ok := map[Escolha]string{Manter: "KEEP", Aceitar: "ACCEPT", Misturar: "BLEND"}[d.Escolha]
		if !ok {
			return nil, fmt.Errorf("escolha inválida: %s", d.Escolha)
		}
		p[i] = decisaoPersistida{d.Trecho, choice, d.Mistura}
	}
	return json.Marshal(p)
}
func decodificarDecisoes(raw []byte, destino *[]Decisao) error {
	var p []decisaoPersistida
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if p == nil {
		*destino = nil
		return nil
	}
	out := make([]Decisao, len(p))
	for i, d := range p {
		choice, ok := map[string]Escolha{"KEEP": Manter, "ACCEPT": Aceitar, "BLEND": Misturar}[d.Escolha]
		if !ok {
			return fmt.Errorf("escolha persistida inválida: %s", d.Escolha)
		}
		out[i] = Decisao{d.Trecho, choice, d.Mistura}
	}
	*destino = out
	return nil
}
