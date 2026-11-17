//go:build integration

package trip_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	"github.com/S-F-I-N-K-S/template/internal/infra/transaction"
	repoposition "github.com/S-F-I-N-K-S/template/internal/repository/position"
	repostrip "github.com/S-F-I-N-K-S/template/internal/repository/trip"
)

func lastPositionAt(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tripID uuid.UUID) *time.Time {
	t.Helper()
	var ts *time.Time
	if err := pool.QueryRow(ctx, `SELECT last_position_at FROM trips WHERE id = $1`, tripID).Scan(&ts); err != nil {
		t.Fatalf("read last_position_at: %v", err)
	}
	return ts
}

func domainPositionFor(tripID uuid.UUID, recordedAt time.Time) (domainposition.Position, error) {
	return domainposition.New(domainposition.NewParams{
		TripID:      tripID,
		Coordinates: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		RecordedAt:  recordedAt,
		Now:         recordedAt,
	})
}

func TestRepository_AdvanceLastPositionAt_SetsOnFirstCall(t *testing.T) {
	pool := setupPool(t)
	repo := repostrip.NewRepository(pool, 5*time.Second)
	ctx := context.Background()

	tr := newTrip(uuid.New())
	if err := repo.Save(ctx, tr); err != nil {
		t.Fatalf("save: %v", err)
	}

	recordedAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := repo.AdvanceLastPositionAt(ctx, tr.ID, recordedAt); err != nil {
		t.Fatalf("AdvanceLastPositionAt: %v", err)
	}

	if got := lastPositionAt(t, ctx, pool, tr.ID); got == nil || !got.Equal(recordedAt) {
		t.Errorf("last_position_at = %v, want %v", got, recordedAt)
	}
}

func TestRepository_AdvanceLastPositionAt_IgnoresOlderTimestamp(t *testing.T) {
	pool := setupPool(t)
	repo := repostrip.NewRepository(pool, 5*time.Second)
	ctx := context.Background()

	tr := newTrip(uuid.New())
	if err := repo.Save(ctx, tr); err != nil {
		t.Fatalf("save: %v", err)
	}

	fresh := time.Now().UTC().Truncate(time.Microsecond)
	stale := fresh.Add(-time.Minute)

	if err := repo.AdvanceLastPositionAt(ctx, tr.ID, fresh); err != nil {
		t.Fatalf("advance (fresh): %v", err)
	}
	if err := repo.AdvanceLastPositionAt(ctx, tr.ID, stale); err != nil {
		t.Fatalf("advance (stale): %v", err)
	}

	if got := lastPositionAt(t, ctx, pool, tr.ID); got == nil || !got.Equal(fresh) {
		t.Errorf("last_position_at = %v, want unchanged %v (stale update must be ignored)", got, fresh)
	}
}

func TestRepository_AdvanceLastPositionAt_NotFound(t *testing.T) {
	pool := setupPool(t)
	repo := repostrip.NewRepository(pool, 5*time.Second)
	ctx := context.Background()

	err := repo.AdvanceLastPositionAt(ctx, uuid.New(), time.Now().UTC())
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("err = %v, want ErrTripNotFound", err)
	}
}

func TestRepository_AdvanceLastPositionAt_CompletedTrip(t *testing.T) {
	pool := setupPool(t)
	repo := repostrip.NewRepository(pool, 5*time.Second)
	ctx := context.Background()

	tr := newTrip(uuid.New())
	if err := repo.Save(ctx, tr); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := repo.Finish(ctx, tr.ID, time.Now().UTC()); err != nil {
		t.Fatalf("finish: %v", err)
	}

	err := repo.AdvanceLastPositionAt(ctx, tr.ID, time.Now().UTC())
	if !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf("err = %v, want ErrTripCompleted", err)
	}
}

