package push_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/S-F-I-N-K-S/template/internal/gateway/push"
)

func TestRequestPosition_Accepted(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/push" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	g := push.New(srv.URL, time.Second)
	driverID, tripID := uuid.New(), uuid.New()
	if err := g.RequestPosition(context.Background(), driverID, tripID); err != nil {
		t.Fatalf("RequestPosition: %v", err)
	}

	if gotBody["recipient_id"] != driverID.String() {
		t.Errorf("recipient_id = %v, want %s", gotBody["recipient_id"], driverID)
	}
	if gotBody["kind"] != "REQUEST_POSITION" {
		t.Errorf("kind = %v, want REQUEST_POSITION", gotBody["kind"])
	}
	data, _ := gotBody["data"].(map[string]any)
	if data["trip_id"] != tripID.String() {
		t.Errorf("data.trip_id = %v, want %s", data["trip_id"], tripID)
	}
}

func TestRequestPosition_ServerError_IsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	g := push.New(srv.URL, time.Second)
	err := g.RequestPosition(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, push.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestRequestPosition_TooManyRequests_IsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	g := push.New(srv.URL, time.Second)
	err := g.RequestPosition(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, push.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestRequestPosition_BadRequest_IsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	g := push.New(srv.URL, time.Second)
	err := g.RequestPosition(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, push.ErrRejected) {
		t.Fatalf("err = %v, want ErrRejected", err)
	}
}

func TestRequestPosition_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	g := push.New(srv.URL, 5*time.Millisecond)
	err := g.RequestPosition(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, push.ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable (timeout)", err)
	}
	g.Close()
}
