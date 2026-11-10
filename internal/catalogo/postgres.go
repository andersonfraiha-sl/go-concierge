package catalogo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"go-concierge/internal/banco/ent"
	entcatalogo "go-concierge/internal/banco/ent/catalogoarquivo"
	"go-concierge/internal/falha"
)

const (
	// teto de cada ida ao banco; o chamador pode apertar com o próprio contexto.
	timeoutCatalogo = 30 * time.Second
	// o upsert vai em lotes para o pedido não crescer com o catálogo inteiro.
	arquivosPorLote = 100
)

// Postgres guarda o catálogo na tabela catalogo_arquivos.
type Postgres struct{ cli *ent.Client }

// NovoPostgres usa cli, que o chamador abre e fecha.
func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

// Mudanca conta o que Substituir levou ao banco.
type Mudanca struct{ Gravados, Removidos int }

// Listar devolve todos os arquivos do catálogo, ordenados por caminho.
func (p *Postgres) Listar(ctx context.Context) ([]Arquivo, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	linhas, err := p.cli.CatalogoArquivo.Query().Order(ent.Asc(entcatalogo.FieldID)).All(ctx)
	if err != nil {
		return nil, falha.ComPilha(fmt.Errorf("listar catálogo: %w", err))
	}
	out := make([]Arquivo, len(linhas))
	for i, l := range linhas {
		out[i] = Arquivo{Caminho: l.ID, Conteudo: l.Conteudo}
	}
	return out, nil
}

// Resumos devolve caminho → sha256 de tudo que está no banco, sem trazer os bytes.
func (p *Postgres) Resumos(ctx context.Context) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	return resumos(ctx, p.cli.CatalogoArquivo)
}

// Vazio diz se a tabela não tem nenhuma linha.
func (p *Postgres) Vazio(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	existe, err := p.cli.CatalogoArquivo.Query().Exist(ctx)
	if err != nil {
		return false, falha.ComPilha(fmt.Errorf("conferir se o catálogo está vazio: %w", err))
	}
	return !existe, nil
}

// Substituir faz o banco ficar igual a noDisco, numa transação: grava o que é novo ou
// mudou e remove o que sumiu (ver Diferenca). Repetir com o mesmo disco não escreve nada.
// Um caminho inválido recusa tudo antes de abrir a transação.
func (p *Postgres) Substituir(ctx context.Context, noDisco []Arquivo) (Mudanca, error) {
	for _, a := range noDisco {
		if err := ValidarCaminho(a.Caminho); err != nil {
			return Mudanca{}, fmt.Errorf("substituir catálogo: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	tx, err := p.cli.Tx(ctx)
	if err != nil {
		return Mudanca{}, falha.ComPilha(fmt.Errorf("substituir catálogo: abrir transação: %w", err))
	}
	mudanca, err := aplicar(ctx, tx, noDisco)
	if err != nil {
		if errDesfazer := tx.Rollback(); errDesfazer != nil {
			err = errors.Join(err, fmt.Errorf("desfazer transação: %w", errDesfazer))
		}
		return Mudanca{}, falha.ComPilha(fmt.Errorf("substituir catálogo: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return Mudanca{}, falha.ComPilha(fmt.Errorf("substituir catálogo: confirmar transação: %w", err))
	}
	return mudanca, nil
}

func aplicar(ctx context.Context, tx *ent.Tx, noDisco []Arquivo) (Mudanca, error) {
	atuais, err := resumos(ctx, tx.CatalogoArquivo)
	if err != nil {
		return Mudanca{}, err
	}
	gravar, remover := Diferenca(atuais, noDisco)
	agora := time.Now().UTC()
	for lote := range slices.Chunk(gravar, arquivosPorLote) {
		if err := gravarLote(ctx, tx, lote, agora); err != nil {
			return Mudanca{}, err
		}
	}
	if len(remover) > 0 {
		if _, err := tx.CatalogoArquivo.Delete().Where(entcatalogo.IDIn(remover...)).Exec(ctx); err != nil {
			return Mudanca{}, fmt.Errorf("remover %d arquivo(s): %w", len(remover), err)
		}
	}
	return Mudanca{Gravados: len(gravar), Removidos: len(remover)}, nil
}

func gravarLote(ctx context.Context, tx *ent.Tx, lote []Arquivo, agora time.Time) error {
	criar := make([]*ent.CatalogoArquivoCreate, len(lote))
	for i, a := range lote {
		criar[i] = tx.CatalogoArquivo.Create().SetID(a.Caminho).SetConteudo(a.Conteudo).
			SetSha256(SHA256(a.Conteudo)).SetAtualizadoEm(agora)
	}
	err := tx.CatalogoArquivo.CreateBulk(criar...).OnConflictColumns(entcatalogo.FieldID).UpdateNewValues().Exec(ctx)
	if err != nil {
		return fmt.Errorf("gravar %d arquivo(s): %w", len(lote), err)
	}
	return nil
}

func resumos(ctx context.Context, cli *ent.CatalogoArquivoClient) (map[string]string, error) {
	var linhas []struct {
		ID     string `json:"caminho"`
		Sha256 string `json:"sha256"`
	}
	if err := cli.Query().Select(entcatalogo.FieldID, entcatalogo.FieldSha256).Scan(ctx, &linhas); err != nil {
		return nil, falha.ComPilha(fmt.Errorf("ler resumos do catálogo: %w", err))
	}
	out := make(map[string]string, len(linhas))
	for _, l := range linhas {
		out[l.ID] = l.Sha256
	}
	return out, nil
}
