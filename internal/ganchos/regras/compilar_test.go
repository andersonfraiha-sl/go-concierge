package regras

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-concierge/internal/ganchos"
)

const (
	ferramentaCodigoDeRetirada = "gruvi.bff.building-operation.get-package-delivery-code-controller-handle"
	pastaDasValidas            = "../testdata/regras/validas"
)

func gravadaDoArquivo(t *testing.T, id, arquivo string) RegraGravada {
	t.Helper()
	definicao, err := os.ReadFile(filepath.Join(pastaDasValidas, arquivo))
	require.NoError(t, err)
	return RegraGravada{ID: id, Versao: 1, Ativa: true, Definicao: definicao}
}

func gravadaInline(id, evento, condicoes, decisao string) RegraGravada {
	definicao := fmt.Sprintf(`{"nome":"regra de teste %s","evento":%q,"alvo":{},"condicoes":[%s],"decisao":%s,"ativa":true}`,
		id, evento, condicoes, decisao)
	return RegraGravada{ID: id, Versao: 1, Ativa: true, Definicao: json.RawMessage(definicao)}
}

func compilarSemErro(t *testing.T, rs ...RegraGravada) *ganchos.Indice {
	t.Helper()
	indice, erros := Compilar(rs)
	require.Empty(t, erros)
	return indice
}

func antes(agora string, ev ganchos.AntesDaFerramenta) ganchos.AntesDaFerramenta {
	ev.Agora = momentoUTC(agora)
	return ev
}

// momentoUTC lê um instante em UTC: o servidor roda em UTC, a regra decide no fuso dela.
func momentoUTC(rfc3339 string) time.Time {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func decidirAntes(indice *ganchos.Indice, ev ganchos.AntesDaFerramenta) ganchos.Decisao {
	gs := indice.AntesDaFerramenta.Para(ev.CondominioID, ev.AdministradoraID)
	return ganchos.Avaliar(context.Background(), gs, ev).Decisao
}

func TestCompilarReservaSoAte22h(t *testing.T) {
	indice := compilarSemErro(t, gravadaDoArquivo(t, "r1", "reserva-so-ate-22h.json"))
	reserva := ganchos.AntesDaFerramenta{
		Ferramenta: "areas.reservar", AdministradoraID: "adm-aurora", CondominioID: "cond-flores",
	}
	casos := []struct {
		nome    string
		agora   string
		ev      ganchos.AntesDaFerramenta
		esperar ganchos.TipoDecisao
	}{
		{"bloqueia as 22h30 de Sao Paulo", "2026-11-05T01:30:00Z", reserva, ganchos.Bloquear},
		{"deixa as 21h59 de Sao Paulo", "2026-11-05T00:59:00Z", reserva, ganchos.Deixar},
		{"deixa de manha", "2026-11-04T15:00:00Z", reserva, ganchos.Deixar},
		{"bloqueia de madrugada", "2026-11-04T07:00:00Z", reserva, ganchos.Bloquear},
		{"deixa outra ferramenta", "2026-11-05T01:30:00Z", ganchos.AntesDaFerramenta{
			Ferramenta: "areas.listar", AdministradoraID: "adm-aurora", CondominioID: "cond-flores"}, ganchos.Deixar},
		{"deixa outro condominio", "2026-11-05T01:30:00Z", ganchos.AntesDaFerramenta{
			Ferramenta: "areas.reservar", AdministradoraID: "adm-aurora", CondominioID: "cond-rios"}, ganchos.Deixar},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d := decidirAntes(indice, antes(c.agora, c.ev))
			require.Equal(t, c.esperar, d.Tipo)
			if c.esperar == ganchos.Bloquear {
				require.Equal(t, ganchos.Origem{RegraID: "r1", RegraVersao: 1, Gancho: "regra"}, d.Origem)
				require.Contains(t, d.Motivo, "6h às 22h")
			}
		})
	}
}

