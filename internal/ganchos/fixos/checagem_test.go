package fixos

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/escopo"
	"go-concierge/internal/ganchos"
)

func ganchoDeFerramenta(t *testing.T, nome string) ganchos.Gancho[ganchos.AntesDaFerramenta] {
	t.Helper()
	for _, g := range Checagem().AntesDaFerramenta.Checagem {
		if g.Nome == nome {
			return g
		}
	}
	t.Fatalf("gancho %s não existe", nome)
	return ganchos.Gancho[ganchos.AntesDaFerramenta]{}
}

func TestChecagemMontaOsGanchosNaOrdem(t *testing.T) {
	f := Checagem()
	var antes []string
	for _, g := range f.AntesDaFerramenta.Checagem {
		antes = append(antes, g.Nome)
		if g.Falha != ganchos.Fechada {
			t.Errorf("%s deveria falhar fechado", g.Nome)
		}
	}
	if want := []string{FerramentaPermitida, EscritaConfirmada, ContextoB2C}; !reflect.DeepEqual(antes, want) {
		t.Errorf("ordem = %v, quer %v", antes, want)
	}
	if got := f.MensagemRecebida.Checagem; len(got) != 1 || got[0].Nome != EscopoDaConversa {
		t.Errorf("mensagem recebida = %v", got)
	}
	if len(f.AntesDaFerramenta.Saneamento) != 0 || len(f.MensagemRecebida.Saneamento) != 0 {
		t.Error("checagem não traz saneamento")
	}
}

func mapeamento() admin.Mapeamento {
	completo := func(id, adm, company, tenant string) admin.Condominio {
		return admin.Condominio{ID: id, Administradora: adm, Account: "acc", CompanyID: company, Tenant: tenant}
	}
	return admin.Mapeamento{Condominios: []admin.Condominio{
		completo("c-a", "A", "100", "t-a"),
		completo("c-b", "B", "200", "t-b"),
		{ID: "c-inc", Administradora: "A", CompanyID: "300"},
		completo("c-x1", "A", "", "t-amb"),
		completo("c-x2", "B", "", "t-amb"),
	}}
}

func TestEscopoDaConversa(t *testing.T) {
	tests := []struct {
		name       string
		e          ganchos.MensagemRecebida
		quer       ganchos.TipoDecisao
		condominio string
	}{
		{"condomínio da conversa", ganchos.MensagemRecebida{CompanyID: "100"}, ganchos.Ajustar, "c-a"},
		{"conversa nova pelo tenant", ganchos.MensagemRecebida{Tenant: "t-b"}, ganchos.Ajustar, "c-b"},
		{"mesma dona da sessão", ganchos.MensagemRecebida{CompanyID: "100", ContatoDaSessao: escopo.Contato{AdministradoraID: "A", CondominioID: "c-a"}}, ganchos.Ajustar, "c-a"},
		{"desconhecido", ganchos.MensagemRecebida{CompanyID: "999"}, ganchos.Bloquear, ""},
		{"sem identificação", ganchos.MensagemRecebida{}, ganchos.Bloquear, ""},
		{"fora da implantação", ganchos.MensagemRecebida{CompanyID: "200", Implantacao: escopo.Implantacao{AdministradoraFixa: "A"}}, ganchos.Bloquear, ""},
		{"ambíguo", ganchos.MensagemRecebida{Tenant: "t-amb"}, ganchos.Bloquear, ""},
		{"incompleto", ganchos.MensagemRecebida{CompanyID: "300"}, ganchos.Bloquear, ""},
		{"outro condomínio na sessão", ganchos.MensagemRecebida{CompanyID: "100", ContatoDaSessao: escopo.Contato{AdministradoraID: "A", CondominioID: "c-z"}}, ganchos.Bloquear, ""},
		{"mudou de dona", ganchos.MensagemRecebida{CompanyID: "100", ContatoDaSessao: escopo.Contato{AdministradoraID: "B", CondominioID: "c-a"}}, ganchos.Bloquear, ""},
	}
	gancho := Checagem().MensagemRecebida.Checagem[0]
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.e.Mapeamento = mapeamento()
			saiu, d := gancho.Avaliar(tt.e)
			if d.Tipo != tt.quer {
				t.Fatalf("tipo = %q, quer %q (%+v)", d.Tipo, tt.quer, d)
			}
			if tt.quer == ganchos.Bloquear && d.Motivo != MotivoCondominioNaoIdentificado {
				t.Errorf("motivo = %q", d.Motivo)
			}
			if saiu.Condominio.ID != tt.condominio {
				t.Errorf("condomínio = %q, quer %q", saiu.Condominio.ID, tt.condominio)
			}
		})
	}
}

