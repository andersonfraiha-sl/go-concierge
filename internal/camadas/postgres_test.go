package camadas

import (
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"go-concierge/internal/banco/bancotest"
	"go-concierge/internal/banco/ent"
)

func TestMain(m *testing.M) {
	codigo := m.Run()
	bancotest.Encerrar()
	os.Exit(codigo)
}

const skillMudanca = "agendar_mudanca"

var (
	camadaBase = Camada{Skill: skillMudanca, Tipo: TipoBase}
	camadaAdm  = Camada{Skill: skillMudanca, Tipo: TipoAdministradora, ID: "A"}
	camadaCond = Camada{Skill: skillMudanca, Tipo: TipoCondominio, ID: "c1"}
	instante   = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
)

func novoBanco(t *testing.T) (*Postgres, *ent.Client) {
	t.Helper()
	b := bancotest.Novo(t)
	ctx := t.Context()
	b.Ent.Administradora.Create().SetID("A").SetNome("Adm A").SaveX(ctx)
	b.Ent.Condominio.Create().SetID("c1").SetNome("C1").SetAdministradoraID("A").SaveX(ctx)
	return NovoPostgres(b.Ent), b.Ent
}

func inserir(t *testing.T, r Repo, c Camada, acimaID int, doc Documento) Versao {
	t.Helper()
	v, err := r.InserirVersao(t.Context(), NovaVersao{Camada: c, Documento: doc, AcimaID: acimaID, Origem: OrigemPublicacao, Autor: "ana"})
	if err != nil {
		t.Fatalf("InserirVersao(%+v): %v", c, err)
	}
	return v
}

func TestInserirVersaoNumeraECriaACamada(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()

	v1, err := r.InserirVersao(ctx, NovaVersao{
		Camada: camadaBase, Documento: documentoDeTeste(), CasoYAML: []byte("name: x\n"),
		Origem: OrigemImportacao, Autor: "sistema", Resumo: "primeira", Obrigatorios: []TrechoID{"texto:titulo"},
		EncerraEm: instante,
	})
	if err != nil {
		t.Fatalf("primeira versão: %v", err)
	}
	v2 := inserir(t, r, camadaBase, 0, com(func(d *Documento) { d.Textos.Titulo = "Novo" }))
	adm := inserir(t, r, camadaAdm, v2.ID, documentoDeTeste())

	if v1.Numero != 1 || v2.Numero != 2 || adm.Numero != 1 {
		t.Errorf("números = %d, %d, %d; quer 1, 2, 1", v1.Numero, v2.Numero, adm.Numero)
	}
	if !reflect.DeepEqual(v1.Documento, documentoDeTeste()) || string(v1.CasoYAML) != "name: x\n" ||
		!v1.EncerraEm.Equal(instante) || !reflect.DeepEqual(v1.Obrigatorios, []TrechoID{"texto:titulo"}) {
		t.Errorf("versão 1 não voltou como foi gravada: %+v", v1)
	}
	base, err := r.Camada(ctx, camadaBase)
	if err != nil || base.Cabeca == nil || base.Cabeca.ID != v2.ID {
		t.Errorf("cabeça da base = %+v (err %v); quer a versão %d", base.Cabeca, err, v2.ID)
	}
	estadoAdm, err := r.Camada(ctx, camadaAdm)
	if err != nil || estadoAdm.AcimaID != v2.ID || estadoAdm.Cabeca.Camada != camadaAdm {
		t.Errorf("camada da administradora = %+v (err %v); quer acima %d", estadoAdm, err, v2.ID)
	}
}

