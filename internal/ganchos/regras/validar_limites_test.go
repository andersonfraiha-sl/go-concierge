package regras

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"go-concierge/internal/ganchos"
)

func regraBase() Regra {
	return Regra{
		Nome: "Regra de borda", Evento: ganchos.EventoAntesDaFerramenta, Ativa: true,
		Condicoes: []Condicao{{Tipo: CondicaoFerramenta, Ferramenta: &CondFerramenta{Nomes: []string{"areas.reservar"}}}},
		Decisao:   DecisaoDaRegra{Tipo: ganchos.Bloquear, Mensagem: "Não."},
	}
}

func TestJuntaTodosOsProblemas(t *testing.T) {
	r := regraBase()
	r.Nome = "ab"
	r.Evento = "quando_der"
	r.Fuso = "Local"
	r.Alvo = Alvo{Condominios: []string{"cond-fantasma"}}
	r.Condicoes = nil
	r.Decisao.Mensagem = "oi {{nome}}"
	got := codigos(ValidarRegra(r, catalogoDeTeste()))
	quer := []string{"NOME_TAMANHO", "EVENTO_INVALIDO", "ALVO_SEM_ADMINISTRADORA", "CONDOMINIO_DESCONHECIDO", "FUSO_INVALIDO", "CONDICOES_QUANTIDADE", "TEXTO_NAO_FIXO"}
	for _, c := range quer {
		if !slices.Contains(got, c) {
			t.Errorf("faltou %s em %v", c, got)
		}
	}
}

func TestJuntaListaGrandeComItemDesconhecido(t *testing.T) {
	r := regraBase()
	nomes := make([]string, 21)
	for i := range nomes {
		nomes[i] = fmt.Sprintf("x%d", i)
	}
	r.Condicoes[0].Ferramenta.Nomes = nomes
	got := codigos(ValidarRegra(r, catalogoDeTeste()))
	if !slices.Contains(got, "LISTA_TAMANHO") || !slices.Contains(got, "FERRAMENTA_DESCONHECIDA") {
		t.Fatalf("códigos = %v", got)
	}
}

func TestBordas(t *testing.T) {
	cat := catalogoDeTeste()
	nomesDoCatalogo := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("f%d", i)
			cat.Ferramentas[out[i]] = true
		}
		return out
	}
	naoUtil := func(n int) Condicao {
		return Condicao{Tipo: CondicaoDiaNaoUtil, DiaNaoUtil: &CondNaoUtil{
			Fonte: Fonte{Tipo: FonteAgora}, FeriadosExtras: slices.Repeat([]string{"2026-01-25"}, n)}}
	}
	cadastrado := Condicao{Tipo: CondicaoContatoCadastrado, ContatoCadastrado: &CondCadastrado{Valor: new(bool)}}
	semValor := Condicao{Tipo: CondicaoContatoCadastrado, ContatoCadastrado: &CondCadastrado{}}
	malformada := Condicao{Tipo: CondicaoCaso, Ferramenta: &CondFerramenta{}}
	casos := []struct {
		nome   string
		muda   func(*Regra)
		codigo string
	}{
		{"nome de 80", func(r *Regra) { r.Nome = strings.Repeat("a", 80) }, ""},
		{"nome de 81", func(r *Regra) { r.Nome = strings.Repeat("a", 81) }, "NOME_TAMANHO"},
		{"nome de 3", func(r *Regra) { r.Nome = "abc" }, ""},
		{"mensagem de 300", func(r *Regra) { r.Decisao.Mensagem = strings.Repeat("a", 300) }, ""},
		{"mensagem de 301", func(r *Regra) { r.Decisao.Mensagem = strings.Repeat("a", 301) }, "TEXTO_TAMANHO"},
		{"5 condições", func(r *Regra) { r.Condicoes = slices.Repeat([]Condicao{cadastrado}, 5) }, ""},
		{"6 condições", func(r *Regra) { r.Condicoes = slices.Repeat([]Condicao{cadastrado}, 6) }, "CONDICOES_QUANTIDADE"},
		{"20 nomes", func(r *Regra) { r.Condicoes[0].Ferramenta.Nomes = nomesDoCatalogo(20) }, ""},
		{"21 nomes", func(r *Regra) { r.Condicoes[0].Ferramenta.Nomes = nomesDoCatalogo(21) }, "LISTA_TAMANHO"},
		{"30 feriados extras", func(r *Regra) { r.Condicoes = []Condicao{naoUtil(30)} }, ""},
		{"31 feriados extras", func(r *Regra) { r.Condicoes = []Condicao{naoUtil(31)} }, "LISTA_TAMANHO"},
		{"item vazio", func(r *Regra) { r.Condicoes[0].Ferramenta.Nomes = []string{" "} }, "ITEM_VAZIO"},
		{"contato sem valor", func(r *Regra) { r.Condicoes = []Condicao{semValor} }, "VALOR_AUSENTE"},
		{"condição malformada", func(r *Regra) { r.Condicoes = []Condicao{malformada} }, "CONDICAO_MALFORMADA"},
		{"fuso UTC", func(r *Regra) { r.Fuso = "UTC" }, "FUSO_INVALIDO"},
		{"fuso Local", func(r *Regra) { r.Fuso = "Local" }, "FUSO_INVALIDO"},
		{"fuso IANA", func(r *Regra) { r.Fuso = "America/Manaus" }, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := regraBase()
			c.muda(&r)
			got := codigos(ValidarRegra(r, cat))
			quer := []string{c.codigo}
			if c.codigo == "" {
				quer = []string{}
			}
			if !slices.Equal(got, quer) && !(len(got) == 0 && len(quer) == 0) {
				t.Fatalf("códigos = %v, quer %v", got, quer)
			}
		})
	}
}

