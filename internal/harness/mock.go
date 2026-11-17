package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/okf"
)

// Mock é o modelo falso para rodar sem chave e sem rede. Determinístico:
//
//   - passada de rota: pontua cada rota pelas palavras-chave do roteador (peso 2)
//     e dos termos do glossário que citam a rota (peso 1); empate ou zero → self;
//   - resposta: se há ler_dado e ainda não leu, chama ler_dado no item do índice
//     (knowledge/, assets/) que melhor casa com a mensagem (o primeiro arquivo na
//     raiz de knowledge/ quando nada casa); depois responde com a seção do texto
//     que mais casa, moldada na primeira seção do contrato de saída (se ela traz
//     um JSON de exemplo, preenche o JSON).
type Mock struct {
	cat      *okf.Catalogo
	a        *okf.Agent
	tools    []*schema.ToolInfo
	rotas    map[string]map[string]int // rota → palavra → peso
	nomes    []string
	dados    []okf.Arquivo
	contrato string
}

var _ model.ToolCallingChatModel = (*Mock)(nil)

// NewMock prepara o modelo falso para um agente.
func NewMock(cat *okf.Catalogo, a *okf.Agent) *Mock {
	m := &Mock{cat: cat, a: a, dados: cat.Indice(a), rotas: map[string]map[string]int{}}
	if out, err := cat.LerArquivo(a.Rel, "rules/output.md"); err == nil {
		m.contrato = primeiraSecao(okf.StripFrontmatter(string(out)))
	}
	for _, s := range a.Subagentes {
		m.nomes = append(m.nomes, s.Nome)
		m.rotas[s.Nome] = map[string]int{}
	}
	sort.Strings(m.nomes)
	if a.Tipo == okf.Orquestrador {
		m.indexarRotas(cat.Roteador(a))
		m.indexarGlossario(cat.Conhecimento(a).Termos)
	}
	return m
}

// WithTools devolve uma cópia com as tools ligadas.
func (m *Mock) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	c := *m
	c.tools = tools
	return &c, nil
}

// Stream embrulha Generate numa stream de um pedaço.
func (m *Mock) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	out, err := m.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{out}), nil
}

// Generate decide entre o modo rota e o modo resposta pelo system prompt.
func (m *Mock) Generate(_ context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	o := model.GetCommonOptions(&model.Options{}, opts...)
	tools := m.tools
	if len(o.Tools) > 0 {
		tools = o.Tools
	}
	var sistema, usuario string
	var ultimaTool *schema.Message
	entrada := 0
	for _, msg := range in {
		entrada += len(msg.Content)
		switch msg.Role {
		case schema.System:
			if sistema == "" {
				sistema = msg.Content
			}
		case schema.User:
			usuario = msg.Content
		case schema.Tool:
			ultimaTool = msg
		}
	}

	var resposta *schema.Message
	switch {
	case strings.HasPrefix(sistema, cabecalhoRoteador):
		resposta = schema.AssistantMessage(m.escolherRota(usuario), nil)
	case ultimaTool == nil && temTool(tools, "ler_dado") && len(m.dados) > 0:
		args, _ := json.Marshal(map[string]any{"caminho": m.escolherDado(usuario), "pagina": 1})
		resposta = schema.AssistantMessage("", []schema.ToolCall{{
			ID:       "mock-ler-dado",
			Type:     "function",
			Function: schema.FunctionCall{Name: "ler_dado", Arguments: string(args)},
		}})
	case ultimaTool != nil:
		conteudo := ultimaTool.Content
		var lido struct {
			Conteudo string `json:"conteudo"`
			Erro     string `json:"error"`
		}
		if json.Unmarshal([]byte(conteudo), &lido) == nil {
			if lido.Erro != "" {
				resposta = schema.AssistantMessage("Não consegui consultar a informação agora ("+lido.Erro+").", nil)
				break
			}
			if lido.Conteudo != "" {
				conteudo = lido.Conteudo
			}
		}
		secao, titulo := melhorSecao(conteudo, usuario)
		resposta = schema.AssistantMessage(m.moldar(secao, titulo), nil)
	default:
		texto := m.contrato
		if texto == "" {
			texto = "[mock] " + corta(usuario, 120)
		}
		resposta = schema.AssistantMessage(texto, nil)
	}

	saida := len(resposta.Content)
	for _, tc := range resposta.ToolCalls {
		saida += len(tc.Function.Arguments)
	}
	resposta.ResponseMeta = &schema.ResponseMeta{
		FinishReason: "stop",
		Usage: &schema.TokenUsage{
			PromptTokens:     entrada/4 + 1,
			CompletionTokens: saida/4 + 1,
			TotalTokens:      entrada/4 + saida/4 + 2,
		},
	}
	return resposta, nil
}

