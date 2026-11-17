package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/ganchostest"
	"go-concierge/internal/ganchos/regras"
	"go-concierge/internal/mcp"
)

const (
	cpfValido         = "52998224725"
	telefoneCanal     = "5519981621916"
	ferramentaEmpresa = "gruvi.bff.building-operation.get-company-by-document-controller-handle"
	ferramentaMotivos = "gruvi.bff.building-operation.fetch-visit-reasons-controller-handle"
)

// cenarioGanchos monta o concierge com o MCP falso, payloads B2C e o conjunto dado.
type cenarioGanchos struct {
	ag    *Agente
	fake  *mcpFalso
	turno *ganchos.Turno
}

func novoCenarioGanchos(t *testing.T, conjunto *ganchos.Conjunto, fake *mcpFalso) cenarioGanchos {
	t.Helper()
	turno := conjunto.Turno()
	ag, err := montarConcierge(t, Config{MCP: fake, Ganchos: conjunto, Turno: &turno})
	if err != nil {
		t.Fatal(err)
	}
	return cenarioGanchos{ag: ag, fake: fake, turno: &turno}
}

// montarConcierge completa cfg com o Mock, os payloads B2C e um morador identificado.
func montarConcierge(t *testing.T, cfg Config) (*Agente, error) {
	t.Helper()
	cat, a := catalogoComMCP(t)
	cfg.Mock, cfg.Env = true, semEnv
	cfg.Identidade = Identidade{Authenticated: true, AccountID: "cond-001", Role: "morador"}
	cfg.Payloads = map[string]map[string]any{
		"unidades.listar":     {"companyId": "5982", "filter_mobile_number": telefoneCanal},
		ferramentaMotivos:     {"companyId": "5982"},
		ferramentaCriarVisita: {"companyId": "5982", "resident": map[string]any{"mobile_number": telefoneCanal}},
	}
	return Build(context.Background(), cat, a, cfg)
}

func comRegras(antes []ganchos.Gancho[ganchos.AntesDaFerramenta], depois []ganchos.Gancho[ganchos.DepoisDaFerramenta]) *ganchos.Conjunto {
	return ganchostest.Conjunto(fixos.Todos(), &ganchos.Indice{
		AntesDaFerramenta:  ganchos.Nivel[ganchos.AntesDaFerramenta]{Globais: antes},
		DepoisDaFerramenta: ganchos.Nivel[ganchos.DepoisDaFerramenta]{Globais: depois},
	})
}

func resultadoTexto(texto string) *mcp.CallResult {
	return &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: texto}}}
}

func TestGanchosVeemAsTresEntradasDoInvocador(t *testing.T) {
	antes, depois := &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{}, &ganchostest.Roteiro[ganchos.DepoisDaFerramenta]{}
	conjunto := comRegras(
		[]ganchos.Gancho[ganchos.AntesDaFerramenta]{ganchostest.Gancho("antes", antes)},
		[]ganchos.Gancho[ganchos.DepoisDaFerramenta]{ganchostest.Gancho("depois", depois)},
	)
	c := novoCenarioGanchos(t, conjunto, &mcpFalso{})
	reservas, visita := c.ag.Subagentes["reserva-area-comum"], c.ag.Subagentes["liberacao-visitante"]
	ctx := context.Background()

	entradas := []struct {
		nome    string
		invocar func() (map[string]any, error)
	}{
		{"chamarRecurso", func() (map[string]any, error) {
			return reservas.chamarRecurso(ctx, &argsRecurso{Nome: "unidades.listar"})
		}},
		{"InvocarRecurso", func() (map[string]any, error) {
			return reservas.InvocarRecurso(ctx, "unidades.listar", nil, "")
		}},
		{"ExecutarPassoCaso", func() (map[string]any, error) {
			return visita.ExecutarPassoCaso(ctx, ferramentaMotivos, map[string]any{"filter_visitor_type": "GUEST"}, "")
		}},
	}
	for i, e := range entradas {
		out, err := e.invocar()
		if err != nil || erroDe(out) != "" {
			t.Fatalf("%s: %v %v", e.nome, out, err)
		}
		if len(antes.Eventos()) != i+1 || len(depois.Eventos()) != i+1 {
			t.Fatalf("%s: antes viu %d, depois viu %d; quer %d", e.nome, len(antes.Eventos()), len(depois.Eventos()), i+1)
		}
	}
	if ev := antes.Eventos()[2]; !ev.PeloMotorDoCaso || ev.Caso == "" {
		t.Fatalf("o passo do caso chega marcado e com o caso: %+v", ev)
	}
}

