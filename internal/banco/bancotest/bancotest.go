// Package bancotest dá a cada teste um Postgres 18 próprio e já migrado. Um contêiner sobe
// por pacote de teste, no primeiro Novo; cada teste ganha um banco novo, copiado de um
// template migrado uma vez só, e o banco some no fim do teste. Chame Encerrar no TestMain.
package bancotest

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"go-concierge/internal/banco"
)

const (
	imagem         = "postgres:18-alpine"
	bancoModelo    = "concierge_modelo"
	timeoutInicio  = 3 * time.Minute
	timeoutComando = 30 * time.Second
)

type instancia struct {
	conteiner *postgres.PostgresContainer
	admin     *url.URL // aponta para o banco padrão do contêiner, de onde se criam os outros
}

// estado do pacote de teste: o contêiner é caro demais para subir por teste.
var (
	mu    sync.Mutex
	atual *instancia
)

// Novo cria um banco vazio de dados, com todas as migrations aplicadas, e o remove no fim do
// teste. ajustes vão para banco.Abrir.
func Novo(t testing.TB, ajustes ...func(*pgxpool.Config)) banco.Banco {
	t.Helper()
	inst, err := iniciar()
	if err != nil {
		t.Fatalf("subir postgres de teste: %v", err)
	}
	nome := "t_" + aleatorio()
	if err := inst.executar(t.Context(), "CREATE DATABASE "+ident(nome)+" TEMPLATE "+ident(bancoModelo)); err != nil {
		t.Fatalf("criar banco %s: %v", nome, err)
	}
	b, err := banco.Abrir(t.Context(), inst.url(nome), ajustes...)
	if err != nil {
		t.Fatalf("abrir banco %s: %v", nome, err)
	}
	t.Cleanup(func() {
		if err := b.Fechar(); err != nil {
			t.Errorf("fechar banco %s: %v", nome, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeoutComando)
		defer cancel()
		if err := inst.executar(ctx, "DROP DATABASE "+ident(nome)+" WITH (FORCE)"); err != nil {
			t.Errorf("remover banco %s: %v", nome, err)
		}
	})
	return b
}

// Encerrar derruba o contêiner, se algum teste o subiu. Chame no TestMain, depois de m.Run.
func Encerrar() {
	mu.Lock()
	defer mu.Unlock()
	if atual == nil {
		return
	}
	if err := testcontainers.TerminateContainer(atual.conteiner); err != nil {
		// o Ryuk do testcontainers remove o contêiner mesmo assim; fica o aviso
		fmt.Printf("bancotest: encerrar postgres: %v\n", err)
	}
	atual = nil
}

func iniciar() (*instancia, error) {
	mu.Lock()
	defer mu.Unlock()
	if atual != nil {
		return atual, nil
	}
	if err := sessaoDoProcesso(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutInicio)
	defer cancel()
	inst, err := subir(ctx)
	if err != nil {
		return nil, err
	}
	atual = inst
	return inst, nil
}

// variavelSessao é a sessão do testcontainers, que dá nome e rótulo ao Ryuk (o reaper).
const variavelSessao = "TESTCONTAINERS_SESSION_ID"

// sessaoDoProcesso dá a este binário de teste uma sessão própria, a menos que quem roda já
// tenha escolhido uma. Sem isso a sessão sai do PID do "go test": todos os pacotes de um
// "go test ./..." disputam um Ryuk só, com o mesmo nome, e cada pacote adota o do anterior.
// Com -p 1 o Ryuk do pacote anterior está morrendo (10 s sem conexão) quando o próximo
// sobe; no Docker Desktop do Windows, lento para parar e remover, o próximo achava o
// contêiner a meio caminho e o testcontainers desistia ("reaper: unexpected container
// status 'created'"). Com uma sessão por processo cada pacote sobe o seu Ryuk e nunca adota
// o de outro; a limpeza continua ligada em todo lugar, inclusive no CI Linux.
func sessaoDoProcesso() error {
	if os.Getenv(variavelSessao) != "" {
		return nil
	}
	if err := os.Setenv(variavelSessao, "concierge-"+aleatorio()); err != nil {
		return fmt.Errorf("definir %s: %w", variavelSessao, err)
	}
	return nil
}

func subir(ctx context.Context) (*instancia, error) {
	c, err := postgres.Run(ctx, imagem,
		postgres.WithDatabase("concierge"), postgres.WithUsername("concierge"), postgres.WithPassword("concierge"),
		postgres.BasicWaitStrategies())
	if err != nil {
		return nil, fmt.Errorf("iniciar contêiner %s: %w", imagem, err)
	}
	inst, err := prepararModelo(ctx, c)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("preparar %s: %w", bancoModelo, err), testcontainers.TerminateContainer(c))
	}
	return inst, nil
}

func prepararModelo(ctx context.Context, c *postgres.PostgresContainer) (*instancia, error) {
	bruta, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("endereço do contêiner: %w", err)
	}
	admin, err := url.Parse(bruta)
	if err != nil {
		return nil, fmt.Errorf("ler endereço %q: %w", bruta, err)
	}
	inst := &instancia{conteiner: c, admin: admin}
	if err := inst.executar(ctx, "CREATE DATABASE "+ident(bancoModelo)); err != nil {
		return nil, err
	}
	modelo, err := banco.Abrir(ctx, inst.url(bancoModelo))
	if err != nil {
		return nil, err
	}
	// o template não pode ter conexão aberta na hora da cópia: fecha antes de devolver
	errMigrar := banco.Migrar(ctx, modelo)
	if err := modelo.Fechar(); err != nil {
		return nil, err
	}
	if errMigrar != nil {
		return nil, errMigrar
	}
	return inst, nil
}

func (i *instancia) url(nome string) string {
	u := *i.admin
	u.Path = "/" + nome
	return u.String()
}

// executar roda um comando de administração (CREATE/DROP DATABASE), que não cabe no ent
// nem no sqlc: o nome do banco é identificador, não parâmetro.
func (i *instancia) executar(ctx context.Context, comando string) error {
	conn, err := pgx.Connect(ctx, i.admin.String())
	if err != nil {
		return fmt.Errorf("conectar ao postgres de teste: %w", err)
	}
	_, errExec := conn.Exec(ctx, comando)
	if errExec != nil {
		errExec = fmt.Errorf("executar %q: %w", comando, errExec)
	}
	return errors.Join(errExec, conn.Close(context.WithoutCancel(ctx)))
}

func ident(nome string) string { return pgx.Identifier{nome}.Sanitize() }

func aleatorio() string { return strings.ToLower(rand.Text()) }
