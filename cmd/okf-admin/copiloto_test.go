package main

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/copiloto"
	"go-concierge/internal/harness"
	"go-concierge/internal/okf"
)

// copilotoFalso devolve sempre as mesmas operações e guarda o último pedido recebido.
type copilotoFalso struct {
	ops     []copiloto.Operacao
	recebeu harness.PedidoAoCopiloto
	err     error
}

func (f *copilotoFalso) responder(_ context.Context, p harness.PedidoAoCopiloto) (harness.RespostaDoModelo, error) {
	f.recebeu = p
	if f.err != nil {
		return harness.RespostaDoModelo{}, f.err
	}
	return harness.RespostaDoModelo{Mensagem: "Pronto.", Operacoes: f.ops}, nil
}

// servidorDoCopiloto: catálogo real copiado, Vila Verde da adm-exemplo (a do gestor) e
// Solar de outra administradora.
func servidorDoCopiloto(t *testing.T, falso *copilotoFalso) *servidor {
	t.Helper()
	s := servidorDeTeste(t, func(string) {})
	s.admin = admin.NewStore(filepath.Join(t.TempDir(), "mapeamento.json"))
	for _, a := range []admin.Administradora{{ID: "adm-exemplo", Nome: "Exemplo"}, {ID: "adm-outra", Nome: "Outra"}} {
		if _, err := s.admin.SalvarAdministradora(a); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []admin.Condominio{
		{ID: "vila-verde", Nome: "Vila Verde", Administradora: "adm-exemplo"},
		{ID: "solar", Nome: "Solar", Administradora: "adm-outra"},
	} {
		if _, err := s.admin.SalvarCondominio(c); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.admin.SalvarOrientacao("solar", admin.Orientacao{Geral: "segredo do Solar"}); err != nil {
		t.Fatal(err)
	}
	if falso != nil {
		s.copiloto = falso.responder
	}
	return s
}

func handlerDoCopiloto(s *servidor) http.Handler {
	mux := http.NewServeMux()
	s.rotasCopiloto(mux)
	return mux
}

func perguntar(t *testing.T, h http.Handler, papel string, ctx copiloto.Contexto) (int, respostaDoCopiloto) {
	t.Helper()
	corpo := corpoJSON(t, map[string]any{"mensagem": "Faça isso", "historico": []any{}, "contexto": ctx})
	w := chamar(t, h, pedido{metodo: "POST", caminho: "/api/copiloto", corpo: texto(corpo), cabecalhos: map[string]string{"X-Studio-Papel": papel}})
	var resp respostaDoCopiloto
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
	}
	return w.Code, resp
}

func ligado() *bool { v := true; return &v }

func TestCopilotoIndisponivel(t *testing.T) {
	h := handlerDoCopiloto(servidorDoCopiloto(t, nil))

	var estado estadoDoCopiloto
	lerJSON(t, h, "/api/copiloto/estado", &estado)
	if estado.Disponivel {
		t.Fatal("sem provedor o Copiloto não está disponível")
	}
	corpo := corpoJSON(t, map[string]any{"mensagem": "Oi", "contexto": copiloto.Contexto{Tela: copiloto.TelaSkills}})
	w := chamar(t, h, pedido{metodo: "POST", caminho: "/api/copiloto", corpo: texto(corpo)})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", w.Code)
	}
	var erro map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &erro); err != nil {
		t.Fatal(err)
	}
	exigirErroCom("COPILOTO_INDISPONIVEL", "O Copiloto está fora do ar. O resto do Studio continua funcionando.")(t, erro)
}

func TestCopilotoDisponivel(t *testing.T) {
	h := handlerDoCopiloto(servidorDoCopiloto(t, &copilotoFalso{}))
	var estado estadoDoCopiloto
	lerJSON(t, h, "/api/copiloto/estado", &estado)
	if !estado.Disponivel {
		t.Fatal("com provedor o Copiloto está disponível")
	}
}

func TestCopilotoDevolvePropostasDaTela(t *testing.T) {
	falso := &copilotoFalso{ops: []copiloto.Operacao{{
		Tipo: copiloto.HabilitarSkill, Skill: "reserva-area-comum", Condominio: "vila-verde", Habilitado: ligado(),
	}}}
	h := handlerDoCopiloto(servidorDoCopiloto(t, falso))

	status, resp := perguntar(t, h, "gestor", copiloto.Contexto{Tela: copiloto.TelaCondominio, Condominio: "vila-verde"})
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if resp.Mensagem != "Pronto." || len(resp.Recusas) != 0 || len(resp.Propostas) != 1 {
		t.Fatalf("resposta = %+v", resp)
	}
	p := resp.Propostas[0]
	if p.ID != "0" || p.Resumo != "Ligar Reserva de Área Comum em Vila Verde" || p.Operacao.Tipo != copiloto.HabilitarSkill {
		t.Fatalf("proposta = %+v", p)
	}
	if !slices.Contains(falso.recebeu.Operacoes, copiloto.HabilitarSkill) || slices.Contains(falso.recebeu.Operacoes, copiloto.TrocarFluxo) {
		t.Fatalf("operações mandadas ao modelo = %v", falso.recebeu.Operacoes)
	}
}

