package save_test

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
	"go.uber.org/mock/gomock"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
	"github.com/S-F-I-N-K-S/template/internal/http/trip/position/save"
	"github.com/S-F-I-N-K-S/template/internal/model"
)

type discardLogger struct{}

func (discardLogger) InfoCtx(context.Context, string, ...any)  {}
func (discardLogger) ErrorCtx(context.Context, string, ...any) {}

func newHandler(uc *Mockusecase, tripID uuid.UUID) http.HandlerFunc {
	h := save.New(uc, domainerror.NewWriter(discardLogger{}))
	return func(w http.ResponseWriter, r *http.Request) {
		h.Handle(w, r, tripID)
	}
}

const validBody = `{"coordinates":{"latitude":55.75,"longitude":37.62},"recorded_at":"2026-10-27T12:00:00Z"}`

func doSave(t *testing.T, h http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/x/positions", io.NopCloser(strings.NewReader(body)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	tripID := uuid.New()
	recordedAt := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, in model.SavePositionInput) (domainposition.Position, error) {
			if in.TripID != tripID || in.Coordinates.Latitude != 55.75 || !in.RecordedAt.Equal(recordedAt) {
				t.Errorf("usecase input = %+v", in)
			}
			return domainposition.Position{ID: 1, TripID: tripID, Coordinates: in.Coordinates, RecordedAt: in.RecordedAt}, nil
		})

	rec := doSave(t, newHandler(uc, tripID), validBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body = %s", rec.Code, rec.Body)
	}

	var got api.TripPosition
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Id != 1 || got.TripId != tripID {
		t.Errorf("response = %+v", got)
	}
}

func TestHandler_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

	rec := doSave(t, newHandler(uc, uuid.New()), `{bad json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestHandler_MissingCoordinates(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

	rec := doSave(t, newHandler(uc, uuid.New()), `{"recorded_at":"2026-10-27T12:00:00Z"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestHandler_MissingRecordedAt(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

	rec := doSave(t, newHandler(uc, uuid.New()), `{"coordinates":{"latitude":1,"longitude":2}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestHandler_UnknownField(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Times(0)

	rec := doSave(t, newHandler(uc, uuid.New()),
		`{"coordinates":{"latitude":1,"longitude":2},"recorded_at":"2026-10-27T12:00:00Z","speed":90}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}

func TestHandler_TripNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domainposition.Position{}, domaintrip.ErrTripNotFound)

	rec := doSave(t, newHandler(uc, uuid.New()), validBody)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body)
	}
}

func TestHandler_TripCompleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domainposition.Position{}, domaintrip.ErrTripCompleted)

	rec := doSave(t, newHandler(uc, uuid.New()), validBody)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body)
	}
}

func TestHandler_ValidationErrorFromUsecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockusecase(ctrl)
	uc.EXPECT().Execute(gomock.Any(), gomock.Any()).
		Return(domainposition.Position{}, &domaintrip.ValidationError{Field: "recorded_at", Reason: "must not be more than 1 minute in the future"})

	rec := doSave(t, newHandler(uc, uuid.New()), validBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body)
	}
}