func TestResolverCondominioEmbrulhaOMudou(t *testing.T) {
	e := ganchos.MensagemRecebida{
		Mapeamento: mapeamento(), CompanyID: "100",
		ContatoDaSessao: escopo.Contato{AdministradoraID: "B", CondominioID: "c-a"},
	}
	if _, err := resolverCondominio(e); !errors.Is(err, ErrCondominioMudou) {
		t.Fatalf("err = %v", err)
	}
}

func TestFerramentaPermitida(t *testing.T) {
	tests := []struct {
		name string
		e    ganchos.AntesDaFerramenta
		quer string
	}{
		{"permitida", ganchos.AntesDaFerramenta{Pedida: "agenda.listar", Declarada: true, Permitida: true}, ""},
		{"não declarada sem nenhuma permitida", ganchos.AntesDaFerramenta{Pedida: "x"},
			`recurso "x" não está declarado por este agente e nenhum recurso está permitido agora`},
		{"não declarada com permitidas", ganchos.AntesDaFerramenta{Pedida: "x", Permitidas: []string{"a", "b"}},
			`recurso "x" não está declarado por este agente; permitidos: a, b`},
		{"subagente", ganchos.AntesDaFerramenta{Pedida: "liberacao", Subagente: true},
			`"liberacao" é um subagente (rota), não um recurso: não execute o caso você mesmo; peça ao morador o pedido de forma direta para a rota levar ao especialista, ou responda só o que você sabe`},
		{"não permitida", ganchos.AntesDaFerramenta{Pedida: "reserva", Declarada: true, MotivoRecusa: "fora do horário"},
			"recurso reserva recusado: fora do horário"},
		{"MCP fora", ganchos.AntesDaFerramenta{Pedida: "unidades.listar", Declarada: true, Permitida: true, MCPObrigatoria: true},
			"tool MCP unidades.listar indisponível; o fluxo não pode usar dados simulados"},
		{"MCP no ar", ganchos.AntesDaFerramenta{Pedida: "unidades.listar", Declarada: true, Permitida: true, MCPObrigatoria: true, MCPDisponivel: true}, ""},
	}
	gancho := ganchoDeFerramenta(t, FerramentaPermitida)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, d := gancho.Avaliar(tt.e)
			if tt.quer == "" {
				if d.Tipo != ganchos.Deixar {
					t.Fatalf("decisão = %+v", d)
				}
				return
			}
			if d.Tipo != ganchos.Bloquear || d.Motivo != tt.quer {
				t.Fatalf("decisão = %+v, quer motivo %q", d, tt.quer)
			}
		})
	}
}