// Review Focus 4: horário que cruza a meia-noite, com o servidor em UTC.
func TestCompilarHorarioQueCruzaMeiaNoiteUsaOFusoDaRegra(t *testing.T) {
	horario := `{"tipo":"fora_do_horario","inicio":"18:00","fim":"02:00","fonte":{"tipo":"agora"}}`
	regra := gravadaInline("r1", "mensagem_recebida", horario, `{"tipo":"bloquear","mensagem":"Fora do plantão."}`)
	indice := compilarSemErro(t, regra)
	casos := []struct {
		nome    string
		agoraSP string
		esperar ganchos.TipoDecisao
	}{
		{"deixa as 18h00, primeiro minuto do horario", "2026-11-04T18:00:00-03:00", ganchos.Deixar},
		{"deixa as 23h, dentro do horario", "2026-11-04T23:00:00-03:00", ganchos.Deixar},
		{"deixa a meia-noite", "2026-11-05T00:00:00-03:00", ganchos.Deixar},
		{"deixa as 01h59, ultimo minuto do horario", "2026-11-05T01:59:00-03:00", ganchos.Deixar},
		{"bloqueia as 02h00, fora do horario", "2026-11-05T02:00:00-03:00", ganchos.Bloquear},
		{"bloqueia as 10h00, fora do horario", "2026-11-05T10:00:00-03:00", ganchos.Bloquear},
		{"bloqueia as 17h59, fora do horario", "2026-11-04T17:59:00-03:00", ganchos.Bloquear},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			ev := ganchos.MensagemRecebida{Agora: momentoUTC(c.agoraSP)}
			require.Equal(t, time.UTC, ev.Agora.Location(), "o teste precisa rodar com o relógio em UTC")
			r := ganchos.Avaliar(context.Background(), indice.MensagemRecebida.Para("", ""), ev)
			require.Equal(t, c.esperar, r.Decisao.Tipo)
		})
	}
}

// Review Focus 3: parâmetro ausente na gravação conta como "não vale".
func TestCompilarObraSoComecaEmDiaUtil(t *testing.T) {
	indice := compilarSemErro(t, gravadaDoArquivo(t, "r1", "obra-so-comeca-em-dia-util.json"))
	casos := []struct {
		nome    string
		params  map[string]any
		previa  bool
		esperar ganchos.TipoDecisao
	}{
		{"bloqueia inicio num sabado", map[string]any{"inicio": "2026-11-07"}, false, ganchos.Bloquear},
		{"bloqueia inicio num feriado", map[string]any{"inicio": "2026-11-20T09:00:00-03:00"}, false, ganchos.Bloquear},
		{"deixa inicio numa segunda", map[string]any{"inicio": "2026-11-09"}, false, ganchos.Deixar},
		{"deixa sem inicio na previa", map[string]any{}, true, ganchos.Deixar},
		{"deixa sem inicio na gravacao", map[string]any{}, false, ganchos.Deixar},
		{"deixa params nulos na gravacao", nil, false, ganchos.Deixar},
		{"deixa inicio que nao e texto", map[string]any{"inicio": 20261107}, false, ganchos.Deixar},
		{"deixa inicio que nao e data", map[string]any{"inicio": "sabado"}, false, ganchos.Deixar},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			ev := ganchos.AntesDaFerramenta{
				Ferramenta: "obras.comunicar", AdministradoraID: "adm-aurora", Params: c.params, Previa: c.previa,
			}
			require.Equal(t, c.esperar, decidirAntes(indice, ev).Tipo)
		})
	}
}

func TestCompilarFeriadoExtraDaRegraBloqueia(t *testing.T) {
	condicao := `{"tipo":"dia_nao_util","fonte":{"tipo":"parametro","nome":"inicio"},"feriados_extras":["2026-11-10"]}`
	indice := compilarSemErro(t, gravadaInline("r1", "antes_da_ferramenta", condicao, `{"tipo":"bloquear","mensagem":"Escolha outro dia."}`))
	ev := ganchos.AntesDaFerramenta{Params: map[string]any{"inicio": "2026-11-10"}}
	require.Equal(t, ganchos.Bloquear, decidirAntes(indice, ev).Tipo)
}

