package atende

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"go-concierge/internal/escopo"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/ganchostest"
	"go-concierge/internal/ganchos/pii"
	"go-concierge/internal/harness"
	"go-concierge/internal/memoria"
)

const (
	cpfDoMorador     = "529.982.247-25"
	cpfSoDigitos     = "52998224725"
	respostaDoModelo = "Encontrei o cadastro do CPF «cpf:1»."
)

// roteirosDoTurno anota cada evento do turno numa regra global que deixa passar.
type roteirosDoTurno struct {
	entrada    ganchostest.Roteiro[ganchos.MensagemRecebida]
	antes      ganchostest.Roteiro[ganchos.AntesDaFerramenta]
	depois     ganchostest.Roteiro[ganchos.DepoisDaFerramenta]
	resposta   ganchostest.Roteiro[ganchos.AntesDeResponder]
	transbordo ganchostest.Roteiro[ganchos.TransbordoOuEncerramento]
}

func (r *roteirosDoTurno) conjunto() *ganchos.Conjunto {
	return ganchostest.Conjunto(fixos.Todos(), &ganchos.Indice{
		MensagemRecebida:         ganchos.Nivel[ganchos.MensagemRecebida]{Globais: []ganchos.Gancho[ganchos.MensagemRecebida]{ganchostest.Gancho("regra", &r.entrada)}},
		AntesDaFerramenta:        ganchos.Nivel[ganchos.AntesDaFerramenta]{Globais: []ganchos.Gancho[ganchos.AntesDaFerramenta]{ganchostest.Gancho("regra", &r.antes)}},
		DepoisDaFerramenta:       ganchos.Nivel[ganchos.DepoisDaFerramenta]{Globais: []ganchos.Gancho[ganchos.DepoisDaFerramenta]{ganchostest.Gancho("regra", &r.depois)}},
		AntesDeResponder:         ganchos.Nivel[ganchos.AntesDeResponder]{Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{ganchostest.Gancho("regra", &r.resposta)}},
		TransbordoOuEncerramento: ganchos.Nivel[ganchos.TransbordoOuEncerramento]{Globais: []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{ganchostest.Gancho("regra", &r.transbordo)}},
	})
}

// registroDoModelo guarda tudo o que o modelo roteirizado leu, entre as cópias de WithTools.
type registroDoModelo struct {
	mu       sync.Mutex
	vistos   []string
	geracoes int
}

func (r *registroDoModelo) ver(in []*schema.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.geracoes++
	for _, m := range in {
		r.vistos = append(r.vistos, m.Content)
		for _, tc := range m.ToolCalls {
			r.vistos = append(r.vistos, tc.Function.Arguments)
		}
	}
}

func (r *registroDoModelo) leu() (string, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.vistos, "\n"), r.geracoes
}

// modeloRoteirizado é o LLM do turno: roteia para listar-unidades, chama o
// get-company-by-document com o marcador do CPF que leu e responde citando o marcador.
type modeloRoteirizado struct {
	registro *registroDoModelo
	tools    []*schema.ToolInfo
}

var reMarcadorCPF = regexp.MustCompile(`«cpf:\d+»`)

func (m modeloRoteirizado) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.tools = tools
	return m, nil
}

func (m modeloRoteirizado) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	out, err := m.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{out}), nil
}

func (m modeloRoteirizado) Generate(_ context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.registro.ver(in)
	o := model.GetCommonOptions(&model.Options{}, opts...)
	tools := m.tools
	if len(o.Tools) > 0 {
		tools = o.Tools
	}
	ultima := in[len(in)-1]
	switch {
	case in[0].Role == schema.System && strings.HasPrefix(in[0].Content, "# Roteador"):
		return schema.AssistantMessage("listar-unidades", nil), nil
	case ultima.Role == schema.User && temChamarRecurso(tools):
		args, _ := json.Marshal(map[string]any{"nome": ferramentaEmpresa, "params": map[string]any{"document": reMarcadorCPF.FindString(ultima.Content)}})
		return schema.AssistantMessage("", []schema.ToolCall{{ID: "consulta", Type: "function", Function: schema.FunctionCall{Name: "chamar_recurso", Arguments: string(args)}}}), nil
	}
	return schema.AssistantMessage(respostaDoModelo, nil), nil
}

