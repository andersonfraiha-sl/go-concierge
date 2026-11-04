package pedidos

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

// ambienteComPrevia é o ambienteFalso que também prevê a escrita: bloqueia quando
// bloqueiaSe aceita os params recebidos. Conta os bloqueios que o motor aplicou em
// registradas.
type ambienteComPrevia struct {
	*ambienteFalso
	bloqueiaSe  func(params map[string]any) bool
	previas     []map[string]any
	registradas int
}

func (a *ambienteComPrevia) Prever(_ context.Context, _, recurso string, params map[string]any) Previsao {
	if recurso != "areas.reservar" {
		return Previsao{}
	}
	a.previas = append(a.previas, params)
	if !a.bloqueiaSe(params) {
		return Previsao{}
	}
	return Previsao{Bloqueio: "Reservas só até as 22h.", Registrar: func(context.Context) { a.registradas++ }}
}

func novoPedidoDaReserva(t *testing.T) (*Contrato, *Pedido, *Carteira) {
	t.Helper()
	Agora = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.Local) }
	t.Cleanup(func() { Agora = time.Now })
	ct := casoReserva(t)
	cart := &Carteira{AdministradoraID: "adm-a", Telefone: "5511999"}
	return ct, cart.Novo(ct, "reserva-area-comum", "cond-1", Origem{ProtocoloAtende: "ATD-1"}), cart
}

func TestPreviaBloqueiaAntesDaPrimeiraPergunta(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	amb := &ambienteComPrevia{
		ambienteFalso: &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}},
		bloqueiaSe:    func(map[string]any) bool { return true },
	}
	s := Avancar(context.Background(), ct, p, cart, amb)
	if s.Tipo != "cancelado" || s.Texto != "Reservas só até as 22h." || p.Estado != Cancelado {
		t.Fatalf("devia cancelar com o motivo da regra: %+v estado=%s", s, p.Estado)
	}
	if len(amb.chamadas) != 0 {
		t.Fatalf("nenhum recurso devia rodar antes do bloqueio: %v", amb.chamadas)
	}
}

func TestPreviaBloqueiaSoQuandoOParametroDaRegraChega(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	amb := &ambienteComPrevia{
		ambienteFalso: &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}},
		bloqueiaSe:    func(params map[string]any) bool { return params["data"] != nil },
	}
	ctx := context.Background()
	if s := Avancar(ctx, ct, p, cart, amb); s.Tipo != "pergunta" || s.Slot != "area" {
		t.Fatalf("sem data a previsão deixa passar: %+v", s)
	}
	Aplicar(ct, p, map[string]any{"area": "salao"}, OrigemMorador)
	if s := Avancar(ctx, ct, p, cart, amb); s.Tipo != "pergunta" || s.Slot != "data" {
		t.Fatalf("devia perguntar a data: %+v", s)
	}
	Aplicar(ct, p, map[string]any{"data": "15/11"}, OrigemMorador)
	s := Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "cancelado" || p.Estado != Cancelado {
		t.Fatalf("com a data, a previsão bloqueia: %+v estado=%s", s, p.Estado)
	}
	if got := amb.previas[len(amb.previas)-1]["data"]; got != "2026-11-15" {
		t.Fatalf("a previsão recebe os params coletados: %v", amb.previas)
	}
}

func TestPreviaSemBloqueioNaoMudaOFluxo(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	amb := &ambienteComPrevia{
		ambienteFalso: &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}},
		bloqueiaSe:    func(map[string]any) bool { return false },
	}
	ctx := context.Background()
	Avancar(ctx, ct, p, cart, amb)
	Aplicar(ct, p, map[string]any{"area": "salao", "data": "15/11"}, OrigemMorador)
	if s := Avancar(ctx, ct, p, cart, amb); s.Tipo != "confirmar" {
		t.Fatalf("devia pedir a confirmação: %+v", s)
	}
	n := len(amb.previas)
	Confirmar(p, "sim", OrigemMorador)
	s := Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "concluido" || !strings.Contains(s.Texto, "R-77") {
		t.Fatalf("a gravação segue o caminho normal: %+v", s)
	}
	if len(amb.previas) != n {
		t.Fatalf("na vez da gravação não prevê de novo: %d previsões", len(amb.previas))
	}
}

