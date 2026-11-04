package regras

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/ganchosregra"
	"go-concierge/internal/banco/ent/ganchosregraversao"
	"go-concierge/internal/banco/ent/schema"
	"go-concierge/internal/falha"
	"go-concierge/internal/ganchos"
)

// timeoutConsulta limita cada leitura ou escrita das regras (vale também para as rotas HTTP).
const timeoutConsulta = 3 * time.Second

// Postgres guarda as regras em ganchos_regras (estado atual) e ganchos_regras_versoes (cada
// versão gravada, só acrescenta).
type Postgres struct {
	cli *ent.Client
	// aoAlterar recebe as regras de cada gravação depois do commit (o Cache), num aviso só por
	// transação. A versão vem do banco e cresce por regra, então quem recebe pode descartar um
	// aviso atrasado.
	aoAlterar func([]RegraGravada)
}

func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

// Salvar grava r se ninguém gravou a regra depois da versaoLida (0 = regra nova) e devolve a
// regra com a versão e a data novas. Versão desatualizada dá ErrVersaoVelha.
func (p *Postgres) Salvar(ctx context.Context, r RegraGravada, versaoLida int) (RegraGravada, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	r.Versao = versaoLida + 1
	r.AlteradaEm = time.Now().UTC().Truncate(time.Microsecond) // precisão do timestamptz
	salva, err := emTransacao(ctx, p.cli, func(tx *ent.Tx) (RegraGravada, error) { return gravar(ctx, tx, r, versaoLida) })
	if err != nil {
		return RegraGravada{}, fmt.Errorf("salvar regra %q: %w", r.Nome, err)
	}
	p.avisar([]RegraGravada{salva})
	return salva, nil
}

// timeoutLote limita a transação de SalvarVarias: até 1000 regras, três comandos cada.
const timeoutLote = 30 * time.Second

// SalvarVarias grava rs numa transação só (tudo ou nada), cada uma contra a versão lida de
// mesmo índice (0 = regra nova, com o id de r quando vier preenchido), e avisa uma vez depois
// do commit. Uma versão desatualizada desfaz tudo com ErrVersaoVelha.
func (p *Postgres) SalvarVarias(ctx context.Context, rs []RegraGravada, versoesLidas []int) ([]RegraGravada, error) {
	if len(rs) != len(versoesLidas) {
		return nil, fmt.Errorf("salvar %d regras com %d versões lidas", len(rs), len(versoesLidas))
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutLote)
	defer cancel()
	agora := time.Now().UTC().Truncate(time.Microsecond)
	salvas, err := emTransacao(ctx, p.cli, func(tx *ent.Tx) ([]RegraGravada, error) {
		out := make([]RegraGravada, len(rs))
		for i, r := range rs {
			r.Versao, r.AlteradaEm = versoesLidas[i]+1, agora
			salva, err := gravar(ctx, tx, r, versoesLidas[i])
			if err != nil {
				return nil, fmt.Errorf("regra %d (%q): %w", i, r.Nome, err)
			}
			out[i] = salva
		}
		return out, nil
	})
	if err != nil {
		return nil, fmt.Errorf("salvar %d regras: %w", len(rs), err)
	}
	p.avisar(salvas)
	return salvas, nil
}

func (p *Postgres) avisar(rs []RegraGravada) {
	if p.aoAlterar != nil {
		p.aoAlterar(rs)
	}
}

// VersoesAtuais devolve a versão de cada id que já existe; os que não existem ficam fora.
func (p *Postgres) VersoesAtuais(ctx context.Context, ids []string) (map[string]int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	uids := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		uid, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("versões atuais: id %q: %w", id, err)
		}
		uids[i] = uid
	}
	linhas, err := p.cli.GanchosRegra.Query().Where(ganchosregra.IDIn(uids...)).
		Select(ganchosregra.FieldID, ganchosregra.FieldVersao).All(ctx)
	if err != nil {
		return nil, falha.ComPilha(fmt.Errorf("versões atuais: %w", err))
	}
	versoes := make(map[string]int, len(linhas))
	for _, l := range linhas {
		versoes[l.ID.String()] = l.Versao
	}
	return versoes, nil
}