func temChamarRecurso(tools []*schema.ToolInfo) bool {
	return slices.ContainsFunc(tools, func(t *schema.ToolInfo) bool { return t.Name == "chamar_recurso" })
}

// extratorQueCopia devolve a mensagem como fato (descartado: tem marcador) e um fato limpo.
type extratorQueCopia struct{}

func (extratorQueCopia) Extrair(_ context.Context, mensagem, _ string, _ memoria.Memoria) ([]memoria.Fato, error) {
	return []memoria.Fato{{Chave: "documento", Texto: mensagem}, {Chave: "pet", Texto: "tem um cachorro"}}, nil
}

// servicoComLLM é o concierge sem o registro de pedidos, com o modelo roteirizado.
func servicoComLLM(t *testing.T, conjunto *ganchos.Conjunto) (*Servico, *Outbox, *mcpFluxoFalso, *registroDoModelo) {
	t.Helper()
	svc, out, _ := servicoComPedidos(t)
	fake := &mcpFluxoFalso{falhas: map[string]error{}, pacotesStatus: map[string][]any{}}
	registro := &registroDoModelo{}
	svc.Pedidos = nil
	svc.Memorias, svc.Extrator = memoria.NovoFake(), extratorQueCopia{}
	svc.Ganchos = conjunto
	svc.Config.MCP = fake
	svc.Config.ChatModel = modeloRoteirizado{registro: registro}
	svc.Config.Env = func(k string) string {
		if k == "ORQUESTRACAO_MODO" {
			return harness.ModoRouter
		}
		return ""
	}
	return svc, out, fake, registro
}

// documentoConsultado é o document que chegou ao executor do get-company-by-document. O
// harness serve essa tool do cache da sessão quando o morador já se identificou, então o
// MCP falso não a vê de novo: vale o params que antes_da_ferramenta liberou.
func documentoConsultado(eventos []ganchos.AntesDaFerramenta) any {
	for _, e := range eventos {
		if e.Ferramenta == ferramentaEmpresa {
			return e.Params["document"]
		}
	}
	return nil
}

func textoEnviado(t *testing.T, out *Outbox, sess *Sessao) string {
	t.Helper()
	itens := out.Itens(sess.ID)
	if len(itens) == 0 {
		t.Fatalf("nada enviado ao morador: %+v", sess.Turnos)
	}
	return itens[len(itens)-1].Texto
}

