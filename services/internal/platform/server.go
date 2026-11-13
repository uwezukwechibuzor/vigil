package platform

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewRouter builds a chi router with shared middleware and standard endpoints.
func NewRouter(_ *Config, logger *slog.Logger) chi.Router {
	r := chi.NewRouter()

	r.Use(Recovery(logger))
	r.Use(RequestID)
	r.Use(RequestLogger(logger))

	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz)
	r.Handle("/metrics", promhttp.Handler())

	return r
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"ok"}`)
}

func readyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"ready"}`)
}

// Run starts the HTTP server and blocks until SIGTERM/SIGINT, then shuts down gracefully.
func Run(cfg *Config, handler http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      otelhttp.NewHandler(handler, cfg.ServiceName),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		BaseContext:  func(net.Listener) context.Context { return context.Background() },
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", srv.Addr, "service", cfg.ServiceName)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-quit:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("server stopped")
	return nil
}

// MustLoadConfig loads config and exits on error.
func MustLoadConfig(serviceName string) *Config {
	cfg, err := LoadConfig(serviceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

// StartService is the standard entrypoint for all Vigil services.
func StartService(serviceName string, register func(r chi.Router)) {
	cfg := MustLoadConfig(serviceName)
	logger := NewLogger(cfg.LogLevel)

	ctx := context.Background()
	shutdownTracing, err := InitTracing(ctx, cfg, logger)
	if err != nil {
		logger.Error("tracing init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(ctx)
	}()

	// Main router: shared middleware + health/metrics only (no auth required).
	r := chi.NewRouter()
	r.Use(Recovery(logger))
	r.Use(RequestID)
	r.Use(RequestLogger(logger))
	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz)
	r.Handle("/metrics", promhttp.Handler())

	// Service sub-router: the callback can freely add its own middleware and
	// routes without chi complaining about middleware-after-routes on the main mux.
	sub := chi.NewRouter()
	register(sub)
	r.Mount("/", sub)

	if err := Run(cfg, r, logger); err != nil {
		logger.Error("service error", "error", err)
		os.Exit(1)
	}
}
