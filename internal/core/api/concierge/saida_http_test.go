package conciergeapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	okf "go-concierge/internal/core/catalogo/arquivos"
	canaiscore "go-concierge/internal/core/clientes/canais"
	"go-concierge/internal/entrada"
	harness "go-concierge/internal/harness/agentes"
	atende "go-concierge/internal/harness/atendimento"
)

type transferidorSaidaTeste struct {
	chamadas atomic.Int32
	err      error
}

func (t *transferidorSaidaTeste) Transferir(context.Context, entrada.Destino, string) error {
	t.chamadas.Add(1)
	return t.err
}

type perderResposta struct {
	base     http.RoundTripper
	chamadas atomic.Int32
}

func (p *perderResposta) RoundTrip(r *http.Request) (*http.Response, error) {
	p.chamadas.Add(1)
	res, err := p.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	_ = res.Body.Close()
	return nil, errors.New("resposta perdida")
}

func destinoSaidaTeste() entrada.Destino {
	return entrada.Destino{Conector: "atende", CompanyID: "empresa", ChannelID: uuid.New(), Protocolo: "protocolo-teste", Telefone: "5511999999999"}
}

func TestSaidaHTTPEntregaIniciadaContinuaAposMudanca(t *testing.T) {
	for _, falhaSegunda := range []bool{false, true} {
		nome := "integral"
		if falhaSegunda {
			nome = "falha na segunda parte"
		}
		t.Run(nome, func(t *testing.T) {
			iniciada, liberar := make(chan struct{}), make(chan struct{})
			var liberarUmaVez sync.Once
			var chamadas atomic.Int32
			provedor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				n := chamadas.Add(1)
				if n == 1 {
					close(iniciada)
					<-liberar
				}
				if n == 2 && falhaSegunda {
					w.WriteHeader(http.StatusBadGateway)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer provedor.Close()
			servidor, cliente, err := iniciarSaidaHTTP(&canaiscore.ATENDESender{BaseURL: provedor.URL, APIKey: "teste"}, &transferidorSaidaTeste{})
			if err != nil {
				t.Fatal(err)
			}
			defer liberarUmaVez.Do(func() { close(liberar) })
			t.Cleanup(func() {
				ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancelar()
				if err := servidor.Fechar(ctx); err != nil {
					t.Error(err)
				}
			})
			var permitido atomic.Bool
			permitido.Store(true)
			ctx, cancelar := context.WithCancel(context.WithValue(t.Context(), entrada.ChaveControleEntrega{}, func() bool { return permitido.Load() }))
			resultado := make(chan error, 1)
			go func() { resultado <- cliente.Enviar(ctx, destinoSaidaTeste(), strings.Repeat("x", 8500)) }()
			select {
			case <-iniciada:
			case <-time.After(3 * time.Second):
				t.Fatal("primeira parte não iniciou")
			}
			permitido.Store(false)
			cancelar()
			liberarUmaVez.Do(func() { close(liberar) })
			err = <-resultado
			if falhaSegunda {
				var parcial *entrada.EntregaParcial
				if !errors.As(err, &parcial) || len(parcial.Partes) != 1 || len(parcial.Partes[0]) != 4000 || chamadas.Load() != 2 {
					t.Fatalf("resultado parcial: erro=%v chamadas=%d", err, chamadas.Load())
				}
			} else if err != nil || chamadas.Load() != 3 {
				t.Fatalf("entrega integral: erro=%v chamadas=%d", err, chamadas.Load())
			}
		})
	}
}

func TestSaidaHTTPPerdaDaRespostaNaoRepete(t *testing.T) {
	var efeitos atomic.Int32
	provedor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { efeitos.Add(1); w.WriteHeader(http.StatusOK) }))
	defer provedor.Close()
	servidor, cliente, err := iniciarSaidaHTTP(&canaiscore.ATENDESender{BaseURL: provedor.URL, APIKey: "teste"}, &transferidorSaidaTeste{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = servidor.Fechar(context.Background()) }()
	perda := &perderResposta{base: cliente.HTTP.Transport}
	cliente.HTTP = &http.Client{Transport: perda}
	err = cliente.Enviar(t.Context(), destinoSaidaTeste(), "resposta")
	var incerta *entrada.EntregaIncerta
	if !errors.As(err, &incerta) || perda.chamadas.Load() != 1 || efeitos.Load() != 1 {
		t.Fatalf("perda: erro=%v requisições=%d efeitos=%d", err, perda.chamadas.Load(), efeitos.Load())
	}
}

