package finish

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func doFinish(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/trips/x/finish", nil))
	return rec
}

func completedTrip() domaintrip.Trip {
	started := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	fin := started.Add(time.Hour)
	return domaintrip.Trip{
		ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(),
		StartPoint: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.94, Longitude: 30.31},
		Price:      100, Status: domaintrip.StatusCompleted, StartedAt: started, FinishedAt: &fin,
	}
}

func TestHandler_Success(t *testing.T) {
	tr := completedTrip()

	rec := doFinish(t, newHandler(&fakeUsecase{trip: tr}))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}
	var got api.Trip
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Status != api.TripStatus(domaintrip.StatusCompleted) || got.FinishedAt == nil {
		t.Errorf("trip = %+v", got)
	}
}

func TestHandler_AlreadyCompleted(t *testing.T) {
	rec := doFinish(t, newHandler(&fakeUsecase{err: domaintrip.ErrTripCompleted}))

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body)
	}
	var p api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Code != domainerror.CodeTripCompleted {
		t.Errorf("code = %q, want trip_completed", p.Code)
	}
}

func TestHandler_InternalErrorNoDetails(t *testing.T) {
	rec := doFinish(t, newHandler(&fakeUsecase{err: errorsNew("secret tx detail")}))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "secret tx detail") {
		t.Errorf("response leaks internal details: %s", rec.Body)
	}
}

func errorsNew(s string) error { return &fmtErr{s} }

type fmtErr struct{ msg string }

func (e *fmtErr) Error() string { return e.msg }

type discardLogger struct{}

func (discardLogger) InfoCtx(context.Context, string, ...any)  {}
func (discardLogger) ErrorCtx(context.Context, string, ...any) {}
