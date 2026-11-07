package create

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/http/domainerror"
	"github.com/S-F-I-N-K-S/template/internal/model"
)

type Handler struct {
	uc   usecase
	errs *domainerror.Writer
}

func New(uc usecase, errs *domainerror.Writer) *Handler {
	return &Handler{uc: uc, errs: errs}
}

const maxBodyBytes = 1 << 20 // 1 MiB

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, _ api.CreateTripParams) {
	body, err := decodeTripData(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		h.errs.RequestError(w, r, err)
		return
	}

	t, err := h.uc.Execute(r.Context(), model.CreateTripInput{
		UserID:     body.UserId,
		DriverID:   body.DriverId,
		StartPoint: toDomainCoords(body.StartPoint),
		EndPoint:   toDomainCoords(body.EndPoint),
		Price:      body.Price,
	})
	if err != nil {
		h.errs.Write(w, r, err)
		return
	}

	w.Header().Set("Location", locationFor(t.ID))
	writeJSON(w, http.StatusCreated, toAPI(t))
}

type rawCoordinates struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type rawTripData struct {
	UserId     *uuid.UUID      `json:"user_id"`
	DriverId   *uuid.UUID      `json:"driver_id"`
	StartPoint *rawCoordinates `json:"start_point"`
	EndPoint   *rawCoordinates `json:"end_point"`
	Price      *int64          `json:"price"`
}

func decodeTripData(r io.Reader) (api.TripData, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var raw rawTripData
	if err := dec.Decode(&raw); err != nil {
		return api.TripData{}, err
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		if err == nil {
			return api.TripData{}, errors.New("body must contain a single JSON object")
		}
		return api.TripData{}, err
	}

	if err := requireCoordinates("start_point", raw.StartPoint); err != nil {
		return api.TripData{}, err
	}
	if err := requireCoordinates("end_point", raw.EndPoint); err != nil {
		return api.TripData{}, err
	}
	switch {
	case raw.UserId == nil:
		return api.TripData{}, errors.New("user_id is required")
	case raw.DriverId == nil:
		return api.TripData{}, errors.New("driver_id is required")
	case raw.Price == nil:
		return api.TripData{}, errors.New("price is required")
	}

	return api.TripData{
		UserId:   *raw.UserId,
		DriverId: *raw.DriverId,
		StartPoint: api.Coordinates{
			Latitude:  *raw.StartPoint.Latitude,
			Longitude: *raw.StartPoint.Longitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  *raw.EndPoint.Latitude,
			Longitude: *raw.EndPoint.Longitude,
		},
		Price: *raw.Price,
	}, nil
}

func requireCoordinates(field string, c *rawCoordinates) error {
	if c == nil {
		return fmt.Errorf("%s is required", field)
	}
	if c.Latitude == nil {
		return fmt.Errorf("%s.latitude is required", field)
	}
	if c.Longitude == nil {
		return fmt.Errorf("%s.longitude is required", field)
	}
	return nil
}

func locationFor(id uuid.UUID) string {
	return "/api/v1/trips/" + id.String()
}