func gravar(ctx context.Context, tx *ent.Tx, r RegraGravada, versaoLida int) (RegraGravada, error) {
	id, err := gravarRegra(ctx, tx, r, versaoLida)
	if err != nil {
		return RegraGravada{}, err
	}
	r.ID = id.String()
	err = tx.GanchosRegraVersao.Create().
		SetRegraID(id).
		SetVersao(r.Versao).
		SetDefinicao(r.Definicao).
		SetAlteradaPor(r.AlteradaPor).
		SetAlteradaEm(r.AlteradaEm).
		Exec(ctx)
	if err != nil {
		return RegraGravada{}, fmt.Errorf("gravar versão %d: %w", r.Versao, err)
	}
	return r, nil
}

func gravarRegra(ctx context.Context, tx *ent.Tx, r RegraGravada, versaoLida int) (uuid.UUID, error) {
	if versaoLida == 0 {
		return criarRegra(ctx, tx, r)
	}
	id, err := uuid.Parse(r.ID)
	if err != nil {
		return uuid.Nil, ErrRegraDesconhecida
	}
	alteradas, err := tx.GanchosRegra.Update().
		Where(ganchosregra.IDEQ(id), ganchosregra.VersaoEQ(versaoLida)).
		SetVersao(r.Versao).
		SetNome(r.Nome).
		SetEvento(string(r.Evento)).
		SetAdministradoraID(r.AdministradoraID).
		SetCondominios(schema.GanchosCondominios(r.Condominios)).
		SetAtiva(r.Ativa).
		SetDefinicao(r.Definicao).
		SetAlteradaPor(r.AlteradaPor).
		SetAlteradaEm(r.AlteradaEm).
		Save(ctx)
	if err != nil {
		return uuid.Nil, falha.ComPilha(fmt.Errorf("atualizar regra: %w", err))
	}
	if alteradas > 0 {
		return id, nil
	}
	existe, err := tx.GanchosRegra.Query().Where(ganchosregra.IDEQ(id)).Exist(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("conferir regra: %w", err)
	}
	if !existe {
		return uuid.Nil, ErrRegraDesconhecida
	}
	return uuid.Nil, ErrVersaoVelha
}

// criarRegra usa o id de r quando vem preenchido (importação) e gera um quando vem vazio.
func criarRegra(ctx context.Context, tx *ent.Tx, r RegraGravada) (uuid.UUID, error) {
	criar := tx.GanchosRegra.Create().
		SetVersao(r.Versao).
		SetNome(r.Nome).
		SetEvento(string(r.Evento)).
		SetAdministradoraID(r.AdministradoraID).
		SetCondominios(schema.GanchosCondominios(r.Condominios)).
		SetAtiva(r.Ativa).
		SetDefinicao(r.Definicao).
		SetAlteradaPor(r.AlteradaPor).
		SetAlteradaEm(r.AlteradaEm)
	if r.ID != "" {
		id, err := uuid.Parse(r.ID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("id %q: %w", r.ID, err)
		}
		criar.SetID(id)
	}
	criada, err := criar.Save(ctx)
	if ent.IsConstraintError(err) {
		// o id do arquivo já foi criado por outra gravação depois da versão lida (0)
		return uuid.Nil, ErrVersaoVelha
	}
	if err != nil {
		return uuid.Nil, falha.ComPilha(fmt.Errorf("criar regra: %w", err))
	}
	return criada.ID, nil
}

func (p *Postgres) Ler(ctx context.Context, id string) (RegraGravada, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	linha, err := lerLinha(ctx, p.cli, id)
	if err != nil {
		return RegraGravada{}, fmt.Errorf("ler regra %q: %w", id, err)
	}
	return deLinha(linha), nil
}

// Limites da página de Listar.
const (
	limiteMinimo = 1
	limiteMaximo = 200
)

