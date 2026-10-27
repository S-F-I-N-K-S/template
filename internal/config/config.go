package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

const protocolHTTPProtobuf = "http/protobuf"

type Config struct {
	HTTPAddr        string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration

	HTTPReadTimeout       time.Duration
	HTTPReadHeaderTimeout time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration

	DatabaseURL             string
	DatabaseMaxConns        int32
	DatabaseMinConns        int32
	DatabaseConnectTimeout  time.Duration
	DatabaseQueryTimeout    time.Duration
	DatabaseMaxConnLifetime time.Duration

	ServiceVersion string
	OTLPEndpoint   string
	OTLPProtocol   string
}

func Load() (*Config, error) {
	var errs []string

	cfg := &Config{
		HTTPAddr:       getEnvDefault("HTTP_ADDR", ":8080"),
		ServiceVersion: getEnvDefault("SERVICE_VERSION", "dev"),
	}

	cfg.LogLevel = parseLogLevel(getEnvDefault("LOG_LEVEL", "info"), &errs)

	cfg.HTTPReadTimeout = getDurationDefault("HTTP_READ_TIMEOUT", 5*time.Second, &errs)
	cfg.HTTPReadHeaderTimeout = getDurationDefault("HTTP_READ_HEADER_TIMEOUT", 3*time.Second, &errs)
	cfg.HTTPWriteTimeout = getDurationDefault("HTTP_WRITE_TIMEOUT", 10*time.Second, &errs)
	cfg.HTTPIdleTimeout = getDurationDefault("HTTP_IDLE_TIMEOUT", 60*time.Second, &errs)

	cfg.ShutdownTimeout = mustDuration("SHUTDOWN_TIMEOUT", &errs)
	cfg.DatabaseURL = mustString("DATABASE_URL", &errs)
	cfg.DatabaseMaxConns = mustInt32("DATABASE_MAX_CONNS", &errs)
	cfg.DatabaseMinConns = mustInt32("DATABASE_MIN_CONNS", &errs)
	cfg.DatabaseConnectTimeout = mustDuration("DATABASE_CONNECT_TIMEOUT", &errs)
	cfg.DatabaseQueryTimeout = mustDuration("DATABASE_QUERY_TIMEOUT", &errs)
	cfg.DatabaseMaxConnLifetime = mustDuration("DATABASE_MAX_CONN_LIFETIME", &errs)

	// Сам SDK читает OTEL_* из окружения. Здесь только fail-fast проверка:
	// адрес не задан — ошибка конфига; коллектор недоступен — штатная работа.
	cfg.OTLPEndpoint = mustString("OTEL_EXPORTER_OTLP_ENDPOINT", &errs)
	cfg.OTLPProtocol = getEnvDefault("OTEL_EXPORTER_OTLP_PROTOCOL", protocolHTTPProtobuf)
	if cfg.OTLPProtocol != protocolHTTPProtobuf {
		errs = append(errs, fmt.Sprintf("OTEL_EXPORTER_OTLP_PROTOCOL: unsupported %q, only %q", cfg.OTLPProtocol, protocolHTTPProtobuf))
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid config: %v", errs)
	}
	return cfg, nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseLogLevel(v string, errs *[]string) slog.Level {
	var level slog.Level
	if err := level.UnmarshalText([]byte(v)); err != nil {
		*errs = append(*errs, fmt.Sprintf("LOG_LEVEL: invalid level %q: %v", v, err))
		return slog.LevelInfo
	}
	return level
}

func getDurationDefault(key string, def time.Duration, errs *[]string) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s: invalid duration %q: %v", key, v, err))
		return def
	}
	return d
}

func mustString(key string, errs *[]string) string {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, fmt.Sprintf("%s is required", key))
	}
	return v
}

func mustDuration(key string, errs *[]string) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, fmt.Sprintf("%s is required", key))
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s: invalid duration %q: %v", key, v, err))
	}
	return d
}

func mustInt32(key string, errs *[]string) int32 {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, fmt.Sprintf("%s is required", key))
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s: invalid int32 %q: %v", key, v, err))
		return 0
	}
	return int32(n) //nolint:gosec // ParseInt с bitSize=32 гарантирует диапазон int32
}
