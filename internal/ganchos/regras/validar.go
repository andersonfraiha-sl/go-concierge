package regras

import (
	"fmt"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // fuso da regra existe mesmo em imagem sem zoneinfo
	"unicode/utf8"

	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/pii"
)

const (
	limiteAtivasPorEvento = 20
	limiteAtivasTotal     = 10_000
	limiteCondicoes       = 5
	limiteLista           = 20
	limiteFeriadosExtras  = 30
	limiteTexto           = 300
	tamanhoMinimoNome     = 3
	tamanhoMaximoNome     = 80
)

// Problema é um achado da validação; ValidarRegra devolve todos de uma vez.
type Problema struct{ Campo, Codigo, Mensagem string }

// Catalogo é o que a validação consulta para saber o que existe.
type Catalogo struct {
	Ferramentas, Casos map[string]bool
	AdministradoraDe   map[string]string // condomínio -> administradora
	NomesDosFixos      []string
}

// aceita é a matriz da spec §4: o que cada evento aceita de regra declarativa.
// Ajustar não aparece porque é exclusivo de gancho fixo; transbordo só admite Deixar,
// então nenhuma decisão de regra vale ali.
var aceita = map[ganchos.NomeDoEvento]struct {
	condicoes []TipoCondicao
	decisoes  []ganchos.TipoDecisao
}{
	ganchos.EventoMensagemRecebida: {
		[]TipoCondicao{CondicaoForaDoHorario, CondicaoDiaDaSemana, CondicaoDiaNaoUtil, CondicaoContatoCadastrado, CondicaoPapelDoMorador, CondicaoTextoContem},
		[]ganchos.TipoDecisao{ganchos.Bloquear},
	},
	ganchos.EventoAntesDaFerramenta: {
		[]TipoCondicao{CondicaoFerramenta, CondicaoCaso, CondicaoForaDoHorario, CondicaoDiaDaSemana, CondicaoDiaNaoUtil, CondicaoContatoCadastrado, CondicaoPapelDoMorador},
		[]ganchos.TipoDecisao{ganchos.Bloquear, ganchos.Confirmar},
	},
	ganchos.EventoDepoisDaFerramenta: {
		[]TipoCondicao{CondicaoFerramenta, CondicaoCaso, CondicaoForaDoHorario, CondicaoDiaDaSemana, CondicaoDiaNaoUtil, CondicaoContatoCadastrado, CondicaoPapelDoMorador},
		[]ganchos.TipoDecisao{ganchos.Bloquear},
	},
	ganchos.EventoAntesDeResponder: {
		[]TipoCondicao{CondicaoForaDoHorario, CondicaoDiaDaSemana, CondicaoDiaNaoUtil, CondicaoContatoCadastrado, CondicaoPapelDoMorador, CondicaoTextoContem},
		[]ganchos.TipoDecisao{ganchos.Bloquear},
	},
	ganchos.EventoTransbordoOuEncerramento: {
		[]TipoCondicao{CondicaoForaDoHorario, CondicaoDiaDaSemana, CondicaoDiaNaoUtil, CondicaoContatoCadastrado, CondicaoPapelDoMorador},
		nil,
	},
	ganchos.EventoCopilotoAntesDeAceitar: {
		[]TipoCondicao{CondicaoForaDoHorario, CondicaoDiaDaSemana, CondicaoDiaNaoUtil},
		[]ganchos.TipoDecisao{ganchos.Bloquear, ganchos.Confirmar},
	},
}

// eventosComParametro são os eventos em que Fonte{parametro} faz sentido.
var eventosComParametro = []ganchos.NomeDoEvento{ganchos.EventoAntesDaFerramenta, ganchos.EventoDepoisDaFerramenta}

var diasDaSemana = []string{"seg", "ter", "qua", "qui", "sex", "sab", "dom"}

// ValidarRegra devolve todos os problemas da regra; vazio significa válida. Só pula a
// checagem que depende de um campo já quebrado (a matriz sem evento válido).
func ValidarRegra(r Regra, cat Catalogo) []Problema {
	ev, eventoOK := aceita[r.Evento]
	var ps []Problema
	ps = append(ps, validarNome(r.Nome, cat.NomesDosFixos)...)
	if !eventoOK {
		ps = append(ps, Problema{"evento", "EVENTO_INVALIDO", "evento desconhecido: " + string(r.Evento)})
	}
	ps = append(ps, validarAlvo(r.Alvo, cat)...)
	ps = append(ps, validarFuso(r.Fuso)...)
	ps = append(ps, validarCondicoes(r, ev.condicoes, eventoOK, cat)...)
	return append(ps, validarDecisao(r.Decisao, ev.decisoes, eventoOK)...)
}