func TestDecodificarAplicaFusoPadrao(t *testing.T) {
	raw := []byte(`{"nome":"Regra","evento":"antes_de_responder","alvo":{},"condicoes":[{"tipo":"contato_cadastrado","valor":true}],"decisao":{"tipo":"bloquear","mensagem":"Não."},"ativa":true}`)
	r, err := Decodificar(raw)
	if err != nil || r.Fuso != FusoPadrao {
		t.Fatalf("fuso = %q, err = %v", r.Fuso, err)
	}
}

func ativas(n int, evento ganchos.NomeDoEvento, alvo Alvo) []Regra {
	out := make([]Regra, n)
	for i := range out {
		out[i] = Regra{ID: fmt.Sprintf("r%d-%s-%v", i, alvo.Administradora, alvo.Condominios), Evento: evento, Alvo: alvo, Ativa: true}
	}
	return out
}

func inativas(rs []Regra) []Regra {
	for i := range rs {
		rs[i].Ativa = false
	}
	return rs
}

func TestLimites(t *testing.T) {
	cat := catalogoDeTeste()
	cat.AdministradoraDe["cond-rosas"] = "adm-aurora"
	flores := Alvo{Administradora: "adm-aurora", Condominios: []string{"cond-flores"}}
	rosas := Alvo{Administradora: "adm-aurora", Condominios: []string{"cond-rosas"}}
	aurora := Alvo{Administradora: "adm-aurora"}
	ev := ganchos.EventoAntesDaFerramenta
	nova := Regra{ID: "nova", Evento: ev, Alvo: flores, Ativa: true}
	novaAurora := Regra{ID: "a", Evento: ev, Alvo: aurora, Ativa: true}
	novaGlobal := Regra{ID: "g", Evento: ev, Ativa: true}
	total := ativas(limiteAtivasTotal, ganchos.EventoMensagemRecebida, Alvo{Administradora: "adm-x"})
	for i := range total {
		total[i].ID = fmt.Sprintf("t%d", i)
	}
	editada := ativas(limiteAtivasPorEvento, ev, flores)
	editada[0].ID = "nova"

	casos := []struct {
		nome   string
		ativas []Regra
		nova   Regra
		codigo string
	}{
		{"19 do condomínio aceita a vigésima", ativas(19, ev, flores), nova, ""},
		{"20 do condomínio recusa a vigésima primeira", ativas(20, ev, flores), nova, "LIMITE_POR_EVENTO"},
		{"misto 10 + 5 + 4 aceita a vigésima", slices.Concat(ativas(10, ev, flores), ativas(5, ev, aurora), ativas(4, ev, Alvo{})), nova, ""},
		{"misto 10 + 5 + 5 recusa a vigésima primeira", slices.Concat(ativas(10, ev, flores), ativas(5, ev, aurora), ativas(5, ev, Alvo{})), nova, "LIMITE_POR_EVENTO"},
		{"20 da administradora recusam regra de condomínio dela", ativas(20, ev, aurora), nova, "LIMITE_POR_EVENTO"},
		{"20 globais recusam regra de condomínio", ativas(20, ev, Alvo{}), nova, "LIMITE_POR_EVENTO"},
		{"20 de outra administradora não contam", ativas(20, ev, Alvo{Administradora: "adm-outra"}), nova, ""},
		{"20 de outro condomínio não contam", ativas(20, ev, rosas), nova, ""},
		{"nova global recusa se um condomínio alcançado está cheio", ativas(20, ev, rosas), novaGlobal, "LIMITE_POR_EVENTO"},
		{"nova da administradora aceita com condomínios folgados", ativas(19, ev, rosas), novaAurora, ""},
		{"outro evento não conta", ativas(20, ganchos.EventoAntesDeResponder, flores), nova, ""},
		{"inativas não contam", inativas(ativas(20, ev, flores)), nova, ""},
		{"editar a própria regra não conta em dobro", editada, nova, ""},
		{"nova inativa nunca estoura", ativas(20, ev, flores), Regra{Evento: ev, Alvo: flores}, ""},
		{"10 mil ativas recusa a próxima", total, nova, "LIMITE_TOTAL"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := codigos(ValidarLimites(c.ativas, c.nova, cat))
			if c.codigo == "" && len(got) > 0 || c.codigo != "" && !slices.Contains(got, c.codigo) {
				t.Fatalf("códigos = %v, quer %q", got, c.codigo)
			}
		})
	}
}