func TestInserirVersaoLimpaORascunhoSoQuandoPedido(t *testing.T) {
	tests := []struct {
		name           string
		limpar         bool
		querRascunho   bool
		versaoEsperada string
	}{
		{"mantem o rascunho quando nao pede", false, true, "1"},
		{"limpa o rascunho mas guarda o contador", true, false, "1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg, _ := novoBanco(t)
			r, ctx := pg.Repo(), t.Context()
			if _, err := r.SalvarRascunho(ctx, camadaAdm, "", documentoDeTeste()); err != nil {
				t.Fatalf("SalvarRascunho: %v", err)
			}

			_, err := r.InserirVersao(ctx, NovaVersao{Camada: camadaAdm, Documento: documentoDeTeste(), Origem: OrigemPublicacao, Autor: "ana", LimparRascunho: tt.limpar})
			if err != nil {
				t.Fatalf("InserirVersao: %v", err)
			}

			estado, err := r.Camada(ctx, camadaAdm)
			if err != nil {
				t.Fatalf("Camada: %v", err)
			}
			if (estado.Rascunho != nil) != tt.querRascunho || estado.VersaoRascunho != tt.versaoEsperada {
				t.Errorf("rascunho = %v, versão %q; quer rascunho %v, versão %q", estado.Rascunho, estado.VersaoRascunho, tt.querRascunho, tt.versaoEsperada)
			}
		})
	}
}

func TestInserirVersaoRecusaCamadaSemDonoCoerente(t *testing.T) {
	pg, _ := novoBanco(t)
	tests := []struct {
		name   string
		camada Camada
	}{
		{"base com dono", Camada{Skill: skillMudanca, Tipo: TipoBase, ID: "A"}},
		{"administradora sem dono", Camada{Skill: skillMudanca, Tipo: TipoAdministradora}},
		{"condominio sem dono", Camada{Skill: skillMudanca, Tipo: TipoCondominio}},
		{"tipo desconhecido", Camada{Skill: skillMudanca, Tipo: "grupo", ID: "g"}},
		{"sem skill", Camada{Tipo: TipoBase}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pg.Repo().InserirVersao(t.Context(), NovaVersao{Camada: tt.camada, Documento: documentoDeTeste(), Origem: OrigemPublicacao, Autor: "ana"})
			if !errors.Is(err, ErrCamadaInvalida) {
				t.Errorf("erro = %v; quer ErrCamadaInvalida", err)
			}
		})
	}
}

func TestCamadaSemLinhaVoltaSoComOIdentificador(t *testing.T) {
	pg, _ := novoBanco(t)

	got, err := pg.Repo().Camada(t.Context(), camadaCond)

	if err != nil || !reflect.DeepEqual(got, EstadoDaCamada{Camada: camadaCond}) {
		t.Errorf("Camada = %+v, %v; quer só o identificador", got, err)
	}
}

func TestRascunhoComVersaoVelhaFalha(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	passos := []struct {
		name    string
		vista   string
		want    string
		wantErr error
	}{
		{"primeiro salvamento parte de vazio", "", "1", nil},
		{"quem ainda nao viu o rascunho perde", "", "", ErrVersaoDesatualizada},
		{"versao vista atual salva e avanca", "1", "2", nil},
		{"versao anterior perde", "1", "", ErrVersaoDesatualizada},
		{"versao que nunca foi emitida perde", "abc", "", ErrVersaoDesatualizada},
	}
	for _, p := range passos {
		got, err := r.SalvarRascunho(ctx, camadaAdm, p.vista, documentoDeTeste())
		if !errors.Is(err, p.wantErr) || got != p.want {
			t.Errorf("%s: SalvarRascunho(%q) = %q, %v; quer %q, %v", p.name, p.vista, got, err, p.want, p.wantErr)
		}
	}
	estado, err := r.Camada(ctx, camadaAdm)
	if err != nil || estado.Rascunho == nil || estado.VersaoRascunho != "2" ||
		!reflect.DeepEqual(*estado.Rascunho, documentoDeTeste()) {
		t.Errorf("estado = %+v (err %v); quer rascunho na versão 2", estado, err)
	}
}