func validarNome(nome string, fixos []string) []Problema {
	var ps []Problema
	if n := utf8.RuneCountInString(strings.TrimSpace(nome)); n < tamanhoMinimoNome || n > tamanhoMaximoNome {
		ps = append(ps, Problema{"nome", "NOME_TAMANHO", fmt.Sprintf("o nome precisa ter de %d a %d caracteres", tamanhoMinimoNome, tamanhoMaximoNome)})
	}
	if slices.ContainsFunc(fixos, func(f string) bool { return strings.EqualFold(strings.TrimSpace(f), strings.TrimSpace(nome)) }) {
		ps = append(ps, Problema{"nome", "NOME_DE_FIXO", "o nome é de um gancho fixo; escolha outro"})
	}
	return ps
}

func validarAlvo(a Alvo, cat Catalogo) []Problema {
	var ps []Problema
	if len(a.Condominios) > 0 && a.Administradora == "" {
		ps = append(ps, Problema{"alvo.administradora", "ALVO_SEM_ADMINISTRADORA", "condomínios exigem a administradora do alvo"})
	}
	for _, c := range a.Condominios {
		adm, existe := cat.AdministradoraDe[c]
		switch {
		case !existe:
			ps = append(ps, Problema{"alvo.condominios", "CONDOMINIO_DESCONHECIDO", "condomínio fora do catálogo: " + c})
		case a.Administradora != "" && adm != a.Administradora:
			ps = append(ps, Problema{"alvo.condominios", "CONDOMINIO_DE_OUTRA_ADMINISTRADORA", "o condomínio " + c + " não é da administradora do alvo"})
		}
	}
	return ps
}

// validarFuso aceita só zona IANA no formato Area/Cidade: "Local" e "UTC" dependem do
// servidor ou não são zona de negócio. Vazio vira FusoPadrao no Decodificar.
func validarFuso(fuso string) []Problema {
	if fuso == "" {
		return nil
	}
	if _, err := time.LoadLocation(fuso); err != nil || !strings.Contains(fuso, "/") {
		return []Problema{{"fuso", "FUSO_INVALIDO", "use uma zona IANA no formato Area/Cidade: " + fuso}}
	}
	return nil
}

func validarCondicoes(r Regra, aceitas []TipoCondicao, eventoOK bool, cat Catalogo) []Problema {
	var ps []Problema
	if n := len(r.Condicoes); n < 1 || n > limiteCondicoes {
		ps = append(ps, Problema{"condicoes", "CONDICOES_QUANTIDADE", fmt.Sprintf("a regra precisa ter de 1 a %d condições", limiteCondicoes)})
	}
	for i, c := range r.Condicoes {
		campo := fmt.Sprintf("condicoes[%d]", i)
		if !conferePreenchimento(c) {
			ps = append(ps, Problema{campo, "CONDICAO_MALFORMADA", "a condição precisa trazer só os campos do tipo " + string(c.Tipo)})
			continue
		}
		if eventoOK && !slices.Contains(aceitas, c.Tipo) {
			ps = append(ps, Problema{campo, "CONDICAO_NAO_ACEITA", fmt.Sprintf("o evento %s não aceita a condição %s", r.Evento, c.Tipo)})
			continue
		}
		ps = append(ps, validarCondicao(campo, c, r.Evento, cat)...)
	}
	return ps
}

// conferePreenchimento exige que só o ponteiro do tipo da condição esteja preenchido.
func conferePreenchimento(c Condicao) bool {
	preenchidos := map[TipoCondicao]bool{
		CondicaoFerramenta:        c.Ferramenta != nil,
		CondicaoCaso:              c.Caso != nil,
		CondicaoForaDoHorario:     c.ForaDoHorario != nil,
		CondicaoDiaDaSemana:       c.DiaDaSemana != nil,
		CondicaoDiaNaoUtil:        c.DiaNaoUtil != nil,
		CondicaoContatoCadastrado: c.ContatoCadastrado != nil,
		CondicaoPapelDoMorador:    c.PapelDoMorador != nil,
		CondicaoTextoContem:       c.TextoContem != nil,
	}
	if !preenchidos[c.Tipo] {
		return false
	}
	n := 0
	for _, ok := range preenchidos {
		if ok {
			n++
		}
	}
	return n == 1
}