// Listar devolve uma página das regras, ativas ou não, em ordem de id, começando depois do id
// depois ("" = primeira página). limite fica entre 1 e 200.
func (p *Postgres) Listar(ctx context.Context, depois string, limite int) ([]RegraGravada, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	consulta := p.cli.GanchosRegra.Query().Limit(min(max(limite, limiteMinimo), limiteMaximo))
	if depois != "" {
		cursor, err := uuid.Parse(depois)
		if err != nil {
			return nil, falha.ComPilha(fmt.Errorf("listar regras depois de %q: %w", depois, err))
		}
		consulta.Where(ganchosregra.IDGT(cursor))
	}
	return lerRegras(ctx, consulta, "listar regras")
}

// Ativas devolve só as regras ativas, para o cache.
func (p *Postgres) Ativas(ctx context.Context) ([]RegraGravada, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	return p.ativas(ctx)
}

// ativas lê no prazo de quem chama, sem o teto de 3 s das rotas (o boot tem 5 s).
// ponytail: sem LIMIT; o teto de 10 mil ativas (spec §7) segura o tamanho.
func (p *Postgres) ativas(ctx context.Context) ([]RegraGravada, error) {
	return lerRegras(ctx, p.cli.GanchosRegra.Query().Where(ganchosregra.AtivaEQ(true)), "ler regras ativas")
}

func lerRegras(ctx context.Context, consulta *ent.GanchosRegraQuery, operacao string) ([]RegraGravada, error) {
	linhas, err := consulta.Order(ent.Asc(ganchosregra.FieldID)).All(ctx)
	if err != nil {
		return nil, falha.ComPilha(fmt.Errorf("%s: %w", operacao, err))
	}
	regras := make([]RegraGravada, len(linhas))
	for i, linha := range linhas {
		regras[i] = deLinha(linha)
	}
	return regras, nil
}

// LerVersao devolve uma versão gravada da regra, só com o que a tabela de versões guarda.
func (p *Postgres) LerVersao(ctx context.Context, id string, versao int) (VersaoGravada, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	uid, err := uuid.Parse(id)
	if err != nil {
		return VersaoGravada{}, fmt.Errorf("ler regra %q versão %d: %w", id, versao, ErrRegraDesconhecida)
	}
	gravada, err := p.cli.GanchosRegraVersao.Query().
		Where(ganchosregraversao.RegraIDEQ(uid), ganchosregraversao.VersaoEQ(versao)).
		Only(ctx)
	if err != nil {
		return VersaoGravada{}, fmt.Errorf("ler regra %q versão %d: %w", id, versao, traduzir(err))
	}
	return VersaoGravada{
		RegraID:     id,
		Versao:      gravada.Versao,
		Definicao:   gravada.Definicao,
		AlteradaPor: gravada.AlteradaPor,
		AlteradaEm:  gravada.AlteradaEm,
	}, nil
}

func lerLinha(ctx context.Context, cli *ent.Client, id string) (*ent.GanchosRegra, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, ErrRegraDesconhecida
	}
	linha, err := cli.GanchosRegra.Get(ctx, uid)
	return linha, traduzir(err)
}

func traduzir(err error) error {
	if ent.IsNotFound(err) {
		return ErrRegraDesconhecida
	}
	return err
}

func deLinha(l *ent.GanchosRegra) RegraGravada {
	return RegraGravada{
		ID:               l.ID.String(),
		Versao:           l.Versao,
		Nome:             l.Nome,
		Evento:           ganchos.NomeDoEvento(l.Evento),
		AdministradoraID: l.AdministradoraID,
		Condominios:      []string(l.Condominios),
		Ativa:            l.Ativa,
		Definicao:        l.Definicao,
		AlteradaPor:      l.AlteradaPor,
		AlteradaEm:       l.AlteradaEm,
	}
}

func emTransacao[T any](ctx context.Context, cli *ent.Client, fn func(*ent.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := cli.Tx(ctx)
	if err != nil {
		return zero, falha.ComPilha(fmt.Errorf("abrir transação: %w", err))
	}
	r, err := fn(tx)
	if err != nil {
		if errDesfazer := tx.Rollback(); errDesfazer != nil {
			err = errors.Join(err, fmt.Errorf("desfazer transação: %w", errDesfazer))
		}
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, falha.ComPilha(fmt.Errorf("confirmar transação: %w", err))
	}
	return r, nil
}
