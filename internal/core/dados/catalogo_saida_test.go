package dados

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go-concierge/internal/core/banco/bancotest"
	"go-concierge/internal/core/catalogo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
)

func TestCredenciaisDeSaidaHTTPExigemCapacidadeSeparada(t *testing.T) {
	r, _, _ := registroValido(t)
	repo := &repositorioCanalTeste{canal: canais.Canal{ID: r.ChannelID, CompanyID: r.CompanyID, APIKey: "chave de entrega", WebhookSecret: "segredo de entrada", APIBaseURL: "http://destino"}}
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Canais: repo})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, cliente.base+rotaCanalDeSaida, strings.NewReader(`{"company_id":"empresa","channel_id":"`+r.ChannelID.String()+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+cliente.token)
	res, err := cliente.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("token runtime recebeu credencial: HTTP %d", res.StatusCode)
	}
	canal, err := servidor.CanaisDeSaida().PorID(t.Context(), r.CompanyID, r.ChannelID)
	if err != nil || canal.APIKey != "chave de entrega" || canal.WebhookSecret != "" {
		t.Fatalf("saida=%+v erro=%v", canal, err)
	}
	canal, err = cliente.PorID(t.Context(), r.CompanyID, r.ChannelID)
	if err != nil || canal.APIKey != "" || canal.WebhookSecret != "" || canal.APIBaseURL != "" {
		t.Fatalf("runtime recebeu segredos: %+v erro=%v", canal, err)
	}
}

func TestCatalogoHTTPPreservaProjecaoRollbackEIncerteza(t *testing.T) {
	b := bancotest.Novo(t)
	servidor, cliente, _, err := IniciarConfigurado(Config{Turno: gravadorFunc(nil), Catalogo: catalogo.NovoPostgres(b.Ent)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = servidor.Fechar(context.Background()) })
	pg := servidor.Catalogo()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, cliente.base+rotaCatalogo, strings.NewReader(`{"operacao":"`+uuid.NewString()+`","acao":"listar"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+cliente.token)
	res, err := cliente.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("runtime acessou catalogo: HTTP %d", res.StatusCode)
	}
	arquivo := catalogo.Arquivo{Caminho: "agentes/root/caso.yaml", Conteudo: []byte("id: caso-http\nname: Caso HTTP\nsteps:\n  - compose: teste\noutput: pronto\n")}
	if _, err := pg.Substituir(t.Context(), []catalogo.Arquivo{arquivo}); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := b.Pool.QueryRow(t.Context(), `SELECT count(*) FROM use_cases WHERE id='caso-http'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("projecao=%d erro=%v", n, err)
	}
	invalido := arquivo
	invalido.Conteudo = []byte("steps: []")
	if _, err := pg.Substituir(t.Context(), []catalogo.Arquivo{invalido}); !errors.Is(err, atende.ErrGravacaoNaoAplicada) {
		t.Fatalf("rollback=%v", err)
	}
	lista, err := pg.Listar(t.Context())
	if err != nil || len(lista) != 1 || string(lista[0].Conteudo) != string(arquivo.Conteudo) {
		t.Fatalf("estado apos recusa=%+v erro=%v", lista, err)
	}
	perdida := &perderRespostaAuditoria{destino: pg.http.Transport}
	pg.http.Transport = perdida
	_, err = pg.Substituir(t.Context(), []catalogo.Arquivo{{Caminho: "agentes/root/persona.md", Conteudo: []byte("nova")}})
	pg.http.Transport = perdida.destino
	if !errors.Is(err, atende.ErrGravacaoIncerta) || errors.Is(err, atende.ErrGravacaoNaoAplicada) || perdida.chamadas != 1 {
		t.Fatalf("resultado=%v chamadas=%d", err, perdida.chamadas)
	}
	lista, err = pg.Listar(t.Context())
	if err != nil || len(lista) != 2 {
		t.Fatalf("commit apos resposta perdida: arquivos=%d erro=%v", len(lista), err)
	}
}
