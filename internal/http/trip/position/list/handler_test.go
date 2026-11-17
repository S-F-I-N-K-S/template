package list_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/position/list"
)

type discardLogger struct{}

func (discardLogger) InfoCtx(context.Context, string, ...any)  {}
func (discardLogger) ErrorCtx(context.Context, string, ...any) {}

func newHandler(uc *Mockusecase, tripID uuid.UUID) http.HandlerFunc {
	h := list.New(uc, domainerror.NewWriter(discardLogger{}))
	return func(w http.ResponseWriter, r *http.Request) {
		h.Handle(w, r, tripID)
	}
}

func doList(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/x/positions", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	tripID := uuid.New()
	recordedAt := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), tripID).Return([]domainposition.Position{
		{ID: 1, TripID: tripID, Coordinates: domaintrip.Coordinates{Latitude: 1, Longitude: 2}, RecordedAt: recordedAt},
		{ID: 2, TripID: tripID, Coordinates: domaintrip.Coordinates{Latitude: 3, Longitude: 4}, RecordedAt: recordedAt.Add(time.Minute)},
	}, nil)

	rec := doList(t, newHandler(uc, tripID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	var got api.TripPositionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Positions) != 2 || got.Positions[0].Id != 1 || got.Positions[1].Id != 2 {
		t.Errorf("response = %+v", got)
	}
}

func TestHandler_EmptyRoute(t *testing.T) {
	ctrl := gomock.NewController(t)
	tripID := uuid.New()
	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), tripID).Return(nil, nil)

	rec := doList(t, newHandler(uc, tripID))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body)
	}

	var got api.TripPositionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Positions) != 0 {
		t.Errorf("response = %+v, want empty positions", got)
	}
}

func TestHandler_TripNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	tripID := uuid.New()
	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), tripID).Return(nil, domaintrip.ErrTripNotFound)

	rec := doList(t, newHandler(uc, tripID))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body)
	}
}
