package platform

import (
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := LoadConfig("test-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServiceName != "test-service" {
		t.Errorf("service name = %q, want %q", cfg.ServiceName, "test-service")
	}
	if cfg.Env != "local" {
		t.Errorf("env = %q, want %q", cfg.Env, "local")
	}
	if cfg.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Port)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("read timeout = %v, want 10s", cfg.ReadTimeout)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := LoadConfig("svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "production" {
		t.Errorf("env = %q, want production", cfg.Env)
	}
	if cfg.Port != 9090 {
		t.Errorf("port = %d, want 9090", cfg.Port)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("log level = %q, want debug", cfg.LogLevel)
	}
}

func TestLoadConfig_EmptyServiceName(t *testing.T) {
	_, err := LoadConfig("")
	if err == nil {
		t.Error("expected error for empty service name")
	}
}
