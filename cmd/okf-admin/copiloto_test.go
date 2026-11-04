package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

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
	s.admin = admin.NovaEmMemoria()
	for _, a := range []admin.Administradora{{ID: "adm-exemplo", Nome: "Exemplo"}, {ID: "adm-outra", Nome: "Outra"}} {
		if _, err := s.admin.SalvarAdministradora(t.Context(), a); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []admin.Condominio{
		{ID: "vila-verde", Nome: "Vila Verde", Administradora: "adm-exemplo"},
		{ID: "solar", Nome: "Solar", Administradora: "adm-outra"},
	} {
		if _, err := s.admin.SalvarCondominio(t.Context(), c); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.admin.SalvarOrientacao(t.Context(), admin.Alvo{Condominio: "solar", Dona: "adm-outra"}, admin.Orientacao{Geral: "segredo do Solar"}); err != nil {
		t.Fatal(err)
	}
	if falso != nil {
		s.copiloto = falso.responder
	}
	s.limiteCopiloto = novoLimiteDoCopiloto(time.Now)
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
	casos := []struct {
		nome        string
		tela        copiloto.Contexto
		ops         []copiloto.Operacao
		querMotivos []string
	}{
		{
			nome: "fora da tela e fora do escopo",
			tela: copiloto.Contexto{Tela: copiloto.TelaCondominio, Condominio: "solar"},
			ops: []copiloto.Operacao{
				{Tipo: copiloto.TrocarFluxo, Skill: "agendar-mudanca", Fluxo: &okf.Fluxo{}},
				{Tipo: copiloto.HabilitarSkill, Skill: "reserva-area-comum", Condominio: "solar", Habilitado: ligado()},
			},
			querMotivos: []string{"Esta tela não faz isso.", "Esse condomínio não está com você."},
		},
		{
			nome:        "a tela aceita e so o papel recusa",
			tela:        copiloto.Contexto{Tela: copiloto.TelaFluxo, Skill: "agendar-mudanca"},
			ops:         []copiloto.Operacao{{Tipo: copiloto.TrocarFluxo, Skill: "agendar-mudanca", Fluxo: &okf.Fluxo{}}},
			querMotivos: []string{"Você não tem permissão para isso."},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			falso := &copilotoFalso{ops: c.ops}
			status, resp := perguntar(t, handlerDoCopiloto(servidorDoCopiloto(t, falso)), "gestor", c.tela)
			if status != http.StatusOK || len(resp.Propostas) != 0 {
				t.Fatalf("status = %d, resposta = %+v", status, resp)
			}
			motivos := make([]string, len(resp.Recusas))
			for i, r := range resp.Recusas {
				if r.Indice != i {
					t.Fatalf("recusa %d com índice %d", i, r.Indice)
				}
				motivos[i] = r.Motivo
			}
			if !slices.Equal(motivos, c.querMotivos) {
				t.Fatalf("motivos = %q, quer %q", motivos, c.querMotivos)
			}
			if strings.Contains(falso.recebeu.Contexto, "segredo do Solar") {
				t.Fatal("o contexto vazou a orientação de um condomínio de outra administradora")
			}
		})
	}
}

