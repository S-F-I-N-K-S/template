//go:build integration

package trip_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	"github.com/S-F-I-N-K-S/template/internal/infra/transaction"
	repostrip "github.com/S-F-I-N-K-S/template/internal/repository/trip"
)

const defaultDatabaseURL = "postgres://tripgo:tripgo@localhost:5432/tripgo?sslmode=disable" //nolint:gosec // G101: тестовые креды локальной dev-базы, не секрет

func databaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return defaultDatabaseURL
}

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), databaseURL())
	if err != nil {
		t.Fatalf("connect pool: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database (поднимите БД: tripgoctl environment start или см. README): %v", err)
	}

	applyMigrations(t, ctx, pool)
	return pool
}

func applyMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations")

	drops := []string{
		`DROP TABLE IF EXISTS trip_status_history`,
		`DROP TABLE IF EXISTS trips`,
	}
	for _, q := range drops {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("drop: %v", err)
		}
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("migrations not found in %s: %v", dir, err)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration: %v", err)
		}
		// Файлы в формате goose (-- +goose Up / Down): для тестов берём только Up-секцию.
		sql := extractGooseUp(string(data))
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}
}

func extractGooseUp(s string) string {
	var out []string
	inUp := false
	for _, line := range splitLines(s) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose") {
			inUp = strings.HasSuffix(trimmed, "Up")
			continue
		}
		if inUp {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func newTrip(driverID uuid.UUID) domaintrip.Trip {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return domaintrip.Trip{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		DriverID:   driverID,
		StartPoint: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.94, Longitude: 30.31},
		Price:      1450,
		Status:     domaintrip.StatusActive,
		StartedAt:  now,
	}
}

func TestRepository_SaveGetFinish(t *testing.T) {
	pool := setupPool(t)
	repo := repostrip.NewRepository(pool, 5*time.Second)
	history := repostrip.NewHistoryRepository(pool, 5*time.Second)
	txm := transaction.New(pool)
	ctx := context.Background()

	driverID := uuid.New()
	tr := newTrip(driverID)

	err := txm.Do(ctx, func(ctx context.Context) error {
		if err := repo.Save(ctx, tr); err != nil {
			return err
		}
		return history.AppendHistory(ctx, tr.ID, "", string(domaintrip.StatusActive), "trip created")
	})
	if err != nil {
		t.Fatalf("save in tx: %v", err)
	}

	got, err := repo.GetByID(ctx, tr.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DriverID != driverID || got.Status != domaintrip.StatusActive || got.Price != tr.Price {
		t.Errorf("got = %+v", got)
	}

	finishedAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := repo.Finish(ctx, tr.ID, finishedAt); err != nil {
		t.Fatalf("finish: %v", err)
	}
	got, err = repo.GetByID(ctx, tr.ID)
	if err != nil {
		t.Fatalf("get after finish: %v", err)
	}
	if got.Status != domaintrip.StatusCompleted || got.FinishedAt == nil {
		t.Errorf("after finish got = %+v", got)
	}

	if err := repo.Finish(ctx, tr.ID, finishedAt); !errors.Is(err, domaintrip.ErrTripCompleted) {
		t.Errorf("second finish err = %v, want ErrTripCompleted", err)
	}

	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Errorf("missing get err = %v, want ErrTripNotFound", err)
	}
}

func TestRepository_DriverBusy(t *testing.T) {
	pool := setupPool(t)
	repo := repostrip.NewRepository(pool, 5*time.Second)
	ctx := context.Background()

	driverID := uuid.New()
	if err := repo.Save(ctx, newTrip(driverID)); err != nil {
		t.Fatalf("first save: %v", err)
	}

	err := repo.Save(ctx, newTrip(driverID))
	if !errors.Is(err, domaintrip.ErrDriverBusy) {
		t.Fatalf("second save err = %v, want ErrDriverBusy", err)
	}

	first, err := repo.GetByID(ctx, firstTripID(t, ctx, pool, driverID))
	if err != nil {
		t.Fatalf("get first: %v", err)
	}
	if err := repo.Finish(ctx, first.ID, time.Now().UTC()); err != nil {
		t.Fatalf("finish first: %v", err)
	}
	if err := repo.Save(ctx, newTrip(driverID)); err != nil {
		t.Errorf("save after finish: %v, want nil", err)
	}
}

func firstTripID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, driverID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT id FROM trips WHERE driver_id = $1 AND status = 'active' ORDER BY started_at LIMIT 1`,
		driverID).Scan(&id)
	if err != nil {
		t.Fatalf("find first active trip: %v", err)
	}
	return id
}

func TestTransaction_Rollback(t *testing.T) {
	pool := setupPool(t)
	repo := repostrip.NewRepository(pool, 5*time.Second)
	history := repostrip.NewHistoryRepository(pool, 5*time.Second)
	txm := transaction.New(pool)
	ctx := context.Background()

	tr := newTrip(uuid.New())
	boom := errors.New("boom")

	err := txm.Do(ctx, func(ctx context.Context) error {
		if err := repo.Save(ctx, tr); err != nil {
			return err
		}
		if err := history.AppendHistory(ctx, uuid.New(), "", string(domaintrip.StatusActive), "ghost"); err != nil {
			return fmt.Errorf("%w (wrapped)", err)
		}
		return boom
	})
	if err == nil {
		t.Fatal("expected error from tx.Do")
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM trips WHERE id = $1`, tr.ID).Scan(&count); err != nil {
		t.Fatalf("count trips: %v", err)
	}
	if count != 0 {
		t.Errorf("trips rows after rollback = %d, want 0 (INSERT должен быть откачен)", count)
	}
}

func TestTransaction_Commit(t *testing.T) {
	pool := setupPool(t)
	repo := repostrip.NewRepository(pool, 5*time.Second)
	history := repostrip.NewHistoryRepository(pool, 5*time.Second)
	txm := transaction.New(pool)
	ctx := context.Background()

	tr := newTrip(uuid.New())
	if err := txm.Do(ctx, func(ctx context.Context) error {
		if err := repo.Save(ctx, tr); err != nil {
			return err
		}
		return history.AppendHistory(ctx, tr.ID, "", string(domaintrip.StatusActive), "trip created")
	}); err != nil {
		t.Fatalf("tx commit path: %v", err)
	}

	var hist int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM trip_status_history WHERE trip_id = $1`, tr.ID).Scan(&hist); err != nil {
		t.Fatalf("count history: %v", err)
	}
	if hist != 1 {
		t.Errorf("history rows = %d, want 1", hist)
	}
}