func temTool(tools []*schema.ToolInfo, nome string) bool {
	for _, t := range tools {
		if t != nil && t.Name == nome {
			return true
		}
	}
	return false
}

// ---- rota -----------------------------------------------------------------

var (
	reRotaMock = regexp.MustCompile("rota\\s+`([^`]+)`")
	reItemMock = regexp.MustCompile(`^\s*[-*]\s+`)
)

// indexarRotas lê os itens "critério → rota `x`" do roteador e guarda as palavras.
func (m *Mock) indexarRotas(texto string) {
	var blocos [][]string
	for _, l := range strings.Split(texto, "\n") {
		if reItemMock.MatchString(l) || len(blocos) == 0 {
			blocos = append(blocos, []string{l})
		} else {
			blocos[len(blocos)-1] = append(blocos[len(blocos)-1], l)
		}
	}
	for _, b := range blocos {
		junto := strings.Join(b, " ")
		mm := reRotaMock.FindStringSubmatch(junto)
		if mm == nil {
			continue
		}
		rota := strings.TrimSpace(mm[1])
		pesos, ok := m.rotas[rota]
		if !ok {
			continue
		}
		criterio, _, _ := strings.Cut(junto, mm[0])
		for _, p := range palavras(criterio) {
			if p == rota {
				continue
			}
			if pesos[p] < 2 {
				pesos[p] = 2
			}
		}
	}
}

// indexarGlossario liga os sinônimos de um termo às rotas que a definição ou os
// labels citam (ex.: obra → regimento).
func (m *Mock) indexarGlossario(termos []okf.Termo) {
	for _, t := range termos {
		contexto := palavrasSet(strings.Join(t.Labels, " ") + " " + t.Definicao)
		for _, rota := range m.nomes {
			if !contexto[normalizar(rota)] {
				continue
			}
			for _, s := range append([]string{t.Nome, t.ID}, t.Sinonimos...) {
				for _, p := range palavras(s) {
					if m.rotas[rota][p] == 0 {
						m.rotas[rota][p] = 1
					}
				}
			}
		}
	}
}