func TestRepository_ListStaleActive(t *testing.T) {
	pool := setupPool(t)
	repo := repostrip.NewRepository(pool, 5*time.Second)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Microsecond)

	oldest := newTrip(uuid.New())
	oldest.StartedAt = now.Add(-10 * time.Minute)
	if err := repo.Save(ctx, oldest); err != nil {
		t.Fatalf("save oldest: %v", err)
	}

	middle := newTrip(uuid.New())
	middle.StartedAt = now.Add(-20 * time.Minute)
	if err := repo.Save(ctx, middle); err != nil {
		t.Fatalf("save middle: %v", err)
	}
	if err := repo.AdvanceLastPositionAt(ctx, middle.ID, now.Add(-5*time.Minute)); err != nil {
		t.Fatalf("advance middle: %v", err)
	}

	fresh := newTrip(uuid.New())
	if err := repo.Save(ctx, fresh); err != nil {
		t.Fatalf("save fresh: %v", err)
	}
	if err := repo.AdvanceLastPositionAt(ctx, fresh.ID, now.Add(-time.Second)); err != nil {
		t.Fatalf("advance fresh: %v", err)
	}

	completed := newTrip(uuid.New())
	completed.StartedAt = now.Add(-time.Hour)
	if err := repo.Save(ctx, completed); err != nil {
		t.Fatalf("save completed: %v", err)
	}
	if err := repo.Finish(ctx, completed.ID, now.Add(-30*time.Minute)); err != nil {
		t.Fatalf("finish completed: %v", err)
	}

	stale, err := repo.ListStaleActive(ctx, time.Minute, now)
	if err != nil {
		t.Fatalf("ListStaleActive: %v", err)
	}

	var got []uuid.UUID
	for _, s := range stale {
		got = append(got, s.TripID)
	}
	want := []uuid.UUID{oldest.ID, middle.ID}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ListStaleActive() trip ids = %v, want %v (oldest first, fresh/completed excluded)", got, want)
	}
	if stale[0].DriverID != oldest.DriverID {
		t.Errorf("DriverID = %v, want %v", stale[0].DriverID, oldest.DriverID)
	}
}

func TestSavePosition_AtomicWithAdvanceLastPositionAt(t *testing.T) {
	pool := setupPool(t)
	trips := repostrip.NewRepository(pool, 5*time.Second)
	positions := repoposition.NewRepository(pool, 5*time.Second)
	txm := transaction.New(pool)
	ctx := context.Background()

	tr := newTrip(uuid.New())
	if err := trips.Save(ctx, tr); err != nil {
		t.Fatalf("save trip: %v", err)
	}

	recordedAt := time.Now().UTC().Truncate(time.Microsecond)
	pos, err := domainPositionFor(tr.ID, recordedAt)
	if err != nil {
		t.Fatalf("build position: %v", err)
	}

	err = txm.Do(ctx, func(ctx context.Context) error {
		if err := trips.AdvanceLastPositionAt(ctx, tr.ID, recordedAt); err != nil {
			return err
		}
		_, err := positions.Save(ctx, pos)
		return err
	})
	if err != nil {
		t.Fatalf("tx commit path: %v", err)
	}

	if got := lastPositionAt(t, ctx, pool, tr.ID); got == nil || !got.Equal(recordedAt) {
		t.Errorf("last_position_at = %v, want %v", got, recordedAt)
	}

	route, err := positions.ListByTrip(ctx, tr.ID)
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}
	if len(route) != 1 {
		t.Fatalf("route len = %d, want 1", len(route))
	}
}

func TestSavePosition_RolledBackWhenTripCompleted(t *testing.T) {
	pool := setupPool(t)
	trips := repostrip.NewRepository(pool, 5*time.Second)
	positions := repoposition.NewRepository(pool, 5*time.Second)
	txm := transaction.New(pool)
	ctx := context.Background()

	tr := newTrip(uuid.New())
	if err := trips.Save(ctx, tr); err != nil {
		t.Fatalf("save trip: %v", err)
	}
	if err := trips.Finish(ctx, tr.ID, time.Now().UTC()); err != nil {
		t.Fatalf("finish trip: %v", err)
	}

	recordedAt := time.Now().UTC().Truncate(time.Microsecond)
	pos, err := domainPositionFor(tr.ID, recordedAt)
	if err != nil {
		t.Fatalf("build position: %v", err)
	}

	err = txm.Do(ctx, func(ctx context.Context) error {
		if err := trips.AdvanceLastPositionAt(ctx, tr.ID, recordedAt); err != nil {
			return err
		}
		_, err := positions.Save(ctx, pos)
		return err
	})
	if !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Fatalf("err = %v, want ErrTripCompleted", err)
	}

	route, err := positions.ListByTrip(ctx, tr.ID)
	if err != nil {
		t.Fatalf("list positions: %v", err)
	}
	if len(route) != 0 {
		t.Errorf("route len = %d, want 0 (INSERT должен быть откачен вместе с AdvanceLastPositionAt)", len(route))
	}
}
