package ganchos

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func registroDe(tipo TipoDecisao, o Origem, falhou bool, causa Causa) Registro {
	return Registro{
		Evento: EventoAntesDaFerramenta, Decisao: Decisao{Tipo: tipo, Origem: o},
		Falhou: falhou, Causa: causa, Duracao: 1500 * time.Microsecond,
	}
}

func observacaoDeTeste(rs ...Registro) Observacao {
	return Observacao{
		Evento: EventoAntesDaFerramenta, CondominioID: "c1", AdministradoraID: "a1",
		Duracao: 3 * time.Millisecond, Registros: rs,
	}
}

func observadorDeTeste(t *testing.T, log *slog.Logger, r *sdkmetric.ManualReader) func(context.Context, Observacao) {
	t.Helper()
	opcao, err := ComObservabilidade(log, sdkmetric.NewMeterProvider(sdkmetric.WithReader(r)).Meter("teste"))
	if err != nil {
		t.Fatal(err)
	}
	c := &Conjunto{}
	opcao(c)
	return c.observar
}

func TestLogDaDecisao(t *testing.T) {
	regra := Origem{Gancho: "horario", RegraID: "r-1", RegraVersao: 3}
	fixo := Origem{Gancho: "mascara", Fixo: true}
	casos := []struct {
		nome     string
		registro Registro
		nivel    string
	}{
		{"bloqueio de regra", registroDe(Bloquear, regra, false, ""), "INFO"},
		{"falha de regra", registroDe(Deixar, regra, true, CausaPrazo), "WARN"},
		{"falha de fixo", registroDe(Bloquear, fixo, true, CausaPanic), "ERROR"},
		{"deixar", registroDe(Deixar, regra, false, ""), ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var buf bytes.Buffer
			obs := observadorDeTeste(t, slog.New(slog.NewJSONHandler(&buf, nil)), sdkmetric.NewManualReader())
			obs(context.Background(), observacaoDeTeste(c.registro))
			if c.nivel == "" {
				if buf.Len() != 0 {
					t.Fatalf("deixar não deveria logar: %s", buf.String())
				}
				return
			}
			var linha map[string]any
			if err := json.Unmarshal(buf.Bytes(), &linha); err != nil {
				t.Fatal(err)
			}
			if linha["level"] != c.nivel {
				t.Errorf("nível = %v, quer %s", linha["level"], c.nivel)
			}
			esperadas := []string{"operacao", "gancho", "regra_id", "regra_versao", "decisao", "duracao_ms", "administradora_id", "condominio_id"}
			if c.nivel != "INFO" {
				esperadas = append(esperadas, "causa")
			}
			esperadas = append(esperadas, "time", "level", "msg")
			chaves := slices.Sorted(maps.Keys(linha))
			if !slices.Equal(chaves, slices.Sorted(slices.Values(esperadas))) {
				t.Errorf("chaves = %v, quer %v", chaves, slices.Sorted(slices.Values(esperadas)))
			}
			if c.nivel == "INFO" && linha["msg"] != "gancho decidiu" {
				t.Errorf("msg = %v", linha["msg"])
			}
		})
	}
}

func coletar(t *testing.T, r *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := r.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	porNome := map[string]metricdata.Metrics{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			porNome[m.Name] = m
		}
	}
	return porNome
}

func TestMetricas(t *testing.T) {
	leitor := sdkmetric.NewManualReader()
	obs := observadorDeTeste(t, slog.New(slog.DiscardHandler), leitor)
	obs(context.Background(), observacaoDeTeste(
		registroDe(Bloquear, Origem{Gancho: "horario", RegraID: "r-1"}, false, ""),
		registroDe(Bloquear, Origem{Gancho: "outra", RegraID: "r-2"}, false, ""),
		registroDe(Deixar, Origem{Gancho: "mascara", Fixo: true}, true, CausaPrazo),
	))
	ms := coletar(t, leitor)

	decisoes := ms["ganchos.decisoes"].Data.(metricdata.Sum[int64])
	porGancho := map[string]int64{}
	for _, p := range decisoes.DataPoints {
		g, _ := p.Attributes.Value(attribute.Key("gancho"))
		if _, ok := p.Attributes.Value(attribute.Key("evento")); !ok {
			t.Error("falta o atributo evento")
		}
		porGancho[g.AsString()] += p.Value
	}
	if porGancho["regra"] != 2 || porGancho["mascara"] != 1 || len(porGancho) != 2 {
		t.Errorf("decisoes por gancho = %v", porGancho)
	}
	falhas := ms["ganchos.falhas"].Data.(metricdata.Sum[int64])
	if len(falhas.DataPoints) != 1 || falhas.DataPoints[0].Value != 1 {
		t.Errorf("falhas = %+v", falhas.DataPoints)
	}
	duracao := ms["ganchos.duracao"].Data.(metricdata.Histogram[float64])
	if len(duracao.DataPoints) != 1 || duracao.DataPoints[0].Sum != 3 {
		t.Errorf("duracao = %+v", duracao.DataPoints)
	}
}

func TestDuracaoContaTambemDeixar(t *testing.T) {
	leitor := sdkmetric.NewManualReader()
	obs := observadorDeTeste(t, slog.New(slog.DiscardHandler), leitor)
	obs(context.Background(), observacaoDeTeste())
	h := coletar(t, leitor)["ganchos.duracao"].Data.(metricdata.Histogram[float64])
	if len(h.DataPoints) != 1 || h.DataPoints[0].Count != 1 {
		t.Errorf("duracao = %+v", h.DataPoints)
	}
}