func TestCopilotoSoPropoeParaOItemAberto(t *testing.T) {
	textos := &copiloto.TextosParciais{Resumo: ptr("Novo resumo.")}
	conteudo := "novo"
	casos := []struct {
		nome   string
		tela   copiloto.Contexto
		certa  copiloto.Operacao
		outra  copiloto.Operacao
		motivo string
	}{
		{
			"skill", copiloto.Contexto{Tela: copiloto.TelaSkill, Skill: "reserva-area-comum"},
			copiloto.Operacao{Tipo: copiloto.PreencherTextos, Skill: "reserva-area-comum", Textos: textos},
			copiloto.Operacao{Tipo: copiloto.PreencherTextos, Skill: "agendar-mudanca", Textos: textos},
			motivoOutroItem,
		},
		{
			"condominio", copiloto.Contexto{Tela: copiloto.TelaCondominio, Condominio: "vila-verde"},
			copiloto.Operacao{Tipo: copiloto.Orientar, Condominio: "vila-verde", Texto: "oi"},
			copiloto.Operacao{Tipo: copiloto.Orientar, Condominio: "solar", Texto: "oi"},
			"Esse condomínio não está com você.", // Filtrar já prende a ficha ao condomínio aberto
		},
		{
			"arquivo", copiloto.Contexto{Tela: copiloto.TelaAvancado, Agente: "agentes/concierge", Caminho: "knowledge/a.md"},
			copiloto.Operacao{Tipo: copiloto.EditarArquivo, Agente: "agentes/concierge", Caminho: "knowledge/a.md", Conteudo: &conteudo},
			copiloto.Operacao{Tipo: copiloto.EditarArquivo, Agente: "agentes/concierge", Caminho: "knowledge/b.md", Conteudo: &conteudo},
			motivoOutroItem,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			falso := &copilotoFalso{ops: []copiloto.Operacao{c.certa, c.outra}}
			status, resp := perguntar(t, handlerDoCopiloto(servidorDoCopiloto(t, falso)), "construtor", c.tela)
			if status != http.StatusOK {
				t.Fatalf("status = %d", status)
			}
			if len(resp.Propostas) != 1 || resp.Propostas[0].ID != "0" {
				t.Fatalf("propostas = %+v", resp.Propostas)
			}
			if len(resp.Recusas) != 1 || resp.Recusas[0].Indice != 1 || resp.Recusas[0].Motivo != c.motivo {
				t.Fatalf("recusas = %+v", resp.Recusas)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

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
		{"fala do historico longa", map[string]any{"mensagem": "oi", "historico": []harness.MensagemDoCopiloto{{Papel: "pessoa", Texto: strings.Repeat("a", 4001)}}, "contexto": copiloto.Contexto{Tela: copiloto.TelaSkills}}},
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

func TestHistoricoNoOrcamento(t *testing.T) {
	fala := func(n int) harness.MensagemDoCopiloto {
		return harness.MensagemDoCopiloto{Papel: "pessoa", Texto: strings.Repeat("a", n)}
	}
	cheias := func(n int) []harness.MensagemDoCopiloto {
		out := make([]harness.MensagemDoCopiloto, n)
		for i := range out {
			out[i] = fala(4000)
		}
		return out
	}
	casos := []struct {
		nome      string
		historico []harness.MensagemDoCopiloto
		querFalas int
	}{
		{"mantem o que cabe", cheias(2), 2},
		{"mantem o teto exato", cheias(5), 5},
		{"tira as mais antigas", append([]harness.MensagemDoCopiloto{fala(1)}, cheias(5)...), 5},
		{"vazio", nil, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := historicoNoOrcamento(c.historico)
			if len(got) != c.querFalas {
				t.Fatalf("falas = %d, quer %d", len(got), c.querFalas)
			}
			if len(got) > 0 && &got[len(got)-1] != &c.historico[len(c.historico)-1] {
				t.Fatal("ficou sem a fala mais nova")
			}
		})
	}
}

func TestCopilotoCortaHistoricoAntesDoModelo(t *testing.T) {
	falso := &copilotoFalso{}
	h := handlerDoCopiloto(servidorDoCopiloto(t, falso))
	historico := make([]harness.MensagemDoCopiloto, 6)
	for i := range historico {
		historico[i] = harness.MensagemDoCopiloto{Papel: "pessoa", Texto: strings.Repeat("a", 4000)}
	}
	corpo := corpoJSON(t, map[string]any{"mensagem": "oi", "historico": historico, "contexto": copiloto.Contexto{Tela: copiloto.TelaSkills}})
	if w := chamar(t, h, pedido{metodo: "POST", caminho: "/api/copiloto", corpo: texto(corpo)}); w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if len(falso.recebeu.Historico) != 5 {
		t.Fatalf("o modelo recebeu %d falas, quer 5", len(falso.recebeu.Historico))
	}
}

func TestCopilotoTruncaArquivoDoAvancado(t *testing.T) {
	falso := &copilotoFalso{}
	s := servidorDoCopiloto(t, falso)
	grande := strings.Repeat("x", maxArquivoCopiloto) + "FIM"
	if err := s.cat.SalvarArquivo("agentes/concierge", "knowledge/grande.md", []byte(grande)); err != nil {
		t.Fatal(err)
	}
	if err := s.cat.SalvarArquivo("agentes/concierge", "knowledge/pequeno.md", []byte("curto")); err != nil {
		t.Fatal(err)
	}
	h := handlerDoCopiloto(s)
	casos := []struct {
		nome, papel, caminho string
		querTem, querSem     []string
	}{
		{"corta o arquivo grande", "construtor", "knowledge/grande.md", []string{"arquivo truncado"}, []string{"FIM"}},
		{"manda o arquivo pequeno inteiro", "construtor", "knowledge/pequeno.md", []string{"curto"}, []string{"arquivo truncado"}},
		{"sem acesso ao Avançado não manda o arquivo", "gestor", "knowledge/pequeno.md", nil, []string{"curto"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			status, _ := perguntar(t, h, c.papel, copiloto.Contexto{Tela: copiloto.TelaAvancado, Agente: "agentes/concierge", Caminho: c.caminho})
			if status != http.StatusOK {
				t.Fatalf("status = %d", status)
			}
			for _, quer := range c.querTem {
				if !strings.Contains(falso.recebeu.Contexto, quer) {
					t.Fatalf("contexto sem %q", quer)
				}
			}
			for _, nao := range c.querSem {
				if strings.Contains(falso.recebeu.Contexto, nao) {
					t.Fatalf("contexto com %q", nao)
				}
			}
			if n := utf8.RuneCountInString(falso.recebeu.Contexto); n > maxArquivoCopiloto+200 {
				t.Fatalf("contexto com %d caracteres", n)
			}
		})
	}
}

// copilotoPreso segura cada chamada até soltar fechar, para ocupar as vagas.
type copilotoPreso struct {
	entrou chan struct{}
	soltar chan struct{}
}

func (p *copilotoPreso) responder(ctx context.Context, _ harness.PedidoAoCopiloto) (harness.RespostaDoModelo, error) {
	p.entrou <- struct{}{}
	select {
	case <-p.soltar:
	case <-ctx.Done():
		return harness.RespostaDoModelo{}, ctx.Err()
	}
	return harness.RespostaDoModelo{Mensagem: "Pronto."}, nil
}

func pedirAoCopiloto(t *testing.T, h http.Handler, papel string) *httptest.ResponseRecorder {
	t.Helper()
	corpo := corpoJSON(t, map[string]any{"mensagem": "oi", "contexto": copiloto.Contexto{Tela: copiloto.TelaSkills}})
	return chamar(t, h, pedido{metodo: "POST", caminho: "/api/copiloto", corpo: texto(corpo), cabecalhos: map[string]string{"X-Studio-Papel": papel}})
}

func TestCopilotoLimitaChamadasSimultaneas(t *testing.T) {
	s := servidorDoCopiloto(t, nil)
	// entrou com folga: a chamada depois de soltar também passa pelo falso
	preso := &copilotoPreso{entrou: make(chan struct{}, chamadasSimultaneasCopiloto+1), soltar: make(chan struct{})}
	s.copiloto = preso.responder
	h := handlerDoCopiloto(s)

	var espera sync.WaitGroup
	for range chamadasSimultaneasCopiloto {
		espera.Go(func() { pedirAoCopiloto(t, h, "construtor") })
		<-preso.entrou
	}
	w := pedirAoCopiloto(t, h, "gestor")
	close(preso.soltar)
	espera.Wait()

	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "5" {
		t.Fatalf("status = %d, Retry-After = %q", w.Code, w.Header().Get("Retry-After"))
	}
	var erro map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &erro); err != nil {
		t.Fatal(err)
	}
	exigirErro(t, erro, "COPILOTO_OCUPADO")
	if w := pedirAoCopiloto(t, h, "gestor"); w.Code != http.StatusOK {
		t.Fatalf("vagas liberadas: status = %d", w.Code)
	}
}

func TestCopilotoLimitaChamadasPorUsuario(t *testing.T) {
	s := servidorDoCopiloto(t, &copilotoFalso{})
	agora := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	s.limiteCopiloto = novoLimiteDoCopiloto(func() time.Time { return agora })
	h := handlerDoCopiloto(s)

	for i := range chamadasPorMinutoCopiloto {
		if w := pedirAoCopiloto(t, h, "construtor"); w.Code != http.StatusOK {
			t.Fatalf("chamada %d: status = %d", i+1, w.Code)
		}
	}
	w := pedirAoCopiloto(t, h, "construtor")
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") != "6" {
		t.Fatalf("11ª chamada: status = %d, Retry-After = %q", w.Code, w.Header().Get("Retry-After"))
	}
	if w := pedirAoCopiloto(t, h, "gestor"); w.Code != http.StatusOK {
		t.Fatalf("outro usuário tem o próprio balde: status = %d", w.Code)
	}
	agora = agora.Add(6 * time.Second)
	if w := pedirAoCopiloto(t, h, "construtor"); w.Code != http.StatusOK {
		t.Fatalf("depois de 6 s volta uma ficha: status = %d", w.Code)
	}
	if w := pedirAoCopiloto(t, h, "construtor"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("só uma ficha voltou: status = %d", w.Code)
	}
}

func TestLimiteDoCopilotoTiraBaldesParados(t *testing.T) {
	inicio := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	agora := inicio
	l := novoLimiteDoCopiloto(func() time.Time { return agora })
	tirar := func(usuario string) {
		t.Helper()
		if _, ok := l.tirarFicha(usuario); !ok {
			t.Fatalf("%s sem ficha às %s", usuario, agora.Sub(inicio))
		}
	}

	for range chamadasPorMinutoCopiloto {
		tirar("ana")
	}
	tirar("bia")
	agora = inicio.Add(5 * time.Minute)
	tirar("caio")
	if len(l.baldes) != 3 {
		t.Fatalf("antes da janela: %d baldes, quer 3", len(l.baldes))
	}
	agora = inicio.Add(baldeParado)
	tirar("caio")
	if len(l.baldes) != 1 {
		t.Fatalf("depois da janela: %d baldes, quer 1 (só caio)", len(l.baldes))
	}
	for range chamadasPorMinutoCopiloto {
		tirar("ana") // o balde novo de ana está cheio, como estaria o antigo
	}
}

func TestLimiteDoCopilotoVarreNoMaximoUmaVezPorMinuto(t *testing.T) {
	inicio := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	agora := inicio
	l := novoLimiteDoCopiloto(func() time.Time { return agora })
	l.tirarFicha("ana")
	agora = inicio.Add(baldeParado - 30*time.Second)
	l.tirarFicha("bia") // varre aqui: ana ainda não está parada
	agora = inicio.Add(baldeParado)
	l.tirarFicha("bia") // 30 s depois da última varrida: não varre
	if _, ok := l.baldes["ana"]; !ok {
		t.Fatal("varreu antes de um minuto")
	}
	agora = inicio.Add(baldeParado + 30*time.Second)
	l.tirarFicha("bia")
	if _, ok := l.baldes["ana"]; ok {
		t.Fatal("ana parada há mais de 10 min continuou no mapa")
	}
}
