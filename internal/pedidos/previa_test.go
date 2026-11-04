package pedidos

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ambienteComPrevia é o ambienteFalso que também prevê a escrita: bloqueia quando
// bloqueiaSe aceita os params recebidos, senão faz a pergunta (vazia = nenhuma). Conta as
// decisões que o motor aplicou em registradas.
type ambienteComPrevia struct {
	*ambienteFalso
	bloqueiaSe  func(params map[string]any) bool
	pergunta    string
	previas     []map[string]any
	registradas []string
}

func (a *ambienteComPrevia) Prever(_ context.Context, _, recurso string, params map[string]any) Previsao {
	if recurso != "areas.reservar" {
		return Previsao{}
	}
	a.previas = append(a.previas, params)
	if a.bloqueiaSe(params) {
		return a.previsao(Previsao{Bloqueio: "Reservas só até as 22h."})
	}
	if a.pergunta != "" {
		return a.previsao(Previsao{Pergunta: a.pergunta})
	}
	return Previsao{}
}

func (a *ambienteComPrevia) previsao(pv Previsao) Previsao {
	decisao := pv.Bloqueio + pv.Pergunta
	pv.Registrar = func(context.Context) { a.registradas = append(a.registradas, decisao) }
	return pv
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
	if len(amb.registradas) != 1 {
		t.Fatalf("o bloqueio que cancelou devia ser registrado uma vez: %v", amb.registradas)
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
	if len(amb.previas) < 2 || len(amb.registradas) != 0 {
		t.Fatalf("prévia que só deixa passar não registra: %d previsões, %v", len(amb.previas), amb.registradas)
	}
}

const perguntaDaRegra = "Já é tarde, quer mesmo reservar assim?"

// confirmacaoComPerguntaDaRegra leva o pedido da reserva até a confirmação com a pergunta
// da regra junto.
func confirmacaoComPerguntaDaRegra(t *testing.T) (*Contrato, *Pedido, *Carteira, *ambienteComPrevia) {
	t.Helper()
	ct, p, cart := novoPedidoDaReserva(t)
	amb := &ambienteComPrevia{
		ambienteFalso: &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}},
		bloqueiaSe:    func(map[string]any) bool { return false },
		pergunta:      perguntaDaRegra,
	}
	ctx := context.Background()
	if s := Avancar(ctx, ct, p, cart, amb); s.Tipo != "pergunta" {
		t.Fatalf("a pergunta da regra espera a confirmação do caso: %+v", s)
	}
	Aplicar(ct, p, map[string]any{"area": "salao", "data": "15/11"}, OrigemMorador)
	s := Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "confirmar" || p.Estado != Confirmando {
		t.Fatalf("devia pedir a confirmação: %+v estado=%s", s, p.Estado)
	}
	if !strings.Contains(s.Texto, "Salão de festas") || !strings.HasSuffix(s.Texto, "\n"+perguntaDaRegra) {
		t.Fatalf("a confirmação do caso e a pergunta da regra saem juntas: %q", s.Texto)
	}
	return ct, p, cart, amb
}

func TestPerguntaDaRegraEntraNaConfirmacaoEUmSimResponde(t *testing.T) {
	ct, p, cart, amb := confirmacaoComPerguntaDaRegra(t)
	Confirmar(p, "sim", OrigemMorador)
	s := Avancar(context.Background(), ct, p, cart, amb)
	if s.Tipo != "concluido" || p.Estado != Concluido {
		t.Fatalf("um sim responde as duas perguntas: %+v estado=%s", s, p.Estado)
	}
	if got := amb.confirms[len(amb.confirms)-1]; got != "sim" {
		t.Fatalf("a gravação leva o sim do morador: %q", got)
	}
	if len(amb.registradas) != 1 || amb.registradas[0] != perguntaDaRegra {
		t.Fatalf("a pergunta aplicada é registrada uma vez: %v", amb.registradas)
	}
}

func TestPerguntaDaRegraRepetidaNaoRegistraDeNovo(t *testing.T) {
	ct, p, cart, amb := confirmacaoComPerguntaDaRegra(t)
	s := Avancar(context.Background(), ct, p, cart, amb)
	if s.Tipo != "confirmar" || !strings.HasSuffix(s.Texto, perguntaDaRegra) {
		t.Fatalf("sem resposta, a confirmação continua esperando: %+v", s)
	}
	if len(amb.registradas) != 1 {
		t.Fatalf("a mesma pergunta não é registrada de novo: %v", amb.registradas)
	}
}

func TestNaoAPerguntaDaRegraCancelaComoONaoDoCaso(t *testing.T) {
	ct, p, cart, amb := confirmacaoComPerguntaDaRegra(t)
	Confirmar(p, "nao", OrigemMorador)
	s := Avancar(context.Background(), ct, p, cart, amb)
	if s.Tipo != "cancelado" || s.Texto != CanceladoAPedido || p.Estado != Cancelado {
		t.Fatalf("o não cancela a pedido do morador: %+v estado=%s", s, p.Estado)
	}
	if len(amb.confirms) != 0 {
		t.Fatalf("a escrita não roda: %v", amb.confirms)
	}
}