func TestEscritaConfirmada(t *testing.T) {
	const criar = ferramentaCriarVisita
	tests := []struct {
		name string
		e    ganchos.AntesDaFerramenta
		quer string
	}{
		{"prévia de escrita não grava", ganchos.AntesDaFerramenta{Pedida: "reserva", Escrita: true, Previa: true, Confirmacao: "quero reservar"}, ""},
		{"leitura passa", ganchos.AntesDaFerramenta{Pedida: "x"}, ""},
		{"escrita com sim", ganchos.AntesDaFerramenta{Pedida: "reserva", Escrita: true, Confirmacao: "Sim, pode!"}, ""},
		{"escrita sem sim", ganchos.AntesDaFerramenta{Pedida: "reserva", Escrita: true, Confirmacao: "quero reservar"},
			`reserva é escrita e exige a confirmação explícita do morador: a mensagem atual ("quero reservar") não é um sim. Resuma o que vai registrar, pergunte "confirma? (sim/não)" e pare`},
		{"criar visita fora do motor", ganchos.AntesDaFerramenta{Pedida: criar, Ferramenta: criar, Escrita: true, Confirmacao: "sim"},
			criar + " só pode ser executada pelo motor determinístico do caso de liberação de visitante"},
		{"criar visita pelo motor com sim", ganchos.AntesDaFerramenta{Pedida: criar, Ferramenta: criar, Escrita: true, PeloMotorDoCaso: true, Confirmacao: "sim"}, ""},
		{"criar visita pelo motor sem sim", ganchos.AntesDaFerramenta{Pedida: criar, Ferramenta: criar, Escrita: true, PeloMotorDoCaso: true, Confirmacao: "não"},
			criar + ` é escrita e exige a confirmação explícita do morador: a mensagem atual ("não") não é um sim. Resuma o que vai registrar, pergunte "confirma? (sim/não)" e pare`},
	}
	gancho := ganchoDeFerramenta(t, EscritaConfirmada)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, d := gancho.Avaliar(tt.e)
			if tt.quer == "" && d.Tipo != ganchos.Deixar {
				t.Fatalf("decisão = %+v", d)
			}
			if tt.quer != "" && (d.Tipo != ganchos.Bloquear || d.Motivo != tt.quer) {
				t.Fatalf("decisão = %+v, quer motivo %q", d, tt.quer)
			}
		})
	}
}

func TestContextoB2C(t *testing.T) {
	const (
		empresa = ferramentaEmpresaPorDocumento
		criar   = ferramentaCriarVisita
		motivos = ferramentaMotivosVisita
	)
	tests := []struct {
		name     string
		tool     string
		params   map[string]any
		contexto map[string]any
		quer     map[string]any
		recusa   string
	}{
		{"ferramenta sem contexto", "agenda.listar", map[string]any{"a": 1}, nil, map[string]any{"a": 1}, ""},
		{"companyId sobrescreve o do modelo", motivos, map[string]any{"companyId": "falso"}, map[string]any{"companyId": "c1"}, map[string]any{"companyId": "c1"}, ""},
		{"telefone e filtro", empresa, map[string]any{"filter": map[string]any{"document": "1"}},
			map[string]any{"companyId": "c1", "filter_mobile_number": "5511"},
			map[string]any{"companyId": "c1", "filter_mobile_number": "5511", "filter": map[string]any{"document": "1", "mobile-number": "5511"}}, ""},
		{"visita cria resident", criar, map[string]any{},
			map[string]any{"companyId": "c1", "resident": map[string]any{"mobile_number": "5511"}},
			map[string]any{"companyId": "c1", "resident": map[string]any{"mobile_number": "5511"}}, ""},
		{"visita sobrescreve resident", criar, map[string]any{"resident": map[string]any{"mobile_number": "falso", "name": "Ana"}},
			map[string]any{"companyId": "c1", "resident": map[string]any{"mobile_number": "5511"}},
			map[string]any{"companyId": "c1", "resident": map[string]any{"mobile_number": "5511", "name": "Ana"}}, ""},
		{"companyId ausente", motivos, map[string]any{}, map[string]any{}, nil, "contexto B2C inválido para t: companyId B2C ausente"},
		{"companyId em branco", motivos, map[string]any{}, map[string]any{"companyId": " "}, nil, "contexto B2C inválido para t: companyId B2C ausente"},
		{"telefone ausente", empresa, map[string]any{}, map[string]any{"companyId": "c1"}, nil, "contexto B2C inválido para t: telefone B2C ausente"},
		{"telefone do resident ausente", criar, map[string]any{}, map[string]any{"companyId": "c1"}, nil, "contexto B2C inválido para t: resident.mobile_number B2C ausente"},
		{"resident não objeto", criar, map[string]any{"resident": "Ana"},
			map[string]any{"companyId": "c1", "resident": map[string]any{"mobile_number": "5511"}}, nil, "contexto B2C inválido para t: resident deve ser objeto"},
		{"params nulos", motivos, nil, map[string]any{"companyId": "c1"}, map[string]any{"companyId": "c1"}, ""},
	}
	gancho := ganchoDeFerramenta(t, ContextoB2C)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saiu, d := gancho.Avaliar(ganchos.AntesDaFerramenta{Pedida: "t", Ferramenta: tt.tool, Params: tt.params, ContextoB2C: tt.contexto})
			if tt.recusa != "" {
				if d.Tipo != ganchos.Bloquear || d.Motivo != tt.recusa {
					t.Fatalf("decisão = %+v, quer motivo %q", d, tt.recusa)
				}
				return
			}
			quer := ganchos.Ajustar
			if !comContextoB2C[tt.tool] {
				quer = ganchos.Deixar
			}
			if d.Tipo != quer {
				t.Errorf("tipo = %q, quer %q", d.Tipo, quer)
			}
			if !reflect.DeepEqual(saiu.Params, tt.quer) {
				t.Errorf("params = %v, quer %v", saiu.Params, tt.quer)
			}
		})
	}
}

