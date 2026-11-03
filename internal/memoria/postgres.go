package memoria

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
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
// linha por contato) e memoria_fatos (uma linha por fato). Ler lê as duas numa query pgx, que
// cabe no lote do turno; Listar vai pelo ent; Gravar grava as duas num comando só do sqlc.
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
	var m Memoria
	lote := &pgx.Batch{}
	p.LerNoLote(lote, c, func(lida Memoria) { m = lida })
	if err := p.b.Pool.SendBatch(ctx, lote).Close(); err != nil {
		return Memoria{}, fmt.Errorf("ler memória de %s: %w", c.Mascarado(), err)
	}
	return m, nil
}

// sqlLerMemoria traz o contato e os fatos numa query só. Por que não ent nem sqlc: entra no
// pgx.Batch que carrega o turno a frio (sessão, memória e pedidos numa ida), e o sqlc não
// junta queries diferentes num lote. Sem linha de contato não há fatos (Gravar grava os dois).
const sqlLerMemoria = `SELECT c.nome, c.atualizada_em, f.chave, f.texto, f.fonte, f.condominio_id, f.quando, f.confianca
FROM memoria_contatos c
LEFT JOIN memoria_fatos f ON f.administradora_id = c.administradora_id AND f.telefone = c.telefone
WHERE c.administradora_id = $1 AND c.telefone = $2
ORDER BY f.id`

// LerNoLote põe no lote a leitura da memória do contato; guardar recebe a memória quando o
// lote roda. Um erro de leitura volta no Close do lote.
func (p *Postgres) LerNoLote(lote *pgx.Batch, c escopo.Contato, guardar func(Memoria)) {
	lote.Queue(sqlLerMemoria, c.AdministradoraID, c.Telefone).Query(func(rows pgx.Rows) error {
		m, err := memoriaDasLinhas(c, rows)
		if err != nil {
			return err
		}
		guardar(m)
		return nil
	})
}

func memoriaDasLinhas(c escopo.Contato, rows pgx.Rows) (Memoria, error) {
	m := vazia(c)
	for rows.Next() {
		var chave, texto, fonte, condominio pgtype.Text
		var quando pgtype.Timestamptz
		var confianca pgtype.Float4
		if err := rows.Scan(&m.Nome, &m.AtualizadaEm, &chave, &texto, &fonte, &condominio, &quando, &confianca); err != nil {
			return Memoria{}, err
		}
		if !chave.Valid { // contato sem fatos
			continue
		}
		m.Fatos = append(m.Fatos, Fato{
			Chave: chave.String, Texto: texto.String, Fonte: fonte.String, CondominioID: condominio.String,
			Quando: quando.Time, Confianca: confiancaDe(confianca.Float32),
		})
	}
	return m, rows.Err()
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
