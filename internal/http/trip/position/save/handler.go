package save

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

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

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	body, err := decodePositionData(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		h.errs.RequestError(w, r, err)
		return
	}

	p, err := h.uc.Execute(r.Context(), model.SavePositionInput{
		TripID:      tripID,
		Coordinates: toDomainCoords(body.Coordinates),
		RecordedAt:  body.RecordedAt,
	})
	if err != nil {
		h.errs.Write(w, r, err)
		return
	}

	WriteTripPosition(w, http.StatusCreated, p)
}

type rawCoordinates struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type rawPositionData struct {
	Coordinates *rawCoordinates `json:"coordinates"`
	RecordedAt  *time.Time      `json:"recorded_at"`
}

func decodePositionData(r io.Reader) (api.PositionData, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	var raw rawPositionData
	if err := dec.Decode(&raw); err != nil {
		return api.PositionData{}, err
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		if err == nil {
			return api.PositionData{}, errors.New("body must contain a single JSON object")
		}
		return api.PositionData{}, err
	}

	if err := requireCoordinates("coordinates", raw.Coordinates); err != nil {
		return api.PositionData{}, err
	}
	if raw.RecordedAt == nil {
		return api.PositionData{}, errors.New("recorded_at is required")
	}

	return api.PositionData{
		Coordinates: api.Coordinates{
			Latitude:  *raw.Coordinates.Latitude,
			Longitude: *raw.Coordinates.Longitude,
		},
		RecordedAt: *raw.RecordedAt,
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
