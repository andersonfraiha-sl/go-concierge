package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/metric/noop"

	"go-concierge/internal/admin"
	"go-concierge/internal/atende"
	"go-concierge/internal/entrada"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/mcp"
	"go-concierge/internal/okf"
	"go-concierge/internal/pedidos"
)

// Regras de exemplo ponta a ponta: as quatro de internal/ganchos/testdata/regras/validas
// entram pela API, no servidor inteiro (mux, montarGanchos, Postgres), e o turno do
// concierge as obedece sem reiniciar. Só o LLM, o MCP do Gruvi e o Sender são falsos.

const (
	telefoneCadastrado   = "5519981621916"
	telefoneDesconhecido = "5511900000000"
	ferramentaCodigo     = "gruvi.bff.building-operation.get-package-delivery-code-controller-handle"
	ferramentaEmpresa    = "gruvi.bff.building-operation.get-company-by-document-controller-handle"
	ferramentaPacotes    = "gruvi.bff.building-operation.fetch-unit-packages-controller-handle"
	administradoraE2E    = "adm-aurora"
	condominioE2E        = "cond-flores"
)

// relogioE2E é o relógio injetado no Servico e no harness; lido por goroutines do turno.
type relogioE2E struct{ nanos atomic.Int64 }

func (r *relogioE2E) Agora() time.Time { return time.Unix(0, r.nanos.Load()).UTC() }

func (r *relogioE2E) Marcar(t *testing.T, rfc3339 string) {
	t.Helper()
	instante, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		t.Fatal(err)
	}
	r.nanos.Store(instante.UnixNano())
}

// gruviFalso é o MCP do Gruvi: o vínculo do telefone cadastrado e uma encomenda na portaria.
type gruviFalso struct {
	mu       sync.Mutex
	chamadas []string
}

func (g *gruviFalso) ValidarToken(context.Context) error { return nil }

func (g *gruviFalso) CallTool(_ context.Context, nome string, _ map[string]any) (*mcp.CallResult, error) {
	g.mu.Lock()
	g.chamadas = append(g.chamadas, nome)
	g.mu.Unlock()
	return textoMCP(respostasDoGruvi[nome]), nil
}

func (g *gruviFalso) chamou(nome string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Contains(g.chamadas, nome)
}

var respostasDoGruvi = map[string]string{
	ferramentaEmpresa: `{"data":{"units":[{"id":"unit-109","id_fastpass":"109","label":"A - 101","unit":"A - 101",` +
		`"resident":{"name":"Vanderlei Martins","mobile_number":"` + telefoneCadastrado + `"}}]}}`,
	ferramentaPacotes: `{"data":[{"id":42,"description":"Encomenda teste"}]}`,
	ferramentaCodigo:  `{"data":{"delivery_code":"4831"}}`,
}

func textoMCP(texto string) *mcp.CallResult {
	if texto == "" {
		texto = "{}"
	}
	return &mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: texto}}}
}

// comToolsDoGruvi soma ao catálogo as tools MCP do Gruvi que o caso do código de retirada usa.
func comToolsDoGruvi(cat *okf.Catalogo) {
	original := cat.Recursos
	cat.Recursos = func() []okf.Recurso {
		out := original()
		for _, nome := range []string{ferramentaEmpresa, ferramentaPacotes, ferramentaCodigo} {
			out = append(out, okf.Recurso{Name: nome, Kind: okf.KindAPI, Visibility: "public", Origem: okf.OrigemMCP, Params: []okf.ParamMeta{
				{Name: "companyId", Type: "string", Required: true},
				{Name: "filter_mobile_number", Type: "string", Required: true},
			}})
		}
		return out
	}
}

type ambienteE2E struct {
	ambienteGanchos
	svc *atende.Servico
	// svcSemVinculo atende qualquer telefone, sem identidade comprovada
	svcSemVinculo *atende.Servico
	gruvi         *gruviFalso
	relogio       *relogioE2E
	regras        map[string]regraVisao // pelo nome do arquivo
}

