package datastore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

// PostgresDB wraps a pgx database connection pool
type PostgresDB struct {
	*pgxpool.Pool
}

// Conn is a minimal connection interface compatible with workers
type Conn interface {
    Exec(ctx context.Context, sql string, args ...interface{}) error
    Release()
}

// NewPostgresConnection creates a new PostgreSQL database connection pool
func NewPostgresConnection(databaseURL string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure connection pool like cascade-server
	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{Pool: pool}, nil
}

// Acquire returns a connection wrapper implementing Exec/Release used by workers
func (db *PostgresDB) Acquire(ctx context.Context) (Conn, error) {
    conn, err := db.Pool.Acquire(ctx)
    if err != nil {
        return nil, err
    }
    return &pgxConnWrapper{conn: conn}, nil
}

// pgxConnWrapper adapts pgxpool.Conn to the Worker Conn interface
type pgxConnWrapper struct{ conn *pgxpool.Conn }

func (w *pgxConnWrapper) Exec(ctx context.Context, sql string, args ...interface{}) error {
    _, err := w.conn.Exec(ctx, sql, args...)
    return err
}

func (w *pgxConnWrapper) Release() { w.conn.Release() }