func TestAmbienteSemPreviaSegueIgual(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	amb := &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}}
	if s := Avancar(context.Background(), ct, p, cart, amb); s.Tipo != "pergunta" || s.Slot != "area" {
		t.Fatalf("sem Prever o motor pergunta a área: %+v", s)
	}
}

func TestPreviaIgnoraEscritaComSkipIf(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	pulando := *ct
	pulando.Passos = append([]Passo(nil), ct.Passos...)
	for i, passo := range pulando.Passos {
		if passo.Recurso == "areas.reservar" {
			pulando.Passos[i].SkipIf = "ja_reservado"
		}
	}
	p.Dados = map[string]any{"ja_reservado": true}
	amb := &ambienteComPrevia{
		ambienteFalso: &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}},
		bloqueiaSe:    func(map[string]any) bool { return true },
	}
	if s := Avancar(context.Background(), &pulando, p, cart, amb); s.Tipo != "pergunta" || len(amb.previas) != 0 {
		t.Fatalf("escrita pulada não se prevê nem cancela: %+v previsões=%d", s, len(amb.previas))
	}
}

func TestPreviaQueBloqueiaRegistraUmaVez(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	amb := &ambienteComPrevia{
		ambienteFalso: &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}},
		bloqueiaSe:    func(map[string]any) bool { return true },
	}
	Avancar(context.Background(), ct, p, cart, amb)
	Avancar(context.Background(), ct, p, cart, amb)
	if amb.registradas != 1 {
		t.Fatalf("o bloqueio que cancelou devia ser registrado uma vez: %d", amb.registradas)
	}
}

func TestPreviaSemDecisaoNaoRegistra(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	amb := &ambienteComPrevia{
		ambienteFalso: &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}},
		bloqueiaSe:    func(map[string]any) bool { return false },
	}
	ctx := context.Background()
	Avancar(ctx, ct, p, cart, amb)
	Aplicar(ct, p, map[string]any{"area": "salao"}, OrigemMorador)
	Avancar(ctx, ct, p, cart, amb)
	if len(amb.previas) < 2 || amb.registradas != 0 {
		t.Fatalf("prévia que só deixa passar não registra: %d previsões, %d", len(amb.previas), amb.registradas)
	}
}

const perguntaDaRegra = "Já é tarde, quer mesmo reservar assim?"

// ambienteQuePergunta é o ambienteFalso em que os ganchos pedem confirmação na primeira
// chamada da reserva, como a regra que ainda não teve a pergunta dela respondida.
type ambienteQuePergunta struct {
	*ambienteFalso
	perguntou bool
}

func (a *ambienteQuePergunta) Chamar(ctx context.Context, agente, recurso string, params map[string]any, confirmacao string) (any, error) {
	if recurso == "areas.reservar" && !a.perguntou {
		a.perguntou = true
		a.confirms = append(a.confirms, confirmacao)
		return nil, &PerguntaDosGanchos{Pergunta: perguntaDaRegra}
	}
	return a.ambienteFalso.Chamar(ctx, agente, recurso, params, confirmacao)
}

func novoAmbienteQuePergunta() *ambienteQuePergunta {
	return &ambienteQuePergunta{ambienteFalso: &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}}}
}

// ateAPerguntaDaRegra leva a reserva até o sim do caso, que dispara a pergunta da regra.
func ateAPerguntaDaRegra(t *testing.T, ct *Contrato, p *Pedido, cart *Carteira, amb Ambiente) {
	t.Helper()
	ctx := context.Background()
	Avancar(ctx, ct, p, cart, amb)
	Aplicar(ct, p, map[string]any{"area": "salao", "data": "15/11"}, OrigemMorador)
	if s := Avancar(ctx, ct, p, cart, amb); s.Tipo != "confirmar" || !strings.Contains(s.Texto, "Confirma a reserva") {
		t.Fatalf("primeiro a confirmação do caso: %+v", s)
	}
	Confirmar(p, "sim", OrigemMorador)
	s := Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "confirmar" || s.Texto != perguntaDaRegra || p.Estado != Confirmando || !p.Confirmacao.Ganchos {
		t.Fatalf("a pergunta da regra vira a confirmação em aberto, não falha: %+v estado=%s", s, p.Estado)
	}
}