func TestCriarAtualizacaoSubstituiAAnterior(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	b1 := inserir(t, r, camadaBase, 0, documentoDeTeste())
	b2 := inserir(t, r, camadaBase, 0, com(func(d *Documento) { d.Textos.Titulo = "B2" }))
	b3 := inserir(t, r, camadaBase, 0, com(func(d *Documento) { d.Textos.Titulo = "B3" }))
	inserir(t, r, camadaAdm, b1.ID, documentoDeTeste())
	prazo := instante.Add(14 * 24 * time.Hour)

	primeira, err := r.CriarAtualizacao(ctx, camadaAdm, b1.ID, b2.ID, prazo)
	if err != nil {
		t.Fatalf("primeira atualização: %v", err)
	}
	segunda, err := r.CriarAtualizacao(ctx, camadaAdm, b2.ID, b3.ID, prazo)
	if err != nil {
		t.Fatalf("segunda atualização: %v", err)
	}

	anterior, err := r.Atualizacao(ctx, primeira)
	if err != nil || anterior.Estado != EstadoSubstituida {
		t.Errorf("primeira = %+v (err %v); quer substituida", anterior, err)
	}
	atual, err := r.Atualizacao(ctx, segunda)
	if err != nil || atual.Estado != EstadoPendente || atual.Camada != camadaAdm || atual.DeID != b2.ID || atual.ParaID != b3.ID || !atual.PrazoEm.Equal(prazo) {
		t.Errorf("segunda = %+v (err %v); quer pendente de %d para %d", atual, err, b2.ID, b3.ID)
	}
	abertas, err := r.AtualizacoesAbertas(ctx, 10)
	if err != nil || len(abertas) != 1 || abertas[0].ID != segunda {
		t.Errorf("abertas = %+v (err %v); quer só a %d", abertas, err, segunda)
	}
}

func TestCriarAtualizacaoSemCamadaFalha(t *testing.T) {
	pg, _ := novoBanco(t)

	_, err := pg.Repo().CriarAtualizacao(t.Context(), camadaAdm, 1, 2, instante)

	if !errors.Is(err, ErrCamadaNaoEncontrada) {
		t.Errorf("erro = %v; quer ErrCamadaNaoEncontrada", err)
	}
}

func TestAtualizacaoInexistenteFalha(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()

	_, errLer := r.Atualizacao(ctx, 999)
	errDecidir := r.RegistrarDecisao(ctx, 999, EstadoAceita, nil, "ana", instante)
	errPrazo := r.MarcarPrazoAplicado(ctx, 999)

	for nome, err := range map[string]error{"Atualizacao": errLer, "RegistrarDecisao": errDecidir, "MarcarPrazoAplicado": errPrazo} {
		if !errors.Is(err, ErrAtualizacaoNaoEncontrada) {
			t.Errorf("%s: erro = %v; quer ErrAtualizacaoNaoEncontrada", nome, err)
		}
	}
}

func TestAtualizacoesVencidasIgnoraJaAplicadas(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	b1 := inserir(t, r, camadaBase, 0, documentoDeTeste())
	b2 := inserir(t, r, camadaBase, 0, documentoDeTeste())
	inserir(t, r, camadaAdm, b1.ID, documentoDeTeste())
	inserir(t, r, camadaCond, b1.ID, documentoDeTeste())
	vencida, _ := r.CriarAtualizacao(ctx, camadaAdm, b1.ID, b2.ID, instante.Add(-time.Hour))
	aplicada, _ := r.CriarAtualizacao(ctx, camadaCond, b1.ID, b2.ID, instante.Add(-time.Hour))
	if err := r.MarcarPrazoAplicado(ctx, aplicada); err != nil {
		t.Fatalf("MarcarPrazoAplicado: %v", err)
	}

	got, err := r.AtualizacoesVencidas(ctx, instante, 10)

	if err != nil || len(got) != 1 || got[0].ID != vencida {
		t.Errorf("vencidas = %+v (err %v); quer só a %d", got, err, vencida)
	}
	antes, err := r.AtualizacoesVencidas(ctx, instante.Add(-2*time.Hour), 10)
	if err != nil || len(antes) != 0 {
		t.Errorf("vencidas antes do prazo = %+v (err %v); quer nenhuma", antes, err)
	}
}

