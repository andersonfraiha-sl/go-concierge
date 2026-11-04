package admin

import (
	"context"
	stdsql "database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5/pgconn"

	"go-concierge/internal/banco/ent"
	entadministradora "go-concierge/internal/banco/ent/administradora"
	entcondominio "go-concierge/internal/banco/ent/condominio"
	enthabilitacao "go-concierge/internal/banco/ent/habilitacao"
	"go-concierge/internal/banco/ent/payload"
	"go-concierge/internal/banco/ent/predicate"
	"go-concierge/internal/banco/ent/schema"
	"go-concierge/internal/falha"
)

// timeoutConsulta limita cada leitura ou escrita do mapeamento (vale também para as rotas HTTP).
const timeoutConsulta = 3 * time.Second

// Postgres guarda o mapeamento nas tabelas administradoras, condominios, habilitacoes,
// payloads e orientacoes. condominio_id NULL é o padrão ("*").
type Postgres struct {
	cli *ent.Client
	// aoAlterar recebe o mapeamento de cada escrita confirmada e a versão dela (o Cache).
	// escritas põe em fila as escritas deste processo do começo da transação até o aviso, e
	// versao cresce nessa ordem: a ordem das versões é a dos commits.
	aoAlterar func(m Mapeamento, versao uint64)
	escritas  sync.Mutex
	versao    uint64
}

func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

func (p *Postgres) Ler(ctx context.Context) (Mapeamento, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	// uma foto só das cinco tabelas, mesmo com uma escrita no meio
	foto := &stdsql.TxOptions{Isolation: stdsql.LevelRepeatableRead, ReadOnly: true}
	return emTransacao(ctx, p.cli, foto, ler)
}

// gravacao escreve no banco o que a regra mudou, lendo do mapeamento novo.
type gravacao func(ctx context.Context, cli *ent.Client, novo Mapeamento) error

// tentativasSerializacao: quantas vezes alterar tenta antes de desistir por conflito.
const tentativasSerializacao = 3

// codigoFalhaSerializacao é o SQLSTATE 40001 (serialization_failure).
const codigoFalhaSerializacao = "40001"

// alterar roda numa transação serializável: lê, valida com a regra pura, grava e relê. A
// serialização impede que duas escritas simultâneas percam uma à outra (os ajustes mesclam
// chave a chave) e que o padrão de orientações ganhe duas linhas. Como toda escrita lê as
// tabelas inteiras, escritas simultâneas conflitam: o Postgres aborta uma (40001) e ela é
// refeita, até tentativasSerializacao vezes dentro do mesmo timeoutConsulta.
func (p *Postgres) alterar(ctx context.Context, operacao string, r regra, gravar gravacao) (Mapeamento, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	// escritas simultâneas no mesmo processo já conflitariam (40001); em fila, a versão
	// avisada ao cache segue a ordem dos commits
	p.escritas.Lock()
	defer p.escritas.Unlock()
	serializavel := &stdsql.TxOptions{Isolation: stdsql.LevelSerializable}
	aplicar := func(ctx context.Context, cli *ent.Client) (Mapeamento, error) { return alterarNa(ctx, cli, r, gravar) }
	for tentativa := 1; ; tentativa++ {
		m, err := emTransacao(ctx, p.cli, serializavel, aplicar)
		if err == nil {
			p.versao++
			if p.aoAlterar != nil {
				p.aoAlterar(m, p.versao)
			}
			return m, nil
		}
		if !falhaDeSerializacao(err) {
			return Mapeamento{}, fmt.Errorf("%s: %w", operacao, err)
		}
		if tentativa == tentativasSerializacao || esperarNovaTentativa(ctx, tentativa) != nil {
			return Mapeamento{}, fmt.Errorf("%s: %w após %d tentativas: %w", operacao, ErrConflitoConcorrente, tentativa, err)
		}
	}
}

func falhaDeSerializacao(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == codigoFalhaSerializacao
}

// esperarNovaTentativa espaça as tentativas com um pouco de acaso, para os dois lados do
// conflito não colidirem de novo no mesmo instante.
func esperarNovaTentativa(ctx context.Context, tentativa int) error {
	espera := time.Duration(tentativa) * (10*time.Millisecond + rand.N(20*time.Millisecond))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(espera):
		return nil
	}
}

