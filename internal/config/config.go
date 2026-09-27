package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	LogLevel        string
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
}

func Load() (*Config, error) {
	var errs []string

	cfg := &Config{
		HTTPAddr: getEnvDefault("HTTP_ADDR", ":8080"),
		LogLevel: getEnvDefault("LOG_LEVEL", "info"),
	}

	cfg.HTTPReadTimeout = getDurationDefault("HTTP_READ_TIMEOUT", 5*time.Second, &errs)
	cfg.HTTPReadHeaderTimeout = getDurationDefault("HTTP_READ_HEADER_TIMEOUT", 3*time.Second, &errs)
	cfg.HTTPWriteTimeout = getDurationDefault("HTTP_WRITE_TIMEOUT", 10*time.Second, &errs)
	cfg.HTTPIdleTimeout = getDurationDefault("HTTP_IDLE_TIMEOUT", 60*time.Second, &errs)

	cfg.ShutdownTimeout = mustDuration("SHUTDOWN_TIMEOUT", &errs)
	cfg.DatabaseURL = mustString("DATABASE_URL", &errs)
	cfg.DatabaseMaxConns = int32(mustInt("DATABASE_MAX_CONNS", &errs))
	cfg.DatabaseMinConns = int32(mustInt("DATABASE_MIN_CONNS", &errs))
	cfg.DatabaseConnectTimeout = mustDuration("DATABASE_CONNECT_TIMEOUT", &errs)
	cfg.DatabaseQueryTimeout = mustDuration("DATABASE_QUERY_TIMEOUT", &errs)
	cfg.DatabaseMaxConnLifetime = mustDuration("DATABASE_MAX_CONN_LIFETIME", &errs)

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

func mustInt(key string, errs *[]string) int {
	v := os.Getenv(key)
	if v == "" {
		*errs = append(*errs, fmt.Sprintf("%s is required", key))
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		*errs = append(*errs, fmt.Sprintf("%s: invalid int %q: %v", key, v, err))
	}
	return n
}