// espelha TestChamarRecursoRecusaEscritaENaoDeclarado: create-visit sem payloads recebe a
// recusa do motor, não a de contexto B2C.
func TestOrdemDaChecagemSegueInvocar(t *testing.T) {
	e := ganchos.AntesDaFerramenta{
		Pedida: ferramentaCriarVisita, Ferramenta: ferramentaCriarVisita,
		Declarada: true, Permitida: true, Escrita: true,
	}
	for _, g := range Checagem().AntesDaFerramenta.Checagem {
		if _, d := g.Avaliar(e); d.Tipo == ganchos.Bloquear {
			if !strings.Contains(d.Motivo, "motor determinístico") {
				t.Fatalf("%s bloqueou com %q", g.Nome, d.Motivo)
			}
			return
		}
	}
	t.Fatal("nenhum gancho bloqueou")
}

func TestNaoMudaParamsOriginais(t *testing.T) {
	filtro := map[string]any{"document": "1"}
	resident := map[string]any{"name": "Ana"}
	params := map[string]any{"filter": filtro, "resident": resident}
	antes := map[string]any{"filter": map[string]any{"document": "1"}, "resident": map[string]any{"name": "Ana"}}
	contexto := map[string]any{"companyId": "c1", "filter_mobile_number": "5511", "resident": map[string]any{"mobile_number": "5511"}}
	for _, tool := range []string{ferramentaEmpresaPorDocumento, ferramentaCriarVisita} {
		saiu, d := ganchoDeFerramenta(t, ContextoB2C).Avaliar(ganchos.AntesDaFerramenta{Pedida: tool, Ferramenta: tool, Params: params, ContextoB2C: contexto})
		if d.Tipo != ganchos.Ajustar {
			t.Fatalf("%s: decisão = %+v", tool, d)
		}
		if reflect.DeepEqual(saiu.Params, params) {
			t.Errorf("%s: saída igual à entrada, nada foi fixado", tool)
		}
	}
	if !reflect.DeepEqual(params, antes) || len(filtro) != 1 || len(resident) != 1 {
		t.Errorf("params originais mudaram: %v", params)
	}
}

func TestConfirmado(t *testing.T) {
	tests := map[string]bool{
		"sim": true, "pode ser": true, "não, espera": false, "sim mas não hoje": false,
		"confirmo a reserva": true, "qual o horário?": false, "ok!": true, "": false,
	}
	for msg, quer := range tests {
		if got := Confirmado(msg); got != quer {
			t.Errorf("Confirmado(%q) = %v, quer %v", msg, got, quer)
		}
	}
}
