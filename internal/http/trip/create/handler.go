package create

import (
	"encoding/json"
	"net/http"

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
	var body api.TripData
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&body); err != nil {
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

	writeJSON(w, http.StatusCreated, toAPI(t))
}