func TestGanchosVeemOTurnoInteiro(t *testing.T) {
	tests := []struct {
		nome  string
		falar func(t *testing.T, conjunto *ganchos.Conjunto) *Sessao
	}{
		{"motor do caso", func(t *testing.T, conjunto *ganchos.Conjunto) *Sessao {
			fixarAgoraDoFluxo(t)
			svc, _ := servicoComMCPFluxo(t)
			svc.ExtratorPedidos, svc.Ganchos = extratorVisitaMCP(), conjunto
			return falar(t, svc, "ganchos-caso", "quero liberar Ana Souza hoje")
		}},
		{"LLM", func(t *testing.T, conjunto *ganchos.Conjunto) *Sessao {
			svc, _, _, _ := servicoComLLM(t, conjunto)
			sess := falar(t, svc, "ganchos-llm", "meu cpf é "+cpfDoMorador)
			svc.Aguardar()
			return sess
		}},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			r := &roteirosDoTurno{}
			sess := tt.falar(t, r.conjunto())
			if n := len(r.entrada.Eventos()); n != 1 {
				t.Fatalf("mensagem_recebida: %d eventos", n)
			}
			if len(r.antes.Eventos()) == 0 || len(r.depois.Eventos()) == 0 {
				t.Fatalf("ferramenta sem ganchos: antes=%d depois=%d", len(r.antes.Eventos()), len(r.depois.Eventos()))
			}
			if n := len(r.resposta.Eventos()); n != 1 {
				t.Fatalf("antes_de_responder: %d eventos", n)
			}
			for _, e := range r.antes.Eventos() {
				if e.CondominioID != "cond-001" || e.AdministradoraID != "adm-a" {
					t.Fatalf("antes_da_ferramenta sem o alvo: %q/%q", e.AdministradoraID, e.CondominioID)
				}
			}
			for _, e := range r.depois.Eventos() {
				if e.CondominioID != "cond-001" || e.AdministradoraID != "adm-a" {
					t.Fatalf("depois_da_ferramenta sem o alvo: %q/%q", e.AdministradoraID, e.CondominioID)
				}
			}
			if len(sess.registrosDoTurno()) == 0 {
				t.Fatal("registros do turno vazios")
			}
		})
	}
}

func TestGanchosNoTransbordo(t *testing.T) {
	r := &roteirosDoTurno{}
	svc, _, _ := servicoMultiAdministradora(t, escopo.Implantacao{})
	svc.Ganchos = r.conjunto()
	sess, err := svc.Processar(context.Background(), mensagemDe(t, "P-X", "tenant-x", "999", "meu cpf é "+cpfDoMorador))
	if err != nil {
		t.Fatal(err)
	}
	exigirTransbordoSemConcierge(t, sess)
	eventos := r.transbordo.Eventos()
	if len(eventos) != 1 || eventos[0].Motivo != fixos.MotivoCondominioNaoIdentificado || eventos[0].Encerramento {
		t.Fatalf("transbordo_ou_encerramento: %+v", eventos)
	}
	if _, err := svc.Encerrar(context.Background(), mensagemDe(t, "P-X", "tenant-x", "999", "fim")); err != nil {
		t.Fatal(err)
	}
	if eventos := r.transbordo.Eventos(); len(eventos) != 2 || !eventos[1].Encerramento {
		t.Fatalf("encerramento sem gancho: %+v", eventos)
	}
}

func TestGanchosMascaramOCPFNoTurnoDoLLM(t *testing.T) {
	var log bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(pii.NovoHandler(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))))
	t.Cleanup(func() { slog.SetDefault(anterior) })
	r := &roteirosDoTurno{}
	svc, out, _, modelo := servicoComLLM(t, r.conjunto())

	sess := falar(t, svc, "ganchos-cpf", "meu cpf é "+cpfDoMorador+", pode consultar?")
	svc.Aguardar()

	if documento := documentoConsultado(r.antes.Eventos()); documento != cpfDoMorador {
		t.Fatalf("get-company-by-document devia receber o CPF real, veio %v", documento)
	}
	if enviado := textoEnviado(t, out, sess); !strings.Contains(enviado, cpfDoMorador) {
		t.Fatalf("a resposta ao morador devia trazer o CPF do cofre: %q", enviado)
	}
	lido, _ := modelo.leu()
	turnos, _ := json.Marshal(sess.Turnos)
	var historico []string
	for _, m := range sess.historicoSnapshot() {
		historico = append(historico, m.Content)
	}
	mem, err := svc.Memorias.Ler(context.Background(), sess.contato())
	if err != nil {
		t.Fatal(err)
	}
	memJSON, _ := json.Marshal(mem)
	lugares := map[string]string{
		"modelo": lido, "turnos": string(turnos), "histórico": strings.Join(historico, "\n"),
		"memória": string(memJSON), "log": log.String(),
	}
	for lugar, texto := range lugares {
		if strings.Contains(texto, cpfDoMorador) || strings.Contains(texto, cpfSoDigitos) {
			t.Errorf("%s tem o CPF: %s", lugar, texto)
		}
	}
	if !strings.Contains(lido, "«cpf:1»") || !strings.Contains(string(turnos), "«cpf:1»") {
		t.Fatalf("o modelo e os turnos deviam ver o marcador: modelo=%q", lido)
	}
	if len(mem.Fatos) != 1 || mem.Fatos[0].Chave != "pet" {
		t.Fatalf("fato com marcador devia ser descartado: %+v", mem.Fatos)
	}
}

