package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	URL                string
	MaxConns, MinConns int32
	MaxConnLifetime    time.Duration
	ConnectTimeout     time.Duration
	Tracer             pgx.QueryTracer
}

func NewPool(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	panic("TODO: postgres.NewPool")
}
