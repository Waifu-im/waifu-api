// Package store owns the Postgres connection.
//
// It is deliberately thin. The C# service uses Entity Framework Core, and the
// closest idiomatic Go equivalent is sqlc: plain SQL checked against a schema
// at build time. This package holds only the connection, the pool
// configuration, and the health check; queries live alongside their handlers
// and are added as endpoints are ported.
//
// # Supabase connection mode
//
// Supabase caps direct connections (about 15 on the free tier) and its
// transaction-mode pooler does not support prepared statements. pgx uses
// prepared statements by default, so the two supported
// modes are handled explicitly here:
//
//   - direct or session pooler: default query execution mode, prepared
//     statements are reused across calls
//   - transaction pooler: simple protocol, which avoids server-side prepared
//     statements entirely
//
// The mode is chosen by URL, not by guesswork, because guessing wrong produces
// intermittent errors that look like database instability.
package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config describes how to reach the database.
type Config struct {
	// URL is the Postgres connection string.
	URL string

	// MaxConnections caps the pool. It must stay well under the server's
	// connection limit; on Supabase free that limit is around 15.
	MaxConnections int32
}

// DB wraps a pgx connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// Open connects to Postgres and verifies the connection with a ping.
//
// It returns an error rather than a lazy handle, so a misconfigured deployment
// fails at startup instead of on the first request.
func Open(ctx context.Context, cfg Config) (*DB, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("database URL is empty")
	}

	maxConns := cfg.MaxConnections
	if maxConns <= 0 {
		maxConns = 8
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	poolConfig.MaxConns = maxConns

	// Bound how long a request may wait for a pooled connection. Without this a
	// traffic spike queues indefinitely and the request times out with no
	// useful diagnostic.
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	// Apply the protocol mode implied by the connection URL.
	if UsesTransactionPooler(cfg.URL) {
		// The transaction pooler multiplexes connections onto a single backend
		// session, so server-side prepared statements would leak across
		// unrelated callers. The simple protocol avoids creating them.
		poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

// UsesTransactionPooler reports whether the URL points at Supabase's
// transaction-mode pooler.
//
// Supabase encodes the pooler in the host name, for example
// "aws-0-region.pooler.supabase.com" for the session pooler and
// "aws-0-region.pooler.supabase.com" with a pool_mode=transaction parameter
// for the transaction pooler. When the mode is not stated, the session pooler
// is assumed, because it behaves like a direct connection.
func UsesTransactionPooler(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	if mode := parsed.Query().Get("pool_mode"); mode != "" {
		return strings.EqualFold(mode, "transaction")
	}
	return false
}

// Pool exposes the underlying pool for packages that need to run queries.
//
// Exposing it keeps this package from having to wrap every query method that
// sqlc will eventually generate.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// Ping verifies the database is reachable. Used by the health endpoint.
func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// Close releases every pooled connection.
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
}

// Stats reports pool statistics, used for diagnostics and alerting.
func (db *DB) Stats() *pgxpool.Stat {
	return db.pool.Stat()
}
