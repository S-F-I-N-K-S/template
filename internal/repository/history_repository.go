package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/S-F-I-N-K-S/template/internal/txmanager"
)

type HistoryRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewHistoryRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *HistoryRepository {
	return &HistoryRepository{pool: pool, queryTimeout: queryTimeout}
}

func (r *HistoryRepository) Append(ctx context.Context, tripID uuid.UUID, fromStatus, toStatus, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	q := txmanager.QuerierFromContext(ctx, r.pool)

	query, args, err := sq.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason").
		Values(tripID, nullIfEmpty(fromStatus), toStatus, reason).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert history query: %w", err)
	}

	if _, err := q.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert history: %w", err)
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