// novoAmbienteE2E sobe o servidor inteiro com o Postgres de verdade e o turno do concierge
// ligado ao mesmo Conjunto, às mesmas regras e ao mesmo banco da API.
func novoAmbienteE2E(t *testing.T) *ambienteE2E {
	t.Helper()
	amb := novoAmbienteGanchos(t, "", segredoGanchos)
	semearAdministradoraE2E(t, amb.repo)
	conjunto, auditor, err := amb.s.montarGanchos(t.Context(), noop.NewMeterProvider().Meter("ganchos"))
	if err != nil {
		t.Fatal(err)
	}
	relogio, gruviMCP := &relogioE2E{}, &gruviFalso{}
	cat := amb.s.cat
	comToolsDoGruvi(cat)
	servico := func(telefoneB2C string) *atende.Servico {
		return &atende.Servico{
			Cat: cat, Agente: "agentes/concierge", Mapeamento: amb.repo, Sender: &atende.Outbox{},
			Sessoes:     atende.NewSessoes(atende.NovoSessaoPostgres(amb.banco)),
			Pedidos:     pedidos.NovoPostgres(amb.banco),
			TelefoneB2C: telefoneB2C, Gruvi: &gruvi.Client{CompanyID: "123"},
			Config:  harness.Config{Mock: true, Env: func(string) string { return "" }, MCP: gruviMCP, Ganchos: conjunto, Agora: relogio.Agora},
			Ganchos: conjunto, Auditoria: auditor, Agora: relogio.Agora,
		}
	}
	// o token B2C só atende o telefone dele: o contato sem vínculo entra pelo serviço sem token
	amb.s.atende = servico(telefoneCadastrado)
	usuarios := usuariosTeste()
	usuarios["gestor-aurora"] = naAdministradora("gestor_administradora", administradoraE2E)
	amb.s.acessos = acessosFake{atribuicoes: usuarios, mapeamento: amb.repo}
	amb.handler = amb.s.rotear(nil)
	return &ambienteE2E{ambienteGanchos: amb, svc: amb.s.atende, svcSemVinculo: servico(""), gruvi: gruviMCP, relogio: relogio, regras: map[string]regraVisao{}}
}

func semearAdministradoraE2E(t *testing.T, repo *admin.EmMemoria) {
	t.Helper()
	if _, err := repo.SalvarAdministradora(t.Context(), admin.Administradora{ID: administradoraE2E, Nome: "Aurora"}); err != nil {
		t.Fatal(err)
	}
	flores := admin.Condominio{ID: condominioE2E, Nome: "Flores", Administradora: administradoraE2E, Account: "cond-001", CompanyID: "123", Tenant: "tenant-x"}
	if _, err := repo.SalvarCondominio(t.Context(), flores); err != nil {
		t.Fatal(err)
	}
}

func arquivoDeRegra(t *testing.T, nome string) string {
	t.Helper()
	corpo, err := os.ReadFile(filepath.Join("..", "..", "internal", "ganchos", "testdata", "regras", "validas", nome))
	if err != nil {
		t.Fatal(err)
	}
	return string(corpo)
}

// gravar cria a regra pela API e guarda a visão devolvida.
func (a *ambienteE2E) gravar(t *testing.T, arquivo string) regraVisao {
	t.Helper()
	regra := a.criar(t, arquivoDeRegra(t, arquivo))
	a.regras[arquivo] = regra
	return regra
}

// falar manda uma mensagem do morador no protocolo e devolve a resposta enviada.
func (a *ambienteE2E) falar(t *testing.T, protocolo, telefone, texto string) string {
	t.Helper()
	msg := mensagemDoMoradorE2E(t, protocolo, telefone, texto, fmt.Sprintf("evt-%s-%d", protocolo, time.Now().UnixNano()))
	svc := a.svc
	if telefone == telefoneDesconhecido {
		svc = a.svcSemVinculo
	}
	sess, err := svc.Processar(t.Context(), msg)
	if err != nil {
		t.Fatalf("%q: %v", texto, err)
	}
	svc.Aguardar()
	itens := svc.Sender.(*atende.Outbox).Itens(sess.ID)
	if len(itens) == 0 {
		t.Fatalf("%q: nada enviado ao morador", texto)
	}
	return itens[len(itens)-1].Texto
}

