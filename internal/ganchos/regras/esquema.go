// Package regras define o esquema das regras declarativas dos ganchos, a validação na
// gravação e os feriados nacionais usados por dia_nao_util.
package regras

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"go-concierge/internal/ganchos"
)

// Regra é uma regra declarativa. Decisão de regra só Bloqueia ou Confirma: Ajustar é
// exclusivo de gancho fixo.
type Regra struct {
	ID        string               `json:"id,omitempty"`
	Versao    int                  `json:"versao,omitempty"`
	Nome      string               `json:"nome"`
	Evento    ganchos.NomeDoEvento `json:"evento"`
	Alvo      Alvo                 `json:"alvo"`
	Fuso      string               `json:"fuso,omitempty"`
	Condicoes []Condicao           `json:"condicoes"`
	Decisao   DecisaoDaRegra       `json:"decisao"`
	Ativa     bool                 `json:"ativa"`
}

// Alvo vazio vale para todas as administradoras e condomínios.
type Alvo struct {
	Administradora string   `json:"administradora,omitempty"`
	Condominios    []string `json:"condominios,omitempty"`
}

type TipoCondicao string

const (
	CondicaoFerramenta        TipoCondicao = "ferramenta"
	CondicaoCaso              TipoCondicao = "caso"
	CondicaoForaDoHorario     TipoCondicao = "fora_do_horario"
	CondicaoDiaDaSemana       TipoCondicao = "dia_da_semana"
	CondicaoDiaNaoUtil        TipoCondicao = "dia_nao_util"
	CondicaoContatoCadastrado TipoCondicao = "contato_cadastrado"
	CondicaoPapelDoMorador    TipoCondicao = "papel_do_morador"
	CondicaoTextoContem       TipoCondicao = "texto_contem"
)

// Condicao é uma união por Tipo: só o ponteiro do tipo vem preenchido.
type Condicao struct {
	Tipo              TipoCondicao
	Ferramenta        *CondFerramenta
	Caso              *CondCaso
	ForaDoHorario     *CondHorario
	DiaDaSemana       *CondDias
	DiaNaoUtil        *CondNaoUtil
	ContatoCadastrado *CondCadastrado
	PapelDoMorador    *CondPapeis
	TextoContem       *CondTermos
}

type CondFerramenta struct {
	Nomes []string `json:"nomes"`
}
type CondCaso struct {
	IDs []string `json:"ids"`
}
type CondHorario struct {
	Inicio string `json:"inicio"`
	Fim    string `json:"fim"`
	Fonte  Fonte  `json:"fonte"`
}
type CondDias struct {
	Dias  []string `json:"dias"`
	Fonte Fonte    `json:"fonte"`
}
type CondNaoUtil struct {
	Fonte          Fonte    `json:"fonte"`
	FeriadosExtras []string `json:"feriados_extras,omitempty"`
}
type CondCadastrado struct {
	Valor bool `json:"valor"`
}
type CondPapeis struct {
	Papeis []string `json:"papeis"`
}
type CondTermos struct {
	Termos []string `json:"termos"`
}

const (
	FonteAgora     = "agora"
	FonteParametro = "parametro"
)

// Fonte diz de onde vem o instante: "agora" ou o parâmetro de uma ferramenta.
type Fonte struct {
	Tipo string `json:"tipo"`
	Nome string `json:"nome,omitempty"`
}

// DecisaoDaRegra: Mensagem em bloquear, Pergunta em confirmar. Ajuste só existe para o
// validador recusar com código claro uma regra que tente ajustar.
type DecisaoDaRegra struct {
	Tipo     ganchos.TipoDecisao
	Mensagem string
	Pergunta string
	Ajuste   *Ajuste
}

type Ajuste struct {
	Tipo  string `json:"tipo"`
	Campo string `json:"campo,omitempty"`
	Texto string `json:"texto,omitempty"`
}

// ErroDeEsquema é o erro de Decodificar: JSON que nem chega a formar uma Regra.
type ErroDeEsquema struct{ Campo, Codigo, Mensagem string }

func (e *ErroDeEsquema) Error() string { return e.Campo + ": " + e.Codigo + ": " + e.Mensagem }

// Decodificar lê o JSON da spec. Campo desconhecido, tipo desconhecido e campo de
// outro tipo são erro (*ErroDeEsquema).
func Decodificar(raw []byte) (Regra, error) {
	var r Regra
	if err := decodificarEstrito(raw, &r); err != nil {
		return Regra{}, fmt.Errorf("decodificar regra: %w", err)
	}
	return r, nil
}

