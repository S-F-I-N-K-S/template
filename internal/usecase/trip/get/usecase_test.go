package get_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	. "github.com/S-F-I-N-K-S/template/internal/usecase/trip/get"
)

func sampleTrip() domaintrip.Trip {
	return domaintrip.Trip{
		ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(),
		Status:    domaintrip.StatusActive,
		StartedAt: time.Now(),
	}
}

func TestExecute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := NewMocktripReader(ctrl)
	log := NewMocklogger(ctrl)
	want := sampleTrip()

	reader.EXPECT().GetByID(gomock.Any(), want.ID).Return(want, nil)

	uc := New(reader, log)
	got, err := uc.Execute(context.Background(), want.ID)
	if err != nil || got.ID != want.ID {
		t.Fatalf("Execute = (%v, %v)", got, err)
	}
}

func TestExecute_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := NewMocktripReader(ctrl)
	log := NewMocklogger(ctrl)
	id := uuid.New()

	reader.EXPECT().GetByID(gomock.Any(), id).Return(domaintrip.Trip{}, domaintrip.ErrTripNotFound)
	log.EXPECT().Info("trip not found", gomock.Any(), gomock.Any())

	uc := New(reader, log)
	_, err := uc.Execute(context.Background(), id)
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("err = %v, want ErrTripNotFound", err)
	}
}

func TestExecute_InternalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := NewMocktripReader(ctrl)
	log := NewMocklogger(ctrl)
	id := uuid.New()
	dbErr := errors.New("db down")

	reader.EXPECT().GetByID(gomock.Any(), id).Return(domaintrip.Trip{}, dbErr)
	log.EXPECT().Error("trip read failed", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	uc := New(reader, log)
	_, err := uc.Execute(context.Background(), id)
	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want db error", err)
	}
}
