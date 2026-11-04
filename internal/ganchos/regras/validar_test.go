package regras

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go-concierge/internal/ganchos"
)

func catalogoDeTeste() Catalogo {
	return Catalogo{
		Ferramentas: map[string]bool{
			"areas.reservar": true, "obras.comunicar": true,
			"gruvi.bff.building-operation.get-package-delivery-code-controller-handle": true,
		},
		Casos:            map[string]bool{"reserva_area_comum": true, "comunicar_obra": true},
		AdministradoraDe: map[string]string{"cond-flores": "adm-aurora", "cond-outra": "adm-outra"},
		NomesDosFixos:    []string{"escopo_da_conversa"},
	}
}

func arquivos(t *testing.T, pasta string) []string {
	t.Helper()
	achados, err := filepath.Glob(filepath.Join("..", "testdata", "regras", pasta, "*.json"))
	if err != nil || len(achados) == 0 {
		t.Fatalf("sem arquivos em %s: %v", pasta, err)
	}
	return achados
}

func codigos(ps []Problema) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Codigo
	}
	return out
}

func TestExemplosValidos(t *testing.T) {
	for _, arq := range arquivos(t, "validas") {
		t.Run(filepath.Base(arq), func(t *testing.T) {
			raw, err := os.ReadFile(arq)
			if err != nil {
				t.Fatal(err)
			}
			r, err := Decodificar(raw)
			if err != nil {
				t.Fatalf("Decodificar: %v", err)
			}
			if ps := ValidarRegra(r, catalogoDeTeste()); len(ps) > 0 {
				t.Fatalf("problemas: %+v", ps)
			}
		})
	}
}

// Cada inválido declara o código esperado antes do "__" no nome do arquivo.
func TestExemplosInvalidos(t *testing.T) {
	for _, arq := range arquivos(t, "invalidas") {
		nome := filepath.Base(arq)
		esperado, _, _ := strings.Cut(nome, "__")
		t.Run(nome, func(t *testing.T) {
			raw, err := os.ReadFile(arq)
			if err != nil {
				t.Fatal(err)
			}
			if got := codigosDe(raw); !slices.Contains(got, esperado) {
				t.Fatalf("códigos %v não contêm %s", got, esperado)
			}
		})
	}
}

func codigosDe(raw []byte) []string {
	r, err := Decodificar(raw)
	var esq *ErroDeEsquema
	if errors.As(err, &esq) {
		return []string{esq.Codigo}
	}
	if err != nil {
		return []string{err.Error()}
	}
	return codigos(ValidarRegra(r, catalogoDeTeste()))
}

func TestRegraComAjusteNuncaPassa(t *testing.T) {
	r := Regra{
		Nome: "Aviso de teste", Evento: ganchos.EventoAntesDeResponder, Ativa: true,
		Condicoes: []Condicao{{Tipo: CondicaoContatoCadastrado, ContatoCadastrado: &CondCadastrado{}}},
		Decisao:   DecisaoDaRegra{Tipo: ganchos.Ajustar, Ajuste: &Ajuste{Tipo: "acrescentar_aviso", Texto: "oi"}},
	}
	if got := codigos(ValidarRegra(r, catalogoDeTeste())); !slices.Equal(got, []string{"AJUSTE_NAO_PERMITIDO"}) {
		t.Fatalf("códigos = %v", got)
	}
}

