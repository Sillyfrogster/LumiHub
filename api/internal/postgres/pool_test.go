package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sillyfrogster/LumiHub/api/internal/postgres"
	"github.com/Sillyfrogster/LumiHub/api/internal/testdb"
	"github.com/jackc/pgx/v5/pgconn"
)

// Postgres reports a statement it cut off, and a transaction it closed for
// sitting idle, with these codes.
const (
	queryCanceled  = "57014"
	idleInTransact = "25P03"
)

func TestAQueryPastTheStatementLimitIsCutOff(t *testing.T) {
	pool := testdb.ConnectWith(t, func(s *postgres.Settings) {
		s.StatementTimeout = 200 * time.Millisecond
	})

	_, err := pool.Exec(context.Background(), "select pg_sleep(5)")
	if err == nil {
		t.Fatal("a query past the statement limit ran to the end, so the limit never reached the connection")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != queryCanceled {
		t.Fatalf("query gave %v, want the database to cancel it with %s", err, queryCanceled)
	}
}

func TestATransactionLeftIdlePastItsLimitLosesItsConnection(t *testing.T) {
	pool := testdb.ConnectWith(t, func(s *postgres.Settings) {
		s.IdleTransactionTimeout = 200 * time.Millisecond
	})
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "select 1"); err != nil {
		t.Fatalf("first statement: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	_, err = tx.Exec(ctx, "select 1")
	if err == nil {
		t.Fatal("a transaction left idle past its limit kept its connection")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != idleInTransact {
		t.Fatalf("idle transaction gave %v, want the database to close it with %s", err, idleInTransact)
	}
}