func TestRegistrarDecisaoGravaEstadoEDecisoes(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	b1 := inserir(t, r, camadaBase, 0, documentoDeTeste())
	b2 := inserir(t, r, camadaBase, 0, documentoDeTeste())
	inserir(t, r, camadaAdm, b1.ID, documentoDeTeste())
	id, _ := r.CriarAtualizacao(ctx, camadaAdm, b1.ID, b2.ID, instante)
	decisoes := []Decisao{{Trecho: IDTitulo, Escolha: Aceitar}, {Trecho: "orientacao", Escolha: Misturar, Mistura: "texto"}}

	if err := r.RegistrarDecisao(ctx, id, EstadoParcial, decisoes, "ana", instante); err != nil {
		t.Fatalf("RegistrarDecisao: %v", err)
	}

	got, err := r.Atualizacao(ctx, id)
	if err != nil || got.Estado != EstadoParcial || !reflect.DeepEqual(got.Decisoes, decisoes) {
		t.Errorf("atualização = %+v (err %v); quer parcial com as decisões", got, err)
	}
}

func TestLerRetratoTrazSoCabecas(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	inserir(t, r, camadaBase, 0, documentoDeTeste())
	baseNova, err := r.InserirVersao(ctx, NovaVersao{
		Camada: camadaBase, Documento: com(func(d *Documento) { d.Textos.Titulo = "B2" }), CasoYAML: []byte("name: b2\n"),
		Origem: OrigemPublicacao, Autor: "ana", EncerraEm: instante,
	})
	if err != nil {
		t.Fatalf("base nova: %v", err)
	}
	inserir(t, r, camadaAdm, baseNova.ID, documentoDeTeste())
	admNova := inserir(t, r, camadaAdm, baseNova.ID, com(func(d *Documento) { d.Orientacao = "A2" }))
	cond := inserir(t, r, camadaCond, admNova.ID, documentoDeTeste())
	if _, err := r.SalvarRascunho(ctx, Camada{Skill: "so_rascunho", Tipo: TipoBase}, "", documentoDeTeste()); err != nil {
		t.Fatalf("rascunho sem versão: %v", err)
	}

	got, err := r.LerRetrato(ctx)

	if err != nil {
		t.Fatalf("LerRetrato: %v", err)
	}
	base := got.Bases[skillMudanca]
	if len(got.Bases) != 1 || base.VersaoID != baseNova.ID || base.Documento.Textos.Titulo != "B2" ||
		string(base.CasoYAML) != "name: b2\n" || !base.EncerraEm.Equal(instante) {
		t.Errorf("bases = %+v; quer só a cabeça %d", got.Bases, baseNova.ID)
	}
	if a := got.Administradoras[skillMudanca]["A"]; a.VersaoID != admNova.ID || a.Documento.Orientacao != "A2" {
		t.Errorf("administradora = %+v; quer a cabeça %d", a, admNova.ID)
	}
	if c := got.Condominios[skillMudanca]["c1"]; c.VersaoID != cond.ID {
		t.Errorf("condomínio = %+v; quer a cabeça %d", c, cond.ID)
	}
}

func TestLerRetratoSemCamadasVoltaMapasVazios(t *testing.T) {
	pg, _ := novoBanco(t)

	got, err := pg.Repo().LerRetrato(t.Context())

	if err != nil || got.Bases == nil || got.Administradoras == nil || got.Condominios == nil || len(got.Bases) != 0 {
		t.Errorf("retrato = %+v (err %v); quer mapas vazios e não nulos", got, err)
	}
}

func TestTemBaseSoComBasePublicada(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	passos := []struct {
		name string
		faz  func()
		want bool
	}{
		{"banco vazio", func() {}, false},
		{"rascunho de base nao conta", func() { _, _ = r.SalvarRascunho(ctx, camadaBase, "", documentoDeTeste()) }, false},
		{"versao de administradora nao conta", func() { inserir(t, r, camadaAdm, 0, documentoDeTeste()) }, false},
		{"versao de base conta", func() { inserir(t, r, camadaBase, 0, documentoDeTeste()) }, true},
	}
	for _, p := range passos {
		p.faz()
		if got, err := r.TemBase(ctx); err != nil || got != p.want {
			t.Errorf("%s: TemBase = %v, %v; quer %v", p.name, got, err, p.want)
		}
	}
}

