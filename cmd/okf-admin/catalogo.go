package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"go-concierge/internal/banco"
	"go-concierge/internal/catalogo"
	"go-concierge/internal/okf"
)

const (
	// o banco tem o catálogo; o disco em -root é a semente e a cópia de trabalho fica ao lado.
	subpastaDaCopia  = "var/catalogo"
	timeoutPersistir = 5 * time.Second
)

// abrirCatalogo põe o catálogo do Postgres em disco e o abre. semente é o -root: a pasta com
// agentes/ e nexus/ que vem no repositório ou na imagem. Toda escrita no catálogo volta ao
// banco pelo gancho Persistir.
func abrirCatalogo(semente string, db banco.Banco) (*okf.Catalogo, error) {
	copia := filepath.Join(semente, filepath.FromSlash(subpastaDaCopia))
	pg := catalogo.NovoPostgres(db.Ent)
	ctx, cancel := context.WithTimeout(context.Background(), timeoutBoot)
	defer cancel()
	if err := prepararCopia(ctx, pg, semente, copia); err != nil {
		return nil, err
	}
	cat, err := okf.NewCatalogo(copia)
	if err != nil {
		return nil, fmt.Errorf("abrir catálogo: %w", err)
	}
	cat.Persistir = persistirNoBanco(pg, cat.Root)
	return cat, nil
}

// prepararCopia: tabela vazia recebe a semente; com dado, o banco vale e a semente diferente
// só gera aviso. Depois refaz a cópia de trabalho a partir do banco.
func prepararCopia(ctx context.Context, pg *catalogo.Postgres, semente, copia string) error {
	if _, err := os.Stat(semente); err != nil {
		return fmt.Errorf("preparar catálogo: semente %s: %w (use -root com a pasta que tem agentes/ e nexus/)", semente, err)
	}
	if err := semearOuAvisar(ctx, pg, semente); err != nil {
		return fmt.Errorf("preparar catálogo: %w (confira DATABASE_URL e as migrations)", err)
	}
	arquivos, err := pg.Listar(ctx)
	if err != nil {
		return fmt.Errorf("preparar catálogo: %w", err)
	}
	if err := os.MkdirAll(copia, 0o755); err != nil {
		return fmt.Errorf("preparar catálogo: criar %s: %w", copia, err)
	}
	if err := catalogo.Materializar(copia, arquivos); err != nil {
		return fmt.Errorf("preparar catálogo: %w", err)
	}
	return nil
}

func semearOuAvisar(ctx context.Context, pg *catalogo.Postgres, semente string) error {
	vazio, err := pg.Vazio(ctx)
	if err != nil {
		return err
	}
	naSemente, err := catalogo.LerDisco(semente)
	if err != nil {
		return err
	}
	if vazio {
		m, err := pg.Substituir(ctx, naSemente)
		if err != nil {
			return err
		}
		slog.InfoContext(ctx, "catálogo semeado no banco a partir da pasta", "operacao", "semear_catalogo",
			"semente", semente, "arquivos", m.Gravados)
		return nil
	}
	noBanco, err := pg.Resumos(ctx)
	if err != nil {
		return err
	}
	gravar, remover := catalogo.Diferenca(noBanco, naSemente)
	if len(gravar)+len(remover) > 0 {
		slog.WarnContext(ctx, "a pasta do catálogo difere do banco; o banco vale e nada foi sobrescrito. "+
			"Para levar a pasta ao banco, rode okf-admin importar-catalogo",
			"operacao", "semear_catalogo", "semente", semente, "diferentes", len(gravar), "ausentes_na_pasta", len(remover))
	}
	return nil
}

// persistirNoBanco é o gancho do okf.Catalogo: lê a cópia de trabalho inteira e deixa o banco
// igual. A comparação é do catálogo todo, então uma falha de banco se corrige na próxima
// escrita que der certo.
func persistirNoBanco(pg *catalogo.Postgres, copia string) func() error {
	return func() error {
		noDisco, err := catalogo.LerDisco(copia)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeoutPersistir)
		defer cancel()
		_, err = pg.Substituir(ctx, noDisco)
		return err
	}
}