func TestEventoDoSpan(t *testing.T) {
	gravador := tracetest.NewSpanRecorder()
	ctx, span := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(gravador)).Tracer("teste").Start(context.Background(), "turno")
	obs := observadorDeTeste(t, slog.New(slog.DiscardHandler), sdkmetric.NewManualReader())
	obs(ctx, observacaoDeTeste(
		registroDe(Bloquear, Origem{Gancho: "horario", RegraID: "r-1"}, false, ""),
		registroDe(Deixar, Origem{Gancho: "outra"}, false, ""),
	))
	span.End()

	eventos := gravador.Ended()[0].Events()
	if len(eventos) != 1 || eventos[0].Name != "gancho" {
		t.Fatalf("eventos = %+v", eventos)
	}
	for _, a := range eventos[0].Attributes {
		if strings.Contains(a.Value.String(), "r-1") {
			t.Errorf("id da regra vazou no span: %v", a)
		}
	}
}

func TestSemSpanNaoQuebra(t *testing.T) {
	obs := observadorDeTeste(t, slog.New(slog.DiscardHandler), sdkmetric.NewManualReader())
	obs(context.Background(), observacaoDeTeste(registroDe(Bloquear, Origem{Gancho: "x"}, false, "")))
}

func TestContadores(t *testing.T) {
	leitor := sdkmetric.NewManualReader()
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(leitor)).Meter("teste")
	indice := &Indice{Ativas: map[NomeDoEvento]int{EventoAntesDaFerramenta: 4, EventoMensagemRecebida: 1}}
	recarga, auditoriaFalhou, err := Contadores(meter, func() *Indice { return indice })
	if err != nil {
		t.Fatal(err)
	}
	recarga("ok")
	recarga("erro")
	auditoriaFalhou("copiloto")
	ms := coletar(t, leitor)

	if n := len(ms["ganchos.cache.recargas"].Data.(metricdata.Sum[int64]).DataPoints); n != 2 {
		t.Errorf("recargas = %d pontos, quer 2", n)
	}
	if n := len(ms["ganchos.auditoria.falhas"].Data.(metricdata.Sum[int64]).DataPoints); n != 1 {
		t.Errorf("auditoria.falhas = %d pontos, quer 1", n)
	}
	var total int64
	for _, p := range ms["ganchos.regras.ativas"].Data.(metricdata.Gauge[int64]).DataPoints {
		total += p.Value
	}
	if total != 5 {
		t.Errorf("regras ativas = %d, quer 5", total)
	}
}

func TestBucketsDaDuracao(t *testing.T) {
	leitor := sdkmetric.NewManualReader()
	obs := observadorDeTeste(t, slog.New(slog.DiscardHandler), leitor)
	obs(context.Background(), observacaoDeTeste())
	h := coletar(t, leitor)["ganchos.duracao"].Data.(metricdata.Histogram[float64])
	quer := []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 25}
	if !slices.Equal(h.DataPoints[0].Bounds, quer) {
		t.Errorf("bounds = %v, quer %v", h.DataPoints[0].Bounds, quer)
	}
}

func TestNomeDeRegraNaoVazaParaLogNemSpan(t *testing.T) {
	var buf bytes.Buffer
	gravador := tracetest.NewSpanRecorder()
	ctx, span := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(gravador)).Tracer("teste").Start(context.Background(), "turno")
	obs := observadorDeTeste(t, slog.New(slog.NewJSONHandler(&buf, nil)), sdkmetric.NewManualReader())
	obs(ctx, observacaoDeTeste(registroDe(Bloquear, Origem{Gancho: "Bloqueio CPF 123", RegraID: "r-1", RegraVersao: 2}, false, "")))
	span.End()

	if strings.Contains(buf.String(), "Bloqueio CPF 123") {
		t.Errorf("nome da regra vazou no log: %s", buf.String())
	}
	for _, a := range gravador.Ended()[0].Events()[0].Attributes {
		if strings.Contains(a.Value.String(), "Bloqueio CPF 123") {
			t.Errorf("nome da regra vazou no span: %v", a)
		}
	}
}

func TestRotulosLimitados(t *testing.T) {
	leitor := sdkmetric.NewManualReader()
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(leitor)).Meter("teste")
	recarga, auditoriaFalhou, err := Contadores(meter, nil)
	if err != nil {
		t.Fatal(err)
	}
	recarga(RecargaParcial)
	recarga("qualquer coisa")
	auditoriaFalhou(OrigemConcierge)
	auditoriaFalhou("id-do-usuario-9")
	ms := coletar(t, leitor)
	valores := func(nome, chave string) []string {
		var vs []string
		for _, p := range ms[nome].Data.(metricdata.Sum[int64]).DataPoints {
			v, _ := p.Attributes.Value(attribute.Key(chave))
			vs = append(vs, v.AsString())
		}
		slices.Sort(vs)
		return vs
	}
	if got := valores("ganchos.cache.recargas", "resultado"); !slices.Equal(got, []string{RotuloOutro, RecargaParcial}) {
		t.Errorf("resultado = %v", got)
	}
	if got := valores("ganchos.auditoria.falhas", "origem"); !slices.Equal(got, []string{OrigemConcierge, RotuloOutro}) {
		t.Errorf("origem = %v", got)
	}
}

func TestLoggerNilUsaPadrao(t *testing.T) {
	if _, err := ComObservabilidade(nil, sdkmetric.NewMeterProvider().Meter("teste")); err != nil {
		t.Fatal(err)
	}
	obs := observadorDeTeste(t, nil, sdkmetric.NewManualReader())
	obs(context.Background(), observacaoDeTeste(registroDe(Bloquear, Origem{Gancho: "x"}, false, "")))
}