func TestGanchosAliasUnidadesListar(t *testing.T) {
	roteiro := &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{}
	c := novoCenarioGanchos(t, comRegras([]ganchos.Gancho[ganchos.AntesDaFerramenta]{ganchostest.Gancho("antes", roteiro)}, nil), &mcpFalso{})
	params := map[string]any{"companyId": "9999", "filter_mobile_number": "000", "filter": map[string]any{"mobile-number": "000"}}

	out, err := c.ag.Subagentes["reserva-area-comum"].InvocarRecurso(context.Background(), "unidades.listar", params, "")
	if err != nil || erroDe(out) != "" {
		t.Fatalf("alias: %v %v", out, err)
	}
	ev := roteiro.Eventos()[0]
	if ev.Pedida != "unidades.listar" || ev.Ferramenta != ferramentaEmpresa {
		t.Fatalf("Pedida é o nome do modelo, Ferramenta o MCP: %q %q", ev.Pedida, ev.Ferramenta)
	}
	args := c.fake.argsPorNome[ferramentaEmpresa]
	filtro, _ := args["filter"].(map[string]any)
	if args["companyId"] != "5982" || args["filter_mobile_number"] != telefoneCanal || filtro["mobile-number"] != telefoneCanal {
		t.Fatalf("companyId e telefone B2C vêm do canal: %+v", args)
	}
	if params["companyId"] != "9999" {
		t.Fatalf("o mapa do chamador não muda: %+v", params)
	}
}

func TestGanchosDecisaoAntesDaFerramenta(t *testing.T) {
	casos := []struct {
		nome     string
		decisoes []ganchos.Decisao
		querErro string
	}{
		{"bloquear devolve o texto do gancho", []ganchos.Decisao{{Tipo: ganchos.Bloquear, Motivo: "reserva só até 22h"}}, "reserva só até 22h"},
		{"confirmar junta as perguntas", []ganchos.Decisao{
			{Tipo: ganchos.Confirmar, Motivo: "é para hoje?"},
			{Tipo: ganchos.Confirmar, Motivo: "são duas pessoas?"},
		}, "é para hoje?\nsão duas pessoas?"},
	}
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			var regras []ganchos.Gancho[ganchos.AntesDaFerramenta]
			for _, d := range tt.decisoes {
				regras = append(regras, ganchostest.Gancho("regra", &ganchostest.Roteiro[ganchos.AntesDaFerramenta]{Decisao: d}))
			}
			c := novoCenarioGanchos(t, comRegras(regras, nil), &mcpFalso{})
			col := &coletor{transbordo: new(string)}
			ctx := context.WithValue(context.Background(), chaveColetor{}, col)

			out, err := c.ag.Subagentes["reserva-area-comum"].chamarRecurso(ctx, &argsRecurso{Nome: "unidades.listar"})
			if err != nil || erroDe(out) != tt.querErro {
				t.Fatalf("erro %q, quer %q (err=%v)", erroDe(out), tt.querErro, err)
			}
			if len(c.fake.chamadas) != 0 {
				t.Fatalf("o MCP não pode ser chamado: %v", c.fake.chamadas)
			}
			if len(registrosDe(col.registros, "regra")) != len(tt.decisoes) {
				t.Fatalf("registros no coletor: %+v", col.registros)
			}
		})
	}
}

func TestGanchosPIINoResultadoENosParams(t *testing.T) {
	fake := &mcpFalso{resultados: map[string]*mcp.CallResult{
		ferramentaEmpresa: resultadoTexto(`{"morador":{"cpf":"` + cpfValido + `"}}`),
	}}
	c := novoCenarioGanchos(t, ganchos.NovoConjunto(fixos.Todos(), nil), fake)
	reservas := c.ag.Subagentes["reserva-area-comum"]

	out, err := reservas.InvocarRecurso(context.Background(), "unidades.listar", nil, "")
	if err != nil || erroDe(out) != "" {
		t.Fatalf("leitura: %v %v", out, err)
	}
	cru, _ := json.Marshal(out)
	if strings.Contains(string(cru), cpfValido) || !strings.Contains(string(cru), "«cpf:1»") {
		t.Fatalf("o CPF do resultado chega mascarado: %s", cru)
	}
	if c.turno.Cofre().Ler()["«cpf:1»"] != cpfValido {
		t.Fatal("o valor do CPF fica no cofre do turno")
	}

	if _, err := reservas.InvocarRecurso(context.Background(), "unidades.listar", map[string]any{"document": "«cpf:1»"}, ""); err != nil {
		t.Fatal(err)
	}
	if got := fake.argsPorNome[ferramentaEmpresa]["document"]; got != cpfValido {
		t.Fatalf("o marcador chega ao MCP como o CPF real; veio %v", got)
	}
}

