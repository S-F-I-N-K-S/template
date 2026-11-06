package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/S-F-I-N-K-S/template/internal/domain"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/service"
)

const maxBodyBytes = 1 << 20

type Pinger interface {
	Ping(ctx context.Context) error
}

type TripHandler struct {
	api.Unimplemented
	trips  *service.TripService
	pinger Pinger
	log    *slog.Logger
}

func NewTripHandler(trips *service.TripService, pinger Pinger, log *slog.Logger) *TripHandler {
	return &TripHandler{trips: trips, pinger: pinger, log: log}
}

func (h *TripHandler) CreateTrip(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	body, err := decodeTripData(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", err.Error())
		return
	}
	if err := validateTripData(body); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", err.Error())
		return
	}

	trip, err := h.trips.CreateTrip(r.Context(), service.CreateTripInput{
		UserID:     body.UserId,
		DriverID:   body.DriverId,
		StartPoint: domain.Coordinates{Latitude: body.StartPoint.Latitude, Longitude: body.StartPoint.Longitude},
		EndPoint:   domain.Coordinates{Latitude: body.EndPoint.Latitude, Longitude: body.EndPoint.Longitude},
		Price:      body.Price,
	})
	if err != nil {
		if errors.Is(err, domain.ErrDriverBusy) {
			writeProblem(w, r, http.StatusConflict, "driver_busy", "Driver busy", "driver already has an active trip")
			return
		}
		h.internalError(w, r, "create trip", err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
	writeJSON(w, http.StatusCreated, toAPITrip(trip))
}

func (h *TripHandler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	trip, err := h.trips.GetTrip(r.Context(), tripId)
	if err != nil {
		if errors.Is(err, domain.ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "no trip with this id")
			return
		}
		h.internalError(w, r, "get trip", err)
		return
	}
	writeJSON(w, http.StatusOK, toAPITrip(trip))
}

func (h *TripHandler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	trip, err := h.trips.FinishTrip(r.Context(), tripId)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTripNotFound):
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "no trip with this id")
		case errors.Is(err, domain.ErrTripCompleted):
			writeProblem(w, r, http.StatusConflict, "trip_completed", "Trip already completed", "trip is already finished")
		default:
			h.internalError(w, r, "finish trip", err)
		}
		return
	}
	writeJSON(w, http.StatusOK, toAPITrip(trip))
}

func (h *TripHandler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *TripHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.pinger.Ping(ctx); err != nil {
		h.log.WarnContext(r.Context(), "readiness check failed", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *TripHandler) internalError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.log.ErrorContext(r.Context(), "request failed", "op", op, "path", r.URL.Path, "error", err)
	writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "unexpected error")
}

func decodeTripData(src io.Reader) (api.TripData, error) {
	raw, err := io.ReadAll(src)
	if err != nil {
		return api.TripData{}, fmt.Errorf("read body: %w", err)
	}

	var body api.TripData
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		return api.TripData{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return api.TripData{}, errors.New("body must contain a single JSON object")
	}

	if err := requireFields(raw); err != nil {
		return api.TripData{}, err
	}
	return body, nil
}

func requireFields(raw []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return errors.New("body must be a JSON object")
	}

	for _, f := range []string{"user_id", "driver_id", "price"} {
		if isMissing(top[f]) {
			return fmt.Errorf("%s is required", f)
		}
	}
	for _, f := range []string{"start_point", "end_point"} {
		if isMissing(top[f]) {
			return fmt.Errorf("%s is required", f)
		}
		var point map[string]json.RawMessage
		if err := json.Unmarshal(top[f], &point); err != nil {
			return fmt.Errorf("%s must be an object", f)
		}
		for _, c := range []string{"latitude", "longitude"} {
			if isMissing(point[c]) {
				return fmt.Errorf("%s.%s is required", f, c)
			}
		}
	}
	return nil
}

func isMissing(v json.RawMessage) bool {
	v = bytes.TrimSpace(v)
	return len(v) == 0 || bytes.Equal(v, []byte("null"))
}

func toAPITrip(t domain.Trip) api.Trip {
	return api.Trip{
		Id:         t.ID,
		UserId:     t.UserID,
		DriverId:   t.DriverID,
		StartPoint: api.Coordinates{Latitude: t.StartPoint.Latitude, Longitude: t.StartPoint.Longitude},
		EndPoint:   api.Coordinates{Latitude: t.EndPoint.Latitude, Longitude: t.EndPoint.Longitude},
		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}

func validateTripData(data api.TripData) error {
	if data.UserId == uuid.Nil {
		return errors.New("user_id must not be the nil UUID")
	}
	if data.DriverId == uuid.Nil {
		return errors.New("driver_id must not be the nil UUID")
	}
	if data.Price < 0 {
		return errors.New("price must be >= 0")
	}
	if err := validateCoordinates(data.StartPoint); err != nil {
		return fmt.Errorf("start_point: %w", err)
	}
	if err := validateCoordinates(data.EndPoint); err != nil {
		return fmt.Errorf("end_point: %w", err)
	}
	return nil
}

func validateCoordinates(c api.Coordinates) error {
	if c.Latitude < -90 || c.Latitude > 90 {
		return errors.New("latitude out of range")
	}
	if c.Longitude < -180 || c.Longitude > 180 {
		return errors.New("longitude out of range")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	instance := r.URL.Path
	_ = json.NewEncoder(w).Encode(api.Problem{
		Type:     "https://tripgo.example/problems/" + code,
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Code:     code,
		Instance: &instance,
	})
}