func TestMatrizDoEvento(t *testing.T) {
	cadastrado := Condicao{Tipo: CondicaoContatoCadastrado, ContatoCadastrado: &CondCadastrado{}}
	ferramenta := Condicao{Tipo: CondicaoFerramenta, Ferramenta: &CondFerramenta{Nomes: []string{"areas.reservar"}}}
	texto := Condicao{Tipo: CondicaoTextoContem, TextoContem: &CondTermos{Termos: []string{"oi"}}}
	bloquear := DecisaoDaRegra{Tipo: ganchos.Bloquear, Mensagem: "Não."}
	confirmar := DecisaoDaRegra{Tipo: ganchos.Confirmar, Pergunta: "Certo?"}

	casos := []struct {
		nome   string
		evento ganchos.NomeDoEvento
		cond   Condicao
		dec    DecisaoDaRegra
		codigo string // vazio = aceita
	}{
		{"bloquear no transbordo recusado", ganchos.EventoTransbordoOuEncerramento, cadastrado, bloquear, "DECISAO_NAO_ACEITA"},
		{"ferramenta antes de responder recusada", ganchos.EventoAntesDeResponder, ferramenta, bloquear, "CONDICAO_NAO_ACEITA"},
		{"confirmar depois da ferramenta recusado", ganchos.EventoDepoisDaFerramenta, ferramenta, confirmar, "DECISAO_NAO_ACEITA"},
		{"confirmar na mensagem recebida recusado", ganchos.EventoMensagemRecebida, cadastrado, confirmar, "DECISAO_NAO_ACEITA"},
		{"texto antes da ferramenta recusado", ganchos.EventoAntesDaFerramenta, texto, bloquear, "CONDICAO_NAO_ACEITA"},
		{"ferramenta e confirmar antes da ferramenta aceitos", ganchos.EventoAntesDaFerramenta, ferramenta, confirmar, ""},
		{"texto na mensagem recebida aceito", ganchos.EventoMensagemRecebida, texto, bloquear, ""},
		{"bloquear antes de responder aceito", ganchos.EventoAntesDeResponder, cadastrado, bloquear, ""},
		{"confirmar no copiloto aceito", ganchos.EventoCopilotoAntesDeAceitar, Condicao{
			Tipo: CondicaoDiaNaoUtil, DiaNaoUtil: &CondNaoUtil{Fonte: Fonte{Tipo: FonteAgora}}}, confirmar, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := Regra{Nome: "Regra de matriz", Evento: c.evento, Condicoes: []Condicao{c.cond}, Decisao: c.dec}
			got := codigos(ValidarRegra(r, catalogoDeTeste()))
			if c.codigo == "" && len(got) > 0 || c.codigo != "" && !slices.Contains(got, c.codigo) {
				t.Fatalf("códigos = %v, quer %q", got, c.codigo)
			}
		})
	}
}

func ativas(n int, evento ganchos.NomeDoEvento, alvo Alvo) []Regra {
	out := make([]Regra, n)
	for i := range out {
		out[i] = Regra{ID: fmt.Sprintf("r%d-%s", i, alvo.Administradora), Evento: evento, Alvo: alvo, Ativa: true}
	}
	return out
}

func TestLimites(t *testing.T) {
	flores := Alvo{Administradora: "adm-aurora", Condominios: []string{"cond-flores"}}
	nova := Regra{ID: "nova", Evento: ganchos.EventoAntesDaFerramenta, Alvo: flores, Ativa: true}
	total := ativas(limiteAtivasTotal, ganchos.EventoMensagemRecebida, Alvo{Administradora: "adm-x"})
	for i := range total {
		total[i].ID = fmt.Sprintf("t%d", i)
	}
	editada := ativas(limiteAtivasPorEvento, ganchos.EventoAntesDaFerramenta, flores)
	editada[0].ID = "nova"

	casos := []struct {
		nome   string
		ativas []Regra
		nova   Regra
		codigo string
	}{
		{"19 ativas aceita a vigésima", ativas(19, nova.Evento, flores), nova, ""},
		{"20 ativas recusa a vigésima primeira", ativas(20, nova.Evento, flores), nova, "LIMITE_POR_EVENTO"},
		{"outro evento não conta", ativas(20, ganchos.EventoAntesDeResponder, flores), nova, ""},
		{"outro condomínio não conta", ativas(20, nova.Evento, Alvo{Administradora: "adm-aurora", Condominios: []string{"cond-x"}}), nova, ""},
		{"inativas não contam", func() []Regra {
			rs := ativas(20, nova.Evento, flores)
			for i := range rs {
				rs[i].Ativa = false
			}
			return rs
		}(), nova, ""},
		{"editar a própria regra não conta em dobro", editada, nova, ""},
		{"nova inativa nunca estoura", ativas(20, nova.Evento, flores), Regra{Evento: nova.Evento, Alvo: flores}, ""},
		{"10 mil ativas recusa a próxima", total, nova, "LIMITE_TOTAL"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := codigos(ValidarLimites(c.ativas, c.nova))
			if c.codigo == "" && len(got) > 0 || c.codigo != "" && !slices.Contains(got, c.codigo) {
				t.Fatalf("códigos = %v, quer %q", got, c.codigo)
			}
		})
	}
}