func TestRegraQueBloqueiaAEntradaRespondeEPara(t *testing.T) {
	const aviso = "Não atendemos por aqui depois das 22h."
	conjunto := ganchostest.Conjunto(fixos.Todos(), &ganchos.Indice{
		MensagemRecebida: ganchos.Nivel[ganchos.MensagemRecebida]{Globais: []ganchos.Gancho[ganchos.MensagemRecebida]{
			ganchostest.Gancho("regra", &ganchostest.Roteiro[ganchos.MensagemRecebida]{Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: aviso}}),
		}},
	})
	svc, out, fake, modelo := servicoComLLM(t, conjunto)

	sess := falar(t, svc, "ganchos-regra", "meu cpf é "+cpfDoMorador)

	if enviado := textoEnviado(t, out, sess); enviado != aviso {
		t.Fatalf("enviado = %q, want a mensagem da regra", enviado)
	}
	if ultimoConcierge(sess).Texto != aviso {
		t.Fatalf("turno do concierge = %+v", ultimoConcierge(sess))
	}
	if _, geracoes := modelo.leu(); geracoes != 0 || len(fake.chamadas) != 0 {
		t.Fatalf("o agente não devia rodar: %d gerações, chamadas %+v", geracoes, fake.chamadas)
	}
	if sess.Transbordo != nil {
		t.Fatalf("bloqueio de regra não transborda: %+v", sess.Transbordo)
	}
}

func TestRegraQueBloqueiaARespostaTrocaOTexto(t *testing.T) {
	const troca = "Resposta trocada pela regra."
	conjunto := ganchostest.Conjunto(fixos.Todos(), &ganchos.Indice{
		AntesDeResponder: ganchos.Nivel[ganchos.AntesDeResponder]{Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{
			ganchostest.Gancho("regra", &ganchostest.Roteiro[ganchos.AntesDeResponder]{Decisao: ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: troca}}),
		}},
	})
	svc, out, _, _ := servicoComLLM(t, conjunto)

	sess := falar(t, svc, "ganchos-resposta", "meu cpf é "+cpfDoMorador)
	svc.Aguardar()

	if enviado := textoEnviado(t, out, sess); enviado != troca {
		t.Fatalf("enviado = %q, want o texto da regra", enviado)
	}
	if ultimoConcierge(sess).Texto != troca {
		t.Fatalf("turno do concierge = %q", ultimoConcierge(sess).Texto)
	}
}

// panico é um gancho que quebra. Fixo usa a política fechada (padrão); regra, a aberta
// que o compilador dá a ela nesses eventos (spec §11).
func panico[E ganchos.Ocorrencia](nome string, falha ganchos.Politica) ganchos.Gancho[E] {
	return ganchos.Gancho[E]{Nome: nome, Falha: falha, Avaliar: func(E) (E, ganchos.Decisao) { panic("quebrou") }}
}

func comFixoExtra(mudar func(*ganchos.Fixos)) *ganchos.Conjunto {
	f := fixos.Todos()
	mudar(&f)
	return ganchos.NovoConjunto(f, nil)
}

