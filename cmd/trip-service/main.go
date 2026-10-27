package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/S-F-I-N-K-S/template/internal/config"
	"github.com/S-F-I-N-K-S/template/internal/container"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	c, err := container.New(ctx, cfg)
	if err != nil {
		panic(err)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- c.Run() }()

	select {
	case err := <-errCh:
		if err != nil {
			panic(err)
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := c.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintln(os.Stderr, "shutdown:", err)
		os.Exit(1)
	}
}
