package list

import (
	"encoding/json"
	"net/http"

	domainposition "github.com/S-F-I-N-K-S/template/internal/domain/position"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
)

func toAPIList(positions []domainposition.Position) []api.TripPosition {
	out := make([]api.TripPosition, 0, len(positions))
	for _, p := range positions {
		out = append(out, api.TripPosition{
			Id:     p.ID,
			TripId: p.TripID,
			Coordinates: api.Coordinates{
				Latitude:  p.Coordinates.Latitude,
				Longitude: p.Coordinates.Longitude,
			},
			RecordedAt: p.RecordedAt,
		})
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