func emTransacao(ctx context.Context, cli *ent.Client, opcoes *stdsql.TxOptions, fn func(context.Context, *ent.Client) (Mapeamento, error)) (Mapeamento, error) {
	tx, err := cli.BeginTx(ctx, opcoes)
	if err != nil {
		return Mapeamento{}, falha.ComPilha(fmt.Errorf("abrir transação: %w", err))
	}
	m, err := fn(ctx, tx.Client())
	if err != nil {
		if errDesfazer := tx.Rollback(); errDesfazer != nil {
			err = errors.Join(err, fmt.Errorf("desfazer transação: %w", errDesfazer))
		}
		return Mapeamento{}, err
	}
	if err := tx.Commit(); err != nil {
		return Mapeamento{}, falha.ComPilha(fmt.Errorf("confirmar transação: %w", err))
	}
	return m, nil
}

func alterarNa(ctx context.Context, cli *ent.Client, r regra, gravar gravacao) (Mapeamento, error) {
	atual, err := ler(ctx, cli)
	if err != nil {
		return Mapeamento{}, err
	}
	novo, err := r(atual)
	if err != nil {
		return Mapeamento{}, err
	}
	if err := gravar(ctx, cli, novo); err != nil {
		return Mapeamento{}, err
	}
	return ler(ctx, cli)
}

func (p *Postgres) SalvarAdministradora(ctx context.Context, a Administradora) (Mapeamento, error) {
	return p.gravarAdministradora(ctx, "salvar", a, Mapeamento.salvarAdministradora)
}

func (p *Postgres) CriarAdministradora(ctx context.Context, a Administradora) (Mapeamento, error) {
	return p.gravarAdministradora(ctx, "criar", a, Mapeamento.criarAdministradora)
}

func (p *Postgres) EditarAdministradora(ctx context.Context, a Administradora) (Mapeamento, error) {
	return p.gravarAdministradora(ctx, "editar", a, Mapeamento.editarAdministradora)
}

// gravarAdministradora aplica a regra e grava a administradora (upsert: a regra já decidiu se
// criar ou editar vale).
func (p *Postgres) gravarAdministradora(ctx context.Context, verbo string, a Administradora, r func(Mapeamento, Administradora) (Mapeamento, error)) (Mapeamento, error) {
	a, err := prepararAdministradora(a)
	if err != nil {
		return Mapeamento{}, err
	}
	return p.alterar(ctx, verbo+" administradora "+a.ID,
		func(m Mapeamento) (Mapeamento, error) { return r(m, a) },
		func(ctx context.Context, cli *ent.Client, _ Mapeamento) error {
			return cli.Administradora.Create().SetID(a.ID).SetNome(a.Nome).
				OnConflictColumns(entadministradora.FieldID).UpdateNewValues().Exec(ctx)
		})
}

func (p *Postgres) RemoverAdministradora(ctx context.Context, id string) (Mapeamento, error) {
	return p.alterar(ctx, "remover administradora "+id,
		func(m Mapeamento) (Mapeamento, error) { return m.removerAdministradora(id) },
		func(ctx context.Context, cli *ent.Client, _ Mapeamento) error {
			_, err := cli.Administradora.Delete().Where(entadministradora.ID(id)).Exec(ctx)
			return err
		})
}

func (p *Postgres) SalvarCondominio(ctx context.Context, c Condominio) (Mapeamento, error) {
	return p.gravarCondominio(ctx, "salvar", c, Mapeamento.salvarCondominio)
}

func (p *Postgres) CriarCondominio(ctx context.Context, c Condominio) (Mapeamento, error) {
	return p.gravarCondominio(ctx, "criar", c, Mapeamento.criarCondominio)
}

func (p *Postgres) EditarCondominio(ctx context.Context, c Condominio, visto Chaves) (Mapeamento, error) {
	return p.gravarCondominio(ctx, "editar", c, func(m Mapeamento, c Condominio) (Mapeamento, error) {
		return m.editarCondominio(c, visto)
	})
}

// gravarCondominio aplica a regra e grava o condomínio (upsert: a regra já decidiu se criar
// ou editar vale).
func (p *Postgres) gravarCondominio(ctx context.Context, verbo string, c Condominio, r func(Mapeamento, Condominio) (Mapeamento, error)) (Mapeamento, error) {
	c, err := prepararCondominio(c)
	if err != nil {
		return Mapeamento{}, err
	}
	return p.alterar(ctx, verbo+" condomínio "+c.ID,
		func(m Mapeamento) (Mapeamento, error) { return r(m, c) },
		func(ctx context.Context, cli *ent.Client, _ Mapeamento) error {
			upsert := cli.Condominio.Create().SetID(c.ID).SetNome(c.Nome).
				SetNillableAdministradoraID(opcional(c.Administradora)).
				SetAccount(c.Account).SetCompanyID(c.CompanyID).SetTenant(c.Tenant).
				OnConflictColumns(entcondominio.FieldID).UpdateNewValues()
			if c.Administradora == "" {
				// UpdateNewValues só regrava o que foi setado; sem dona, a coluna tem de virar NULL
				upsert = upsert.ClearAdministradoraID()
			}
			return upsert.Exec(ctx)
		})
}