func TestSaidaHTTPRecusaTokenInvalido(t *testing.T) {
	var efeitos atomic.Int32
	provedor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { efeitos.Add(1); w.WriteHeader(http.StatusOK) }))
	defer provedor.Close()
	servidor, cliente, err := iniciarSaidaHTTP(&canaiscore.ATENDESender{BaseURL: provedor.URL, APIKey: "teste"}, &transferidorSaidaTeste{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = servidor.Fechar(context.Background()) }()
	cliente.Token = "inválido"
	err = cliente.Enviar(t.Context(), destinoSaidaTeste(), "resposta")
	var incerta *entrada.EntregaIncerta
	if err == nil || errors.As(err, &incerta) || efeitos.Load() != 0 {
		t.Fatalf("token inválido: erro=%v efeitos=%d", err, efeitos.Load())
	}
}

func TestSaidaHTTPSuprimeAntesDoEnvio(t *testing.T) {
	var efeitos atomic.Int32
	provedor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { efeitos.Add(1); w.WriteHeader(http.StatusOK) }))
	defer provedor.Close()
	servidor, cliente, err := iniciarSaidaHTTP(&canaiscore.ATENDESender{BaseURL: provedor.URL, APIKey: "teste"}, &transferidorSaidaTeste{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = servidor.Fechar(context.Background()) }()
	ctx := context.WithValue(t.Context(), entrada.ChaveControleEntrega{}, func() bool { return false })
	if err := cliente.Enviar(ctx, destinoSaidaTeste(), "resposta"); !errors.Is(err, entrada.ErrEntregaSuprimida) || efeitos.Load() != 0 {
		t.Fatalf("supressão: erro=%v efeitos=%d", err, efeitos.Load())
	}
}

func TestSaidaHTTPPropagaTransferenciaIncerta(t *testing.T) {
	transferidor := &transferidorSaidaTeste{err: &entrada.TransferenciaIncerta{Erro: errors.New("retorno perdido")}}
	servidor, cliente, err := iniciarSaidaHTTP(&canaiscore.ATENDESender{}, transferidor)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = servidor.Fechar(context.Background()) }()
	err = cliente.Transferir(t.Context(), destinoSaidaTeste(), "teste")
	var incerta *entrada.TransferenciaIncerta
	if !errors.As(err, &incerta) || transferidor.chamadas.Load() != 1 {
		t.Fatalf("transferência: erro=%v chamadas=%d", err, transferidor.chamadas.Load())
	}
}

