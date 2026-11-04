package pedidos

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ambienteComPrevia é o ambienteFalso que também prevê a escrita: bloqueia quando
// bloqueiaSe aceita os params recebidos.
type ambienteComPrevia struct {
	*ambienteFalso
	bloqueiaSe func(params map[string]any) bool
	previas    []map[string]any
}

func (a *ambienteComPrevia) Prever(_ context.Context, _, recurso string, params map[string]any) (string, bool) {
	if recurso != "areas.reservar" {
		return "", false
	}
	a.previas = append(a.previas, params)
	if a.bloqueiaSe(params) {
		return "Reservas só até as 22h.", true
	}
	return "", false
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
