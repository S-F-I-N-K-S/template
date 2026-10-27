package get

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
)

type fakeUsecase struct {
	trip domaintrip.Trip
	err  error
}

func (f *fakeUsecase) Execute(_ context.Context, _ uuid.UUID) (domaintrip.Trip, error) {
	return f.trip, f.err
}

func newHandler(uc usecase) http.HandlerFunc {
	h := New(uc, domainerror.NewWriter(discardLogger{}))
	return func(w http.ResponseWriter, r *http.Request) {
		h.Handle(w, r, uuid.New())
	}
}

func doGET(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/trips/x", nil))
	return rec
}

func TestHandler_Success(t *testing.T) {
	started := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	tr := domaintrip.Trip{
		ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(),
		StartPoint: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.94, Longitude: 30.31},
		Price:      100, Status: domaintrip.StatusActive, StartedAt: started,
	}

	rec := doGET(t, newHandler(&fakeUsecase{trip: tr}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
	var got api.Trip
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Id != tr.ID || got.Status != api.TripStatus(domaintrip.StatusActive) {
		t.Errorf("trip = %+v", got)
	}
}

func TestHandler_NotFound(t *testing.T) {
	rec := doGET(t, newHandler(&fakeUsecase{err: domaintrip.ErrTripNotFound}))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body)
	}
	var p api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Code != domainerror.CodeTripNotFound {
		t.Errorf("code = %q, want trip_not_found", p.Code)
	}
}

type discardLogger struct{}

func (discardLogger) InfoCtx(context.Context, string, ...any)  {}
func (discardLogger) ErrorCtx(context.Context, string, ...any) {}