func TestPoliticaDeFalhaNoTurno(t *testing.T) {
	tests := []struct {
		nome       string
		conjunto   *ganchos.Conjunto
		tenant     string
		transbordo string // motivo esperado; vazio = sem transbordo
		enviado    string // texto exato ao morador; vazio = só confere que respondeu
	}{
		{
			nome: "fixo em pânico em mensagem_recebida transborda",
			conjunto: comFixoExtra(func(f *ganchos.Fixos) {
				f.MensagemRecebida.Checagem = append(f.MensagemRecebida.Checagem, panico[ganchos.MensagemRecebida]("fixo_quebrado", ganchos.Fechada))
			}),
			tenant: tenantExemplo, transbordo: MotivoFalhaDeGancho,
		},
		{
			nome: "fixo em pânico em antes_de_responder envia a frase de falha",
			conjunto: comFixoExtra(func(f *ganchos.Fixos) {
				f.AntesDeResponder.Checagem = append(f.AntesDeResponder.Checagem, panico[ganchos.AntesDeResponder]("fixo_quebrado", ganchos.Fechada))
			}),
			tenant: tenantExemplo, enviado: FraseFalha,
		},
		{
			nome: "fixo em pânico no transbordo deixa o transbordo seguir",
			conjunto: comFixoExtra(func(f *ganchos.Fixos) {
				f.TransbordoOuEncerramento.Saneamento = []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{panico[ganchos.TransbordoOuEncerramento](ganchos.GanchoTransbordoSemPII, ganchos.Aberta)}
			}),
			tenant: "tenant-x", transbordo: ganchos.MotivoTransbordoSolicitado,
		},
		{
			nome: "regra em pânico nos três eventos deixa passar",
			conjunto: ganchostest.Conjunto(fixos.Todos(), &ganchos.Indice{
				MensagemRecebida:         ganchos.Nivel[ganchos.MensagemRecebida]{Globais: []ganchos.Gancho[ganchos.MensagemRecebida]{panico[ganchos.MensagemRecebida]("regra", ganchos.Aberta)}},
				AntesDeResponder:         ganchos.Nivel[ganchos.AntesDeResponder]{Globais: []ganchos.Gancho[ganchos.AntesDeResponder]{panico[ganchos.AntesDeResponder]("regra", ganchos.Aberta)}},
				TransbordoOuEncerramento: ganchos.Nivel[ganchos.TransbordoOuEncerramento]{Globais: []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{panico[ganchos.TransbordoOuEncerramento]("regra", ganchos.Aberta)}},
			}),
			tenant: tenantExemplo,
		},
		{
			nome: "regra em pânico no transbordo deixa o motivo saneado",
			conjunto: ganchostest.Conjunto(fixos.Todos(), &ganchos.Indice{
				TransbordoOuEncerramento: ganchos.Nivel[ganchos.TransbordoOuEncerramento]{Globais: []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{panico[ganchos.TransbordoOuEncerramento]("regra", ganchos.Aberta)}},
			}),
			tenant: "tenant-x", transbordo: fixos.MotivoCondominioNaoIdentificado,
		},
	}
	for _, tt := range tests {
		t.Run(tt.nome, func(t *testing.T) {
			svc, _, _ := servicoMultiAdministradora(t, escopo.Implantacao{})
			out := &Outbox{}
			svc.Sender, svc.Ganchos = out, tt.conjunto
			sess, err := svc.Processar(context.Background(), mensagemDe(t, "P-F", tt.tenant, "", "oi"))
			if err != nil {
				t.Fatal(err)
			}
			if tt.transbordo != "" {
				exigirTransbordoSemConcierge(t, sess)
				if sess.Transbordo.Motivo != tt.transbordo {
					t.Fatalf("motivo = %q, want %q", sess.Transbordo.Motivo, tt.transbordo)
				}
				return
			}
			if sess.Transbordo != nil {
				t.Fatalf("não devia transbordar: %+v", sess.Transbordo)
			}
			enviado := textoEnviado(t, out, sess)
			if tt.enviado != "" && enviado != tt.enviado {
				t.Fatalf("enviado = %q, want %q", enviado, tt.enviado)
			}
		})
	}
}