func validarCondicao(campo string, c Condicao, ev ganchos.NomeDoEvento, cat Catalogo) []Problema {
	switch c.Tipo {
	case CondicaoFerramenta:
		return validarLista(campo+".nomes", c.Ferramenta.Nomes, limiteLista, "FERRAMENTA_DESCONHECIDA", cat.Ferramentas)
	case CondicaoCaso:
		return validarLista(campo+".ids", c.Caso.IDs, limiteLista, "CASO_DESCONHECIDO", cat.Casos)
	case CondicaoForaDoHorario:
		return append(validarFonte(campo+".fonte", c.ForaDoHorario.Fonte, ev),
			validarHorario(campo, c.ForaDoHorario)...)
	case CondicaoDiaDaSemana:
		return append(validarFonte(campo+".fonte", c.DiaDaSemana.Fonte, ev),
			validarDias(campo+".dias", c.DiaDaSemana.Dias)...)
	case CondicaoDiaNaoUtil:
		return append(validarFonte(campo+".fonte", c.DiaNaoUtil.Fonte, ev),
			validarDatas(campo+".feriados_extras", c.DiaNaoUtil.FeriadosExtras)...)
	case CondicaoContatoCadastrado:
		if c.ContatoCadastrado.Valor == nil {
			return []Problema{{campo + ".valor", "VALOR_AUSENTE", "informe valor: true ou false"}}
		}
		return nil
	case CondicaoPapelDoMorador:
		return validarLista(campo+".papeis", c.PapelDoMorador.Papeis, limiteLista, "", nil)
	case CondicaoTextoContem:
		return validarLista(campo+".termos", c.TextoContem.Termos, limiteLista, "", nil)
	}
	return nil
}

// validarLista confere tamanho 1..max, itens não vazios e, com catálogo, a existência.
func validarLista(campo string, itens []string, max int, codigoDesconhecido string, catalogo map[string]bool) []Problema {
	var ps []Problema
	if n := len(itens); n < 1 || n > max {
		ps = append(ps, Problema{campo, "LISTA_TAMANHO", fmt.Sprintf("a lista precisa ter de 1 a %d itens", max)})
	}
	for _, it := range itens {
		switch {
		case strings.TrimSpace(it) == "":
			ps = append(ps, Problema{campo, "ITEM_VAZIO", "a lista tem item vazio"})
		case codigoDesconhecido != "" && !catalogo[it]:
			ps = append(ps, Problema{campo, codigoDesconhecido, "fora do catálogo: " + it})
		}
	}
	return ps
}

func validarFonte(campo string, f Fonte, ev ganchos.NomeDoEvento) []Problema {
	switch {
	case f.Tipo == FonteAgora && f.Nome == "":
		return nil
	case f.Tipo == FonteParametro && f.Nome != "" && slices.Contains(eventosComParametro, ev):
		return nil
	case f.Tipo == FonteParametro && f.Nome != "":
		return []Problema{{campo, "FONTE_PARAMETRO_NAO_ACEITA", "parâmetro só existe antes e depois da ferramenta"}}
	}
	return []Problema{{campo, "FONTE_INVALIDA", "fonte é agora (sem nome) ou parametro (com nome)"}}
}

func validarHorario(campo string, h *CondHorario) []Problema {
	var ps []Problema
	for _, hhmm := range []struct{ nome, valor string }{{"inicio", h.Inicio}, {"fim", h.Fim}} {
		if _, err := time.Parse("15:04", hhmm.valor); err != nil || len(hhmm.valor) != 5 {
			ps = append(ps, Problema{campo + "." + hhmm.nome, "HORARIO_INVALIDO", "use HH:MM, de 00:00 a 23:59"})
		}
	}
	return ps
}

func validarDias(campo string, dias []string) []Problema {
	ps := validarLista(campo, dias, len(diasDaSemana), "", nil)
	for _, d := range dias {
		if !slices.Contains(diasDaSemana, d) {
			ps = append(ps, Problema{campo, "DIA_INVALIDO", "dia da semana inválido: " + d})
		}
	}
	return ps
}

func validarDatas(campo string, datas []string) []Problema {
	var ps []Problema
	if len(datas) > limiteFeriadosExtras {
		ps = append(ps, Problema{campo, "LISTA_TAMANHO", fmt.Sprintf("no máximo %d feriados extras", limiteFeriadosExtras)})
	}
	for _, d := range datas {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			ps = append(ps, Problema{campo, "DATA_INVALIDA", "use AAAA-MM-DD: " + d})
		}
	}
	return ps
}