// Linhas da spec §11 para antes e depois da ferramenta.
func TestPoliticaDeFalhaNaFerramenta(t *testing.T) {
	visitaAutorizada := resultadoTexto(`{"data":{"visit":{"clearance_status":"AUTHORIZED"}}}`)
	casos := []struct {
		nome        string
		conjunto    *ganchos.Conjunto
		escrita     bool
		querChamada bool
		querErro    string
	}{
		{"fixo em pânico antes não chama o MCP", conjuntoComFixoQuebrado(true), false, false, ganchos.MotivoDeFalha},
		{"fixo em pânico depois devolve indisponível", conjuntoComFixoQuebrado(false), false, true, ferramentaMotivos + " indisponível"},
		{"regra em pânico antes de escrita bloqueia", comRegras([]ganchos.Gancho[ganchos.AntesDaFerramenta]{quebrado[ganchos.AntesDaFerramenta](ganchos.FechadaSeGrava)}, nil), true, false, ganchos.MotivoDeFalha},
		{"regra em pânico antes de leitura deixa passar", comRegras([]ganchos.Gancho[ganchos.AntesDaFerramenta]{quebrado[ganchos.AntesDaFerramenta](ganchos.FechadaSeGrava)}, nil), false, true, ""},
		{"regra em pânico depois deixa passar", comRegras(nil, []ganchos.Gancho[ganchos.DepoisDaFerramenta]{quebrado[ganchos.DepoisDaFerramenta](ganchos.Aberta)}), false, true, ""},
	}
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			fake := &mcpFalso{resultados: map[string]*mcp.CallResult{ferramentaCriarVisita: visitaAutorizada}}
			c := novoCenarioGanchos(t, tt.conjunto, fake)
			nome, params, sim := ferramentaMotivos, map[string]any{"filter_visitor_type": "GUEST"}, ""
			if tt.escrita {
				nome, params, sim = ferramentaCriarVisita, map[string]any{"unitId": 109, "reason": 1, "visitors": []any{map[string]any{"name": "Ana"}}}, "sim"
			}

			out, err := c.ag.Subagentes["liberacao-visitante"].ExecutarPassoCaso(context.Background(), nome, params, sim)
			if err != nil {
				t.Fatal(err)
			}
			if erroDe(out) != tt.querErro {
				t.Fatalf("erro %q, quer %q", erroDe(out), tt.querErro)
			}
			if chamou := len(fake.chamadas) > 0; chamou != tt.querChamada {
				t.Fatalf("MCP chamado=%v, quer %v", chamou, tt.querChamada)
			}
			if tt.querErro != "" && out["resultado"] != nil {
				t.Fatalf("o resultado não volta: %+v", out)
			}
		})
	}
}

func registrosDe(rs []ganchos.Registro, gancho string) []ganchos.Registro {
	var out []ganchos.Registro
	for _, r := range rs {
		if r.Decisao.Origem.Gancho == gancho {
			out = append(out, r)
		}
	}
	return out
}

func quebrado[E ganchos.Ocorrencia](falha ganchos.Politica) ganchos.Gancho[E] {
	return ganchos.Gancho[E]{Nome: "quebrado", Falha: falha, Avaliar: func(E) (E, ganchos.Decisao) { panic("quebrou") }}
}

// conjuntoComFixoQuebrado acrescenta aos fixos um que entra em pânico antes ou depois.
func conjuntoComFixoQuebrado(antes bool) *ganchos.Conjunto {
	f := fixos.Todos()
	if antes {
		f.AntesDaFerramenta.Checagem = append(f.AntesDaFerramenta.Checagem, quebrado[ganchos.AntesDaFerramenta](ganchos.Fechada))
	} else {
		f.DepoisDaFerramenta.Saneamento = append(f.DepoisDaFerramenta.Saneamento, quebrado[ganchos.DepoisDaFerramenta](ganchos.Fechada))
	}
	return ganchos.NovoConjunto(f, nil)
}

