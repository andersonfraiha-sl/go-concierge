package conteudo

import (
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"maps"
	"slices"
)

// TipoBloco é o tipo de um bloco do Studio; cada um corresponde a uma chave de passo do caso.yaml.
type TipoBloco string

const (
	BlocoBuscar       TipoBloco = "buscar_dados"
	BlocoGravar       TipoBloco = "gravar_no_sistema"
	BlocoEscolha      TipoBloco = "pedir_escolha"
	BlocoPerguntar    TipoBloco = "perguntar"
	BlocoFormulario   TipoBloco = "formulario"
	BlocoConfirmar    TipoBloco = "confirmar"
	BlocoMontarTexto  TipoBloco = "montar_texto"
	BlocoReaproveitar TipoBloco = "reaproveitar"
	BlocoResponder    TipoBloco = "responder"
)

// CampoValor é uma entrada do `with` de um recurso. O valor é o payload dinâmico do caso.
type CampoValor struct {
	Campo string `json:"campo"`
	Valor any    `json:"valor"`
}

// Bloco é um passo do caso visto pelo Studio. Extra guarda as chaves que o Studio não
// edita; voltam intactas na gravação.
type Bloco struct {
	ID           string         `json:"id"`
	Tipo         TipoBloco      `json:"tipo"`
	Recurso      string         `json:"recurso,omitempty"`
	Com          []CampoValor   `json:"com,omitempty"`
	GuardarEm    string         `json:"guardarEm,omitempty"`
	Param        string         `json:"param,omitempty"`
	Titulo       string         `json:"titulo,omitempty"`
	De           string         `json:"de,omitempty"`
	Rotulo       string         `json:"rotulo,omitempty"`
	Valor        string         `json:"valor,omitempty"`
	Pergunta     string         `json:"pergunta,omitempty"`
	SeVazio      string         `json:"seVazio,omitempty"`
	Reaproveitar string         `json:"reaproveitar,omitempty"`
	Campos       []string       `json:"campos,omitempty"`
	Confirmacao  string         `json:"confirmacao,omitempty"`
	Texto        string         `json:"texto,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

// Opcao é uma alternativa de um parâmetro de seleção.
type Opcao struct {
	Valor  string `json:"valor"`
	Rotulo string `json:"rotulo"`
}

// Informacao é um parâmetro (`params[i]`) do caso.
type Informacao struct {
	Nome     string  `json:"nome"`
	Titulo   string  `json:"titulo,omitempty"`
	Pergunta string  `json:"pergunta,omitempty"`
	Tipo     string  `json:"tipo,omitempty"`
	Opcoes   []Opcao `json:"opcoes,omitempty"`
	// OpcoesSimples: `options` era uma lista de textos (valor = rótulo).
	OpcoesSimples bool           `json:"opcoesSimples,omitempty"`
	Extra         map[string]any `json:"extra,omitempty"` // validate, required, placeholder, help, min, max…
}

// Fluxo é o que o Studio edita de um caso: os blocos (steps + output) e as informações (params).
type Fluxo struct {
	Blocos      []Bloco      `json:"blocos"`
	Informacoes []Informacao `json:"informacoes"`
}

// IDDaResposta é o id fixo do bloco Responder: o `output` do caso não tem lugar para um id.
const IDDaResposta = "responder"

// chaveNo troca (ou remove, se No for nil) uma chave do mapa raiz do caso.
type chaveNo struct {
	Chave string
	No    *yaml.Node
}

type Entrada struct {
	Chave string
	no    *yaml.Node
}

var chavesDePasso = []string{"resource", "choose", "collect", "form", "confirm", "compose", "reuse"}

// FluxoDeYAML lê steps, output e params do caso. `escrita` decide se um recurso grava no sistema.
func FluxoDeYAML(raw []byte, escrita func(recurso string) bool) (Fluxo, error) {
	_, raiz, err := lerDocumento(raw)
	if err != nil {
		return Fluxo{}, fmt.Errorf("ler caso: %w", err)
	}
	es, err := entradasDe(raiz)
	if err != nil {
		return Fluxo{}, fmt.Errorf("ler caso: %w", err)
	}
	blocos, err := blocosDe(Buscar(es, "steps"), escrita)
	if err != nil {
		return Fluxo{}, err
	}
	resposta, err := respostaDe(Buscar(es, "output"))
	if err != nil {
		return Fluxo{}, err
	}
	informacoes, err := informacoesDe(Buscar(es, "params"))
	if err != nil {
		return Fluxo{}, err
	}
	numerados := numerar(append(blocos, resposta...))
	if id, repetido := idRepetido(numerados); repetido {
		return Fluxo{}, fmt.Errorf("ler caso: id %q repetido nos passos", id)
	}
	return Fluxo{Blocos: numerados, Informacoes: informacoes}, nil
}

func idRepetido(blocos []Bloco) (string, bool) {
	vistos := make(map[string]bool, len(blocos))
	for _, b := range blocos {
		if vistos[b.ID] {
			return b.ID, true
		}
		vistos[b.ID] = true
	}
	return "", false
}

func blocosDe(passos *yaml.Node, escrita func(string) bool) ([]Bloco, error) {
	if passos == nil {
		return []Bloco{}, nil
	}
	if passos.Kind != yaml.SequenceNode {
		return nil, errors.New("steps deve ser uma lista")
	}
	blocos := make([]Bloco, 0, len(passos.Content))
	for i, passo := range passos.Content {
		bloco, err := blocoDe(passo, escrita)
		if err != nil {
			return nil, fmt.Errorf("passo %d: %w", i+1, err)
		}
		blocos = append(blocos, bloco)
	}
	return blocos, nil
}

func blocoDe(passo *yaml.Node, escrita func(string) bool) (Bloco, error) {
	es, err := entradasDe(passo)
	if err != nil {
		return Bloco{}, err
	}
	id, es := idDoPasso(es)
	bloco, err := blocoDoPrincipal(es, escrita)
	bloco.ID = id
	return bloco, err
}

// idDoPasso tira a chave `id` do passo quando é texto não vazio; qualquer outra forma segue
// nas entradas e é recusada ou vai para o Extra como qualquer chave desconhecida.
func idDoPasso(es []Entrada) (string, []Entrada) {
	id, ok := textoDe(Buscar(es, "id"))
	if !ok || id == "" {
		return "", es
	}
	return id, semChave(es, "id")
}

func blocoDoPrincipal(es []Entrada, escrita func(string) bool) (Bloco, error) {
	principal, resto, ok := separarPrincipal(es)
	if !ok {
		return Bloco{}, fmt.Errorf("nenhuma chave conhecida (esperava %v)", chavesDePasso)
	}
	if principal.Chave != "resource" && len(resto) > 0 {
		return Bloco{}, fmt.Errorf("chave %q não cabe em um passo %s", resto[0].Chave, principal.Chave)
	}
	switch principal.Chave {
	case "resource":
		return blocoRecurso(principal.no, resto, escrita)
	case "choose":
		return blocoEscolha(principal.no)
	case "collect":
		return blocoPerguntar(principal.no)
	case "form":
		return blocoFormulario(principal.no)
	case "confirm":
		return blocoTexto(BlocoConfirmar, principal)
	case "compose":
		return blocoTexto(BlocoMontarTexto, principal)
	default:
		return blocoReaproveitar(principal.no)
	}
}

// separarPrincipal escolhe a chave que define o passo: resource tem prioridade (aceita
// chaves a mais); senão, a primeira chave conhecida.
func separarPrincipal(es []Entrada) (principal Entrada, resto []Entrada, ok bool) {
	i := slices.IndexFunc(es, func(e Entrada) bool { return e.Chave == "resource" })
	if i < 0 {
		i = slices.IndexFunc(es, func(e Entrada) bool { return slices.Contains(chavesDePasso, e.Chave) })
	}
	if i < 0 {
		return Entrada{}, nil, false
	}
	return es[i], slices.Delete(slices.Clone(es), i, i+1), true
}

func blocoRecurso(no *yaml.Node, resto []Entrada, escrita func(string) bool) (Bloco, error) {
	recurso, ok := textoDe(no)
	if !ok {
		return Bloco{}, errors.New("resource deve ser texto")
	}
	com, resto := comDe(resto)
	textos, extra, err := separar(resto, "save_as")
	if err != nil {
		return Bloco{}, err
	}
	tipo := BlocoBuscar
	if escrita(recurso) {
		tipo = BlocoGravar
	}
	return Bloco{Tipo: tipo, Recurso: recurso, Com: com, GuardarEm: textos["save_as"], Extra: extra}, nil
}

// comDe tira o `with` do passo quando é um mapa não vazio; qualquer outra forma segue no Extra.
func comDe(es []Entrada) ([]CampoValor, []Entrada) {
	with := Buscar(es, "with")
	if with == nil || with.Kind != yaml.MappingNode || len(with.Content) == 0 {
		return nil, es
	}
	campos := make([]CampoValor, 0, len(with.Content)/2)
	for i := 0; i+1 < len(with.Content); i += 2 {
		var valor any
		if err := with.Content[i+1].Decode(&valor); err != nil {
			return nil, es
		}
		campos = append(campos, CampoValor{Campo: with.Content[i].Value, Valor: valor})
	}
	return campos, semChave(es, "with")
}

func blocoEscolha(no *yaml.Node) (Bloco, error) {
	es, err := entradasDe(no)
	if err != nil {
		return Bloco{}, fmt.Errorf("choose: %w", err)
	}
	t, extra, err := separar(es, "param", "title", "from", "label", "value", "ask", "empty", "reuse")
	if err != nil {
		return Bloco{}, err
	}
	return Bloco{
		Tipo: BlocoEscolha, Param: t["param"], Titulo: t["title"], De: t["from"], Rotulo: t["label"],
		Valor: t["value"], Pergunta: t["ask"], SeVazio: t["empty"], Reaproveitar: t["reuse"], Extra: extra,
	}, nil
}

func blocoPerguntar(no *yaml.Node) (Bloco, error) {
	if campo, ok := textoDe(no); ok {
		return Bloco{Tipo: BlocoPerguntar, Campos: []string{campo}}, nil
	}
	campos, ok := listaDeTextos(no)
	if !ok {
		return Bloco{}, errors.New("collect deve ser texto ou lista de textos")
	}
	return Bloco{Tipo: BlocoPerguntar, Campos: campos}, nil
}

func blocoFormulario(no *yaml.Node) (Bloco, error) {
	es, err := entradasDe(no)
	if err != nil {
		return Bloco{}, fmt.Errorf("form: %w", err)
	}
	t, extra, err := separar(es, "fields_from", "save_as")
	if err != nil {
		return Bloco{}, err
	}
	return Bloco{Tipo: BlocoFormulario, De: t["fields_from"], GuardarEm: t["save_as"], Extra: extra}, nil
}

func blocoTexto(tipo TipoBloco, e Entrada) (Bloco, error) {
	texto, ok := textoDe(e.no)
	if !ok {
		return Bloco{}, fmt.Errorf("%s deve ser texto", e.Chave)
	}
	return Bloco{Tipo: tipo, Texto: texto}, nil
}

func blocoReaproveitar(no *yaml.Node) (Bloco, error) {
	es, err := entradasDe(no)
	if err != nil {
		return Bloco{}, fmt.Errorf("reuse: %w", err)
	}
	slots := Buscar(es, "slots")
	if slots != nil && Buscar(es, "slot") != nil {
		return Bloco{}, errors.New("reuse com slot e slots ao mesmo tempo")
	}
	campos, ok := listaDeTextos(slots)
	if slots != nil && !ok {
		return Bloco{}, errors.New("reuse.slots deve ser lista de textos")
	}
	t, extra, err := separar(semChave(es, "slots"), "slot", "confirm", "from", "label", "value", "ask")
	if err != nil {
		return Bloco{}, err
	}
	if t["slot"] != "" {
		campos = []string{t["slot"]}
	}
	return Bloco{
		Tipo: BlocoReaproveitar, Campos: campos, Confirmacao: t["confirm"], De: t["from"],
		Rotulo: t["label"], Valor: t["value"], Pergunta: t["ask"], Extra: extra,
	}, nil
}

func respostaDe(output *yaml.Node) ([]Bloco, error) {
	if output == nil {
		return nil, nil
	}
	texto, ok := textoDe(output)
	if !ok {
		return nil, errors.New("output deve ser texto")
	}
	return []Bloco{{ID: IDDaResposta, Tipo: BlocoResponder, Texto: texto}}, nil
}

func informacoesDe(params *yaml.Node) ([]Informacao, error) {
	if params == nil {
		return []Informacao{}, nil
	}
	if params.Kind != yaml.SequenceNode {
		return nil, errors.New("params deve ser uma lista")
	}
	informacoes := make([]Informacao, 0, len(params.Content))
	for i, item := range params.Content {
		info, err := informacaoDe(item)
		if err != nil {
			return nil, fmt.Errorf("parâmetro %d: %w", i+1, err)
		}
		informacoes = append(informacoes, info)
	}
	return informacoes, nil
}

func informacaoDe(item *yaml.Node) (Informacao, error) {
	es, err := entradasDe(item)
	if err != nil {
		return Informacao{}, err
	}
	opcoes, simples, temOpcoes, err := opcoesDe(Buscar(es, "options"))
	if err != nil {
		return Informacao{}, err
	}
	if temOpcoes {
		es = semChave(es, "options")
	}
	t, extra, err := separar(es, "name", "title", "ask", "type")
	if err != nil {
		return Informacao{}, err
	}
	if t["name"] == "" {
		return Informacao{}, errors.New("name deve ser texto não vazio")
	}
	return Informacao{
		Nome: t["name"], Titulo: t["title"], Pergunta: t["ask"], Tipo: t["type"],
		Opcoes: opcoes, OpcoesSimples: simples, Extra: extra,
	}, nil
}

// opcoesDe aceita lista de textos (valor = rótulo) ou de {value,label}. Lista vazia ou de
// outra forma não vira Opcoes (tem=false) e segue no Extra.
func opcoesDe(no *yaml.Node) (opcoes []Opcao, simples, tem bool, err error) {
	if no == nil || no.Kind != yaml.SequenceNode || len(no.Content) == 0 {
		return nil, false, false, nil
	}
	_, simples = textoDe(no.Content[0])
	for i, item := range no.Content {
		opcao, ehTexto, err := opcaoDe(item)
		if err != nil {
			return nil, false, false, fmt.Errorf("opção %d: %w", i+1, err)
		}
		if ehTexto != simples {
			return nil, false, false, fmt.Errorf("opção %d: mistura texto e {value,label}", i+1)
		}
		opcoes = append(opcoes, opcao)
	}
	return opcoes, simples, true, nil
}

func opcaoDe(item *yaml.Node) (opcao Opcao, ehTexto bool, err error) {
	if texto, ok := textoDe(item); ok {
		return Opcao{Valor: texto, Rotulo: texto}, true, nil
	}
	es, err := entradasDe(item)
	if err != nil {
		return Opcao{}, false, errors.New("esperava texto ou {value, label}")
	}
	if _, ok := textoDe(Buscar(es, "value")); !ok {
		return Opcao{}, false, errors.New("valor da opção deve ser texto")
	}
	t, extra, err := separar(es, "value", "label")
	if err != nil {
		return Opcao{}, false, err
	}
	if len(extra) > 0 {
		return Opcao{}, false, fmt.Errorf("chaves além de value e label: %v", slices.Sorted(maps.Keys(extra)))
	}
	return Opcao{Valor: t["value"], Rotulo: t["label"]}, false, nil
}

// AplicarFluxo troca steps, output e params do caso e mantém todo o resto como está. Só o que
// mudou é regravado: o diff de uma edição é a linha dela.
func AplicarFluxo(raw []byte, f Fluxo) ([]byte, error) {
	passos, resposta, err := passosEResposta(f.Blocos)
	if err != nil {
		return nil, err
	}
	params, err := paramsNo(f.Informacoes)
	if err != nil {
		return nil, err
	}
	doc, raiz, err := lerDocumento(raw)
	if err != nil {
		return nil, fmt.Errorf("ler caso: %w", err)
	}
	for _, c := range []chaveNo{{"params", params}, {"steps", passos}, {"output", resposta}} {
		raiz.Content = mesclarChave(raiz.Content, c)
	}
	return CodificarCaso(doc)
}

// mesclarChave grava só o que mudou numa chave que já existe; chave nova ou removida segue
// a regra de definirChave.
func mesclarChave(conteudo []*yaml.Node, c chaveNo) []*yaml.Node {
	i := indiceDaChave(conteudo, c.Chave)
	if i < 0 || c.No == nil {
		return definirChave(conteudo, c)
	}
	conteudo[i+1] = mesclarNo(conteudo[i+1], c.No)
	return conteudo
}

func passosEResposta(blocos []Bloco) (steps, output *yaml.Node, err error) {
	steps = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for i, b := range blocos {
		if b.Tipo == BlocoResponder {
			if output, err = NoDe(b.Texto); err != nil {
				return nil, nil, fmt.Errorf("bloco %d: %w", i+1, err)
			}
			continue
		}
		pares, err := passoDe(b)
		if err != nil {
			return nil, nil, fmt.Errorf("bloco %d: %w", i+1, err)
		}
		passo, err := NoDe(slices.Concat(opc("id", b.ID), pares))
		if err != nil {
			return nil, nil, fmt.Errorf("bloco %d: %w", i+1, err)
		}
		steps.Content = append(steps.Content, passo)
	}
	return steps, output, nil
}

func passoDe(b Bloco) ([]Par, error) {
	extra := extraPares(b.Extra)
	switch b.Tipo {
	case BlocoBuscar, BlocoGravar:
		return slices.Concat([]Par{{"resource", b.Recurso}}, comPares(b.Com), opc("save_as", b.GuardarEm), extra), nil
	case BlocoEscolha:
		return unico("choose", slices.Concat(
			opc("param", b.Param), opc("title", b.Titulo), opc("from", b.De), opc("label", b.Rotulo),
			opc("value", b.Valor), opc("ask", b.Pergunta), opc("empty", b.SeVazio), opc("reuse", b.Reaproveitar), extra)), nil
	case BlocoPerguntar:
		return []Par{{"collect", Lista(b.Campos)}}, nil
	case BlocoFormulario:
		return unico("form", slices.Concat(opc("fields_from", b.De), opc("save_as", b.GuardarEm), extra)), nil
	case BlocoConfirmar:
		return []Par{{"confirm", b.Texto}}, nil
	case BlocoMontarTexto:
		return []Par{{"compose", b.Texto}}, nil
	case BlocoReaproveitar:
		return unico("reuse", slices.Concat(
			slotsPares(b.Campos), opc("confirm", b.Confirmacao), opc("from", b.De), opc("label", b.Rotulo),
			opc("value", b.Valor), opc("ask", b.Pergunta), extra)), nil
	default:
		return nil, fmt.Errorf("tipo de bloco desconhecido %q", b.Tipo)
	}
}

func slotsPares(campos []string) []Par {
	switch len(campos) {
	case 0:
		return nil
	case 1:
		return []Par{{"slot", campos[0]}}
	default:
		return []Par{{"slots", Lista(campos)}}
	}
}

func comPares(com []CampoValor) []Par {
	if len(com) == 0 {
		return nil
	}
	pares := make([]Par, len(com))
	for i, c := range com {
		pares[i] = Par{c.Campo, c.Valor}
	}
	return []Par{{"with", pares}}
}

func paramsNo(informacoes []Informacao) (*yaml.Node, error) {
	if len(informacoes) == 0 {
		return nil, nil
	}
	itens := make(Seq, len(informacoes))
	for i, info := range informacoes {
		itens[i] = slices.Concat(
			[]Par{{"name", info.Nome}}, opc("title", info.Titulo), opc("ask", info.Pergunta), opc("type", info.Tipo),
			opcoesPares(info), extraPares(info.Extra))
	}
	no, err := NoDe(itens)
	if err != nil {
		return nil, fmt.Errorf("params: %w", err)
	}
	return no, nil
}

func opcoesPares(info Informacao) []Par {
	if len(info.Opcoes) == 0 {
		return nil
	}
	if info.OpcoesSimples {
		valores := make(Lista, len(info.Opcoes))
		for i, o := range info.Opcoes {
			valores[i] = o.Valor
		}
		return []Par{{"options", valores}}
	}
	itens := make(Seq, len(info.Opcoes))
	for i, o := range info.Opcoes {
		itens[i] = slices.Concat([]Par{{"value", o.Valor}}, opc("label", o.Rotulo))
	}
	return []Par{{"options", itens}}
}

// substituirChaves troca (ou remove, com No nil) chaves do mapa raiz; chaves ausentes entram no fim.
func SubstituirChaves(raw []byte, chaves []chaveNo) ([]byte, error) {
	doc, raiz, err := lerDocumento(raw)
	if err != nil {
		return nil, fmt.Errorf("ler caso: %w", err)
	}
	for _, c := range chaves {
		raiz.Content = definirChave(raiz.Content, c)
	}
	return CodificarCaso(doc)
}

// codificarCaso escreve o nó como o caso.yaml é escrito em todo lugar: indentação 2.
func CodificarCaso(doc *yaml.Node) ([]byte, error) {
	var saida bytes.Buffer
	enc := yaml.NewEncoder(&saida)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("escrever caso: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("escrever caso: %w", err)
	}
	return saida.Bytes(), nil
}

func definirChave(conteudo []*yaml.Node, c chaveNo) []*yaml.Node {
	i := indiceDaChave(conteudo, c.Chave)
	switch {
	case i < 0 && c.No == nil:
		return conteudo
	case i < 0:
		chave := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: c.Chave}
		return append(conteudo, chave, c.No)
	case c.No == nil:
		return slices.Delete(conteudo, i, i+2)
	default:
		conteudo[i+1] = c.No
		return conteudo
	}
}

func indiceDaChave(conteudo []*yaml.Node, chave string) int {
	for i := 0; i+1 < len(conteudo); i += 2 {
		if conteudo[i].Value == chave {
			return i
		}
	}
	return -1
}

// par é uma chave e um valor na ordem de escrita. O valor pode ser um *yaml.Node pronto,
// []par (mapa ordenado), seq (lista de nós), lista (lista em linha) ou qualquer valor codificável.
type Par struct {
	Chave string
	Valor any
}

type (
	Seq   []any
	Lista []string
)

func NoDe(v any) (*yaml.Node, error) {
	switch x := v.(type) {
	case *yaml.Node:
		return x, nil
	case []Par:
		return noMapa(x)
	case Seq:
		return noSequencia(x)
	case Lista:
		return noLista(x)
	case []string:
		return noLista(x)
	default:
		var n yaml.Node
		if err := n.Encode(v); err != nil {
			return nil, fmt.Errorf("codificar %T: %w", v, err)
		}
		return &n, nil
	}
}

func noMapa(pares []Par) (*yaml.Node, error) {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, p := range pares {
		k, err := NoDe(p.Chave)
		if err != nil {
			return nil, err
		}
		v, err := NoDe(p.Valor)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Chave, err)
		}
		n.Content = append(n.Content, k, v)
	}
	return n, nil
}

func noSequencia(itens Seq) (*yaml.Node, error) {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, item := range itens {
		no, err := NoDe(item)
		if err != nil {
			return nil, err
		}
		n.Content = append(n.Content, no)
	}
	return n, nil
}

func noLista(textos []string) (*yaml.Node, error) {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
	for _, t := range textos {
		no, err := NoDe(t)
		if err != nil {
			return nil, err
		}
		n.Content = append(n.Content, no)
	}
	return n, nil
}

// opc devolve o par só quando o texto não é vazio.
func opc(chave, texto string) []Par {
	if texto == "" {
		return nil
	}
	return []Par{{chave, texto}}
}

func unico(chave string, corpo []Par) []Par {
	return []Par{{chave, corpo}}
}

func extraPares(extra map[string]any) []Par {
	pares := make([]Par, 0, len(extra))
	for _, chave := range slices.Sorted(maps.Keys(extra)) {
		pares = append(pares, Par{chave, extra[chave]})
	}
	return pares
}

func lerDocumento(raw []byte) (doc, raiz *yaml.Node, err error) {
	doc = &yaml.Node{}
	if err := yaml.Unmarshal(raw, doc); err != nil {
		return nil, nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, nil, errors.New("caso vazio")
	}
	raiz = doc.Content[0]
	if raiz.Kind != yaml.MappingNode {
		return nil, nil, errors.New("caso deve ser um mapa")
	}
	return doc, raiz, nil
}

func entradasDe(n *yaml.Node) ([]Entrada, error) {
	if n.Kind != yaml.MappingNode {
		return nil, errors.New("esperava um mapa")
	}
	es := make([]Entrada, 0, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		es = append(es, Entrada{Chave: n.Content[i].Value, no: n.Content[i+1]})
	}
	return es, nil
}

// buscar devolve o valor da chave, ou nil se não existir.
func Buscar(es []Entrada, chave string) *yaml.Node {
	i := slices.IndexFunc(es, func(e Entrada) bool { return e.Chave == chave })
	if i < 0 {
		return nil
	}
	return es[i].no
}

func semChave(es []Entrada, chave string) []Entrada {
	return slices.DeleteFunc(slices.Clone(es), func(e Entrada) bool { return e.Chave == chave })
}

// textoDe aceita só texto de verdade (`!!str`): 123 e true sem aspas não são texto.
func textoDe(n *yaml.Node) (string, bool) {
	if n == nil {
		return "", false
	}
	return n.Value, n.Kind == yaml.ScalarNode && n.Tag == "!!str"
}

func listaDeTextos(n *yaml.Node) ([]string, bool) {
	if n == nil || n.Kind != yaml.SequenceNode {
		return nil, false
	}
	textos := make([]string, len(n.Content))
	for i, item := range n.Content {
		t, ok := textoDe(item)
		if !ok {
			return nil, false
		}
		textos[i] = t
	}
	return textos, true
}

// separar tira das entradas as chaves conhecidas que são texto não vazio; o resto (inclusive
// conhecidas vazias ou de outra forma) vai para o extra, para voltar intacto na gravação.
func separar(es []Entrada, conhecidas ...string) (map[string]string, map[string]any, error) {
	textos := map[string]string{}
	var extra map[string]any
	for _, e := range es {
		if t, ok := textoDe(e.no); ok && t != "" && slices.Contains(conhecidas, e.Chave) {
			textos[e.Chave] = t
			continue
		}
		var v any
		if err := e.no.Decode(&v); err != nil {
			return nil, nil, fmt.Errorf("chave %q: %w", e.Chave, err)
		}
		if extra == nil {
			extra = map[string]any{}
		}
		extra[e.Chave] = v
	}
	return textos, extra, nil
}

// numerar dá o id posicional (b1, b2…) só ao bloco que veio sem id.
func numerar(blocos []Bloco) []Bloco {
	numerados := make([]Bloco, len(blocos))
	for i, b := range blocos {
		if b.ID == "" {
			b.ID = fmt.Sprintf("b%d", i+1)
		}
		numerados[i] = b
	}
	return numerados
}

// FixarIDsNoCaso grava, em cada passo sem `id`, o id posicional que a leitura já lhe dá, e
// diz se mudou algo. A chave entra primeiro no passo.
func FixarIDsNoCaso(raw []byte) ([]byte, bool, error) {
	fluxo, err := FluxoDeYAML(raw, func(string) bool { return false })
	if err != nil {
		return nil, false, err
	}
	doc, raiz, err := lerDocumento(raw)
	if err != nil {
		return nil, false, fmt.Errorf("ler caso: %w", err)
	}
	es, err := entradasDe(raiz)
	if err != nil {
		return nil, false, fmt.Errorf("ler caso: %w", err)
	}
	passos := Buscar(es, "steps")
	if passos == nil || !prefixarIDs(passos, fluxo.Blocos) {
		return raw, false, nil
	}
	novo, err := CodificarCaso(doc)
	if err != nil {
		return nil, false, err
	}
	return novo, true, nil
}

// prefixarIDs põe o id de cada bloco no passo que não tem e diz se pôs algum. Os passos são
// os primeiros blocos do fluxo, na mesma ordem.
func prefixarIDs(passos *yaml.Node, blocos []Bloco) bool {
	mudou := false
	for i, passo := range passos.Content {
		if indiceDaChave(passo.Content, "id") >= 0 {
			continue
		}
		chave := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"}
		valor := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: blocos[i].ID}
		passo.Content = append([]*yaml.Node{chave, valor}, passo.Content...)
		mudou = true
	}
	return mudou
}
