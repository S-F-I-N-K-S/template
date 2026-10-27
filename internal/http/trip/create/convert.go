package create

import (
	"encoding/json"
	"net/http"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
)

func toDomainCoords(c api.Coordinates) domaintrip.Coordinates {
	return domaintrip.Coordinates{Latitude: c.Latitude, Longitude: c.Longitude}
}

func toAPI(t domaintrip.Trip) api.Trip {
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

func WriteTrip(w http.ResponseWriter, code int, t domaintrip.Trip) {
	writeJSON(w, code, toAPI(t))
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