func (p *Postgres) RemoverCondominio(ctx context.Context, alvo Alvo) (Mapeamento, error) {
	id := alvo.Condominio
	return p.alterar(ctx, "remover condomínio "+id,
		comDona(alvo, func(m Mapeamento) (Mapeamento, error) { return m.removerCondominio(id) }),
		func(ctx context.Context, cli *ent.Client, _ Mapeamento) error {
			// habilitações, payloads e orientações do condomínio saem pelo ON DELETE CASCADE
			_, err := cli.Condominio.Delete().Where(entcondominio.ID(id)).Exec(ctx)
			return err
		})
}

func (p *Postgres) SalvarHabilitacao(ctx context.Context, alvo Alvo, caso string, h Habilitacao, limpar bool) (Mapeamento, error) {
	condoID := alvo.Condominio
	return p.alterar(ctx, "salvar habilitação "+condoID+"/"+caso,
		comDona(alvo, func(m Mapeamento) (Mapeamento, error) { return m.salvarHabilitacao(condoID, caso, h, limpar) }),
		func(ctx context.Context, cli *ent.Client, novo Mapeamento) error {
			h, existe := novo.Habilitacoes[condoID][caso]
			if !existe {
				_, err := cli.Habilitacao.Delete().Where(doCondominio[predicate.Habilitacao](condoID), enthabilitacao.Caso(caso)).Exec(ctx)
				return err
			}
			return cli.Habilitacao.Create().SetNillableCondominioID(condominioID(condoID)).
				SetCaso(caso).SetHabilitado(h.Habilitado).SetPapeis(schema.Papeis(h.Papeis)).
				OnConflict(alvoUpsert(condoID, enthabilitacao.FieldCaso)...).UpdateNewValues().Exec(ctx)
		})
}

func (p *Postgres) SalvarOrientacao(ctx context.Context, alvo Alvo, o Orientacao) (Mapeamento, error) {
	condoID := alvo.Condominio
	return p.alterar(ctx, "salvar orientação "+condoID,
		comDona(alvo, func(m Mapeamento) (Mapeamento, error) { return m.salvarOrientacao(condoID, o) }),
		func(ctx context.Context, cli *ent.Client, novo Mapeamento) error {
			// por que não upsert: a do padrão (NULL) não tem índice único que sirva de alvo;
			// apagar e recriar é seguro na transação serializável
			if _, err := cli.Orientacao.Delete().Where(doCondominio[predicate.Orientacao](condoID)).Exec(ctx); err != nil {
				return err
			}
			o, existe := novo.Orientacoes[condoID]
			if !existe {
				return nil
			}
			return cli.Orientacao.Create().SetNillableCondominioID(condominioID(condoID)).
				SetGeral(o.Geral).SetPorCaso(o.PorCaso).Exec(ctx)
		})
}

func (p *Postgres) SalvarAjustes(ctx context.Context, alvo Alvo, ajustes map[string]map[string]any) (Mapeamento, error) {
	condoID := alvo.Condominio
	return p.alterar(ctx, "salvar ajustes "+condoID,
		comDona(alvo, func(m Mapeamento) (Mapeamento, error) { return m.salvarAjustes(condoID, ajustes) }),
		func(ctx context.Context, cli *ent.Client, novo Mapeamento) error {
			for recurso := range ajustes {
				if err := gravarPayload(ctx, cli, novo, condoID, recurso); err != nil {
					return err
				}
			}
			return nil
		})
}

func (p *Postgres) SalvarPayload(ctx context.Context, alvo Alvo, recurso string, dados map[string]any) (Mapeamento, error) {
	condoID := alvo.Condominio
	return p.alterar(ctx, "salvar payload "+condoID+"/"+recurso,
		comDona(alvo, func(m Mapeamento) (Mapeamento, error) { return m.salvarPayload(condoID, recurso, dados) }),
		func(ctx context.Context, cli *ent.Client, novo Mapeamento) error {
			return gravarPayload(ctx, cli, novo, condoID, recurso)
		})
}

