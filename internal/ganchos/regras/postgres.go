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
	"go-concierge/internal/banco/ent/predicate"
	"go-concierge/internal/banco/ent/schema"
	"go-concierge/internal/ganchos"
)

// timeoutConsulta limita cada leitura ou escrita das regras (vale também para as rotas HTTP).
const timeoutConsulta = 3 * time.Second

// Postgres guarda as regras em ganchos_regras (estado atual) e ganchos_regras_versoes (cada
// versão gravada, só acrescenta).
type Postgres struct {
	cli *ent.Client
	// aoAlterar recebe cada regra gravada depois do commit (o Cache). A versão vem do banco
	// e cresce por regra, então quem recebe pode descartar um aviso atrasado.
	aoAlterar func(RegraGravada)
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
	if p.aoAlterar != nil {
		p.aoAlterar(salva)
	}
	return salva, nil
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
		return uuid.Nil, fmt.Errorf("atualizar regra: %w", err)
	}
	if alteradas == 0 {
		return uuid.Nil, ErrVersaoVelha
	}
	return id, nil
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
	if err != nil {
		return uuid.Nil, fmt.Errorf("criar regra: %w", err)
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

// Listar devolve todas as regras, ativas ou não, para a tela.
func (p *Postgres) Listar(ctx context.Context) ([]RegraGravada, error) {
	return p.listar(ctx, "listar regras")
}

// Ativas devolve só as regras ativas, para o cache.
func (p *Postgres) Ativas(ctx context.Context) ([]RegraGravada, error) {
	return p.listar(ctx, "ler regras ativas", ganchosregra.AtivaEQ(true))
}

// ponytail: sem LIMIT; o teto de 10 mil ativas (spec §7) segura o tamanho. Paginar quando a
// tela precisar.
func (p *Postgres) listar(ctx context.Context, operacao string, filtros ...predicate.GanchosRegra) ([]RegraGravada, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	linhas, err := p.cli.GanchosRegra.Query().Where(filtros...).Order(ent.Asc(ganchosregra.FieldID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", operacao, err)
	}
	regras := make([]RegraGravada, len(linhas))
	for i, linha := range linhas {
		regras[i] = deLinha(linha)
	}
	return regras, nil
}

// LerVersao devolve a regra como era na versão pedida: definição, autor e data dessa versão,
// com nome, evento e alvo atuais (a tabela de versões só guarda o que a auditoria mostra).
func (p *Postgres) LerVersao(ctx context.Context, id string, versao int) (RegraGravada, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	linha, err := lerLinha(ctx, p.cli, id)
	if err != nil {
		return RegraGravada{}, fmt.Errorf("ler regra %q versão %d: %w", id, versao, err)
	}
	gravada, err := p.cli.GanchosRegraVersao.Query().
		Where(ganchosregraversao.RegraIDEQ(linha.ID), ganchosregraversao.VersaoEQ(versao)).
		Only(ctx)
	if err != nil {
		return RegraGravada{}, fmt.Errorf("ler regra %q versão %d: %w", id, versao, traduzir(err))
	}
	r := deLinha(linha)
	r.Versao, r.Definicao, r.AlteradaPor, r.AlteradaEm = gravada.Versao, gravada.Definicao, gravada.AlteradaPor, gravada.AlteradaEm
	return r, nil
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

func emTransacao(ctx context.Context, cli *ent.Client, fn func(*ent.Tx) (RegraGravada, error)) (RegraGravada, error) {
	tx, err := cli.Tx(ctx)
	if err != nil {
		return RegraGravada{}, fmt.Errorf("abrir transação: %w", err)
	}
	r, err := fn(tx)
	if err != nil {
		if errDesfazer := tx.Rollback(); errDesfazer != nil {
			err = errors.Join(err, fmt.Errorf("desfazer transação: %w", errDesfazer))
		}
		return RegraGravada{}, err
	}
	if err := tx.Commit(); err != nil {
		return RegraGravada{}, fmt.Errorf("confirmar transação: %w", err)
	}
	return r, nil
}