func validarDecisao(d DecisaoDaRegra, aceitas []ganchos.TipoDecisao, eventoOK bool) []Problema {
	if d.Tipo == ganchos.Ajustar || d.Ajuste != nil {
		return []Problema{{"decisao", "AJUSTE_NAO_PERMITIDO", "regra só bloqueia ou pede confirmação; ajustar é de gancho fixo"}}
	}
	var ps []Problema
	if eventoOK && !slices.Contains(aceitas, d.Tipo) {
		ps = append(ps, Problema{"decisao.tipo", "DECISAO_NAO_ACEITA", "o evento não aceita a decisão " + string(d.Tipo)})
	}
	switch d.Tipo {
	case ganchos.Confirmar:
		return append(ps, validarTexto("decisao.pergunta", d.Pergunta)...)
	case ganchos.Bloquear:
		return append(ps, validarTexto("decisao.mensagem", d.Mensagem)...)
	}
	return append(ps, Problema{"decisao.tipo", "DECISAO_DESCONHECIDA", "tipo de decisão desconhecido: " + string(d.Tipo)})
}

// validarTexto exige texto fixo: escrito por quem grava a regra, sem PII nem template.
func validarTexto(campo, texto string) []Problema {
	var ps []Problema
	if n := utf8.RuneCountInString(strings.TrimSpace(texto)); n < 1 || n > limiteTexto {
		ps = append(ps, Problema{campo, "TEXTO_TAMANHO", fmt.Sprintf("o texto precisa ter de 1 a %d caracteres", limiteTexto)})
	}
	if pii.TemMarcador(texto) || strings.Contains(texto, "{{") {
		ps = append(ps, Problema{campo, "TEXTO_NAO_FIXO", "o texto é fixo: sem marcador de dado pessoal nem {{ }}"})
	}
	return ps
}

// alcanca diz se a regra vale para o condomínio c: alvo explícito, ou a administradora
// dele, ou todos.
func alcanca(r Regra, c string, cat Catalogo) bool {
	if len(r.Alvo.Condominios) > 0 {
		return slices.Contains(r.Alvo.Condominios, c)
	}
	return r.Alvo.Administradora == "" || cat.AdministradoraDe[c] == r.Alvo.Administradora
}

// condominiosDaNova lista os condomínios que a nova regra alcança, pelo catálogo.
func condominiosDaNova(nova Regra, cat Catalogo) []string {
	if len(nova.Alvo.Condominios) > 0 {
		return nova.Alvo.Condominios
	}
	var out []string
	for c := range cat.AdministradoraDe {
		if alcanca(nova, c, cat) {
			out = append(out, c)
		}
	}
	return out
}

// ValidarLimites confere 20 ativas efetivas por evento e condomínio (as dele, as da
// administradora e as globais, contando a nova) e 10 mil no total. Regra com o mesmo ID
// da nova (edição) e regra inativa não contam.
// ponytail: O(regras do evento x condomínios da nova), só no caminho de gravação.
func ValidarLimites(ativas []Regra, nova Regra, cat Catalogo) []Problema {
	if !nova.Ativa {
		return nil
	}
	outras := slices.DeleteFunc(slices.Clone(ativas), func(r Regra) bool {
		return !r.Ativa || (nova.ID != "" && r.ID == nova.ID)
	})
	var ps []Problema
	if len(outras)+1 > limiteAtivasTotal {
		ps = append(ps, Problema{"ativa", "LIMITE_TOTAL", fmt.Sprintf("já existem %d regras ativas", limiteAtivasTotal)})
	}
	mesmoEvento := slices.DeleteFunc(outras, func(r Regra) bool { return r.Evento != nova.Evento })
	if c, ok := primeiroCondominioCheio(mesmoEvento, nova, cat); ok {
		ps = append(ps, Problema{"ativa", "LIMITE_POR_EVENTO", fmt.Sprintf("o condomínio %s já tem %d regras ativas neste evento", c, limiteAtivasPorEvento)})
	}
	return ps
}

func primeiroCondominioCheio(mesmoEvento []Regra, nova Regra, cat Catalogo) (string, bool) {
	for _, c := range condominiosDaNova(nova, cat) {
		n := 1
		for _, r := range mesmoEvento {
			if alcanca(r, c, cat) {
				n++
			}
		}
		if n > limiteAtivasPorEvento {
			return c, true
		}
	}
	return "", false
}
