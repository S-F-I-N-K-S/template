package trip_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

func okParams() domaintrip.NewParams {
	return domaintrip.NewParams{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.94, Longitude: 30.31},
		Price:      100,
		Now:        time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC),
	}
}

func TestNew_Success(t *testing.T) {
	p := okParams()
	tr, err := domaintrip.New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if tr.Status != domaintrip.StatusActive {
		t.Errorf("status = %q, want active", tr.Status)
	}
	if tr.FinishedAt != nil {
		t.Errorf("finished_at = %v, want nil", *tr.FinishedAt)
	}
	if !tr.StartedAt.Equal(p.Now.UTC()) {
		t.Errorf("started_at = %v, want %v", tr.StartedAt, p.Now.UTC())
	}
}

func TestNew_Validation(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*domaintrip.NewParams)
		wantField string
	}{
		{"empty user_id", func(p *domaintrip.NewParams) { p.UserID = uuid.Nil }, "user_id"},
		{"empty driver_id", func(p *domaintrip.NewParams) { p.DriverID = uuid.Nil }, "driver_id"},
		{"lat too high", func(p *domaintrip.NewParams) { p.StartPoint.Latitude = 91 }, "latitude"},
		{"lat too low", func(p *domaintrip.NewParams) { p.EndPoint.Latitude = -91 }, "latitude"},
		{"lon too high", func(p *domaintrip.NewParams) { p.StartPoint.Longitude = 181 }, "longitude"},
		{"lon too low", func(p *domaintrip.NewParams) { p.EndPoint.Longitude = -181 }, "longitude"},
		{"negative price", func(p *domaintrip.NewParams) { p.Price = -1 }, "price"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := okParams()
			tc.mutate(&p)
			_, err := domaintrip.New(p)
			var ve *domaintrip.ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if ve.Field != tc.wantField {
				t.Errorf("field = %q, want %q", ve.Field, tc.wantField)
			}
		})
	}
}

func TestFinish_Transitions(t *testing.T) {
	at := time.Date(2026, 10, 27, 13, 0, 0, 0, time.UTC)

	t.Run("active -> completed", func(t *testing.T) {
		tr, err := domaintrip.New(okParams())
		if err != nil {
			t.Fatal(err)
		}
		if err := tr.Finish(at); err != nil {
			t.Fatalf("Finish: %v", err)
		}
		if tr.Status != domaintrip.StatusCompleted {
			t.Errorf("status = %q, want completed", tr.Status)
		}
		if tr.FinishedAt == nil || !tr.FinishedAt.Equal(at) {
			t.Errorf("finished_at = %v, want %v", tr.FinishedAt, at)
		}
	})

	t.Run("completed -> completed is ErrTripCompleted", func(t *testing.T) {
		tr, err := domaintrip.New(okParams())
		if err != nil {
			t.Fatal(err)
		}
		if err := tr.Finish(at); err != nil {
			t.Fatal(err)
		}
		if err := tr.Finish(at); !errors.Is(err, domaintrip.ErrTripCompleted) {
			t.Fatalf("err = %v, want ErrTripCompleted", err)
		}
	})
}

func TestStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from, to domaintrip.Status
		want     bool
	}{
		{domaintrip.StatusActive, domaintrip.StatusCompleted, true},
		{domaintrip.StatusCompleted, domaintrip.StatusCompleted, false},
		{domaintrip.StatusCompleted, domaintrip.StatusActive, false},
		{domaintrip.StatusActive, domaintrip.StatusActive, false},
		{domaintrip.Status("unknown"), domaintrip.StatusCompleted, false},
	}
	for _, tc := range tests {
		if got := tc.from.CanTransitionTo(tc.to); got != tc.want {
			t.Errorf("%q->%q = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestValidate_FinishedBeforeStarted(t *testing.T) {
	start := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	before := start.Add(-time.Hour)
	tr := domaintrip.Trip{
		ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(),
		StartPoint: domaintrip.Coordinates{Latitude: 1, Longitude: 1},
		EndPoint:   domaintrip.Coordinates{Latitude: 2, Longitude: 2},
		Status:     domaintrip.StatusCompleted,
		StartedAt:  start, FinishedAt: &before,
	}
	var ve *domaintrip.ValidationError
	if err := tr.Validate(); !errors.As(err, &ve) || ve.Field != "finished_at" {
		t.Fatalf("err = %v, want ValidationError on finished_at", err)
	}
}

func TestAsValidation(t *testing.T) {
	if _, ok := domaintrip.AsValidation(errors.New("plain")); ok {
		t.Error("AsValidation on plain error = true")
	}
	_, err := domaintrip.New(domaintrip.NewParams{Price: -1})
	if ve, ok := domaintrip.AsValidation(err); !ok || ve == nil {
		t.Error("AsValidation wrapped validation = false")
	}
}
