package banco_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"go-concierge/internal/banco/bancotest"
)

// criarContador cria uma tabelinha descartável só para os testes de isolamento: um saldo
// que duas transações vão disputar.
func criarContador(t *testing.T, pool *pgxpool.Pool, saldoInicial int) {
	t.Helper()
	ctx := t.Context()
	if _, err := pool.Exec(ctx, `CREATE TABLE contador_teste (id int PRIMARY KEY, saldo int NOT NULL)`); err != nil {
		t.Fatalf("criar contador_teste: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO contador_teste (id, saldo) VALUES (1, $1)`, saldoInicial); err != nil {
		t.Fatalf("inserir saldo inicial: %v", err)
	}
}

// TestLostUpdateSemLockTravaNaSegundaTransacao: duas transações leem o mesmo saldo,
// decidem o incremento com base no que leram e tentam escrever. Sem travar a linha, a
// segunda escrita simplesmente sobrescreveria a primeira (lost update) se ambas
// commitassem — aqui provamos que SELECT ... FOR UPDATE fecha essa janela: a segunda
// transação espera a primeira comitar e lê o valor já atualizado.
func TestLostUpdateSemLockTravaNaSegundaTransacao(t *testing.T) {
	ctx := t.Context()
	b := bancotest.Novo(t)
	criarContador(t, b.Pool, 100)

	tx1, err := b.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	var saldo1 int
	if err := tx1.QueryRow(ctx, `SELECT saldo FROM contador_teste WHERE id = 1 FOR UPDATE`).Scan(&saldo1); err != nil {
		t.Fatal(err)
	}

	tx2Pronta := make(chan struct{})
	tx2Terminou := make(chan int)
	go func() {
		tx2, err := b.Pool.Begin(ctx)
		if err != nil {
			t.Error(err)
			close(tx2Terminou)
			return
		}
		defer tx2.Rollback(ctx)
		close(tx2Pronta)
		var saldo2 int
		// bloqueia aqui até tx1 comitar ou desfazer: é a prova de que o lock funcionou
		if err := tx2.QueryRow(ctx, `SELECT saldo FROM contador_teste WHERE id = 1 FOR UPDATE`).Scan(&saldo2); err != nil {
			t.Error(err)
		}
		tx2Terminou <- saldo2
	}()

	<-tx2Pronta
	select {
	case saldo2 := <-tx2Terminou:
		t.Fatalf("tx2 não deveria ter conseguido o lock antes de tx1 comitar; leu %d", saldo2)
	case <-time.After(200 * time.Millisecond):
	}

	if _, err := tx1.Exec(ctx, `UPDATE contador_teste SET saldo = $1 WHERE id = 1`, saldo1+10); err != nil {
		t.Fatal(err)
	}
	if err := tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	saldo2 := <-tx2Terminou
	if saldo2 != saldo1+10 {
		t.Fatalf("tx2 deveria ler o saldo já atualizado por tx1 (%d), leu %d", saldo1+10, saldo2)
	}
}

// TestWriteSkewSerializableRecusaOQueReadCommittedDeixaPassar: duas "vagas de plantão"
// (linhas) com a invariante "pelo menos uma ocupada". Em READ COMMITTED, duas transações
// concorrentes podem cada uma ler uma vaga livre, decidir que a outra ainda está ocupada
// e as duas desocuparem — violando a invariante (write skew clássico). Em SERIALIZABLE o
// Postgres detecta a dependência e aborta uma das duas com erro de serialização.
func TestWriteSkewSerializableRecusaOQueReadCommittedDeixaPassar(t *testing.T) {
	ctx := t.Context()
	b := bancotest.Novo(t)
	if _, err := b.Pool.Exec(ctx, `CREATE TABLE plantao_teste (id int PRIMARY KEY, ocupado boolean NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Pool.Exec(ctx, `INSERT INTO plantao_teste (id, ocupado) VALUES (1, true), (2, true)`); err != nil {
		t.Fatal(err)
	}

	desocupar := func(ctx context.Context, meuID, outroID int) error {
		tx, err := b.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		var outroOcupado bool
		if err := tx.QueryRow(ctx, `SELECT ocupado FROM plantao_teste WHERE id = $1`, outroID).Scan(&outroOcupado); err != nil {
			return err
		}
		if !outroOcupado {
			return errors.New("a invariante já estava quebrada antes de eu agir")
		}
		if _, err := tx.Exec(ctx, `UPDATE plantao_teste SET ocupado = false WHERE id = $1`, meuID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	var wg sync.WaitGroup
	erros := make([]error, 2)
	wg.Add(2)
	ini := make(chan struct{})
	go func() { defer wg.Done(); <-ini; erros[0] = desocupar(ctx, 1, 2) }()
	go func() { defer wg.Done(); <-ini; erros[1] = desocupar(ctx, 2, 1) }()
	close(ini)
	wg.Wait()

	falhas := 0
	for _, err := range erros {
		if err != nil {
			falhas++
		}
	}
	if falhas == 0 {
		t.Fatal("SERIALIZABLE deveria ter recusado uma das duas transações (write skew)")
	}

	var ocupados int
	if err := b.Pool.QueryRow(ctx, `SELECT count(*) FROM plantao_teste WHERE ocupado`).Scan(&ocupados); err != nil {
		t.Fatal(err)
	}
	if ocupados < 1 {
		t.Fatalf("invariante quebrada: %d vaga(s) ocupada(s), queria pelo menos 1", ocupados)
	}
}

// TestDeadlockUmaDasDuasTransacoesEAbortadaPeloPostgres: duas transações travam as mesmas
// duas linhas em ordem oposta. O Postgres detecta o ciclo (deadlock_timeout) e cancela uma
// delas com SQLSTATE 40P01, para a outra poder seguir — nenhuma fica travada para sempre.
func TestDeadlockUmaDasDuasTransacoesEAbortadaPeloPostgres(t *testing.T) {
	ctx := t.Context()
	b := bancotest.Novo(t)
	criarContador(t, b.Pool, 0)
	if _, err := b.Pool.Exec(ctx, `INSERT INTO contador_teste (id, saldo) VALUES (2, 0)`); err != nil {
		t.Fatal(err)
	}

	travarEmOrdem := func(ctx context.Context, primeiro, segundo int, iniciouSegunda chan<- struct{}, podeContinuar <-chan struct{}) error {
		tx, err := b.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `UPDATE contador_teste SET saldo = saldo WHERE id = $1`, primeiro); err != nil {
			return err
		}
		close(iniciouSegunda)
		<-podeContinuar
		if _, err := tx.Exec(ctx, `UPDATE contador_teste SET saldo = saldo WHERE id = $1`, segundo); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	aIniciouB := make(chan struct{})
	bIniciouA := make(chan struct{})
	var errA, errB error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); errA = travarEmOrdem(ctx, 1, 2, aIniciouB, bIniciouA) }()
	go func() { defer wg.Done(); errB = travarEmOrdem(ctx, 2, 1, bIniciouA, aIniciouB) }()
	wg.Wait()

	deadlocks := 0
	for _, err := range []error{errA, errB} {
		if err != nil {
			deadlocks++
		}
	}
	if deadlocks != 1 {
		t.Fatalf("esperava exatamente uma transação abortada por deadlock, tive %d (errA=%v errB=%v)", deadlocks, errA, errB)
	}
}
