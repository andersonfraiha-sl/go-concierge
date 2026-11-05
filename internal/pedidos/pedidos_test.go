package pedidos

import (
	"context"
	"encoding/json"
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

func TestAplicarFiltraEscolhaMCPPreservandoIDOriginal(t *testing.T) {
	slot := &Slot{Nome: "motivo", Tipo: "choose", De: "motivos", Rotulo: "reason", Valor: "id", Strict: true,
		Filtro: map[string]any{"type_provider": "{{.categoria}}"}}
	ct := &Contrato{Slots: map[string]*Slot{"motivo": slot}, Ordem: []string{"motivo"}}
	p := &Pedido{
		Valores: map[string]any{"categoria": "GUEST"}, Rotulos: map[string]string{}, Origens: map[string]string{},
		Dados: map[string]any{"motivos": []any{
			map[string]any{"id": int64(42), "reason": "Visita familiar", "type_provider": "GUEST"},
			map[string]any{"id": int64(43), "reason": "Prestador de serviço", "type_provider": "SERVICE_PROVIDER"},
		}},
	}
	if rejeitados := Aplicar(ct, p, map[string]any{"motivo": "Visita familiar"}, OrigemMorador); len(rejeitados) != 0 {
		t.Fatalf("motivo filtrado devia ser aceito: %v", rejeitados)
	}
	if id, ok := p.Valores["motivo"].(float64); !ok || id != 42 || p.Rotulos["motivo"] != "Visita familiar" {
		t.Fatalf("Aplicar deve guardar o ID MCP original e o rótulo: valor=%T(%v), rótulo=%q", p.Valores["motivo"], p.Valores["motivo"], p.Rotulos["motivo"])
	}
}

func TestOpcoesDeDadosMantemCompatibilidadeLegada(t *testing.T) {
	stringsOpcoes := OpcoesDeDados([]any{"um", "dois"}, &Slot{})
	if len(stringsOpcoes) != 2 || stringsOpcoes[0].Valor != "um" || stringsOpcoes[1].Rotulo != "dois" {
		t.Fatalf("lista escalar legada: %+v", stringsOpcoes)
	}
	idFallback := OpcoesDeDados([]any{map[string]any{"id": 42, "label": "Unidade 42"}}, &Slot{Valor: "id_fastpass", Rotulo: "unit"})
	if len(idFallback) != 1 || idFallback[0].Valor != "42" || idFallback[0].Rotulo != "Unidade 42" {
		t.Fatalf("fallback id legado: %+v", idFallback)
	}
	strict := OpcoesDeDados([]any{map[string]any{"id": 42, "unit": "Unidade 42"}}, &Slot{Valor: "id_fastpass", Rotulo: "unit", Strict: true})
	if len(strict) != 0 {
		t.Fatalf("escolha MCP estrita não deve aceitar fallback de id: %+v", strict)
	}
}

func TestIDFastpassElegivelRespeitaLimitesNumericosSeguros(t *testing.T) {
	limite := int64(1<<53 - 1)
	testes := []struct {
		nome   string
		valor  any
		aceita bool
	}{
		{"int minimo seguro", -limite, true},
		{"int64 maximo seguro", limite, true},
		{"int64 acima do limite", limite + 1, false},
		{"uint seguro", uint64(limite), true},
		{"uint acima do limite", uint64(limite) + 1, false},
		{"int8", int8(127), true},
		{"int16", int16(-32768), true},
		{"int32", int32(109), true},
		{"uint8", uint8(255), true},
		{"uint16", uint16(109), true},
		{"uint32", uint32(109), true},
		{"uintptr", uintptr(109), true},
		{"float32 inteiro", float32(109), true},
		{"float64 limite seguro", float64(limite), true},
		{"float64 fora do limite", float64(limite + 1), false},
		{"float64 fracionario", 109.5, false},
		{"json number inteiro", json.Number("109"), true},
		{"json number decimal inteiro", json.Number("109.0"), true},
		{"json number limite", json.Number("9007199254740991"), true},
		{"json number acima", json.Number("9007199254740992"), false},
		{"json number fracionario", json.Number("109.5"), false},
	}
	for _, teste := range testes {
		t.Run(teste.nome, func(t *testing.T) {
			if aceita := idFastpassElegivel(teste.valor); aceita != teste.aceita {
				t.Fatalf("idFastpassElegivel(%T(%v)) = %v, esperado %v", teste.valor, teste.valor, aceita, teste.aceita)
			}
		})
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
	cart := &Carteira{AdministradoraID: "adm-a", Telefone: "5511999"}
	p := cart.Novo(ct, "reserva-area-comum", "cond-1", Origem{ProtocoloAtende: "ATD-1"})
	if p.AdministradoraID != "adm-a" || p.CondominioID != "cond-1" {
		t.Fatalf("escopo do pedido: %+v", p)
	}
	if !strings.HasPrefix(p.Protocolo, "P-") || len(p.Protocolo) != len("P-")+8 || p.Estado != Coletando {
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
	p2 := cart.Novo(ct, "reserva-area-comum", "cond-1", Origem{})
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
	visita := CasoResumo{ID: "liberacao_visitante", Keywords: []string{"liberar", "visitante"}, Slots: []*Slot{{Nome: "data_visita", Tipo: "date"}}}
	for _, teste := range []struct{ mensagem, data string }{{"quero liberar um visitante hoje", "hoje"}, {"quero liberar um visitante amanhã", "amanha"}} {
		ex, _ = r.Extrair(context.Background(), Entrada{Mensagem: teste.mensagem, Casos: []CasoResumo{visita}})
		if len(ex.Pedidos) != 1 || ex.Pedidos[0].Valores["data_visita"] != teste.data {
			t.Fatalf("data relativa %q: %+v", teste.mensagem, ex)
		}
	}
	ex, _ = r.Extrair(context.Background(), Entrada{Mensagem: "quero liberar visitantes toda semana", Casos: []CasoResumo{visita}})
	if len(ex.Pedidos) != 0 || ex.Restante == "" {
		t.Fatalf("pedido recorrente segue para o concierge encaminhar ao app: %+v", ex)
	}
}

func TestEncomendasStatusOpcionalComDefaultWaiting(t *testing.T) {
	ct := casoEncomendas(t)
	caso := CasoResumo{ID: ct.Caso.ID, Nome: ct.Caso.Name, Keywords: ct.Caso.Keywords, Slots: slots(ct)}
	for _, teste := range []struct {
		mensagem string
		status   string
	}{
		{"tenho alguma encomenda?", ""},
		{"minhas encomendas entregues", "delivered"},
		{"tem pacote cancelado?", "canceled"},
		{"quero ver encomendas aguardando retirada", "waiting"},
	} {
		ex, err := (Regras{}).Extrair(context.Background(), Entrada{Mensagem: teste.mensagem, Casos: []CasoResumo{caso}})
		if err != nil || len(ex.Pedidos) != 1 || ex.Pedidos[0].Caso != "encomendas" {
			t.Fatalf("pedido de encomendas %q: %+v erro=%v", teste.mensagem, ex, err)
		}
		status, existe := ex.Pedidos[0].Valores["status"]
		if teste.status == "" && existe || teste.status != "" && status != teste.status {
			t.Fatalf("status de %q = %v, esperado %q", teste.mensagem, status, teste.status)
		}
	}
	p := &Pedido{Dados: map[string]any{}, Valores: map[string]any{"unidade": "fp-1"}}
	params, err := ct.ParamsDe(map[string]any{"unitId": "$unidade", "filter_status": `{{if .status}}{{.status}}{{else}}waiting{{end}}`}, p)
	if err != nil || params["filter_status"] != "waiting" {
		t.Fatalf("status padrão: %+v erro=%v", params, err)
	}
}

func casoEncomendas(t *testing.T) *Contrato {
	t.Helper()
	c, err := okf.LoadCaso(filepath.Join("..", "..", "okf", "agentes", "concierge", "agentes", "encomendas", "caso.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := ContratoDe(c)
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

func TestDatasRelativasUsamFusoSaoPauloPertoDaMeiaNoiteUTC(t *testing.T) {
	original := Agora
	Agora = func() time.Time { return time.Date(2026, 10, 29, 0, 30, 0, 0, time.UTC) }
	defer func() { Agora = original }()
	for _, teste := range []struct{ entrada, esperada string }{{"hoje", "2026-10-28"}, {"amanha", "2026-10-29"}} {
		data, err := parseData(teste.entrada)
		if err != nil || data.Format("2006-01-02") != teste.esperada {
			t.Fatalf("parseData(%q) = %s, erro=%v", teste.entrada, data.Format("2006-01-02"), err)
		}
	}
	fim, err := dataHoraRelativa("hoje", "fim")
	if err != nil || fim != "2026-10-29 02:59" {
		t.Fatalf("fim de hoje no fuso do recurso = %q, erro=%v", fim, err)
	}
	if !dataVisitaPermitida("2026-10-28") || dataVisitaPermitida("2026-10-30") {
		t.Fatal("validação de hoje e datas futuras deve usar o dia de São Paulo")
	}
}

func TestRotuloDataVisita(t *testing.T) {
	Agora = func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.Local) }
	defer func() { Agora = time.Now }()
	for _, teste := range []struct{ data, esperado string }{
		{"2026-10-28", "hoje"},
		{"2026-10-29", "amanhã"},
		{"2026-11-02", "2 de novembro de 2026"},
	} {
		got, err := Render("{{dateLabel .data}}", map[string]any{"data": teste.data})
		if err != nil || got != teste.esperado {
			t.Fatalf("dateLabel(%q): %q, %v; queria %q", teste.data, got, err, teste.esperado)
		}
	}
}

// TestRotuloDataVisitaVirandaDeAnoEAnoBissexto: "amanhã" precisa atravessar virada de ano
// e 29 de fevereiro de ano bissexto do mesmo jeito que qualquer outro dia.
func TestRotuloDataVisitaVirandaDeAnoEAnoBissexto(t *testing.T) {
	for _, teste := range []struct {
		nome, agora, data, esperado string
	}{
		{"31 dez -> 1 jan é amanhã", "2026-12-31", "2027-01-01", "amanhã"},
		{"1 jan -> 31 dez do ano anterior não é hoje nem amanhã", "2027-01-01", "2026-12-31", "31 de dezembro de 2026"},
		{"28 fev de bissexto -> 29 fev é amanhã", "2028-02-28", "2028-02-29", "amanhã"},
		{"29 fev de bissexto -> 1 mar é amanhã", "2028-02-29", "2028-03-01", "amanhã"},
	} {
		t.Run(teste.nome, func(t *testing.T) {
			agora, err := time.ParseInLocation("2006-01-02", teste.agora, localidadeSaoPaulo)
			if err != nil {
				t.Fatal(err)
			}
			Agora = func() time.Time { return agora }
			defer func() { Agora = time.Now }()
			got, err := Render("{{dateLabel .data}}", map[string]any{"data": teste.data})
			if err != nil || got != teste.esperado {
				t.Fatalf("dateLabel(%q) com agora=%q: %q, %v; queria %q", teste.data, teste.agora, got, err, teste.esperado)
			}
		})
	}
}

func slots(ct *Contrato) []*Slot {
	var out []*Slot
	for _, n := range ct.Ordem {
		out = append(out, ct.Slots[n])
	}
	return out
}

func TestResumoParaAtendente(t *testing.T) {
	ct := casoReserva(t)
	var c Carteira
	p := c.Novo(ct, "reserva-area-comum", "cond-1", Origem{ProtocoloAtende: "ATD-9"})
	p.Valor("area", "salao", "Salão de festas", OrigemMorador)
	p.Pendencia = &Pendencia{Slot: "data", Pergunta: "Para qual data? (dd/mm)", Vezes: 2, Motivo: "data inválida"}
	r := ResumoParaAtendente(ct, p, []string{"morador: dia trinta e dois", "concierge: Não consegui usar a resposta"})
	for _, trecho := range []string{p.Protocolo, "Reserva de Área Comum", "Área comum: Salão de festas", "Falta: Unidade; Data da reserva", "feita 2 vezes", "data inválida", "dia trinta e dois"} {
		if !strings.Contains(r, trecho) {
			t.Fatalf("resumo sem %q:\n%s", trecho, r)
		}
	}
}

func TestExtracaoComTrecho(t *testing.T) {
	var ex Extracao
	raw := `{"pedidos":[{"caso":"reserva_area_comum","valores":{"area":"churrasqueira"},"trecho":"alugar a churrasqueira"},{"caso":"x","valores":{}}],"atendente":false}`
	if err := json.Unmarshal([]byte(raw), &ex); err != nil {
		t.Fatal(err)
	}
	if len(ex.Pedidos) != 2 || ex.Pedidos[0].Trecho != "alugar a churrasqueira" || ex.Pedidos[1].Trecho != "" {
		t.Fatalf("%+v", ex.Pedidos)
	}
}
