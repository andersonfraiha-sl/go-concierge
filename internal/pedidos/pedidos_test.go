package pedidos

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-concierge/internal/okf"
)

// ambienteFalso devolve listas fixas e registra as execuções (com a confirmação recebida).
type ambienteFalso struct {
	chamadas []string
	confirms []string
	unidades []any
}

func (a *ambienteFalso) Chamar(_ context.Context, agente, recurso string, params map[string]any, confirmacao string) (any, error) {
	a.chamadas = append(a.chamadas, recurso)
	switch recurso {
	case "unidades.listar":
		return a.unidades, nil
	case "areas.listar":
		return []any{map[string]any{"id": "churrasqueira", "nome": "Churrasqueira"}, map[string]any{"id": "salao", "nome": "Salão de festas"}}, nil
	case "areas.reservar":
		a.confirms = append(a.confirms, confirmacao)
		if confirmacao == "" {
			return nil, errors.New("escrita sem confirmação")
		}
		return map[string]any{"protocolo": "R-77", "params": params}, nil
	}
	return nil, errors.New("recurso desconhecido: " + recurso)
}

func casoReserva(t *testing.T) *Contrato {
	t.Helper()
	c, err := okf.LoadCaso(filepath.Join("..", "..", "okf", "agentes", "concierge", "agentes", "reserva-area-comum", "caso.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := ContratoDe(c)
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

// O contrato da reserva: unidade e área (choose) + data (collect, date), confirm e escrita.
func TestContratoDaReserva(t *testing.T) {
	ct := casoReserva(t)
	if strings.Join(ct.Ordem, ",") != "unidade,area,data" {
		t.Fatalf("ordem dos slots: %v", ct.Ordem)
	}
	if ct.Slots["data"].Tipo != "date" || ct.Slots["unidade"].Reuse != "always" || ct.Slots["area"].De != "areas" {
		t.Fatalf("slots: %+v %+v", ct.Slots["data"], ct.Slots["area"])
	}
	tipos := []string{}
	for _, p := range ct.Passos {
		tipos = append(tipos, p.Tipo)
	}
	if strings.Join(tipos, ",") != "recurso,escolher,recurso,escolher,coletar,confirmar,recurso" {
		t.Fatalf("passos: %v", tipos)
	}
}

// Ciclo completo em código: uma unidade (escolhida sozinha), duas áreas (pergunta com
// opções), data validada, confirmação e escrita com o sim; estados decididos pelos
// parâmetros, não pelo texto.
func TestMotorDaReserva(t *testing.T) {
	Agora = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.Local) }
	defer func() { Agora = time.Now }()
	ct := casoReserva(t)
	amb := &ambienteFalso{unidades: []any{map[string]any{"id": "109", "label": "Jataí - 101"}}}
	cart := &Carteira{Account: "cond-001", Telefone: "5511999"}
	p := cart.Novo(ct, "reserva-area-comum", Origem{ProtocoloAtende: "ATD-1"})
	if !strings.HasPrefix(p.Protocolo, "P-") || len(p.Protocolo) != 7 || p.Estado != Coletando {
		t.Fatalf("pedido novo: %+v", p)
	}
	ctx := context.Background()

	s := Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "pergunta" || s.Slot != "area" || len(s.Opcoes) != 2 || !strings.Contains(s.Texto, "1) Churrasqueira") {
		t.Fatalf("devia perguntar a área com opções: %+v", s)
	}
	if p.Valores["unidade"] != "109" || p.Origens["unidade"] != OrigemSistema {
		t.Fatalf("uma só unidade entra sozinha: %+v", p.Valores)
	}
	if rej := Aplicar(ct, p, map[string]any{"area": "piscina"}, OrigemMorador); rej["area"] == "" {
		t.Fatalf("piscina não é opção: %v", rej)
	}
	s = Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "pergunta" || s.Slot != "area" || p.Pendencia.Vezes != 2 || !strings.Contains(s.Texto, "Não consegui usar a resposta") {
		t.Fatalf("pergunta repetida com a correção: %+v vezes=%d", s, p.Pendencia.Vezes)
	}
	if rej := Aplicar(ct, p, map[string]any{"area": "salão"}, OrigemMorador); len(rej) != 0 || p.Valores["area"] != "salao" || p.Rotulos["area"] != "Salão de festas" {
		t.Fatalf("salão casa com a opção pelo rótulo: %v %+v", rej, p.Valores)
	}
	s = Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "pergunta" || s.Slot != "data" {
		t.Fatalf("agora falta a data: %+v", s)
	}
	if rej := Aplicar(ct, p, map[string]any{"data": "31/02"}, OrigemMorador); rej["data"] == "" {
		t.Fatalf("31/02 é inválida: %v", rej)
	}
	if rej := Aplicar(ct, p, map[string]any{"data": "15/11"}, OrigemMorador); len(rej) != 0 || p.Valores["data"] != "2026-11-15" || p.Rotulos["data"] != "15 de novembro de 2026" {
		t.Fatalf("data normalizada: %v %+v", rej, p.Valores)
	}
	s = Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "confirmar" || p.Estado != Confirmando || !strings.Contains(s.Texto, "Salão de festas em 15 de novembro de 2026 para Jataí - 101") {
		t.Fatalf("confirmação renderizada: %+v estado=%s", s, p.Estado)
	}
	if len(amb.confirms) != 0 {
		t.Fatal("não podia ter executado antes do sim")
	}
	Confirmar(p, "sim", OrigemMorador)
	s = Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != "concluido" || p.Estado != Concluido || p.Resultado == nil || p.Resultado.Protocolo != "R-77" || !strings.Contains(s.Texto, "Protocolo: R-77") {
		t.Fatalf("execução e output: %+v %+v", s, p.Resultado)
	}
	if len(amb.confirms) != 1 || amb.confirms[0] != "sim" {
		t.Fatalf("a escrita recebe a confirmação explícita: %v", amb.confirms)
	}
	// o que o morador já informou não é sobrescrito por extração de outra mensagem
	if rej := Aplicar(ct, p, map[string]any{"area": "churrasqueira", "data": "2026-11-01"}, OrigemMorador); len(rej) != 0 || p.Valores["area"] != "salao" || p.Valores["data"] != "2026-11-15" {
		t.Fatalf("valores do morador são estáveis: %v %+v", rej, p.Valores)
	}
	// o atendente pode corrigir
	if Aplicar(ct, p, map[string]any{"data": "2026-11-16"}, OrigemAtendente); p.Valores["data"] != "2026-11-16" {
		t.Fatalf("atendente corrige: %+v", p.Valores)
	}
	Aplicar(ct, p, map[string]any{"data": "2026-11-15"}, OrigemAtendente)
	// idempotente: avançar de novo não executa de novo
	s = Avancar(ctx, ct, p, cart, amb)
	if s.Tipo != Concluido || len(amb.confirms) != 1 {
		t.Fatalf("pedido concluído não reexecuta: %+v", s)
	}

	// um segundo pedido reaproveita a unidade (reuse: always) mesmo com duas unidades
	amb.unidades = append(amb.unidades, map[string]any{"id": "110", "label": "Jataí - 102"})
	p2 := cart.Novo(ct, "reserva-area-comum", Origem{})
	s = Avancar(ctx, ct, p2, cart, amb)
	if p2.Valores["unidade"] != "109" || p2.Origens["unidade"] != OrigemMemoria || s.Slot != "area" {
		t.Fatalf("unidade reutilizada do pedido anterior: %+v %+v", p2.Valores, s)
	}
	// "não" na confirmação cancela
	Aplicar(ct, p2, map[string]any{"area": "1", "data": "2026-12-01"}, OrigemMorador)
	s = Avancar(ctx, ct, p2, cart, amb)
	if s.Tipo != "confirmar" || p2.Rotulos["area"] != "Churrasqueira" {
		t.Fatalf("opção pelo número: %+v %v", s, p2.Rotulos)
	}
	Confirmar(p2, "nao", OrigemMorador)
	if s = Avancar(ctx, ct, p2, cart, amb); s.Tipo != "cancelado" || p2.Estado != Cancelado || p2.Aberto() {
		t.Fatalf("não confirmou → cancelado: %+v", s)
	}
}

func TestRegrasExtrator(t *testing.T) {
	ct := casoReserva(t)
	casos := []CasoResumo{{ID: ct.Caso.ID, Nome: ct.Caso.Name, Keywords: ct.Caso.Keywords, Slots: slots(ct)}}
	r := Regras{}
	ex, _ := r.Extrair(context.Background(), Entrada{Mensagem: "quero reservar a churrasqueira dia 15/11", Casos: casos})
	if len(ex.Pedidos) != 1 || ex.Pedidos[0].Caso != "reserva_area_comum" || ex.Pedidos[0].Valores["data"] != "15/11" || ex.Restante != "" {
		t.Fatalf("caso por palavra-chave e data: %+v", ex)
	}
	p := &Pedido{Protocolo: "P-1", Caso: ct.Caso.ID, Estado: Coletando, Pendencia: &Pendencia{Slot: "area"}}
	ex, _ = r.Extrair(context.Background(), Entrada{Mensagem: "salão", Casos: casos, Abertos: []*Pedido{p}})
	if len(ex.Pedidos) != 1 || ex.Pedidos[0].Protocolo != "P-1" || ex.Pedidos[0].Valores["area"] != "salão" {
		t.Fatalf("resposta à única pendência: %+v", ex)
	}
	p.Estado, p.Pendencia = Confirmando, nil
	ex, _ = r.Extrair(context.Background(), Entrada{Mensagem: "sim, pode", Casos: casos, Abertos: []*Pedido{p}})
	if len(ex.Confirmacoes) != 1 || ex.Confirmacoes[0].Resposta != "sim" {
		t.Fatalf("confirmação: %+v", ex)
	}
	ex, _ = r.Extrair(context.Background(), Entrada{Mensagem: "quero falar com um atendente", Casos: casos, Abertos: []*Pedido{p}})
	if !ex.Atendente {
		t.Fatalf("pedido de humano: %+v", ex)
	}
	ex, _ = r.Extrair(context.Background(), Entrada{Mensagem: "qual o horário da piscina?", Casos: casos})
	if ex.Restante == "" || len(ex.Pedidos) != 0 {
		t.Fatalf("dúvida vira restante: %+v", ex)
	}
}

func slots(ct *Contrato) []*Slot {
	var out []*Slot
	for _, n := range ct.Ordem {
		out = append(out, ct.Slots[n])
	}
	return out
}

func TestArquivosEResumo(t *testing.T) {
	ct := casoReserva(t)
	st := NewArquivos(t.TempDir())
	ctx := context.Background()
	c, err := st.Ler(ctx, "cond-001", "+55 (11) 99999-0000")
	if err != nil || len(c.Pedidos) != 0 || c.Telefone != "5511999990000" {
		t.Fatalf("carteira vazia: %+v %v", c, err)
	}
	p := c.Novo(ct, "reserva-area-comum", Origem{ProtocoloAtende: "ATD-9"})
	p.Valor("area", "salao", "Salão de festas", OrigemMorador)
	p.Pendencia = &Pendencia{Slot: "data", Pergunta: "Para qual data? (dd/mm)", Vezes: 2, Motivo: "data inválida"}
	if err := st.Gravar(ctx, c); err != nil {
		t.Fatal(err)
	}
	c2, _ := st.Ler(ctx, "cond-001", "5511999990000")
	if len(c2.Pedidos) != 1 || c2.Por(p.Protocolo) == nil || c2.Pedidos[0].Rotulos["area"] != "Salão de festas" {
		t.Fatalf("ida e volta: %+v", c2)
	}
	lista, _ := st.Listar(ctx, "cond-001")
	if len(lista) != 1 {
		t.Fatalf("listar: %d", len(lista))
	}
	r := ResumoParaAtendente(ct, c2.Pedidos[0], []string{"morador: dia trinta e dois", "concierge: Não consegui usar a resposta"})
	for _, trecho := range []string{p.Protocolo, "Reserva de Área Comum", "Área comum: Salão de festas", "Falta: Unidade; Data da reserva", "feita 2 vezes", "data inválida", "dia trinta e dois"} {
		if !strings.Contains(r, trecho) {
			t.Fatalf("resumo sem %q:\n%s", trecho, r)
		}
	}
}