// semContexto tira as linhas de contexto que o canal antepõe à mensagem
// ("[morador identificado: …, unidades: …]"): só o que o morador escreveu pontua.
func semContexto(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func (m *Mock) escolherRota(mensagem string) string {
	ps := palavrasSet(semContexto(mensagem))
	melhor, pontos, empate := "self", 0, false
	for _, rota := range m.nomes {
		total := 0
		for p, peso := range m.rotas[rota] {
			if ps[p] {
				total += peso
			}
		}
		switch {
		case total > pontos:
			melhor, pontos, empate = rota, total, false
		case total == pontos && total > 0:
			empate = true
		}
	}
	if pontos == 0 || empate {
		return "self"
	}
	return melhor
}

// ---- dados e resposta -----------------------------------------------------

func (m *Mock) escolherDado(mensagem string) string {
	ps := palavrasSet(mensagem)
	melhor, pontos := "", -1
	for _, d := range m.dados {
		total := 0
		for p := range palavrasSet(d.Caminho + " " + d.Resumo) {
			if ps[p] {
				total++
			}
		}
		if total > pontos || (total == pontos && naRaiz(d.Caminho) && !naRaiz(melhor)) {
			melhor, pontos = d.Caminho, total
		}
	}
	return melhor
}

// naRaiz: arquivo direto na caixinha (knowledge/x.md), não numa subpasta tipada
// (knowledge/questions/x.md); é o texto "principal" que o mock prefere ler.
func naRaiz(caminho string) bool {
	return caminho != "" && strings.Count(caminho, "/") == 1
}

// melhorSecao divide o texto nos títulos (## ou #) e devolve a seção com mais
// ocorrências das palavras da mensagem; sem acerto, a primeira seção com título.
func melhorSecao(texto, mensagem string) (secao, titulo string) {
	texto = okf.StripFrontmatter(texto)
	linhas := strings.Split(texto, "\n")
	nivel := "## "
	if !strings.Contains(texto, "\n## ") && !strings.HasPrefix(texto, "## ") {
		nivel = "# "
	}
	type sec struct{ titulo, corpo string }
	var secs []sec
	atual := sec{}
	for _, l := range linhas {
		if strings.HasPrefix(l, nivel) {
			if atual.titulo != "" || strings.TrimSpace(atual.corpo) != "" {
				secs = append(secs, atual)
			}
			atual = sec{titulo: strings.TrimSpace(strings.TrimPrefix(l, nivel)), corpo: l + "\n"}
			continue
		}
		atual.corpo += l + "\n"
	}
	if atual.titulo != "" || strings.TrimSpace(atual.corpo) != "" {
		secs = append(secs, atual)
	}
	if len(secs) == 0 {
		return strings.TrimSpace(texto), ""
	}
	ps := palavras(mensagem)
	melhor, pontos := -1, 0
	for i, s := range secs {
		if s.titulo == "" {
			continue
		}
		corpo := normalizar(s.corpo)
		total := 0
		for _, p := range ps {
			total += strings.Count(corpo, p)
		}
		if total > pontos {
			melhor, pontos = i, total
		}
	}
	if melhor == -1 {
		for i, s := range secs {
			if s.titulo != "" {
				melhor = i
				break
			}
		}
		if melhor == -1 {
			melhor = 0
		}
	}
	return strings.TrimSpace(secs[melhor].corpo), secs[melhor].titulo
}

// moldar encaixa a seção no contrato: se a primeira seção do contrato traz um
// JSON de exemplo, a primeira chave string recebe o texto e as listas recebem o
// título; senão devolve o texto (com a primeira seção do contrato como moldura).
func (m *Mock) moldar(secao, titulo string) string {
	if bloco := blocoJSON(m.contrato); bloco != "" {
		var obj map[string]any
		if json.Unmarshal([]byte(bloco), &obj) == nil && len(obj) > 0 {
			chaves := make([]string, 0, len(obj))
			for k := range obj {
				chaves = append(chaves, k)
			}
			sort.Strings(chaves)
			textoPosto := false
			for _, k := range chaves {
				switch obj[k].(type) {
				case string:
					if !textoPosto {
						obj[k] = secao
						textoPosto = true
					} else {
						obj[k] = ""
					}
				case []any:
					if titulo != "" {
						obj[k] = []string{titulo}
					} else {
						obj[k] = []string{}
					}
				}
			}
			out, _ := json.Marshal(obj)
			return string(out)
		}
	}
	if m.contrato == "" {
		return secao
	}
	return m.contrato + "\n\n" + secao
}

// primeiraSecao: o corpo do contrato até o segundo título de nível 2.
func primeiraSecao(texto string) string {
	texto = strings.TrimSpace(texto)
	partes := strings.SplitN("\n"+texto, "\n## ", 3)
	if len(partes) >= 3 {
		return strings.TrimSpace(partes[0] + "\n## " + partes[1])
	}
	return texto
}

var reBlocoJSON = regexp.MustCompile("(?s)```json\\s*(.*?)```")

func blocoJSON(texto string) string {
	if mm := reBlocoJSON.FindStringSubmatch(texto); mm != nil {
		return strings.TrimSpace(mm[1])
	}
	return ""
}

// ---- texto ----------------------------------------------------------------

var ignoradas = map[string]bool{
	"rota": true, "self": true, "para": true, "como": true, "qualquer": true, "outra": true, "outro": true,
	"coisa": true, "mesmo": true, "mesma": true, "sobre": true, "sinal": true, "caso": true, "isso": true,
	"quando": true, "onde": true, "pelo": true, "pela": true, "voce": true, "responda": true,
	"mensagem": true, "morador": true, "linha": true, "melhor": true, "descreve": true, "escolha": true,
	"deixe": true, "ainda": true, "nunca": true, "sempre": true, "entre": true, "pode": true, "posso": true,
	"fazer": true, "depois": true, "antes": true, "quero": true, "queria": true, "gostaria": true, "preciso": true,
	"esta": true, "este": true, "essa": true, "esse": true, "pedido": true, "duvida": true, "geral": true,
	"condominio": true, "mais": true, "menos": true, "abriu": true, "fechou": true, "qual": true, "quais": true,
}

var acentos = map[rune]rune{
	'á': 'a', 'à': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'é': 'e', 'ê': 'e', 'è': 'e', 'ë': 'e',
	'í': 'i', 'ì': 'i', 'î': 'i', 'ï': 'i', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ò': 'o', 'ö': 'o',
	'ú': 'u', 'ù': 'u', 'û': 'u', 'ü': 'u', 'ç': 'c', 'ñ': 'n',
}

func normalizar(s string) string {
	s = strings.ToLower(s)
	return strings.Map(func(r rune) rune {
		if x, ok := acentos[r]; ok {
			return x
		}
		return r
	}, s)
}

// palavras: tokens normalizados com 4+ letras (ou com dígito, como "22h"), sem as
// palavras ignoradas. Repetições mantidas (para contar ocorrências).
func palavras(s string) []string {
	s = normalizar(s)
	campos := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	var out []string
	for _, c := range campos {
		temDigito := strings.ContainsAny(c, "0123456789")
		if (len(c) < 4 && !temDigito) || ignoradas[c] {
			continue
		}
		out = append(out, c)
	}
	return out
}

func palavrasSet(s string) map[string]bool {
	out := map[string]bool{}
	for _, p := range palavras(s) {
		out[p] = true
	}
	return out
}

// String descreve o mock (para a inspeção).
func (m *Mock) String() string {
	return fmt.Sprintf("mock(%s, %d rotas, %d dados)", m.a.Rel, len(m.nomes), len(m.dados))
}
