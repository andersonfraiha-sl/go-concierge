package conciergeapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"go-concierge/internal/core/banco"
	"go-concierge/internal/core/catalogo"
	okf "go-concierge/internal/core/catalogo/arquivos"
)

const timeoutPersistir = 5 * time.Second

func abrirCatalogo(semente string, db banco.Banco) (*okf.Catalogo, error) {
	pg := catalogo.NovoPostgres(db.Ent)
	ctx, cancel := context.WithTimeout(context.Background(), timeoutBoot)
	defer cancel()
	if err := semearOuAvisar(ctx, pg, semente); err != nil {
		return nil, fmt.Errorf("preparar catálogo: %w", err)
	}
	arquivos, err := pg.Listar(ctx)
	if err != nil {
		return nil, err
	}
	return okf.NovoEmMemoria(arquivos)
}

func semearOuAvisar(ctx context.Context, pg *catalogo.Postgres, semente string) error {
	vazio, err := pg.Vazio(ctx)
	if err != nil {
		return err
	}
	_, err = os.Stat(semente)
	if err != nil {
		if !vazio {
			if !errors.Is(err, os.ErrNotExist) {
				slog.WarnContext(ctx, "semente opcional indisponível; o banco continua sendo a fonte", "operacao", "semear_catalogo", "erro", err)
			}
			return nil
		}
		return fmt.Errorf("semente %s: %w (use -root com a pasta que tem agentes/ e nexus/)", semente, err)
	}
	naSemente, err := catalogo.LerDisco(semente)
	if err != nil {
		if !vazio {
			slog.WarnContext(ctx, "semente opcional indisponível; o banco continua sendo a fonte", "operacao", "semear_catalogo", "erro", err)
			return nil
		}
		return err
	}
	if vazio {
		if len(naSemente) == 0 {
			return fmt.Errorf("semente %s não contém arquivos de catálogo", semente)
		}
		m, err := pg.Substituir(ctx, naSemente)
		if err != nil {
			return err
		}
		slog.InfoContext(ctx, "catálogo semeado no banco a partir da pasta", "operacao", "semear_catalogo", "semente", semente, "arquivos", m.Gravados)
		return nil
	}
	noBanco, err := pg.Resumos(ctx)
	if err != nil {
		return err
	}
	for caminho := range noBanco {
		if strings.HasPrefix(caminho, "nexus/catalogo-nexus/") {
			delete(noBanco, caminho)
		}
	}
	naSemente = slices.DeleteFunc(naSemente, func(a catalogo.Arquivo) bool {
		return strings.HasPrefix(a.Caminho, "nexus/catalogo-nexus/")
	})
	gravar, remover := catalogo.Diferenca(noBanco, naSemente)
	if len(gravar)+len(remover) > 0 {
		slog.WarnContext(ctx, "a pasta do catálogo difere do banco; o banco vale e nada foi sobrescrito. Para levar a pasta ao banco, rode okf-admin importar-catalogo", "operacao", "semear_catalogo", "semente", semente, "diferentes", len(gravar), "ausentes_na_pasta", len(remover))
	}
	return nil
}

func migrarCatalogoNexusLegado(raiz, account string, pg *catalogo.Postgres) error {
	account = strings.TrimSpace(account)
	legado, err := okf.NewCatalogoNexusStore(raiz, account)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(legado.Path())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		slog.Warn("cópia Nexus legada indisponível; dados existentes foram preservados", "operacao", "migrar_catalogo_nexus", "erro", err)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutBoot)
	defer cancel()
	caminho := "nexus/catalogo-nexus/" + account + ".yaml"
	return pg.InserirCatalogoNexusSeAusente(ctx, catalogo.Arquivo{Caminho: caminho, Conteudo: raw})
}