func TestCopilotoNaoPropoeAlemDoPapel(t *testing.T) {
	falso := &copilotoFalso{ops: []copiloto.Operacao{
		{Tipo: copiloto.TrocarFluxo, Skill: "agendar-mudanca", Fluxo: &okf.Fluxo{}},
		{Tipo: copiloto.HabilitarSkill, Skill: "reserva-area-comum", Condominio: "solar", Habilitado: ligado()},
	}}
	h := handlerDoCopiloto(servidorDoCopiloto(t, falso))

	status, resp := perguntar(t, h, "gestor", copiloto.Contexto{Tela: copiloto.TelaCondominio, Condominio: "solar"})
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(resp.Propostas) != 0 || len(resp.Recusas) != 2 || resp.Recusas[0].Indice != 0 || resp.Recusas[1].Indice != 1 {
		t.Fatalf("resposta = %+v", resp)
	}
	if strings.Contains(falso.recebeu.Contexto, "segredo do Solar") {
		t.Fatal("o contexto vazou a orientação de um condomínio de outra administradora")
	}
}

func TestCopilotoRecusaFluxoComErro(t *testing.T) {
	s := servidorDoCopiloto(t, nil)
	lido, err := s.cat.LerFluxo(s.caminhoDaSkill("agendar-mudanca"))
	if err != nil {
		t.Fatal(err)
	}
	semConfirmar := lido.Fluxo
	semConfirmar.Blocos = slices.DeleteFunc(slices.Clone(lido.Fluxo.Blocos), func(b okf.Bloco) bool { return b.Tipo == okf.BlocoConfirmar })
	falso := &copilotoFalso{ops: []copiloto.Operacao{
		{Tipo: copiloto.TrocarFluxo, Skill: "agendar-mudanca", Fluxo: &lido.Fluxo},
		{Tipo: copiloto.TrocarFluxo, Skill: "agendar-mudanca", Fluxo: &semConfirmar},
	}}
	s.copiloto = falso.responder

	status, resp := perguntar(t, handlerDoCopiloto(s), "construtor", copiloto.Contexto{Tela: copiloto.TelaFluxo, Skill: "agendar-mudanca"})
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if len(resp.Propostas) != 1 || resp.Propostas[0].ID != "0" {
		t.Fatalf("propostas = %+v", resp.Propostas)
	}
	quer := "A proposta deixa o fluxo com erro: Coloque um Confirmar antes de gravar no sistema."
	if len(resp.Recusas) != 1 || resp.Recusas[0].Indice != 1 || resp.Recusas[0].Motivo != quer {
		t.Fatalf("recusas = %+v", resp.Recusas)
	}
}

func TestCopilotoMandaContextoDoServidor(t *testing.T) {
	falso := &copilotoFalso{}
	h := handlerDoCopiloto(servidorDoCopiloto(t, falso))

	status, _ := perguntar(t, h, "construtor", copiloto.Contexto{Tela: copiloto.TelaSkill, Skill: "reserva-area-comum"})
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if !strings.Contains(falso.recebeu.Contexto, "Reserva de Área Comum") {
		t.Fatalf("contexto = %q", falso.recebeu.Contexto)
	}
	if falso.recebeu.Mensagem != "Faça isso" {
		t.Fatalf("mensagem = %q", falso.recebeu.Mensagem)
	}
}

func TestCopilotoRecusaMensagemVazia(t *testing.T) {
	h := handlerDoCopiloto(servidorDoCopiloto(t, &copilotoFalso{}))
	historicoLongo := make([]harness.MensagemDoCopiloto, 21)
	for i := range historicoLongo {
		historicoLongo[i] = harness.MensagemDoCopiloto{Papel: "pessoa", Texto: "oi"}
	}
	casos := []struct {
		nome  string
		corpo map[string]any
	}{
		{"mensagem vazia", map[string]any{"mensagem": "  ", "contexto": copiloto.Contexto{Tela: copiloto.TelaSkills}}},
		{"mensagem longa", map[string]any{"mensagem": strings.Repeat("a", 4001), "contexto": copiloto.Contexto{Tela: copiloto.TelaSkills}}},
		{"historico longo", map[string]any{"mensagem": "oi", "historico": historicoLongo, "contexto": copiloto.Contexto{Tela: copiloto.TelaSkills}}},
		{"papel desconhecido no historico", map[string]any{"mensagem": "oi", "historico": []harness.MensagemDoCopiloto{{Papel: "sistema", Texto: "x"}}, "contexto": copiloto.Contexto{Tela: copiloto.TelaSkills}}},
		{"tela desconhecida", map[string]any{"mensagem": "oi", "contexto": copiloto.Contexto{Tela: "xyz"}}},
		{"skill sem nome", map[string]any{"mensagem": "oi", "contexto": copiloto.Contexto{Tela: copiloto.TelaSkill}}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := chamar(t, h, pedido{metodo: "POST", caminho: "/api/copiloto", corpo: texto(corpoJSON(t, c.corpo))})
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d %s", w.Code, w.Body.String())
			}
			var erro map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &erro); err != nil {
				t.Fatal(err)
			}
			exigirErro(t, erro, "PEDIDO_INVALIDO")
		})
	}
}

func TestCopilotoDemorou(t *testing.T) {
	falso := &copilotoFalso{err: context.DeadlineExceeded}
	h := handlerDoCopiloto(servidorDoCopiloto(t, falso))
	status, _ := perguntar(t, h, "construtor", copiloto.Contexto{Tela: copiloto.TelaSkills})
	if status != http.StatusGatewayTimeout {
		t.Fatalf("status = %d", status)
	}
}

func TestQuemDoCopiloto(t *testing.T) {
	vazia := ""
	adm := "adm-exemplo"
	casos := []struct {
		nome    string
		id      *string
		quer    string
		querErr bool
	}{
		{"superlogica sem administradora", nil, "", false},
		{"administradora", &adm, "adm-exemplo", false},
		{"administradora vazia nunca vira superlogica", &vazia, "", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			quem, err := quemDoCopiloto(eu{AdministradoraID: c.id})
			if (err != nil) != c.querErr || quem.Administradora != c.quer {
				t.Fatalf("= %+v, %v", quem, err)
			}
		})
	}
}
