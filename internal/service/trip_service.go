package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/S-F-I-N-K-S/template/internal/domain"
	"github.com/S-F-I-N-K-S/template/internal/repository"
	"github.com/S-F-I-N-K-S/template/internal/txmanager"
)

type CreateTripInput struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint domain.Coordinates
	EndPoint   domain.Coordinates
	Price      int64
}

type TripService struct {
	txManager   txmanager.TxManager
	tripRepo    *repository.TripRepository
	historyRepo *repository.HistoryRepository
}

func NewTripService(tm txmanager.TxManager, tripRepo *repository.TripRepository, historyRepo *repository.HistoryRepository) *TripService {
	return &TripService{txManager: tm, tripRepo: tripRepo, historyRepo: historyRepo}
}

func (s *TripService) CreateTrip(ctx context.Context, in CreateTripInput) (domain.Trip, error) {
	trip := domain.Trip{
		ID:         uuid.New(),
		UserID:     in.UserID,
		DriverID:   in.DriverID,
		StartPoint: in.StartPoint,
		EndPoint:   in.EndPoint,
		Price:      in.Price,
		Status:     domain.StatusActive,
		StartedAt:  time.Now().UTC(),
	}

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.tripRepo.Create(ctx, trip); err != nil {
			return err
		}
		return s.historyRepo.Append(ctx, trip.ID, "", string(domain.StatusActive), "trip created")
	})
	if err != nil {
		return domain.Trip{}, err
	}
	return trip, nil
}

func (s *TripService) GetTrip(ctx context.Context, id uuid.UUID) (domain.Trip, error) {
	return s.tripRepo.GetByID(ctx, id)
}

func (s *TripService) FinishTrip(ctx context.Context, id uuid.UUID) (domain.Trip, error) {
	finishedAt := time.Now().UTC()

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		if err := s.tripRepo.Finish(ctx, id, finishedAt); err != nil {
			return err
		}
		return s.historyRepo.Append(ctx, id, string(domain.StatusActive), string(domain.StatusCompleted), "trip finished")
	})
	if err != nil {
		return domain.Trip{}, err
	}
	return s.tripRepo.GetByID(ctx, id)
}