func TestSaidaHTTPPreservaParteConfirmadaComResultadoIncerto(t *testing.T) {
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"resultado":"incerta","partes":["parte confirmada"],"erro":"retorno perdido"}`))
	}))
	defer servidor.Close()
	cliente := &atende.ClienteSaida{URL: servidor.URL, Token: "teste", HTTP: servidor.Client()}
	err := cliente.Enviar(t.Context(), destinoSaidaTeste(), "resposta")
	var parcial *entrada.EntregaParcial
	var incerta *entrada.EntregaIncerta
	if !errors.As(err, &parcial) || !errors.As(err, &incerta) || len(parcial.Partes) != 1 || parcial.Partes[0] != "parte confirmada" {
		t.Fatalf("resultado parcial incerto: %v", err)
	}
}

type modeloSaidaTeste struct{ resposta string }

func (m modeloSaidaTeste) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(m.resposta, nil), nil
}
func (m modeloSaidaTeste) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}
func (m modeloSaidaTeste) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestSaidaHTTPRegistraEntregaDuranteEventoHumanoOuEncerramento(t *testing.T) {
	for _, evento := range []string{"humano", "encerramento"} {
		for _, parcial := range []bool{false, true} {
			t.Run(evento+"/"+map[bool]string{false: "integral", true: "parcial"}[parcial], func(t *testing.T) {
				inicio, liberar := make(chan struct{}), make(chan struct{})
				var liberarUmaVez sync.Once
				var chamadas atomic.Int32
				var primeira string
				provedor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					n := chamadas.Add(1)
					if n == 1 {
						var body struct {
							Message struct {
								Text string `json:"text"`
							} `json:"message"`
						}
						_ = json.NewDecoder(r.Body).Decode(&body)
						primeira = body.Message.Text
						close(inicio)
						<-liberar
					}
					if n == 2 && parcial {
						w.WriteHeader(http.StatusBadGateway)
						return
					}
					w.WriteHeader(http.StatusOK)
				}))
				defer provedor.Close()
				transferidor := &transferidorSaidaTeste{}
				servidor, cliente, err := iniciarSaidaHTTP(&canaiscore.ATENDESender{BaseURL: provedor.URL, APIKey: "teste"}, transferidor)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = servidor.Fechar(context.Background()) }()
				defer liberarUmaVez.Do(func() { close(liberar) })
				raiz := t.TempDir()
				pasta := filepath.Join(raiz, "agentes", "raiz", "workflows")
				if err := os.MkdirAll(pasta, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(pasta, "principal.md"), []byte("# Atendimento\nResponda ao morador."), 0o600); err != nil {
					t.Fatal(err)
				}
				cat, err := okf.NewCatalogo(raiz)
				if err != nil {
					t.Fatal(err)
				}
				store := atende.NovaSessaoEmMemoria()
				texto := strings.Repeat("x", 8500) + ".\n\nVou encaminhar para um atendente."
				svc := &atende.Servico{Cat: cat, Agente: "agentes/raiz", Mapeamento: mapeamentoTeste(t), Sessoes: atende.NewSessoes(store), Sender: cliente, Transferidor: cliente, Config: harness.Config{ChatModel: modeloSaidaTeste{texto}, Env: func(string) string { return "" }}}
				msg := entrada.Mensagem{ID: "evento-1", ConversaID: "protocolo-teste", Tenant: "tenant-x", CompanyID: "123", Contato: entrada.Contato{Phone: "5511999999999"}, Texto: "preciso de ajuda", Origem: entrada.Origem{Conector: "atende", ConexaoID: "teste"}, Destino: destinoSaidaTeste()}
				msg, sess, err := svc.PrepararMensagem(t.Context(), msg)
				if err != nil {
					t.Fatal(err)
				}
				resultado := make(chan error, 1)
				go func() { _, err := svc.Processar(context.Background(), msg); resultado <- err }()
				select {
				case <-inicio:
				case <-time.After(5 * time.Second):
					t.Fatal("envio não iniciou")
				}
				mudanca := msg
				mudanca.ID, mudanca.Autor, mudanca.Texto = "evento-2", "atendente", "Assumi a conversa"
				var aplicado bool
				if evento == "humano" {
					aplicado, err = svc.RegistrarHumano(t.Context(), mudanca)
				} else {
					aplicado, err = svc.Encerrar(t.Context(), mudanca)
				}
				if err != nil || !aplicado {
					t.Fatalf("evento: aplicado=%v erro=%v", aplicado, err)
				}
				liberarUmaVez.Do(func() { close(liberar) })
				err = <-resultado
				if parcial && err == nil || !parcial && err != nil {
					t.Fatalf("resultado da entrega: %v (tamanho inicial=%d, texto=%q)", err, len(primeira), primeira[:min(len(primeira), 100)])
				}
				if transferidor.chamadas.Load() != 0 {
					t.Fatal("transferência não deveria ter ocorrido")
				}
				if parcial && chamadas.Load() != 2 || !parcial && chamadas.Load() != 3 {
					t.Fatalf("partes enviadas: %d (tamanho inicial=%d, texto=%q)", chamadas.Load(), len(primeira), primeira[:min(len(primeira), 100)])
				}
				registro, ok, err := store.Carregar(t.Context(), sess.ID)
				if err != nil || !ok {
					t.Fatalf("sessão não persistida: %v", err)
				}
				var gravada struct {
					Historico         []*schema.Message `json:"historico"`
					Encerrada         bool              `json:"encerrada"`
					AtendimentoHumano bool              `json:"atendimento_humano"`
				}
				if err := json.Unmarshal(registro.Dados, &gravada); err != nil {
					t.Fatal(err)
				}
				if evento == "humano" && !gravada.AtendimentoHumano || evento == "encerramento" && !gravada.Encerrada {
					t.Fatal("estado da sessão não foi preservado")
				}
				quer := strings.Repeat("x", 8500) + ".\n\n\n\nVou solicitar que uma pessoa da administração assuma este protocolo. Ainda não tenho confirmação de que ela assumiu a conversa."
				if parcial {
					quer = strings.Repeat("x", 4000)
				}
				encontrado := false
				for _, item := range gravada.Historico {
					if item.Role == schema.Assistant && item.Content == quer {
						encontrado = true
					}
				}
				if !encontrado {
					t.Fatalf("histórico não contém a resposta confirmada (parcial=%v)", parcial)
				}
			})
		}
	}
}