func decodificarEstrito(raw []byte, destino any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	err := d.Decode(destino)
	if err == nil {
		return nil
	}
	var esq *ErroDeEsquema
	if errors.As(err, &esq) {
		return esq
	}
	return &ErroDeEsquema{Codigo: "JSON_INVALIDO", Mensagem: err.Error()}
}

// tipoDe extrai "tipo" e devolve o resto do objeto para o decode estrito do tipo.
func tipoDe(raw []byte) (string, []byte, error) {
	var objeto map[string]json.RawMessage
	if err := json.Unmarshal(raw, &objeto); err != nil {
		return "", nil, err
	}
	var tipo string
	if err := json.Unmarshal(objeto["tipo"], &tipo); err != nil {
		return "", nil, &ErroDeEsquema{Campo: "tipo", Codigo: "TIPO_AUSENTE", Mensagem: "tipo ausente ou não é texto"}
	}
	delete(objeto, "tipo")
	resto, err := json.Marshal(objeto)
	return tipo, resto, err
}

func (c *Condicao) UnmarshalJSON(raw []byte) error {
	tipo, resto, err := tipoDe(raw)
	if err != nil {
		return err
	}
	c.Tipo = TipoCondicao(tipo)
	aloca, ok := map[TipoCondicao]func(){
		CondicaoFerramenta:        func() { c.Ferramenta = new(CondFerramenta) },
		CondicaoCaso:              func() { c.Caso = new(CondCaso) },
		CondicaoForaDoHorario:     func() { c.ForaDoHorario = new(CondHorario) },
		CondicaoDiaDaSemana:       func() { c.DiaDaSemana = new(CondDias) },
		CondicaoDiaNaoUtil:        func() { c.DiaNaoUtil = new(CondNaoUtil) },
		CondicaoContatoCadastrado: func() { c.ContatoCadastrado = new(CondCadastrado) },
		CondicaoPapelDoMorador:    func() { c.PapelDoMorador = new(CondPapeis) },
		CondicaoTextoContem:       func() { c.TextoContem = new(CondTermos) },
	}[c.Tipo]
	if !ok {
		return &ErroDeEsquema{Campo: "condicoes.tipo", Codigo: "CONDICAO_DESCONHECIDA", Mensagem: "tipo de condição desconhecido: " + tipo}
	}
	aloca()
	return decodificarEstrito(resto, c.preenchida())
}

func (c Condicao) preenchida() any {
	switch {
	case c.Ferramenta != nil:
		return c.Ferramenta
	case c.Caso != nil:
		return c.Caso
	case c.ForaDoHorario != nil:
		return c.ForaDoHorario
	case c.DiaDaSemana != nil:
		return c.DiaDaSemana
	case c.DiaNaoUtil != nil:
		return c.DiaNaoUtil
	case c.ContatoCadastrado != nil:
		return c.ContatoCadastrado
	case c.PapelDoMorador != nil:
		return c.PapelDoMorador
	default:
		return c.TextoContem
	}
}

type decisaoBloquear struct {
	Mensagem string `json:"mensagem"`
}
type decisaoConfirmar struct {
	Pergunta string `json:"pergunta"`
}
type decisaoAjustar struct {
	Ajuste *Ajuste `json:"ajuste"`
}

func (d *DecisaoDaRegra) UnmarshalJSON(raw []byte) error {
	tipo, resto, err := tipoDe(raw)
	if err != nil {
		return err
	}
	d.Tipo = ganchos.TipoDecisao(tipo)
	switch d.Tipo {
	case ganchos.Bloquear:
		var b decisaoBloquear
		err = decodificarEstrito(resto, &b)
		d.Mensagem = b.Mensagem
	case ganchos.Confirmar:
		var c decisaoConfirmar
		err = decodificarEstrito(resto, &c)
		d.Pergunta = c.Pergunta
	case ganchos.Ajustar:
		var a decisaoAjustar
		err = decodificarEstrito(resto, &a)
		d.Ajuste = a.Ajuste
	default:
		return &ErroDeEsquema{Campo: "decisao.tipo", Codigo: "DECISAO_DESCONHECIDA", Mensagem: "tipo de decisão desconhecido: " + tipo}
	}
	return err
}