func TestBuildRecusaGanchosSemFixos(t *testing.T) {
	semFixos := ganchos.NovoConjunto(ganchos.Fixos{}, nil)
	turnoSemFixos := semFixos.Turno()
	casos := []struct {
		nome string
		cfg  Config
	}{
		{"conjunto sem fixos", Config{Ganchos: semFixos}},
		{"conjunto só com saneamento", Config{Ganchos: ganchos.NovoConjunto(fixos.Saneamento(), nil)}},
		{"turno sem fixos", Config{Turno: &turnoSemFixos}},
	}
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			if _, err := montarConcierge(t, tt.cfg); err == nil || !strings.Contains(err.Error(), fixos.FerramentaPermitida) {
				t.Fatalf("Build deveria recusar; err=%v", err)
			}
		})
	}
}

// Não roda em paralelo: troca o slog.Default do processo.
func TestFixoQuebradoNoConjuntoPadraoLogaErro(t *testing.T) {
	var buf bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })

	f := fixos.Todos()
	f.AntesDaFerramenta.Checagem = append(f.AntesDaFerramenta.Checagem, quebrado[ganchos.AntesDaFerramenta](ganchos.Fechada))
	conjunto, err := conjuntoDosFixos(f)
	if err != nil {
		t.Fatal(err)
	}
	conjunto.Turno().AntesDaFerramenta(context.Background(), ganchos.AntesDaFerramenta{Declarada: true, Permitida: true})
	if !strings.Contains(buf.String(), "level=ERROR") {
		t.Fatalf("falha de fixo sai em ERROR; log:\n%s", buf.String())
	}
}

func TestErroDoMCPSaiMascarado(t *testing.T) {
	fake := &mcpFalso{resultados: map[string]*mcp.CallResult{
		ferramentaEmpresa: {IsError: true, Content: []mcp.Content{{Type: "text", Text: "documento " + cpfValido + " não encontrado"}}},
	}}
	c := novoCenarioGanchos(t, ganchos.NovoConjunto(fixos.Todos(), nil), fake)

	out, err := c.ag.Subagentes["reserva-area-comum"].InvocarRecurso(context.Background(), "unidades.listar", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if texto := erroDe(out); strings.Contains(texto, cpfValido) || !strings.Contains(texto, "«cpf:1»") {
		t.Fatalf("o CPF do erro do MCP sai mascarado: %q", texto)
	}
	if c.turno.Cofre().Ler()["«cpf:1»"] != cpfValido {
		t.Fatal("o valor do CPF fica no cofre do turno")
	}
}

// mcpConcorrente serializa o mcpFalso, que não é seguro entre goroutines.
type mcpConcorrente struct {
	mu   sync.Mutex
	fake mcpFalso
}

func (m *mcpConcorrente) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fake.CallTool(ctx, name, args)
}

func TestChamadasParalelasNoMesmoTurno(t *testing.T) {
	const chamadas = 4
	conjunto := ganchos.NovoConjunto(fixos.Todos(), nil)
	turno := conjunto.Turno()
	caller := &mcpConcorrente{fake: mcpFalso{resultados: map[string]*mcp.CallResult{
		ferramentaEmpresa: resultadoTexto(`{"morador":{"cpf":"` + cpfValido + `"}}`),
	}}}
	ag, err := montarConcierge(t, Config{MCP: caller, Ganchos: conjunto, Turno: &turno})
	if err != nil {
		t.Fatal(err)
	}
	col := &coletor{transbordo: new(string)}
	ctx := context.WithValue(context.Background(), chaveColetor{}, col)
	reservas := ag.Subagentes["reserva-area-comum"]

	saidas := make([]map[string]any, chamadas)
	var wg sync.WaitGroup
	for i := range chamadas {
		wg.Go(func() {
			saidas[i], _ = reservas.chamarRecurso(ctx, &argsRecurso{Nome: "unidades.listar"})
		})
	}
	wg.Wait()

	cofre := turno.Cofre().Ler()
	for i, out := range saidas {
		cru, _ := json.Marshal(out)
		if erroDe(out) != "" || strings.Contains(string(cru), cpfValido) {
			t.Fatalf("chamada %d: %s", i, cru)
		}
	}
	if len(cofre) == 0 || len(col.registros) != 2*chamadas {
		t.Fatalf("cofre %d valores, %d registros; quer >0 e %d (contexto_b2c e pii_no_resultado por chamada)", len(cofre), len(col.registros), 2*chamadas)
	}
}

// conversaComRegra simula os turnos do caminho do modelo: cada fala abre um turno, que leva
// as perguntas da regra feitas no turno anterior quando a fala as confirma (o que a borda,
// atende, faz com a sessão).
type conversaComRegra struct {
	t          *testing.T
	conjunto   *ganchos.Conjunto
	fake       *mcpFalso
	anteriores []string
}

