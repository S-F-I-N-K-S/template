package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	"github.com/S-F-I-N-K-S/template/internal/infra/transaction"
)

type StaleTrip struct {
	TripID   uuid.UUID
	DriverID uuid.UUID
	SinceAt  time.Time
}

func (r *Repository) AdvanceLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	q := transaction.QuerierFromContext(ctx, r.pool)

	query, args, err := sq.Update("trips").
		Set("last_position_at", squirrel.Expr(
			"CASE WHEN last_position_at IS NULL OR last_position_at < ? THEN ? ELSE last_position_at END",
			recordedAt, recordedAt,
		)).
		Set("updated_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"id": tripID, "status": string(domaintrip.StatusActive)}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build advance last_position_at query: %w", err)
	}

	tag, err := q.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("advance last_position_at: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}

	existing, getErr := r.GetByID(ctx, tripID)
	switch {
	case errors.Is(getErr, domaintrip.ErrTripNotFound):
		return domaintrip.ErrTripNotFound
	case getErr != nil:
		return fmt.Errorf("advance last_position_at: confirm current state: %w", getErr)
	case existing.Status == domaintrip.StatusCompleted:
		return domaintrip.ErrTripCompleted
	default:
		return fmt.Errorf("advance last_position_at: update matched no rows but trip %s is %q", tripID, existing.Status)
	}
}

func (r *Repository) ListStaleActive(ctx context.Context, staleAfter time.Duration, now time.Time) ([]StaleTrip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	q := transaction.QuerierFromContext(ctx, r.pool)
	threshold := now.Add(-staleAfter)

	query, args, err := sq.Select("id", "driver_id", "COALESCE(last_position_at, started_at) AS since").
		From("trips").
		Where(squirrel.And{
			squirrel.Eq{"status": string(domaintrip.StatusActive)},
			squirrel.Expr("COALESCE(last_position_at, started_at) < ?", threshold),
		}).
		OrderBy("since ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build stale trips query: %w", err)
	}

	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list stale trips: %w", err)
	}
	defer rows.Close()

	var out []StaleTrip
	for rows.Next() {
		var st StaleTrip
		if err := rows.Scan(&st.TripID, &st.DriverID, &st.SinceAt); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale trips: %w", err)
	}
	return out, nil
}
