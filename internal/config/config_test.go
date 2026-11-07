package config_test

import (
	"strings"
	"testing"

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

func TestLoad_MinEqualsMaxIsValid(t *testing.T) {
	baseEnv(t)
	t.Setenv(envDatabaseMaxConns, "3")
	t.Setenv(envDatabaseMinConns, "3")

	if _, err := config.Load(); err != nil {
		t.Fatalf("Load() = %v, want nil (min == max must be allowed)", err)
	}
}