func TestRemoverCondominioApagaACamada(t *testing.T) {
	pg, cli := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	cond := inserir(t, r, camadaCond, 0, documentoDeTeste())
	inserir(t, r, camadaAdm, 0, documentoDeTeste())

	cli.Condominio.DeleteOneID("c1").ExecX(ctx)

	if estado, err := r.Camada(ctx, camadaCond); err != nil || estado.Cabeca != nil {
		t.Errorf("camada do condomínio = %+v (err %v); quer sumida", estado, err)
	}
	if _, err := r.Versao(ctx, cond.ID); !errors.Is(err, ErrVersaoNaoEncontrada) {
		t.Errorf("versão do condomínio: erro = %v; quer ErrVersaoNaoEncontrada", err)
	}
	if estado, err := r.Camada(ctx, camadaAdm); err != nil || estado.Cabeca == nil {
		t.Errorf("camada da administradora = %+v (err %v); quer intacta", estado, err)
	}
}

func TestHistoricoVemDaMaisNovaParaAMaisAntiga(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	for range 3 {
		inserir(t, r, camadaAdm, 0, documentoDeTeste())
	}
	inserir(t, r, camadaCond, 0, documentoDeTeste())

	todas, err := r.Historico(ctx, camadaAdm, 500)
	duas, errLimite := r.Historico(ctx, camadaAdm, 2)

	if err != nil || errLimite != nil || len(todas) != 3 || todas[0].Numero != 3 || todas[2].Numero != 1 {
		t.Errorf("histórico = %+v (err %v)", todas, err)
	}
	if len(duas) != 2 || duas[0].Numero != 3 {
		t.Errorf("histórico com limite 2 = %+v (err %v)", duas, errLimite)
	}
	v, err := r.VersaoPorNumero(ctx, camadaAdm, 2)
	if err != nil || v.Numero != 2 || v.Camada != camadaAdm {
		t.Errorf("VersaoPorNumero = %+v (err %v)", v, err)
	}
	if _, err := r.VersaoPorNumero(ctx, camadaAdm, 9); !errors.Is(err, ErrVersaoNaoEncontrada) {
		t.Errorf("versão inexistente: erro = %v; quer ErrVersaoNaoEncontrada", err)
	}
}

func TestCamadasDaSkillTrazCabecasERascunhos(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	base := inserir(t, r, camadaBase, 0, documentoDeTeste())
	inserir(t, r, camadaAdm, base.ID, documentoDeTeste())
	if _, err := r.SalvarRascunho(ctx, camadaCond, "", documentoDeTeste()); err != nil {
		t.Fatalf("SalvarRascunho: %v", err)
	}
	inserir(t, r, Camada{Skill: "outra", Tipo: TipoBase}, 0, documentoDeTeste())

	got, err := r.CamadasDaSkill(ctx, skillMudanca)

	if err != nil || len(got) != 3 {
		t.Fatalf("camadas = %+v (err %v); quer 3", got, err)
	}
	porTipo := map[Tipo]EstadoDaCamada{}
	for _, e := range got {
		porTipo[e.Camada.Tipo] = e
	}
	if porTipo[TipoBase].Cabeca == nil || porTipo[TipoAdministradora].AcimaID != base.ID ||
		porTipo[TipoCondominio].Cabeca != nil || porTipo[TipoCondominio].Rascunho == nil {
		t.Errorf("camadas por tipo = %+v", porTipo)
	}
}

