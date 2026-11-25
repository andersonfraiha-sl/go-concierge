package atendimento

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"go-concierge/internal/entrada"
)

type entradaControlada struct {
	iniciado     chan struct{}
	liberar      chan struct{}
	humano       chan entrada.Mensagem
	prepararErro error
}

func (s *entradaControlada) PermiteTelefone(string) bool { return true }
func (s *entradaControlada) PrepararMensagem(_ context.Context, msg entrada.Mensagem) (entrada.Mensagem, *Sessao, error) {
	if s.prepararErro != nil {
		return msg, nil, s.prepararErro
	}
	msg.Revisao = 7
	return msg, &Sessao{ID: "sessao-1", Protocolo: msg.ConversaID}, nil
}
func (s *entradaControlada) Processar(ctx context.Context, _ entrada.Mensagem) (*Sessao, error) {
	close(s.iniciado)
	select {
	case <-s.liberar:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (s *entradaControlada) RegistrarHumano(_ context.Context, msg entrada.Mensagem) (bool, error) {
	s.humano <- msg
	return true, nil
}
func (s *entradaControlada) Encerrar(_ context.Context, msg entrada.Mensagem) (bool, error) {
	s.humano <- msg
	return true, nil
}

func TestEntradaHTTPAceitaAntesDoFimEEsperaNoDesligamento(t *testing.T) {
	svc := &entradaControlada{iniciado: make(chan struct{}), liberar: make(chan struct{}), humano: make(chan entrada.Mensagem, 1)}
	servidor, cliente, err := IniciarEntradaHTTP(svc)
	if err != nil {
		t.Fatal(err)
	}
	msg := entrada.Mensagem{ConversaID: "protocolo-1", ID: "evento-1"}
	resultado, status, err := cliente.Aplicar(t.Context(), "iniciar", msg, "request-1")
	if err != nil || status != http.StatusAccepted || resultado.Protocolo != msg.ConversaID || resultado.SessaoID != "sessao-1" {
		t.Fatalf("aceite: %+v status=%d erro=%v", resultado, status, err)
	}
	select {
	case <-svc.iniciado:
	case <-time.After(time.Second):
		t.Fatal("turno não iniciou")
	}
	msg.ID = "evento-humano"
	resultado, status, err = cliente.Aplicar(t.Context(), "humano", msg, "")
	if err != nil || status != http.StatusOK || !resultado.Aplicado {
		t.Fatalf("evento humano: %+v status=%d erro=%v", resultado, status, err)
	}
	if evento := <-svc.humano; evento.ID != msg.ID {
		t.Fatalf("evento aplicado: %+v", evento)
	}
	fim := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		fim <- servidor.Fechar(ctx)
	}()
	select {
	case err := <-fim:
		t.Fatalf("encerrou com turno ativo: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(svc.liberar)
	if err := <-fim; err != nil {
		t.Fatal(err)
	}
}

func TestEntradaHTTPRecusaPreparacaoEAutenticacao(t *testing.T) {
	svc := &entradaControlada{prepararErro: errors.New("preparação falhou")}
	servidor, cliente, err := IniciarEntradaHTTP(svc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := servidor.Fechar(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	_, status, err := cliente.Aplicar(t.Context(), "iniciar", entrada.Mensagem{}, "")
	if err == nil || status != http.StatusInternalServerError {
		t.Fatalf("preparação: status=%d erro=%v", status, err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, cliente.url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := cliente.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status sem token: %d", resp.StatusCode)
	}
}
