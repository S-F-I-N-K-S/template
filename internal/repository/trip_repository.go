package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/S-F-I-N-K-S/template/internal/domain"
	"github.com/S-F-I-N-K-S/template/internal/txmanager"
)

var sq = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type TripRepository struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewTripRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *TripRepository {
	return &TripRepository{pool: pool, queryTimeout: queryTimeout}
}

func (r *TripRepository) Create(ctx context.Context, t domain.Trip) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	q := txmanager.QuerierFromContext(ctx, r.pool)

	query, args, err := sq.Insert("trips").
		Columns(
			"id", "user_id", "driver_id",
			"start_latitude", "start_longitude",
			"end_latitude", "end_longitude",
			"price", "status", "started_at",
		).
		Values(
			t.ID, t.UserID, t.DriverID,
			t.StartPoint.Latitude, t.StartPoint.Longitude,
			t.EndPoint.Latitude, t.EndPoint.Longitude,
			t.Price, t.Status, t.StartedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert trip query: %w", err)
	}

	if _, err := q.Exec(ctx, query, args...); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDriverBusy
		}
		return fmt.Errorf("insert trip: %w", err)
	}
	return nil
}

func (r *TripRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	q := txmanager.QuerierFromContext(ctx, r.pool)

	query, args, err := sq.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude",
		"end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at",
	).From("trips").Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return domain.Trip{}, fmt.Errorf("build select trip query: %w", err)
	}

	var t domain.Trip
	err = q.QueryRow(ctx, query, args...).Scan(
		&t.ID, &t.UserID, &t.DriverID,
		&t.StartPoint.Latitude, &t.StartPoint.Longitude,
		&t.EndPoint.Latitude, &t.EndPoint.Longitude,
		&t.Price, &t.Status, &t.StartedAt, &t.FinishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Trip{}, domain.ErrTripNotFound
	}
	if err != nil {
		return domain.Trip{}, fmt.Errorf("scan trip: %w", err)
	}
	return t, nil
}

func (r *TripRepository) Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	q := txmanager.QuerierFromContext(ctx, r.pool)

	query, args, err := sq.Update("trips").
		Set("status", domain.StatusCompleted).
		Set("finished_at", finishedAt).
		Set("updated_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"id": id, "status": domain.StatusActive}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update trip query: %w", err)
	}

	tag, err := q.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("finish trip: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := r.GetByID(ctx, id); errors.Is(err, domain.ErrTripNotFound) {
			return domain.ErrTripNotFound
		}
		return domain.ErrTripCompleted
	}
	return nil
}
