package create

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
	"github.com/S-F-I-N-K-S/template/internal/model"
)

type fakeUsecase struct {
	trip domaintrip.Trip
	err  error
	in   model.CreateTripInput
}

func (f *fakeUsecase) Execute(_ context.Context, in model.CreateTripInput) (domaintrip.Trip, error) {
	f.in = in
	return f.trip, f.err
}

func newHandler(uc usecase) http.HandlerFunc {
	h := New(uc, domainerror.NewWriter(discardLogger{}))
	return func(w http.ResponseWriter, r *http.Request) {
		h.Handle(w, r, api.CreateTripParams{})
	}
}

func sampleTrip() domaintrip.Trip {
	fin := time.Date(2026, 10, 27, 13, 0, 0, 0, time.UTC)
	return domaintrip.Trip{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: domaintrip.Coordinates{Latitude: 55.75, Longitude: 37.62},
		EndPoint:   domaintrip.Coordinates{Latitude: 59.94, Longitude: 30.31},
		Price:      1450,
		Status:     domaintrip.StatusActive,
		StartedAt:  fin.Add(-time.Hour),
		FinishedAt: &fin,
	}
}

const validBody = `{"user_id":"11111111-1111-1111-1111-111111111111",` +
	`"driver_id":"22222222-2222-2222-2222-222222222222",` +
	`"start_point":{"latitude":55.75,"longitude":37.62},` +
	`"end_point":{"latitude":59.94,"longitude":30.31},"price":1450}`

func doCreate(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", io.NopCloser(strings.NewReader(body)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Success(t *testing.T) {
	tr := sampleTrip()
	uc := &fakeUsecase{trip: tr}

	rec := doCreate(t, newHandler(uc), validBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body)
	}
	var got api.Trip
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Id != tr.ID || got.Status != api.TripStatus(domaintrip.StatusActive) {
		t.Errorf("trip = %+v", got)
	}
	if uc.in.Price != 1450 || uc.in.DriverID.String() != "22222222-2222-2222-2222-222222222222" {
		t.Errorf("usecase input = %+v", uc.in)
	}
}

func TestHandler_InvalidBody(t *testing.T) {
	uc := &fakeUsecase{}

	rec := doCreate(t, newHandler(uc), `{bad json`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
	var p api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p.Code != domainerror.CodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", p.Code)
	}
}

func TestHandler_DriverBusy(t *testing.T) {
	uc := &fakeUsecase{err: domaintrip.ErrDriverBusy}

	rec := doCreate(t, newHandler(uc), validBody)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body)
	}
	var p api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p.Code != domainerror.CodeDriverBusy {
		t.Errorf("code = %q, want driver_busy", p.Code)
	}
}

func TestHandler_InternalErrorNoDetails(t *testing.T) {
	uc := &fakeUsecase{err: &internalErr{}}

	rec := doCreate(t, newHandler(uc), validBody)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	if strings.Contains(body, "secret pgx detail") {
		t.Errorf("response leaks internal error details: %s", body)
	}
	var p api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if p.Code != domainerror.CodeInternal {
		t.Errorf("code = %q, want internal_error", p.Code)
	}
}

type internalErr struct{}

func (*internalErr) Error() string { return "secret pgx detail" }

type discardLogger struct{}

func (discardLogger) InfoCtx(context.Context, string, ...any)  {}
func (discardLogger) ErrorCtx(context.Context, string, ...any) {}