func mensagemDoMoradorE2E(t *testing.T, protocolo, telefone, texto, id string) entrada.Mensagem {
	t.Helper()
	corpo, err := json.Marshal(map[string]any{
		"event": "com.echo.atende.user.protocol.created", "id": id, "tenant": "tenant-x",
		"data": map[string]any{
			"protocol": map[string]any{"number": protocolo},
			"contact":  map[string]any{"phone": telefone, "name": "Morador"},
			"channel":  map[string]any{"type": "whatsapp", "wabaId": "waba-1", "displayNumber": "551940402710"},
			"message":  map[string]any{"id": "msg-" + id, "type": "text", "content": texto},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := atende.Parse(corpo)
	if err != nil {
		t.Fatal(err)
	}
	msg := ev.Normalizar()
	msg.Tenant, msg.CompanyID = "tenant-x", "123"
	msg.Origem = entrada.Origem{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console"}
	msg.Destino = entrada.Destino{Conector: "atende-emulado", Meio: "whatsapp", ConexaoID: "console", Protocolo: msg.ConversaID, Telefone: msg.Contato.Phone, Canal: msg.Canal}
	return msg
}

// linhasDoHistorico lê o histórico pela API, como o usuário, num intervalo fixo.
func (a *ambienteE2E) linhasDoHistorico(t *testing.T, usuario string) []linhaDoHistorico {
	t.Helper()
	status, pagina := a.historico(t, usuario, "desde=2026-11-01T00:00:00Z&ate=2027-01-01T00:00:00Z")
	if status != http.StatusOK {
		t.Fatalf("histórico de %s = %d", usuario, status)
	}
	return pagina.Linhas
}

func linhasDaRegra(linhas []linhaDoHistorico, regra regraVisao) []linhaDoHistorico {
	return slices.DeleteFunc(slices.Clone(linhas), func(l linhaDoHistorico) bool { return l.RegraID != regra.ID })
}

// quer confere que o texto traz o trecho.
func quer(t *testing.T, contexto, texto, trecho string) {
	t.Helper()
	if !strings.Contains(texto, trecho) {
		t.Fatalf("%s: %q não traz %q", contexto, texto, trecho)
	}
}

func fixarDatasDosPedidos(t *testing.T) {
	t.Helper()
	original := pedidos.Agora
	pedidos.Agora = func() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.Local) }
	t.Cleanup(func() { pedidos.Agora = original })
}

func TestE2EGanchosRegrasDeExemplo(t *testing.T) {
	fixarDatasDosPedidos(t)
	amb := novoAmbienteE2E(t)
	reserva := amb.gravar(t, "reserva-so-ate-22h.json")
	obra := amb.gravar(t, "obra-so-comeca-em-dia-util.json")
	codigo := amb.gravar(t, "codigo-de-retirada-so-cadastrado.json")
	confirmar := amb.gravar(t, "confirmar-escrita-fora-do-horario.json")

	t.Run("reserva às 22h30 é bloqueada antes da primeira pergunta", func(t *testing.T) {
		amb.relogio.Marcar(t, "2026-11-04T01:30:00Z") // 22h30 em São Paulo
		texto := amb.falar(t, "prot-reserva", telefoneCadastrado, "quero reservar uma área comum")
		quer(t, "reserva 22h30", texto, "Reservas por aqui só das 6h às 22h")
		if strings.Contains(texto, "Qual área") {
			t.Fatalf("a pergunta da área não devia sair: %q", texto)
		}
	})

	t.Run("obra com início num sábado é bloqueada logo depois da data", func(t *testing.T) {
		amb.relogio.Marcar(t, "2026-11-03T15:00:00Z") // terça, 12h em São Paulo
		fala := func(texto string) string { return amb.falar(t, "prot-obra", telefoneCadastrado, texto) }
		quer(t, "obra: abertura", fala("vou fazer uma reforma no apartamento"), "tipo de obra")
		quer(t, "obra: tipo", fala("pintura"), "Descreva")
		quer(t, "obra: descrição", fala("pintura da sala"), "obra começa")
		texto := fala("07/11") // sábado
		quer(t, "obra: sábado", texto, "Obra só pode começar em dia útil")
	})

	t.Run("código de retirada para contato não cadastrado é bloqueado sem chamar o MCP", func(t *testing.T) {
		texto := amb.falar(t, "prot-codigo", telefoneDesconhecido, "qual o código de retirada da minha encomenda?")
		quer(t, "código de retirada", texto, "Só passo o código de retirada para telefone cadastrado")
		if amb.gruvi.chamou(ferramentaCodigo) {
			t.Fatal("o MCP do Gruvi foi chamado para o código de retirada")
		}
	})

	t.Run("regra de transbordo é recusada com 422", func(t *testing.T) {
		w := amb.pedir(t, "admin", http.MethodPost, "/api/ganchos/regras", arquivoInvalida(t, "AJUSTE_NAO_PERMITIDO__aviso-no-transbordo.json"))
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, quer 422: %s", w.Code, w.Body)
		}
	})

	t.Run("desativar a reserva libera o turno seguinte e a pergunta da regra vai com a confirmação", func(t *testing.T) {
		desativar(t, amb, reserva)
		amb.relogio.Marcar(t, "2026-11-04T01:30:00Z")
		fala := func(texto string) string { return amb.falar(t, "prot-confirmar", telefoneCadastrado, texto) }
		quer(t, "reserva liberada", fala("quero reservar uma área comum"), "Qual área")
		quer(t, "área", fala("salão"), "Para qual data?")
		confirmacao := fala("15/11")
		quer(t, "confirmação do caso", confirmacao, "Confirma a reserva")
		quer(t, "pergunta da regra", confirmacao, "Já é tarde, quer mesmo reservar assim?")
	})

	t.Run("responder sim à pergunta da regra deixa a escrita passar", func(t *testing.T) {
		texto := amb.falar(t, "prot-confirmar", telefoneCadastrado, "sim")
		quer(t, "sim", texto, "Reserva confirmada")
		if strings.Contains(texto, "Não consegui concluir") {
			t.Fatalf("o sim não pode virar falha: %q", texto)
		}
	})

	t.Run("histórico traz a regra, a versão, o condomínio e a hora de cada decisão", func(t *testing.T) {
		linhas := amb.linhasDoHistorico(t, "gestor-aurora")
		for nome, regra := range map[string]regraVisao{"código": codigo, "confirmar": confirmar} {
			achadas := linhasDaRegra(linhas, regra)
			if len(achadas) == 0 {
				t.Fatalf("%s: sem linha no histórico: %+v", nome, linhas)
			}
			l := achadas[0]
			if l.RegraVersao != regra.Versao || l.CondominioID != condominioE2E || l.CriadoEm.IsZero() {
				t.Errorf("%s: linha %+v, quer versão %d e condomínio %s", nome, l, regra.Versao, condominioE2E)
			}
		}
	})

	t.Run("histórico traz uma linha por prévia que cancelou o pedido", func(t *testing.T) {
		linhas := amb.linhasDoHistorico(t, "gestor-aurora")
		for nome, regra := range map[string]regraVisao{"reserva": reserva, "obra": obra} {
			achadas := linhasDaRegra(linhas, regra)
			if len(achadas) != 1 {
				t.Fatalf("%s: %d linhas, quer 1 (só a prévia que cancelou): %+v", nome, len(achadas), achadas)
			}
			l := achadas[0]
			if l.Decisao != "bloquear" || l.RegraVersao != regra.Versao || l.CondominioID != condominioE2E || l.CriadoEm.IsZero() {
				t.Errorf("%s: linha %+v, quer bloquear na versão %d e condomínio %s", nome, l, regra.Versao, condominioE2E)
			}
		}
		if n := len(linhasDaRegra(linhas, confirmar)); n != 1 {
			t.Errorf("confirmar: %d linhas, quer 1 (a pergunta que foi ao morador)", n)
		}
	})

	t.Run("gestor de outra administradora não vê as linhas", func(t *testing.T) {
		if linhas := amb.linhasDoHistorico(t, "gestor-b"); len(linhas) != 0 {
			t.Fatalf("gestor de B viu %d linhas de A", len(linhas))
		}
	})

	t.Run("gestor da administradora recebe 403 nas rotas de edição", func(t *testing.T) {
		alterar := arquivoDeRegra(t, "reserva-so-ate-22h.json")
		for _, rota := range []struct{ metodo, url string }{
			{http.MethodPost, "/api/ganchos/regras"},
			{http.MethodPut, "/api/ganchos/regras/" + reserva.ID},
		} {
			if w := amb.pedir(t, "gestor-aurora", rota.metodo, rota.url, alterar); w.Code != http.StatusForbidden {
				t.Errorf("%s %s = %d, quer 403", rota.metodo, rota.url, w.Code)
			}
		}
	})
}

func arquivoInvalida(t *testing.T, nome string) string {
	t.Helper()
	corpo, err := os.ReadFile(filepath.Join("..", "..", "internal", "ganchos", "testdata", "regras", "invalidas", nome))
	if err != nil {
		t.Fatal(err)
	}
	return string(corpo)
}

// desativar reenvia a regra da API com ativa=false na versão que ela tem.
func desativar(t *testing.T, amb *ambienteE2E, regra regraVisao) {
	t.Helper()
	var definicao map[string]any
	if err := json.Unmarshal(regra.Definicao, &definicao); err != nil {
		t.Fatal(err)
	}
	definicao["ativa"], definicao["versao"] = false, regra.Versao
	corpo, err := json.Marshal(definicao)
	if err != nil {
		t.Fatal(err)
	}
	if w := amb.alterar(t, regra.ID, string(corpo)); w.Code != http.StatusOK {
		t.Fatalf("desativar = %d %s", w.Code, w.Body)
	}
}