func TestPerguntaDosGanchosEsperaOMoradorEOSimRefazAEscrita(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	amb := novoAmbienteQuePergunta()
	ateAPerguntaDaRegra(t, ct, p, cart, amb)
	ctx := context.Background()
	if s := Avancar(ctx, ct, p, cart, amb); s.Tipo != "confirmar" || s.Texto != perguntaDaRegra || len(amb.confirms) != 1 {
		t.Fatalf("sem resposta, a pergunta segue esperando e a escrita não roda: %+v %v", s, amb.confirms)
	}
	Confirmar(p, "sim", OrigemMorador)
	s := Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "concluido" || !strings.Contains(s.Texto, "R-77") {
		t.Fatalf("o sim refaz a escrita: %+v", s)
	}
	if len(amb.confirms) != 2 || amb.confirms[1] != "sim" {
		t.Fatalf("duas tentativas, a segunda com o sim: %v", amb.confirms)
	}
}

func TestNaoAPerguntaDosGanchosCancelaComoONaoDoCaso(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	amb := novoAmbienteQuePergunta()
	ateAPerguntaDaRegra(t, ct, p, cart, amb)
	Confirmar(p, "nao", OrigemMorador)
	s := Avancar(context.Background(), ct, p, cart, amb)
	if s.Tipo != "cancelado" || s.Texto != CanceladoAPedido || p.Estado != Cancelado || len(amb.confirms) != 1 {
		t.Fatalf("o não cancela a pedido do morador, sem nova escrita: %+v estado=%s %v", s, p.Estado, amb.confirms)
	}
}

func TestValorNovoDepoisDaPerguntaDosGanchosVoltaAConfirmacaoDoCaso(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	amb := novoAmbienteQuePergunta()
	ateAPerguntaDaRegra(t, ct, p, cart, amb)
	Aplicar(ct, p, map[string]any{"data": "16/11"}, OrigemAtendente) // o atendente pode trocar um valor já dado
	s := Avancar(context.Background(), ct, p, cart, amb)
	if s.Tipo != "confirmar" || !strings.Contains(s.Texto, "16 de novembro") || p.Confirmacao.Ganchos {
		t.Fatalf("com outro valor, a confirmação do caso volta: %+v", s)
	}
}

func TestPerguntaDosGanchosSemPassoDeConfirmacao(t *testing.T) {
	ct, p, cart := novoPedidoDaReserva(t)
	semConfirmar := *ct
	semConfirmar.Passos = slices.DeleteFunc(slices.Clone(ct.Passos), func(passo Passo) bool { return passo.Tipo == "confirmar" })
	amb := novoAmbienteQuePergunta()
	ctx := context.Background()
	Avancar(ctx, &semConfirmar, p, cart, amb)
	Aplicar(&semConfirmar, p, map[string]any{"area": "salao", "data": "15/11"}, OrigemMorador)
	if s := Avancar(ctx, &semConfirmar, p, cart, amb); s.Tipo != "confirmar" || s.Texto != perguntaDaRegra || p.Estado != Confirmando {
		t.Fatalf("sem passo de confirmação, a regra pergunta antes e o pedido espera: %+v estado=%s", s, p.Estado)
	}
	Confirmar(p, "sim", OrigemMorador)
	if s := Avancar(ctx, &semConfirmar, p, cart, amb); s.Tipo != "concluido" || amb.confirms[len(amb.confirms)-1] != "sim" {
		t.Fatalf("o sim refaz a chamada com a confirmação: %+v %v", s, amb.confirms)
	}
}
