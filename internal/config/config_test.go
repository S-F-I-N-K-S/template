package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/S-F-I-N-K-S/template/internal/config"
)

const (
	envShutdownTimeout        = "SHUTDOWN_TIMEOUT"
	envDatabaseURL            = "DATABASE_URL"
	envDatabaseMaxConns       = "DATABASE_MAX_CONNS"
	envDatabaseMinConns       = "DATABASE_MIN_CONNS"
	envDatabaseConnectTimeout = "DATABASE_CONNECT_TIMEOUT"
	envDatabaseQueryTimeout   = "DATABASE_QUERY_TIMEOUT"
	envDatabaseMaxConnLife    = "DATABASE_MAX_CONN_LIFETIME"
	envOTLPEndpoint           = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envPositionCheckInterval  = "POSITION_CHECK_INTERVAL"
	envPositionStaleAfter     = "POSITION_STALE_AFTER"
	envPositionWorkers        = "POSITION_WORKERS"
	envPositionQueueSize      = "POSITION_QUEUE_SIZE"
	envPushHTTPURL            = "PUSH_HTTP_URL"
	envPushRequestTimeout     = "PUSH_REQUEST_TIMEOUT"
	testDatabaseURL           = "postgres://user:pass@localhost:5432/db" //nolint:gosec // G101: тестовый DSN, не секрет
)

func baseEnv(t *testing.T) {
	t.Helper()
	vars := map[string]string{
		envShutdownTimeout:        "5s",
		envDatabaseURL:            testDatabaseURL,
		envDatabaseMaxConns:       "10",
		envDatabaseMinConns:       "1",
		envDatabaseConnectTimeout: "5s",
		envDatabaseQueryTimeout:   "5s",
		envDatabaseMaxConnLife:    "1h",
		envOTLPEndpoint:           "http://localhost:4318",
		envPushHTTPURL:            "http://localhost:8090",
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	baseEnv(t)

	if _, err := config.Load(); err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
}

func TestLoad_RangeValidation(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "zero shutdown timeout",
			env:     map[string]string{envShutdownTimeout: "0s"},
			wantErr: envShutdownTimeout,
		},
		{
			name:    "negative query timeout",
			env:     map[string]string{envDatabaseQueryTimeout: "-1s"},
			wantErr: envDatabaseQueryTimeout,
		},
		{
			name:    "zero max conn lifetime",
			env:     map[string]string{envDatabaseMaxConnLife: "0s"},
			wantErr: envDatabaseMaxConnLife,
		},
		{
			name:    "zero max conns",
			env:     map[string]string{envDatabaseMaxConns: "0"},
			wantErr: envDatabaseMaxConns,
		},
		{
			name:    "negative min conns",
			env:     map[string]string{envDatabaseMinConns: "-1"},
			wantErr: envDatabaseMinConns,
		},
		{
			name:    "min greater than max",
			env:     map[string]string{envDatabaseMaxConns: "5", envDatabaseMinConns: "10"},
			wantErr: envDatabaseMinConns,
		},
		{
			name:    "zero position check interval",
			env:     map[string]string{envPositionCheckInterval: "0s"},
			wantErr: envPositionCheckInterval,
		},
		{
			name:    "negative position stale after",
			env:     map[string]string{envPositionStaleAfter: "-1s"},
			wantErr: envPositionStaleAfter,
		},
		{
			name:    "zero position workers",
			env:     map[string]string{envPositionWorkers: "0"},
			wantErr: envPositionWorkers,
		},
		{
			name:    "negative position queue size",
			env:     map[string]string{envPositionQueueSize: "-1"},
			wantErr: envPositionQueueSize,
		},
		{
			name:    "missing push http url",
			env:     map[string]string{envPushHTTPURL: ""},
			wantErr: envPushHTTPURL,
		},
		{
			name:    "zero push request timeout",
			env:     map[string]string{envPushRequestTimeout: "0s"},
			wantErr: envPushRequestTimeout,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			_, err := config.Load()
			if err == nil {
				t.Fatalf("Load() = nil, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Load() error = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestLoad_PositionDefaults(t *testing.T) {
	baseEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	if cfg.PositionCheckInterval != 30*time.Second {
		t.Errorf("PositionCheckInterval = %s, want 30s", cfg.PositionCheckInterval)
	}
	if cfg.PositionStaleAfter != 60*time.Second {
		t.Errorf("PositionStaleAfter = %s, want 60s", cfg.PositionStaleAfter)
	}
	if cfg.PositionWorkers != 4 {
		t.Errorf("PositionWorkers = %d, want 4", cfg.PositionWorkers)
	}
	if cfg.PositionQueueSize != 100 {
		t.Errorf("PositionQueueSize = %d, want 100", cfg.PositionQueueSize)
	}
	if cfg.PushRequestTimeout != time.Second {
		t.Errorf("PushRequestTimeout = %s, want 1s", cfg.PushRequestTimeout)
	}
}

func TestLoad_MinEqualsMaxIsValid(t *testing.T) {
	baseEnv(t)
	t.Setenv(envDatabaseMaxConns, "3")
	t.Setenv(envDatabaseMinConns, "3")

	if _, err := config.Load(); err != nil {
		t.Fatalf("Load() = %v, want nil (min == max must be allowed)", err)
	}
}
