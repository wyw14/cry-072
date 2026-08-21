package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
)

type querier interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Repository struct {
	*store
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string, timeout time.Duration) (*Repository, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("database timeout must be positive")
	}
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres configuration: %w", err)
	}
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "cry-072-safety"
	pool, err := pgxpool.NewWithConfig(bounded, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	if err := pool.Ping(bounded); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Repository{pool: pool, store: &store{query: pool, timeout: timeout}}, nil
}

func (r *Repository) Close() { r.pool.Close() }

func (r *Repository) WithinTx(ctx context.Context, callback func(application.Store) error) error {
	bounded, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	err := pgx.BeginTxFunc(bounded, r.pool, pgx.TxOptions{IsoLevel: pgx.Serializable}, func(tx pgx.Tx) error {
		return callback(&store{query: tx, timeout: r.timeout})
	})
	if err != nil {
		return fmt.Errorf("postgres transaction: %w", translate(err))
	}
	return nil
}

type store struct {
	query   querier
	timeout time.Duration
}

func (s *store) bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.timeout)
}

func encode(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode repository value: %w", err)
	}
	return string(payload), nil
}

func decode[T any](payload []byte) (T, error) {
	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		return value, fmt.Errorf("decode repository value: %w", err)
	}
	return value, nil
}

var constraintErrors = map[string]error{
	"23505": domain.ErrDuplicate,
	"23503": domain.ErrNotFound,
	"40001": domain.ErrConflict,
}

func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		if mapped, exists := constraintErrors[postgresError.Code]; exists {
			return mapped
		}
	}
	return err
}

func ensureUpdated(tag pgconn.CommandTag) error {
	if tag.RowsAffected() != 1 {
		return domain.ErrConflict
	}
	return nil
}
