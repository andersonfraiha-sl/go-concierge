package atende

import (
	"strings"
	"testing"
	"time"

	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/ganchostest"
	"go-concierge/internal/ganchos/regras"
	"go-concierge/internal/pedidos"
)

const regraQueBloqueiaReserva = `{"nome":"Reserva fechada","evento":"antes_da_ferramenta",
"alvo":{"administradora":"adm-a","condominios":["cond-001"]},
"condicoes":[{"tipo":"ferramenta","nomes":["areas.reservar"]}],
"decisao":{"tipo":"bloquear","mensagem":"Reservas fechadas por aqui."},"ativa":true}`

func servicoComRegraDeReserva(t *testing.T) *Servico {
	t.Helper()
	indice, erros := regras.Compilar([]regras.RegraGravada{{ID: "r1", Versao: 1, Ativa: true, Definicao: []byte(regraQueBloqueiaReserva)}})
	if len(erros) > 0 {
		t.Fatal(erros)
	}
	svc, _, _ := servicoComPedidos(t)
	svc.Ganchos = ganchostest.Conjunto(fixos.Todos(), indice)
	return svc
}

func TestPreviaCancelaAReservaAntesDaPrimeiraPergunta(t *testing.T) {
	svc := servicoComRegraDeReserva(t)
	sess := falar(t, svc, "previa-1", "quero reservar uma área comum")
	r := ultimoConcierge(sess)
	if !strings.Contains(r.Texto, "Reservas fechadas por aqui.") || strings.Contains(r.Texto, "Qual área") {
		t.Fatalf("devia cancelar com a mensagem da regra, sem perguntar a área: %q", r.Texto)
	}
	if len(r.Pedidos) == 0 || r.Pedidos[len(r.Pedidos)-1].Estado != pedidos.Cancelado {
		t.Fatalf("o pedido devia ficar cancelado: %+v", r.Pedidos)
	}
}

func TestPreviaNaoEntraNaAuditoriaDoTurno(t *testing.T) {
	svc := servicoComRegraDeReserva(t)
	sess := falar(t, svc, "previa-2", "quero reservar uma área comum")
	for _, reg := range sess.registrosDoTurno() {
		if reg.Evento == ganchos.EventoAntesDaFerramenta && reg.Decisao.Tipo == ganchos.Bloquear {
			t.Fatalf("o bloqueio da previsão não pode ser auditado: %+v", reg)
		}
	}
}

func TestPreviaSemRegraNaoMudaOFluxo(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	sess := falar(t, svc, "previa-3", "quero reservar uma área comum")
	if r := ultimoConcierge(sess); !strings.Contains(r.Texto, "Qual área você quer reservar?") {
		t.Fatalf("sem regra o fluxo pergunta a área: %q", r.Texto)
	}
}

const regraDeReservaAte22h = `{"nome":"Reserva só até 22h","evento":"antes_da_ferramenta",
"alvo":{"administradora":"adm-a","condominios":["cond-001"]},
"condicoes":[{"tipo":"ferramenta","nomes":["areas.reservar"]},
{"tipo":"fora_do_horario","inicio":"06:00","fim":"22:00","fonte":{"tipo":"agora"}}],
"decisao":{"tipo":"bloquear","mensagem":"Reservas só das 6h às 22h."},"ativa":true}`

func servicoComRegraAte22h(t *testing.T, agora string) *Servico {
	t.Helper()
	indice, erros := regras.Compilar([]regras.RegraGravada{{ID: "r22", Versao: 1, Ativa: true, Definicao: []byte(regraDeReservaAte22h)}})
	if len(erros) > 0 {
		t.Fatal(erros)
	}
	instante, err := time.Parse(time.RFC3339, agora)
	if err != nil {
		t.Fatal(err)
	}
	svc, _, _ := servicoComPedidos(t)
	svc.Ganchos = ganchostest.Conjunto(fixos.Todos(), indice)
	svc.Agora = func() time.Time { return instante }
	return svc
}

func TestPreviaDaRegraDeHorarioBloqueiaAs22h30(t *testing.T) {
	svc := servicoComRegraAte22h(t, "2026-11-05T01:30:00Z") // 22h30 em São Paulo
	r := ultimoConcierge(falar(t, svc, "previa-22h30", "quero reservar uma área comum"))
	if !strings.Contains(r.Texto, "Reservas só das 6h às 22h.") || strings.Contains(r.Texto, "Qual área") {
		t.Fatalf("às 22h30 devia cancelar antes da primeira pergunta: %q", r.Texto)
	}
}

func TestPreviaDaRegraDeHorarioDeixaPassarAs21h(t *testing.T) {
	svc := servicoComRegraAte22h(t, "2026-11-05T00:00:00Z") // 21h00 em São Paulo
	r := ultimoConcierge(falar(t, svc, "previa-21h", "quero reservar uma área comum"))
	if !strings.Contains(r.Texto, "Qual área você quer reservar?") {
		t.Fatalf("às 21h a pergunta sai: %q", r.Texto)
	}
}

func TestPreviaComGanchoQueFalhaNaoCancela(t *testing.T) {
	quebrado := ganchos.Gancho[ganchos.AntesDaFerramenta]{
		Nome: "regra", Falha: ganchos.FechadaSeGrava, Origem: ganchos.Origem{RegraID: "r-quebrada"},
		Avaliar: func(ganchos.AntesDaFerramenta) (ganchos.AntesDaFerramenta, ganchos.Decisao) { panic("regra quebrada") },
	}
	idx := &ganchos.Indice{AntesDaFerramenta: ganchos.Nivel[ganchos.AntesDaFerramenta]{Globais: []ganchos.Gancho[ganchos.AntesDaFerramenta]{quebrado}}}
	svc, _, _ := servicoComPedidos(t)
	svc.Ganchos = ganchostest.Conjunto(fixos.Todos(), idx)
	r := ultimoConcierge(falar(t, svc, "previa-falha", "quero reservar uma área comum"))
	if !strings.Contains(r.Texto, "Qual área você quer reservar?") {
		t.Fatalf("falha na previsão não cancela o pedido: %q", r.Texto)
	}
}