func TestCompilarCodigoDeRetiradaSoParaCadastrado(t *testing.T) {
	indice := compilarSemErro(t, gravadaDoArquivo(t, "r1", "codigo-de-retirada-so-cadastrado.json"))
	casos := []struct {
		nome       string
		ferramenta string
		cadastrado bool
		esperar    ganchos.TipoDecisao
	}{
		{"bloqueia contato nao cadastrado", ferramentaCodigoDeRetirada, false, ganchos.Bloquear},
		{"deixa contato cadastrado", ferramentaCodigoDeRetirada, true, ganchos.Deixar},
		{"deixa outra ferramenta", "areas.listar", false, ganchos.Deixar},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			ev := ganchos.AntesDaFerramenta{Ferramenta: c.ferramenta, Identidade: ganchos.Identidade{Cadastrado: c.cadastrado}}
			require.Equal(t, c.esperar, decidirAntes(indice, ev).Tipo)
		})
	}
}

func TestCompilarTextoContemIgnoraCaixaEAcento(t *testing.T) {
	indice := compilarSemErro(t, gravadaInline("r1", "mensagem_recebida",
		`{"tipo":"texto_contem","termos":["reclamação"]}`, `{"tipo":"bloquear","mensagem":"Vou chamar alguém."}`))
	casos := map[string]ganchos.TipoDecisao{
		"quero fazer uma RECLAMACAO": ganchos.Bloquear,
		"Reclamação sobre o barulho": ganchos.Bloquear,
		"bom dia":                    ganchos.Deixar,
	}
	for texto, esperar := range casos {
		t.Run(texto, func(t *testing.T) {
			r := ganchos.Avaliar(context.Background(), indice.MensagemRecebida.Para("", ""), ganchos.MensagemRecebida{Texto: texto})
			require.Equal(t, esperar, r.Decisao.Tipo)
		})
	}
}

func TestCompilarFalhaDeRegraFechaSoEmEscrita(t *testing.T) {
	indice := compilarSemErro(t, gravadaDoArquivo(t, "r1", "codigo-de-retirada-so-cadastrado.json"))
	cancelado, cancelar := context.WithCancel(context.Background())
	cancelar() // regra não fixa com o contexto cancelado conta como falha, sob a política dela
	casos := []struct {
		nome    string
		escrita bool
		esperar ganchos.TipoDecisao
	}{
		{"fecha em ferramenta de escrita", true, ganchos.Bloquear},
		{"abre em ferramenta de leitura", false, ganchos.Deixar},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := ganchos.Avaliar(cancelado, indice.AntesDaFerramenta.Para("", ""), ganchos.AntesDaFerramenta{Escrita: c.escrita})
			require.Equal(t, c.esperar, r.Decisao.Tipo)
			require.True(t, r.Registros[0].Falhou)
		})
	}
}

func TestCompilarPoliticaDeFalhaPorEvento(t *testing.T) {
	bloqueio := `{"tipo":"bloquear","mensagem":"Não posso seguir."}`
	horario := `{"tipo":"fora_do_horario","inicio":"08:00","fim":"18:00","fonte":{"tipo":"agora"}}`
	indice := compilarSemErro(t,
		gravadaInline("a", "antes_da_ferramenta", horario, bloqueio),
		gravadaInline("b", "depois_da_ferramenta", horario, bloqueio),
		gravadaInline("c", "mensagem_recebida", horario, bloqueio),
		gravadaInline("d", "antes_de_responder", horario, bloqueio),
	)
	require.Equal(t, ganchos.FechadaSeGrava, indice.AntesDaFerramenta.Globais[0].Falha)
	require.Equal(t, ganchos.Aberta, indice.DepoisDaFerramenta.Globais[0].Falha)
	require.Equal(t, ganchos.Aberta, indice.MensagemRecebida.Globais[0].Falha)
	require.Equal(t, ganchos.Aberta, indice.AntesDeResponder.Globais[0].Falha)
}

