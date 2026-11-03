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
	"go-concierge/internal/banco/ent/memoriafato"
	"go-concierge/internal/banco/sqlc"
	"go-concierge/internal/escopo"
)

// timeoutConsulta limita cada leitura ou gravação da memória.
const timeoutConsulta = 3 * time.Second

// Postgres guarda a memória na tabela memoria_fatos, uma linha por fato. Ler e Listar vão
// pelo ent; Gravar pelo sqlc, que troca os fatos do contato num comando só.
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
	linhas, err := p.b.Ent.MemoriaFato.Query().
		Where(memoriafato.AdministradoraID(c.AdministradoraID), memoriafato.Telefone(c.Telefone)).
		Order(ent.Asc(memoriafato.FieldID)).
		All(ctx)
	if err != nil {
		return Memoria{}, fmt.Errorf("ler memória de %s: %w", c.Mascarado(), err)
	}
	return montar(c, fatosDe(linhas)), nil
}

// Gravar troca os fatos do contato pelos da memória: as chaves que saíram são apagadas e as
// demais gravadas por cima. A administradora e o telefone são os do contato.
func (p *Postgres) Gravar(ctx context.Context, c escopo.Contato, m Memoria) error {
	if err := validarContato(c); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutConsulta)
	defer cancel()
	if err := p.b.SQL.GravarFatos(ctx, paramsDeGravacao(c, m.Fatos)); err != nil {
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
	linhas, err := p.b.Ent.MemoriaFato.Query().
		Where(memoriafato.AdministradoraID(administradoraID)).
		Order(ent.Asc(memoriafato.FieldTelefone), ent.Asc(memoriafato.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listar memórias da administradora %s: %w", administradoraID, err)
	}
	return agruparPorTelefone(administradoraID, linhas), nil
}

// agruparPorTelefone junta as linhas, já ordenadas por telefone, numa memória por contato.
func agruparPorTelefone(administradoraID string, linhas []*ent.MemoriaFato) []Memoria {
	out := []Memoria{}
	for inicio := 0; inicio < len(linhas); {
		telefone := linhas[inicio].Telefone
		fim := inicio + 1
		for fim < len(linhas) && linhas[fim].Telefone == telefone {
			fim++
		}
		c := escopo.Contato{AdministradoraID: administradoraID, Telefone: telefone}
		out = append(out, montar(c, fatosDe(linhas[inicio:fim])))
		inicio = fim
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
	v, err := strconv.ParseFloat(strconv.FormatFloat(float64(real), 'g', -1, 32), 64)
	if err != nil {
		return float64(real)
	}
	return v
}

// paramsDeGravacao põe os fatos em colunas; os arrays nunca são nil, que o pgx mandaria como
// NULL e a cláusula "chave <> ALL(NULL)" não apagaria nada.
func paramsDeGravacao(c escopo.Contato, fatos []Fato) sqlc.GravarFatosParams {
	p := sqlc.GravarFatosParams{
		AdministradoraID: c.AdministradoraID,
		Telefone:         c.Telefone,
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
