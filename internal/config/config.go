package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr        string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration

	DatabaseURL             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration
	DatabaseMaxConnLifetime time.Duration
}

func Load() (*Config, error) {
	var errs []string

	cfg := &Config{
		HTTPAddr: getEnvDefault("HTTP_ADDR", ":8080"),
	}

	cfg.LogLevel = logLevel("LOG_LEVEL", "info", &errs)
	cfg.ShutdownTimeout = positiveDuration("SHUTDOWN_TIMEOUT", &errs)

	cfg.DatabaseURL = requiredString("DATABASE_URL", &errs)
	cfg.DatabaseConnectTimeout = positiveDuration("DATABASE_CONNECT_TIMEOUT", &errs)
	cfg.DatabaseQueryTimeout = positiveDuration("DATABASE_QUERY_TIMEOUT", &errs)
	cfg.DatabaseMaxConnLifetime = positiveDuration("DATABASE_MAX_CONN_LIFETIME", &errs)

	maxConns, maxOK := int32Value("DATABASE_MAX_CONNS", &errs)
	minConns, minOK := int32Value("DATABASE_MIN_CONNS", &errs)
	cfg.DatabaseMaxConns, cfg.DatabaseMinConns = maxConns, minConns
	if maxOK && maxConns <= 0 {
		errs = append(errs, "DATABASE_MAX_CONNS must be greater than 0")
	}
	if minOK && minConns < 0 {
		errs = append(errs, "DATABASE_MIN_CONNS must not be negative")
	}
	if maxOK && minOK && minConns > maxConns {
		errs = append(errs, "DATABASE_MIN_CONNS must not exceed DATABASE_MAX_CONNS")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid config: %s", strings.Join(errs, "; "))
	}
	return cfg, nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func requiredString(key string, errs *[]string) string {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, key+" is required")
	}
	return v
}

func positiveDuration(key string, errs *[]string) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, key+" is required")
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s: invalid duration %q", key, v))
		return 0
	}
	if d <= 0 {
		*errs = append(*errs, fmt.Sprintf("%s must be positive, got %q", key, v))
		return 0
	}
	return d
}

func int32Value(key string, errs *[]string) (int32, bool) {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, key+" is required")
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s: invalid integer %q", key, v))
		return 0, false
	}
	return int32(n), true
}

func logLevel(key, def string, errs *[]string) slog.Level {
	v := getEnvDefault(key, def)
	var level slog.Level
	switch strings.ToLower(v) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		*errs = append(*errs, fmt.Sprintf("%s: unknown level %q (use debug, info, warn or error)", key, v))
	}
	return level
}
