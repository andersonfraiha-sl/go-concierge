package pedidos

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"go-concierge/internal/okf"
)

// Slot é um parâmetro que o pedido precisa: vem de `params:` (coleta) ou de um passo
// `choose` (escolha entre opções devolvidas por um recurso).
type Slot struct {
	Nome        string  `json:"nome"`
	Titulo      string  `json:"titulo"`
	Pergunta    string  `json:"pergunta"`
	Tipo        string  `json:"tipo"` // text | textarea | date | number | phone | select | checkbox | list | choose
	Obrigatorio bool    `json:"obrigatorio"`
	Opcoes      []Opcao `json:"opcoes,omitempty"` // select/checkbox com opções fixas
	// choose: de onde vêm as opções (save_as de um recurso) e quais campos são rótulo e valor
	De     string `json:"de,omitempty"`
	Rotulo string `json:"rotulo,omitempty"`
	Valor  string `json:"valor,omitempty"`
	Vazio  string `json:"vazio,omitempty"`
	Reuse  string `json:"reuse,omitempty"`
}

// Passo é um passo do caso.yaml já interpretado.
type Passo struct {
	Tipo    string         // recurso | escolher | coletar | confirmar | formulario | redigir
	Recurso string         // recurso
	SaveAs  string         // recurso
	With    map[string]any // recurso: parâmetros com templates
	Slot    string         // escolher
	Coleta  []string       // coletar
	Texto   string         // confirmar: template
}

// Contrato é o caso.yaml como máquina de passos: o que o pedido precisa e em que ordem.
type Contrato struct {
	Caso   *okf.Caso
	Passos []Passo
	Slots  map[string]*Slot
	Ordem  []string // slots na ordem em que o fluxo os pede
	Output string
}

// ContratoDe interpreta o caso.yaml. Um caso com passo `form` é marcado (o motor não
// executa formulários dinâmicos; a conversa segue pelo subagente).
func ContratoDe(c *okf.Caso) (*Contrato, error) {
	if c == nil {
		return nil, errors.New("pedidos: caso nulo")
	}
	ct := &Contrato{Caso: c, Slots: map[string]*Slot{}, Output: c.Output}
	params := map[string]okf.CasoParam{}
	for _, p := range c.Params {
		params[p.Name] = p
	}
	for _, s := range c.Steps {
		switch {
		case s["resource"] != nil:
			p := Passo{Tipo: "recurso", Recurso: fmt.Sprint(s["resource"])}
			if sa, ok := s["save_as"].(string); ok {
				p.SaveAs = sa
			}
			if w, ok := s["with"].(map[string]any); ok {
				p.With = w
			}
			ct.Passos = append(ct.Passos, p)
		case s["choose"] != nil:
			m, ok := s["choose"].(map[string]any)
			if !ok {
				return nil, errors.New("pedidos: choose inválido")
			}
			nome := str(m["param"])
			if nome == "" {
				return nil, errors.New("pedidos: choose sem param")
			}
			sl := &Slot{Nome: nome, Titulo: str(m["title"]), Pergunta: str(m["ask"]), Tipo: "choose", Obrigatorio: true,
				De: str(m["from"]), Rotulo: str(m["label"]), Valor: str(m["value"]), Vazio: str(m["empty"]), Reuse: str(m["reuse"])}
			if sl.Titulo == "" {
				sl.Titulo = nome
			}
			if sl.Pergunta == "" {
				sl.Pergunta = "Qual " + strings.ToLower(sl.Titulo) + "?"
			}
			ct.addSlot(sl)
			ct.Passos = append(ct.Passos, Passo{Tipo: "escolher", Slot: nome})
		case s["collect"] != nil:
			var nomes []string
			switch v := s["collect"].(type) {
			case []any:
				for _, x := range v {
					nomes = append(nomes, fmt.Sprint(x))
				}
			case string:
				nomes = append(nomes, v)
			}
			for _, n := range nomes {
				p, ok := params[n]
				if !ok {
					p = okf.CasoParam{Name: n}
				}
				ct.addSlot(slotDeParam(p))
			}
			ct.Passos = append(ct.Passos, Passo{Tipo: "coletar", Coleta: nomes})
		case s["confirm"] != nil:
			ct.Passos = append(ct.Passos, Passo{Tipo: "confirmar", Texto: fmt.Sprint(s["confirm"])})
		case s["form"] != nil:
			ct.Passos = append(ct.Passos, Passo{Tipo: "formulario"})
		case s["compose"] != nil:
			ct.Passos = append(ct.Passos, Passo{Tipo: "redigir", Texto: str(s["compose"])})
		}
	}
	// params declarados que nenhum collect cita ainda entram como opcionais extraíveis
	for _, p := range c.Params {
		if _, ok := ct.Slots[p.Name]; !ok {
			sl := slotDeParam(p)
			ct.addSlot(sl)
		}
	}
	return ct, nil
}