// gravarPayload deixa a linha condomínio × recurso igual ao mapeamento novo.
func gravarPayload(ctx context.Context, cli *ent.Client, novo Mapeamento, condoID, recurso string) error {
	dados, existe := novo.Payloads[condoID][recurso]
	if !existe {
		_, err := cli.Payload.Delete().Where(doCondominio[predicate.Payload](condoID), payload.Recurso(recurso)).Exec(ctx)
		return err
	}
	return cli.Payload.Create().SetNillableCondominioID(condominioID(condoID)).
		SetRecurso(recurso).SetDados(dados).
		OnConflict(alvoUpsert(condoID, payload.FieldRecurso)...).UpdateNewValues().Exec(ctx)
}

func ler(ctx context.Context, cli *ent.Client) (Mapeamento, error) {
	var m Mapeamento
	adms, err := cli.Administradora.Query().Order(entadministradora.ByID()).All(ctx)
	if err != nil {
		return m, falha.ComPilha(fmt.Errorf("ler administradoras: %w", err))
	}
	condos, err := cli.Condominio.Query().Order(entcondominio.ByNome(), entcondominio.ByID()).All(ctx)
	if err != nil {
		return m, falha.ComPilha(fmt.Errorf("ler condomínios: %w", err))
	}
	habs, err := cli.Habilitacao.Query().All(ctx)
	if err != nil {
		return m, falha.ComPilha(fmt.Errorf("ler habilitações: %w", err))
	}
	pays, err := cli.Payload.Query().All(ctx)
	if err != nil {
		return m, falha.ComPilha(fmt.Errorf("ler payloads: %w", err))
	}
	oris, err := cli.Orientacao.Query().All(ctx)
	if err != nil {
		return m, falha.ComPilha(fmt.Errorf("ler orientações: %w", err))
	}
	return montar(adms, condos, habs, pays, oris), nil
}

// montar traduz as linhas do ent para o Mapeamento do domínio.
func montar(adms []*ent.Administradora, condos []*ent.Condominio, habs []*ent.Habilitacao, pays []*ent.Payload, oris []*ent.Orientacao) Mapeamento {
	m := Mapeamento{
		Habilitacoes: map[string]map[string]Habilitacao{},
		Payloads:     map[string]map[string]map[string]any{},
		Orientacoes:  map[string]Orientacao{},
	}
	for _, a := range adms {
		m.Administradoras = append(m.Administradoras, Administradora{ID: a.ID, Nome: a.Nome})
	}
	for _, c := range condos {
		m.Condominios = append(m.Condominios, Condominio{ID: c.ID, Nome: c.Nome, Administradora: valor(c.AdministradoraID),
			Account: c.Account, CompanyID: c.CompanyID, Tenant: c.Tenant})
	}
	for _, h := range habs {
		m.Habilitacoes = comEntrada(m.Habilitacoes, chave(h.CondominioID), h.Caso, Habilitacao{Habilitado: h.Habilitado, Papeis: h.Papeis}, false)
	}
	for _, p := range pays {
		m.Payloads = comEntrada(m.Payloads, chave(p.CondominioID), p.Recurso, p.Dados, false)
	}
	for _, o := range oris {
		m.Orientacoes[chave(o.CondominioID)] = Orientacao{Geral: o.Geral, PorCaso: o.PorCaso}
	}
	m.normalizar()
	return m
}

const colunaCondominio = "condominio_id"

// doCondominio filtra as linhas do condomínio, ou as do padrão (condominio_id NULL).
func doCondominio[P ~func(*sql.Selector)](condoID string) P {
	if condoID == Padrao {
		return P(sql.FieldIsNull(colunaCondominio))
	}
	return P(sql.FieldEQ(colunaCondominio, condoID))
}

// alvoUpsert aponta o ON CONFLICT para o índice parcial certo: o do padrão ou o dos condomínios.
func alvoUpsert(condoID, coluna string) []sql.ConflictOption {
	if condoID == Padrao {
		return []sql.ConflictOption{sql.ConflictColumns(coluna), sql.ConflictWhere(sql.IsNull(colunaCondominio))}
	}
	return []sql.ConflictOption{sql.ConflictColumns(colunaCondominio, coluna), sql.ConflictWhere(sql.NotNull(colunaCondominio))}
}

func condominioID(condoID string) *string {
	if condoID == Padrao {
		return nil
	}
	return &condoID
}

func chave(condoID *string) string {
	if condoID == nil {
		return Padrao
	}
	return *condoID
}

func opcional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func valor(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
