package create

import (
	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	api "github.com/S-F-I-N-K-S/template/internal/generated"
	"github.com/S-F-I-N-K-S/template/internal/model"
)

func toInput(b api.TripData) model.CreateTripInput {
	panic("TODO: toInput")
}

func toAPI(t domaintrip.Trip) api.Trip {
	panic("TODO: toAPI")
}
