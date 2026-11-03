package memoria

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go-concierge/internal/banco"
	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/memoriacontato"
	"go-concierge/internal/banco/ent/memoriafato"
	"go-concierge/internal/banco/sqlc"
	"go-concierge/internal/escopo"
)

// timeoutConsulta limita cada leitura ou gravação da memória.
const timeoutConsulta = 3 * time.Second

// Postgres guarda a memória em duas tabelas: memoria_contatos (o nome e quando mudou, uma
// linha por contato) e memoria_fatos (uma linha por fato). Ler e Listar vão pelo ent; Gravar
// grava as duas num comando só do sqlc.
type Postgres struct {
	b banco.Banco
}

func NovoPostgres(b banco.Banco) *Postgres { return &Postgres{b: b} }

func (p *Postgres) Ler(ctx context.Context, c escopo.Contato) (Memoria, error) {
	if err := validarContato(c); err != nil {
		return Memoria{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	m, err := p.lerContato(ctx, c)
	if err != nil {
		return Memoria{}, fmt.Errorf("ler memória de %s: %w", c.Mascarado(), err)
	}
	linhas, err := p.b.Ent.MemoriaFato.Query().
		Where(memoriafato.AdministradoraID(c.AdministradoraID), memoriafato.Telefone(c.Telefone)).
		Order(ent.Asc(memoriafato.FieldID)).
		All(ctx)
	if err != nil {
		return Memoria{}, fmt.Errorf("ler fatos de %s: %w", c.Mascarado(), err)
	}
	m.Fatos = fatosDe(linhas)
	return m, nil
}

// lerContato devolve a memória sem fatos, com o nome e AtualizadaEm do contato, se houver.
func (p *Postgres) lerContato(ctx context.Context, c escopo.Contato) (Memoria, error) {
	linha, err := p.b.Ent.MemoriaContato.Query().
		Where(memoriacontato.AdministradoraID(c.AdministradoraID), memoriacontato.Telefone(c.Telefone)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return vazia(c), nil
	}
	if err != nil {
		return Memoria{}, err
	}
	m := vazia(c)
	m.Nome, m.AtualizadaEm = linha.Nome, linha.AtualizadaEm
	return m, nil
}

// Gravar troca a memória do contato pela recebida, num comando só: o nome e AtualizadaEm são
// regravados, as chaves que saíram são apagadas e as demais gravadas por cima.
func (p *Postgres) Gravar(ctx context.Context, c escopo.Contato, m Memoria) error {
	if err := validarContato(c); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	if err := p.b.SQL.GravarFatos(ctx, paramsDeGravacao(comoGuardada(c, m, time.Now()))); err != nil {
		return fmt.Errorf("gravar memória de %s: %w", c.Mascarado(), err)
	}
	return nil
}

// Listar devolve as memórias da administradora, a atualizada por último primeiro.
func (p *Postgres) Listar(ctx context.Context, administradoraID string) ([]Memoria, error) {
	if err := validarAdministradora(administradoraID); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	// ponytail: lista a administradora inteira, como a interface pede; paginar quando o Studio listar
	contatos, err := p.b.Ent.MemoriaContato.Query().
		Where(memoriacontato.AdministradoraID(administradoraID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar memórias da administradora %s: %w", administradoraID, err)
	}
	fatos, err := p.b.Ent.MemoriaFato.Query().
		Where(memoriafato.AdministradoraID(administradoraID)).
		Order(ent.Asc(memoriafato.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar fatos da administradora %s: %w", administradoraID, err)
	}
	return juntar(contatos, fatos), nil
}

// juntar monta uma memória por contato com os fatos do telefone dele.
func juntar(contatos []*ent.MemoriaContato, fatos []*ent.MemoriaFato) []Memoria {
	porTelefone := map[string][]*ent.MemoriaFato{}
	for _, f := range fatos {
		porTelefone[f.Telefone] = append(porTelefone[f.Telefone], f)
	}
	out := make([]Memoria, 0, len(contatos))
	for _, c := range contatos {
		out = append(out, Memoria{
			AdministradoraID: c.AdministradoraID, Telefone: c.Telefone, Nome: c.Nome,
			AtualizadaEm: c.AtualizadaEm, Fatos: fatosDe(porTelefone[c.Telefone]),
		})
	}
	slices.SortFunc(out, maisRecentesPrimeiro)
	return out
}

func fatosDe(linhas []*ent.MemoriaFato) []Fato {
	fatos := make([]Fato, 0, len(linhas))
	for _, l := range linhas {
		fatos = append(fatos, Fato{
			Chave: l.Chave, Texto: l.Texto, Fonte: l.Fonte, CondominioID: l.CondominioID,
			Quando: l.Quando, Confianca: confiancaDe(l.Confianca),
		})
	}
	return fatos
}

// confiancaDe devolve o real do banco como o float64 que foi gravado: 0.8 volta 0.8, e não
// 0.800000011920929.
func confiancaDe(real float32) float64 {
	// o texto vem do próprio FormatFloat: é sempre um número válido, o erro não acontece
	v, _ := strconv.ParseFloat(strconv.FormatFloat(float64(real), 'g', -1, 32), 64)
	return v
}

// paramsDeGravacao põe a memória guardada (comoGuardada: sem chave repetida, que o ON CONFLICT
// recusaria) em colunas; os arrays nunca são nil, que o pgx mandaria como NULL e a cláusula
// "chave <> ALL(NULL)" não apagaria nada.
func paramsDeGravacao(m Memoria) sqlc.GravarFatosParams {
	fatos := m.Fatos
	p := sqlc.GravarFatosParams{
		AdministradoraID: m.AdministradoraID,
		Telefone:         m.Telefone,
		Nome:             m.Nome,
		AtualizadaEm:     pgtype.Timestamptz{Time: m.AtualizadaEm, Valid: true},
		Chaves:           make([]string, 0, len(fatos)),
		Condominios:      make([]string, 0, len(fatos)),
		Textos:           make([]string, 0, len(fatos)),
		Fontes:           make([]string, 0, len(fatos)),
		Quandos:          make([]pgtype.Timestamptz, 0, len(fatos)),
		Confiancas:       make([]float32, 0, len(fatos)),
	}
	for _, f := range fatos {
		p.Chaves = append(p.Chaves, f.Chave)
		p.Condominios = append(p.Condominios, f.CondominioID)
		p.Textos = append(p.Textos, f.Texto)
		p.Fontes = append(p.Fontes, f.Fonte)
		p.Quandos = append(p.Quandos, pgtype.Timestamptz{Time: f.Quando, Valid: true})
		p.Confiancas = append(p.Confiancas, float32(f.Confianca))
	}
	return p
}
