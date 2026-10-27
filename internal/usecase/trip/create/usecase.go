package create

import (
	"context"
	"log/slog"

	domaintrip "github.com/S-F-I-N-K-S/template/internal/domain/trip"
	"github.com/S-F-I-N-K-S/template/internal/model"
)

type UseCase struct {
	tx      txManager
	trips   tripRepository
	history historyRepository
	clk     clock
	m       metrics
	log     *slog.Logger
}

func New(tx txManager, trips tripRepository, history historyRepository,
	clk clock, m metrics, log *slog.Logger) *UseCase {
	return &UseCase{tx: tx, trips: trips, history: history, clk: clk, m: m, log: log}
}

func (u *UseCase) Execute(ctx context.Context, in model.CreateTripInput) (domaintrip.Trip, error) {
	panic("TODO: create.UseCase.Execute")
}