func TestAvisosAbertosSaemDepoisDoCiente(t *testing.T) {
	pg, _ := novoBanco(t)
	r, ctx := pg.Repo(), t.Context()
	v := inserir(t, r, camadaAdm, 0, documentoDeTeste())
	aviso := NovoAviso{Camada: camadaAdm, VersaoID: v.ID, Motivo: "falha de segurança", Trechos: []string{"Título"}, EncerraEm: instante}
	if err := r.CriarAviso(ctx, aviso); err != nil {
		t.Fatalf("CriarAviso: %v", err)
	}

	abertos, err := r.AvisosAbertos(ctx, 10)
	if err != nil || len(abertos) != 1 || abertos[0].Camada != camadaAdm || abertos[0].Motivo != "falha de segurança" ||
		!reflect.DeepEqual(abertos[0].Trechos, aviso.Trechos) || !abertos[0].EncerraEm.Equal(instante) {
		t.Fatalf("avisos abertos = %+v (err %v)", abertos, err)
	}
	for range 2 {
		if err := r.MarcarCiente(ctx, abertos[0].ID, "ana", instante); err != nil {
			t.Fatalf("MarcarCiente: %v", err)
		}
	}

	if depois, err := r.AvisosAbertos(ctx, 10); err != nil || len(depois) != 0 {
		t.Errorf("avisos depois do ciente = %+v (err %v); quer nenhum", depois, err)
	}
	if err := r.MarcarCiente(ctx, 999, "ana", instante); !errors.Is(err, ErrAvisoNaoEncontrado) {
		t.Errorf("ciente de aviso inexistente: erro = %v; quer ErrAvisoNaoEncontrado", err)
	}
}

func TestEmTransacaoDesfazTudoNoErro(t *testing.T) {
	pg, _ := novoBanco(t)
	falhou := errors.New("falhou depois de gravar")

	err := pg.EmTransacao(t.Context(), "publicar", func(r Repo) error {
		inserir(t, r, camadaBase, 0, documentoDeTeste())
		return falhou
	})

	if !errors.Is(err, falhou) {
		t.Errorf("erro = %v; quer o erro de f", err)
	}
	if tem, err := pg.Repo().TemBase(t.Context()); err != nil || tem {
		t.Errorf("TemBase = %v, %v; quer false (nada gravado)", tem, err)
	}
}

func TestEmTransacaoConfirmaQuandoNaoHaErro(t *testing.T) {
	pg, _ := novoBanco(t)

	err := pg.EmTransacao(t.Context(), "publicar", func(r Repo) error {
		inserir(t, r, camadaBase, 0, documentoDeTeste())
		return nil
	})

	if tem, errBase := pg.Repo().TemBase(t.Context()); err != nil || errBase != nil || !tem {
		t.Errorf("EmTransacao = %v, TemBase = %v, %v; quer gravado", err, tem, errBase)
	}
}

func TestEmTransacaoRefazNaFalhaDeSerializacao(t *testing.T) {
	pg, _ := novoBanco(t)
	conflito := &pgconn.PgError{Code: codigoFalhaSerializacao}
	tests := []struct {
		name           string
		falhasSeguidas int
		wantChamadas   int
		wantErr        error
	}{
		{"refaz e conclui na terceira tentativa", 2, 3, nil},
		{"desiste depois de tres tentativas", 3, 3, ErrConflitoConcorrente},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chamadas := 0

			err := pg.EmTransacao(t.Context(), "decidir", func(Repo) error {
				chamadas++
				if chamadas <= tt.falhasSeguidas {
					return conflito
				}
				return nil
			})

			if chamadas != tt.wantChamadas || !errors.Is(err, tt.wantErr) {
				t.Errorf("chamadas = %d, erro = %v; quer %d, %v", chamadas, err, tt.wantChamadas, tt.wantErr)
			}
		})
	}
}

func TestEmTransacaoNaoRefazErroComum(t *testing.T) {
	pg, _ := novoBanco(t)
	chamadas := 0

	_ = pg.EmTransacao(t.Context(), "decidir", func(Repo) error {
		chamadas++
		return errors.New("regra recusou")
	})

	if chamadas != 1 {
		t.Errorf("chamadas = %d; quer 1 (só conflito de serialização refaz)", chamadas)
	}
}