func (ct *Contrato) addSlot(s *Slot) {
	if _, ok := ct.Slots[s.Nome]; ok {
		return
	}
	ct.Slots[s.Nome] = s
	ct.Ordem = append(ct.Ordem, s.Nome)
}

// TemFormulario: o caso tem um passo `form`, que o motor não executa.
func (ct *Contrato) TemFormulario() bool {
	for _, p := range ct.Passos {
		if p.Tipo == "formulario" {
			return true
		}
	}
	return false
}

func slotDeParam(p okf.CasoParam) *Slot {
	s := &Slot{Nome: p.Name, Titulo: p.Title, Pergunta: p.Ask, Tipo: strings.ToLower(strings.TrimSpace(p.Type)), Obrigatorio: p.Required == nil || *p.Required, Reuse: p.Reuse}
	if s.Tipo == "" {
		s.Tipo = "text"
	}
	if strings.EqualFold(p.Validate, "date") {
		s.Tipo = "date"
	}
	if s.Titulo == "" {
		s.Titulo = p.Name
	}
	if s.Pergunta == "" {
		s.Pergunta = "Qual " + strings.ToLower(s.Titulo) + "?"
	}
	s.Opcoes = opcoesDe(p.Options)
	return s
}

func opcoesDe(v any) []Opcao {
	lista, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []Opcao
	for _, it := range lista {
		switch o := it.(type) {
		case map[string]any:
			val, rot := str(o["value"]), str(o["label"])
			if val == "" {
				val = rot
			}
			if rot == "" {
				rot = val
			}
			if val != "" {
				out = append(out, Opcao{Valor: val, Rotulo: rot})
			}
		default:
			t := fmt.Sprint(o)
			out = append(out, Opcao{Valor: t, Rotulo: t})
		}
	}
	return out
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// ---- validação e normalização ---------------------------------------------------------

// Validar normaliza um valor para o slot; devolve o valor, o rótulo humano e, se não
// serviu, a razão (para perguntar de novo com a correção).
func (s *Slot) Validar(v any, opcoes []Opcao) (valor any, rotulo string, motivo string) {
	if v == nil {
		return nil, "", "vazio"
	}
	texto := strings.TrimSpace(fmt.Sprint(v))
	switch s.Tipo {
	case "choose", "select":
		ops := opcoes
		if len(ops) == 0 {
			ops = s.Opcoes
		}
		if o, ok := casarOpcao(texto, ops); ok {
			return o.Valor, o.Rotulo, ""
		}
		if len(ops) == 0 {
			return texto, texto, ""
		}
		return nil, "", "não é uma das opções: " + rotulos(ops)
	case "date":
		t, err := parseData(texto)
		if err != nil {
			return nil, "", "data inválida (use dd/mm ou dd/mm/aaaa)"
		}
		return t.Format("2006-01-02"), dataPorExtenso(t), ""
	case "number":
		n, err := strconv.ParseFloat(strings.ReplaceAll(texto, ",", "."), 64)
		if err != nil {
			n2, ok := numeroPorExtenso(texto)
			if !ok {
				return nil, "", "precisa ser um número"
			}
			n = n2
		}
		if n == float64(int64(n)) {
			return int64(n), strconv.FormatInt(int64(n), 10), ""
		}
		return n, texto, ""
	case "phone":
		d := naoDigitos.ReplaceAllString(texto, "")
		if len(d) < 10 || len(d) > 13 {
			return nil, "", "telefone com DDD (10 a 13 dígitos)"
		}
		return d, texto, ""
	case "checkbox":
		if len(s.Opcoes) > 0 {
			itens := listaDe(v)
			var vals []string
			var rots []string
			for _, it := range itens {
				if o, ok := casarOpcao(it, s.Opcoes); ok {
					vals = append(vals, o.Valor)
					rots = append(rots, o.Rotulo)
				}
			}
			if len(vals) == 0 {
				return nil, "", "nenhuma das opções: " + rotulos(s.Opcoes)
			}
			return vals, strings.Join(rots, ", "), ""
		}
		b, ok := booleano(v)
		if !ok {
			return nil, "", "responda sim ou não"
		}
		if b {
			return true, "sim", ""
		}
		return false, "não", ""
	case "list":
		itens := listaDe(v)
		if len(itens) == 0 {
			return nil, "", "informe ao menos um"
		}
		return itens, strings.Join(itens, ", "), ""
	}
	if texto == "" {
		return nil, "", "vazio"
	}
	return texto, texto, ""
}

func rotulos(ops []Opcao) string {
	var r []string
	for _, o := range ops {
		r = append(r, o.Rotulo)
	}
	return strings.Join(r, ", ")
}

func casarOpcao(texto string, ops []Opcao) (Opcao, bool) {
	t := normalizar(texto)
	if t == "" {
		return Opcao{}, false
	}
	for _, o := range ops {
		if normalizar(o.Valor) == t || normalizar(o.Rotulo) == t {
			return o, true
		}
	}
	// número da opção ("2") ou rótulo contido ("salao" em "Salão de festas")
	if n, err := strconv.Atoi(t); err == nil && n >= 1 && n <= len(ops) {
		return ops[n-1], true
	}
	var achada *Opcao
	for i := range ops {
		r := normalizar(ops[i].Rotulo)
		if strings.Contains(r, t) || strings.Contains(t, r) {
			if achada != nil {
				return Opcao{}, false // ambíguo
			}
			achada = &ops[i]
		}
	}
	if achada != nil {
		return *achada, true
	}
	return Opcao{}, false
}

var acentos = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ç", "c")

func normalizar(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = acentos.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func listaDe(v any) []string {
	switch x := v.(type) {
	case []any:
		var out []string
		for _, it := range x {
			if t := strings.TrimSpace(fmt.Sprint(it)); t != "" {
				out = append(out, t)
			}
		}
		return out
	case []string:
		return x
	}
	t := strings.TrimSpace(fmt.Sprint(v))
	if t == "" {
		return nil
	}
	t = regexp.MustCompile(`(?i)\s+e\s+`).ReplaceAllString(t, ",")
	var out []string
	for _, p := range strings.Split(t, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func booleano(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	}
	switch normalizar(fmt.Sprint(v)) {
	case "sim", "s", "true", "yes", "vai", "vou", "tem", "com", "claro", "1":
		return true, true
	case "nao", "n", "false", "no", "sem", "nenhum", "nenhuma", "0":
		return false, true
	}
	return false, false
}

var numerais = map[string]float64{"um": 1, "uma": 1, "dois": 2, "duas": 2, "tres": 3, "quatro": 4, "cinco": 5, "seis": 6, "sete": 7, "oito": 8, "nove": 9, "dez": 10, "quinze": 15, "vinte": 20, "trinta": 30}

func numeroPorExtenso(t string) (float64, bool) {
	for _, p := range strings.Fields(normalizar(t)) {
		if n, ok := numerais[p]; ok {
			return n, true
		}
		if n, err := strconv.ParseFloat(p, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

var meses = []string{"", "janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"}

// Agora é injetável nos testes.
var Agora = time.Now

func parseData(t string) (time.Time, error) {
	t = strings.TrimSpace(t)
	for _, f := range []string{"2006-01-02", "02/01/2006", "02/01/06", "2006-01-02T15:04:05Z07:00"} {
		if d, err := time.Parse(f, t); err == nil {
			return d, nil
		}
	}
	hoje := Agora()
	if m := regexp.MustCompile(`^(\d{1,2})/(\d{1,2})$`).FindStringSubmatch(t); m != nil {
		d, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		return proximaData(hoje, d, mo)
	}
	n := normalizar(t)
	if m := regexp.MustCompile(`^(?:dia )?(\d{1,2}) de ([a-z]+)(?: de (\d{4}))?$`).FindStringSubmatch(n); m != nil {
		d, _ := strconv.Atoi(m[1])
		for i, nome := range meses {
			if i > 0 && normalizar(nome) == m[2] {
				if m[3] != "" {
					y, _ := strconv.Atoi(m[3])
					return time.Date(y, time.Month(i), d, 0, 0, 0, 0, hoje.Location()), nil
				}
				return proximaData(hoje, d, i)
			}
		}
	}
	switch n {
	case "hoje":
		return hoje, nil
	case "amanha":
		return hoje.AddDate(0, 0, 1), nil
	}
	return time.Time{}, errors.New("data inválida")
}

// proximaData: dia/mês sem ano é a próxima ocorrência (este ano, ou o próximo se já passou).
func proximaData(hoje time.Time, dia, mes int) (time.Time, error) {
	if mes < 1 || mes > 12 || dia < 1 || dia > 31 {
		return time.Time{}, errors.New("data inválida")
	}
	d := time.Date(hoje.Year(), time.Month(mes), dia, 0, 0, 0, 0, hoje.Location())
	if d.Day() != dia || int(d.Month()) != mes { // 31/02 "normaliza" para março: não vale
		return time.Time{}, errors.New("data inválida")
	}
	if d.Before(hoje.Truncate(24 * time.Hour)) {
		d = d.AddDate(1, 0, 0)
	}
	return d, nil
}

func dataPorExtenso(t time.Time) string {
	return fmt.Sprintf("%d de %s de %d", t.Day(), meses[t.Month()], t.Year())
}

// ---- templates -------------------------------------------------------------------------

// Vars monta o contexto dos templates: valores, rótulos (<slot>_label) e dados.
func (ct *Contrato) Vars(p *Pedido) map[string]any {
	vars := map[string]any{}
	for nome := range ct.Slots {
		vars[nome] = ""
		vars[nome+"_label"] = ""
	}
	for k, v := range p.Valores {
		vars[k] = v
		vars[k+"_label"] = p.Rotulos[k]
	}
	for k, v := range p.Dados {
		vars[k] = v
	}
	return vars
}

// Render aplica um template do caso.yaml ({{.area_label}}, {{.reserva.protocolo}}).
func Render(texto string, vars map[string]any) (string, error) {
	tpl, err := template.New("t").Option("missingkey=zero").Parse(texto)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := tpl.Execute(&b, vars); err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.ReplaceAll(b.String(), "<no value>", "")), nil
}

// ParamsDe monta os parâmetros de um recurso a partir de `with`: "{{.x}}" renderiza,
// "$lista" passa o valor inteiro, o resto vai como está.
func (ct *Contrato) ParamsDe(with map[string]any, p *Pedido) (map[string]any, error) {
	vars := ct.Vars(p)
	out := map[string]any{}
	for k, v := range with {
		s, ok := v.(string)
		if !ok {
			out[k] = v
			continue
		}
		if strings.HasPrefix(s, "$") {
			if val, ok := p.Valores[strings.TrimPrefix(s, "$")]; ok {
				out[k] = val
			}
			continue
		}
		r, err := Render(s, vars)
		if err != nil {
			return nil, fmt.Errorf("param %s: %w", k, err)
		}
		if r != "" {
			out[k] = r
		}
	}
	return out, nil
}