func TestCompilarRegraQuePerdeAcaoSaiEmErroDeCargaEAsOutrasEntram(t *testing.T) {
	ajusta := gravadaInline("r2", "antes_de_responder", `{"tipo":"texto_contem","termos":["oi"]}`,
		`{"tipo":"ajustar","ajuste":{"tipo":"acrescentar_aviso","texto":"aviso"}}`)
	ruim := RegraGravada{ID: "r3", Versao: 1, Ativa: true, Definicao: json.RawMessage(`{"nome":`)}
	campoDesconhecido := RegraGravada{ID: "r4", Versao: 1, Ativa: true, Definicao: json.RawMessage(`{"nome":"x","extra":1}`)}
	boa := gravadaDoArquivo(t, "r1", "reserva-so-ate-22h.json")

	indice, erros := Compilar([]RegraGravada{ajusta, ruim, campoDesconhecido, boa})

	ids := make([]string, len(erros))
	for i, e := range erros {
		ids[i] = e.RegraID
		require.Error(t, e.Err)
	}
	require.ElementsMatch(t, []string{"r2", "r3", "r4"}, ids)
	require.Equal(t, map[ganchos.NomeDoEvento]int{ganchos.EventoAntesDaFerramenta: 1}, indice.Ativas)
	require.Len(t, indice.AntesDaFerramenta.PorCondominio["cond-flores"], 1)
	require.Contains(t, erros[0].Err.Error(), "AJUSTE_NAO_PERMITIDO")
}

func TestCompilarRecusaEventoSemIndice(t *testing.T) {
	_, erros := Compilar([]RegraGravada{gravadaInline("r1", "transbordo_ou_encerramento",
		`{"tipo":"fora_do_horario","inicio":"08:00","fim":"18:00","fonte":{"tipo":"agora"}}`,
		`{"tipo":"bloquear","mensagem":"Não pode."}`)})
	require.Len(t, erros, 1)
	require.Equal(t, "r1", erros[0].RegraID)
}

func TestCompilarEntraNoNivelDoAlvoOrdenadaPorID(t *testing.T) {
	reserva := gravadaDoArquivo(t, "r3", "reserva-so-ate-22h.json")              // condomínio
	obra := gravadaDoArquivo(t, "r2", "obra-so-comeca-em-dia-util.json")         // administradora
	codigo := gravadaDoArquivo(t, "r1", "codigo-de-retirada-so-cadastrado.json") // global
	outraGlobal := gravadaInline("r0", "antes_da_ferramenta",
		`{"tipo":"contato_cadastrado","valor":false}`, `{"tipo":"bloquear","mensagem":"Só para cadastrados."}`)

	indice := compilarSemErro(t, reserva, obra, codigo, outraGlobal)

	n := indice.AntesDaFerramenta
	require.Len(t, n.PorCondominio["cond-flores"], 1)
	require.Len(t, n.PorAdministradora["adm-aurora"], 1)
	require.Len(t, n.Globais, 2)
	require.Equal(t, 4, indice.Ativas[ganchos.EventoAntesDaFerramenta])
	// r0 antes de r1: a ordem do nível é por id, mesmo com a entrada embaralhada.
	ev := ganchos.AntesDaFerramenta{Ferramenta: ferramentaCodigoDeRetirada}
	r := ganchos.Avaliar(context.Background(), n.Para("", ""), ev)
	require.Equal(t, "r0", r.Decisao.Origem.RegraID)
}

func TestCompilarCondominiosDoAlvoGanhamUmaEntradaCada(t *testing.T) {
	regra := RegraGravada{ID: "r1", Versao: 1, Ativa: true, Definicao: json.RawMessage(`{
		"nome":"duas torres","evento":"antes_da_ferramenta",
		"alvo":{"administradora":"adm-aurora","condominios":["cond-flores","cond-rios"]},
		"condicoes":[{"tipo":"contato_cadastrado","valor":false}],
		"decisao":{"tipo":"bloquear","mensagem":"Só para cadastrados."},"ativa":true}`)}
	indice := compilarSemErro(t, regra)
	require.Len(t, indice.AntesDaFerramenta.PorCondominio["cond-flores"], 1)
	require.Len(t, indice.AntesDaFerramenta.PorCondominio["cond-rios"], 1)
	require.Equal(t, 1, indice.Ativas[ganchos.EventoAntesDaFerramenta])
}