func novaConversaComRegra(t *testing.T, ferramenta, pergunta string) *conversaComRegra {
	t.Helper()
	definicao := `{"nome":"Confirmar ferramenta","evento":"antes_da_ferramenta","alvo":{},` +
		`"condicoes":[{"tipo":"ferramenta","nomes":["` + ferramenta + `"]}],` +
		`"decisao":{"tipo":"confirmar","pergunta":"` + pergunta + `"},"ativa":true}`
	indice, erros := regras.Compilar([]regras.RegraGravada{{ID: "rc", Versao: 1, Ativa: true, Definicao: []byte(definicao)}})
	if len(erros) > 0 {
		t.Fatal(erros)
	}
	return &conversaComRegra{t: t, conjunto: ganchostest.Conjunto(fixos.Todos(), indice), fake: &mcpFalso{}}
}

// falar chama o recurso como o subagente da reserva, num turno novo, com a fala do morador.
func (c *conversaComRegra) falar(mensagem, recurso string, params map[string]any) saida {
	c.t.Helper()
	var confirmadas []string
	if fixos.Confirmado(mensagem) {
		confirmadas = c.anteriores
	}
	turno := c.conjunto.Turno().ComPerguntasConfirmadas(confirmadas)
	ag, err := montarConcierge(c.t, Config{MCP: c.fake, Ganchos: c.conjunto, Turno: &turno})
	if err != nil {
		c.t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), chaveColetor{}, &coletor{mensagem: mensagem, transbordo: new(string)})
	out, err := ag.Subagentes["reserva-area-comum"].chamarRecurso(ctx, &argsRecurso{Nome: recurso, Params: params})
	if err != nil {
		c.t.Fatal(err)
	}
	c.anteriores = turno.PerguntasFeitas()
	return out
}

// Caminho do modelo, leitura: a regra devolve a pergunta como erro da ferramenta; no turno
// seguinte, o sim do morador a responde e a ferramenta roda.
func TestRegraQuePedeConfirmacaoPassaComOSimDoTurnoSeguinte(t *testing.T) {
	c := novaConversaComRegra(t, ferramentaEmpresa, "Quer mesmo ver as unidades?")

	out := c.falar("quais são minhas unidades?", "unidades.listar", nil)
	if erroDe(out) != "Quer mesmo ver as unidades?" || len(c.fake.chamadas) != 0 {
		t.Fatalf("sem o sim, a pergunta da regra volta como erro e o MCP não roda: %q %v", erroDe(out), c.fake.chamadas)
	}
	if perguntas, _ := out[ChavePerguntas].([]string); len(perguntas) != 1 {
		t.Fatalf("a saída leva as perguntas para o motor do caso: %v", out)
	}
	if out := c.falar("sim", "unidades.listar", nil); erroDe(out) != "" || len(c.fake.chamadas) != 1 {
		t.Fatalf("com o sim à pergunta, a ferramenta roda: %q %v", erroDe(out), c.fake.chamadas)
	}
}

// Caminho do modelo, escrita: o primeiro sim responde o escrita_confirmada e só então a
// regra pergunta; o sim seguinte responde a regra e a escrita passa pelos dois.
func TestRegraQueConfirmaEscritaPerguntaDepoisDoSimDaEscrita(t *testing.T) {
	const pergunta = "Já é tarde, quer mesmo reservar assim?"
	c := novaConversaComRegra(t, "areas.reservar", pergunta)
	params := map[string]any{"area": "salao", "data": "2026-11-15", "unidade": "109"}

	if out := c.falar("quero reservar o salão dia 15", "areas.reservar", params); !strings.Contains(erroDe(out), "exige a confirmação explícita") {
		t.Fatalf("sem sim, o escrita_confirmada segura primeiro: %q", erroDe(out))
	}
	if out := c.falar("sim", "areas.reservar", params); erroDe(out) != pergunta {
		t.Fatalf("depois do sim da escrita, a regra pergunta: %q", erroDe(out))
	}
	if out := c.falar("sim", "areas.reservar", params); erroDe(out) != "" {
		t.Fatalf("o sim à pergunta da regra deixa a escrita passar: %q", erroDe(out))
	}
	if out := c.falar("sim", "areas.reservar", params); erroDe(out) != pergunta {
		t.Fatalf("a pergunta respondida vale uma vez: um novo sim sem pergunta antes volta a perguntar: %q", erroDe(out))
	}
}
