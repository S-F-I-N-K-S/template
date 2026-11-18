package position_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
)

var now = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func validParams() domainposition.NewParams {
	return domainposition.NewParams{
		TripID:      uuid.New(),
		Coordinates: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		RecordedAt:  now.Add(-time.Minute),
		Now:         now,
	}
}

func TestNew_Valid(t *testing.T) {
	p, err := domainposition.New(validParams())
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}
	if p.Coordinates.Latitude != 55.75 {
		t.Errorf("Coordinates = %+v", p.Coordinates)
	}
}

func TestNew_ZeroRecordedAt(t *testing.T) {
	params := validParams()
	params.RecordedAt = time.Time{}
	if _, err := domainposition.New(params); err == nil {
		t.Fatal("New() = nil, want error for zero recorded_at")
	}
}

func TestNew_FutureWithinSkew(t *testing.T) {
	params := validParams()
	params.RecordedAt = now.Add(30 * time.Second)
	if _, err := domainposition.New(params); err != nil {
		t.Errorf("New() = %v, want nil (within 1 minute skew)", err)
	}
}

func TestNew_FutureBeyondSkew(t *testing.T) {
	params := validParams()
	params.RecordedAt = now.Add(5 * time.Minute)
	if _, err := domainposition.New(params); err == nil {
		t.Fatal("New() = nil, want error for recorded_at too far in the future")
	}
}

func TestNew_InvalidCoordinates(t *testing.T) {
	tests := []struct {
		name string
		lat  float64
		lon  float64
	}{
		{"latitude too high", 95, 0},
		{"latitude too low", -95, 0},
		{"longitude too high", 0, 185},
		{"longitude too low", 0, -185},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params := validParams()
			params.Coordinates = domaintrip.Coordinates{Latitude: tc.lat, Longitude: tc.lon}
			if _, err := domainposition.New(params); err == nil {
				t.Fatal("New() = nil, want validation error")
			}
		})
	}
}
