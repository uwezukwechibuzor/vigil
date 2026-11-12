// Package platform provides shared server bootstrap, config, logging and telemetry for all Vigil services.
package platform

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	ServiceName string
	Env         string // local | staging | production
	Port        int
	LogLevel    string

	// Postgres
	DatabaseURL string

	// ClickHouse
	ClickHouseURL string

	// Redis
	RedisURL string

	// OpenTelemetry
	OTLPEndpoint string
	OTLPInsecure bool

	// Timeouts
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// LoadConfig reads configuration from environment variables with sane defaults.
func LoadConfig(serviceName string) (*Config, error) {
	c := &Config{
		ServiceName:     serviceName,
		Env:             getEnv("ENV", "local"),
		Port:            getEnvInt("PORT", 8080),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		ClickHouseURL:   getEnv("CLICKHOUSE_URL", ""),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		OTLPEndpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTLPInsecure:    getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", true),
		ReadTimeout:     getEnvDuration("HTTP_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:    getEnvDuration("HTTP_WRITE_TIMEOUT", 30*time.Second),
		ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
	}

	if c.ServiceName == "" {
		return nil, fmt.Errorf("service name must not be empty")
	}
	return c, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
