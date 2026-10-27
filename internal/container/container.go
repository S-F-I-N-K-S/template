package container

import (
	"context"
	"log/slog"
	stdhttp "net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/S-F-I-N-K-S/template/internal/config"
	"github.com/S-F-I-N-K-S/template/internal/infra/telemetry"
)

type Container struct {
	server    *stdhttp.Server
	pool      *pgxpool.Pool
	telemetry *telemetry.Telemetry
	log       *slog.Logger
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	panic("TODO: container.New")
}

func (c *Container) Run() error {
	panic("TODO: Container.Run")
}

func (c *Container) Shutdown(ctx context.Context) error {
	panic("TODO: Container.Shutdown")
}
