package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMaxConns        = 20
	defaultMinConns        = 2
	defaultMaxConnLifetime = 30 * time.Minute
	defaultMaxConnIdleTime = 5 * time.Minute
)

const (
	defaultStatementTimeout       = 5 * time.Second
	defaultIdleTransactionTimeout = 10 * time.Second
)

type Settings struct {
	URL                    string
	MaxConns               int32
	MinConns               int32
	MaxConnLifetime        time.Duration
	MaxConnIdleTime        time.Duration
	StatementTimeout       time.Duration
	IdleTransactionTimeout time.Duration
}

// DefaultSettings are what the server runs with. A test takes these and changes
// the one limit it is about.
func DefaultSettings(url string) Settings {
	return Settings{
		URL:                    url,
		MaxConns:               defaultMaxConns,
		MinConns:               defaultMinConns,
		MaxConnLifetime:        defaultMaxConnLifetime,
		MaxConnIdleTime:        defaultMaxConnIdleTime,
		StatementTimeout:       defaultStatementTimeout,
		IdleTransactionTimeout: defaultIdleTransactionTimeout,
	}
}

// NewPool opens the pool. Connections are made as they are needed, so a
// database that is down shows up on the first query rather than here.
func NewPool(ctx context.Context, s Settings) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(s.URL)
	if err != nil {
		return nil, fmt.Errorf("read database url: %w", err)
	}

	cfg.MaxConns = s.MaxConns
	cfg.MinConns = s.MinConns
	cfg.MaxConnLifetime = s.MaxConnLifetime
	cfg.MaxConnIdleTime = s.MaxConnIdleTime

	// Sent as a connection starts, so connections opened later carry them too.
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = milliseconds(s.StatementTimeout)
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = milliseconds(s.IdleTransactionTimeout)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	return pool, nil
}

func milliseconds(d time.Duration) string {
	return strconv.FormatInt(d.Milliseconds(), 10)
}
