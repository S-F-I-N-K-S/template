package save

import (
	"encoding/json"
	"net/http"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
)

func toDomainCoords(c api.Coordinates) domaintrip.Coordinates {
	return domaintrip.Coordinates{Latitude: c.Latitude, Longitude: c.Longitude}
}

func toAPI(p domainposition.Position) api.TripPosition {
	return api.TripPosition{
		Id:     p.ID,
		TripId: p.TripID,
		Coordinates: api.Coordinates{
			Latitude:  p.Coordinates.Latitude,
			Longitude: p.Coordinates.Longitude,
		},
		RecordedAt: p.RecordedAt,
	}
}

func WriteTripPosition(w http.ResponseWriter, code int, p domainposition.Position) {
	writeJSON(w, code, toAPI(p))
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
