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

// ValidarRegra devolve todos os problemas da regra; vazio significa válida.
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
	if eventoOK {
		ps = append(ps, validarDecisao(r.Decisao, ev.decisoes)...)
	}
	return ps
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
	if len(a.Condominios) > 0 && a.Administradora == "" {
		return []Problema{{"alvo.administradora", "ALVO_SEM_ADMINISTRADORA", "condomínios exigem a administradora do alvo"}}
	}
	var ps []Problema
	for _, c := range a.Condominios {
		adm, existe := cat.AdministradoraDe[c]
		switch {
		case !existe:
			ps = append(ps, Problema{"alvo.condominios", "CONDOMINIO_DESCONHECIDO", "condomínio fora do catálogo: " + c})
		case adm != a.Administradora:
			ps = append(ps, Problema{"alvo.condominios", "CONDOMINIO_DE_OUTRA_ADMINISTRADORA", "o condomínio " + c + " não é da administradora do alvo"})
		}
	}
	return ps
}

func validarFuso(fuso string) []Problema {
	if fuso == "" {
		return nil
	}
	if _, err := time.LoadLocation(fuso); err != nil {
		return []Problema{{"fuso", "FUSO_INVALIDO", "fuso desconhecido: " + fuso}}
	}
	return nil
}

func validarCondicoes(r Regra, aceitas []TipoCondicao, eventoOK bool, cat Catalogo) []Problema {
	if n := len(r.Condicoes); n < 1 || n > limiteCondicoes {
		return []Problema{{"condicoes", "CONDICOES_QUANTIDADE", fmt.Sprintf("a regra precisa ter de 1 a %d condições", limiteCondicoes)}}
	}
	var ps []Problema
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
	case CondicaoPapelDoMorador:
		return validarLista(campo+".papeis", c.PapelDoMorador.Papeis, limiteLista, "", nil)
	case CondicaoTextoContem:
		return validarLista(campo+".termos", c.TextoContem.Termos, limiteLista, "", nil)
	}
	return nil
}

// validarLista confere tamanho 1..max, itens não vazios e, com catálogo, a existência.
func validarLista(campo string, itens []string, max int, codigoDesconhecido string, catalogo map[string]bool) []Problema {
	if n := len(itens); n < 1 || n > max {
		return []Problema{{campo, "LISTA_TAMANHO", fmt.Sprintf("a lista precisa ter de 1 a %d itens", max)}}
	}
	var ps []Problema
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
	if len(datas) > limiteFeriadosExtras {
		return []Problema{{campo, "LISTA_TAMANHO", fmt.Sprintf("no máximo %d feriados extras", limiteFeriadosExtras)}}
	}
	var ps []Problema
	for _, d := range datas {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			ps = append(ps, Problema{campo, "DATA_INVALIDA", "use AAAA-MM-DD: " + d})
		}
	}
	return ps
}

func validarDecisao(d DecisaoDaRegra, aceitas []ganchos.TipoDecisao) []Problema {
	if d.Tipo == ganchos.Ajustar || d.Ajuste != nil {
		return []Problema{{"decisao", "AJUSTE_NAO_PERMITIDO", "regra só bloqueia ou pede confirmação; ajustar é de gancho fixo"}}
	}
	if !slices.Contains(aceitas, d.Tipo) {
		return []Problema{{"decisao.tipo", "DECISAO_NAO_ACEITA", "o evento não aceita a decisão " + string(d.Tipo)}}
	}
	if d.Tipo == ganchos.Confirmar {
		return validarTexto("decisao.pergunta", d.Pergunta)
	}
	return validarTexto("decisao.mensagem", d.Mensagem)
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

type baldeDeLimite struct {
	evento         ganchos.NomeDoEvento
	administradora string // "" com condomínio vazio = regra para todas
	condominio     string
}

// baldes diz em quais índices (evento × condomínio) a regra entra, como no cache.
func baldes(r Regra) []baldeDeLimite {
	if len(r.Alvo.Condominios) == 0 {
		return []baldeDeLimite{{r.Evento, r.Alvo.Administradora, ""}}
	}
	out := make([]baldeDeLimite, len(r.Alvo.Condominios))
	for i, c := range r.Alvo.Condominios {
		out[i] = baldeDeLimite{r.Evento, "", c}
	}
	return out
}

// ValidarLimites confere 20 ativas por evento e condomínio e 10 mil no total, contando
// a nova. Regra com o mesmo ID de uma ativa é a mesma regra (edição) e não conta em dobro.
func ValidarLimites(ativas []Regra, nova Regra) []Problema {
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
	porBalde := map[baldeDeLimite]int{}
	for _, r := range outras {
		for _, b := range baldes(r) {
			porBalde[b]++
		}
	}
	for _, b := range baldes(nova) {
		if porBalde[b]+1 > limiteAtivasPorEvento {
			ps = append(ps, Problema{"ativa", "LIMITE_POR_EVENTO", fmt.Sprintf("já existem %d regras ativas para este evento e alvo", limiteAtivasPorEvento)})
			break
		}
	}
	return ps
}