func TestCompilarConfirmaReservaForaDoHorarioDaRegra(t *testing.T) {
	indice := compilarSemErro(t, gravadaDoArquivo(t, "r1", "confirmar-escrita-fora-do-horario.json"))
	casos := []struct {
		nome    string
		agora   string
		esperar ganchos.TipoDecisao
	}{
		{"confirma as 22h30 de Sao Paulo", "2026-11-05T01:30:00Z", ganchos.Confirmar},
		{"confirma de madrugada", "2026-11-04T07:00:00Z", ganchos.Confirmar},
		{"deixa ao meio-dia", "2026-11-04T15:00:00Z", ganchos.Deixar},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			ev := ganchos.AntesDaFerramenta{Ferramenta: "areas.reservar", Agora: momentoUTC(c.agora)}
			r := ganchos.Avaliar(context.Background(), indice.AntesDaFerramenta.Para("", ""), ev)
			require.Equal(t, c.esperar, r.Decisao.Tipo)
		})
	}
}

func TestCompilarConfirmarDeixaPassarQuandoOMoradorConfirma(t *testing.T) {
	indice := compilarSemErro(t, gravadaDoArquivo(t, "r1", "confirmar-escrita-fora-do-horario.json"))
	casos := []struct {
		nome        string
		confirmacao string
		esperar     ganchos.TipoDecisao
	}{
		{"pergunta sem mensagem", "", ganchos.Confirmar},
		{"pergunta quando a mensagem não é um sim", "quero o salão", ganchos.Confirmar},
		{"pergunta quando o sim vem negado", "sim, mas não agora", ganchos.Confirmar},
		{"deixa passar com o sim", "sim", ganchos.Deixar},
		{"deixa passar com outra forma de sim", "pode reservar", ganchos.Deixar},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			ev := ganchos.AntesDaFerramenta{Ferramenta: "areas.reservar", Agora: momentoUTC("2026-11-05T01:30:00Z"), Confirmacao: c.confirmacao}
			r := ganchos.Avaliar(context.Background(), indice.AntesDaFerramenta.Para("", ""), ev)
			require.Equal(t, c.esperar, r.Decisao.Tipo)
		})
	}
}

func TestCompilarBloquearIgnoraOSimDoMorador(t *testing.T) {
	indice := compilarSemErro(t, gravadaDoArquivo(t, "r1", "reserva-so-ate-22h.json"))
	ev := ganchos.AntesDaFerramenta{
		Ferramenta: "areas.reservar", Agora: momentoUTC("2026-11-05T01:30:00Z"), Confirmacao: "sim",
		AdministradoraID: "adm-aurora", CondominioID: "cond-flores",
	}
	require.Equal(t, ganchos.Bloquear, decidirAntes(indice, ev).Tipo)
}

func TestCompilarFalhaDeRegraGuardaAOrigemDela(t *testing.T) {
	g := gravadaDoArquivo(t, "r7", "codigo-de-retirada-so-cadastrado.json")
	g.Versao = 3
	rega := compilarSemErro(t, g).AntesDaFerramenta.Globais[0]
	rega.Avaliar = func(ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) { panic("quebrou") }

	r := ganchos.Avaliar(context.Background(), []ganchos.Gancho[ganchos.AntesDaFerramenta]{rega}, ganchos.AntesDaFerramenta{Escrita: true})

	require.Len(t, r.Registros, 1)
	require.True(t, r.Registros[0].Falhou)
	require.Equal(t, ganchos.CausaPanic, r.Registros[0].Causa)
	require.Equal(t, ganchos.Origem{Gancho: "regra", RegraID: "r7", RegraVersao: 3}, r.Registros[0].Decisao.Origem)
	require.Equal(t, ganchos.Bloquear, r.Decisao.Tipo)
}
