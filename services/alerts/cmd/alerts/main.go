// Package main is the entrypoint for the vigil-alerts service.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/uwezukwechibuzor/vigil/services/alerts/internal/channels"
	"github.com/uwezukwechibuzor/vigil/services/alerts/internal/evaluator"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
	"github.com/uwezukwechibuzor/vigil/services/internal/platform"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	var pg *dbpg.Pool
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		p, err := dbpg.New(ctx, dsn)
		if err != nil {
			logger.Error("postgres connect", "error", err)
			os.Exit(1)
		}
		pg = p
	}

	dispatchers := []channels.Dispatcher{
		channels.NewNoopDispatcher("telegram", logger),
		channels.NewNoopDispatcher("discord", logger),
		channels.NewNoopDispatcher("email", logger),
		channels.NewNoopDispatcher("pagerduty", logger),
		channels.NewWebhookDispatcher(),
	}
	if url := os.Getenv("SLACK_WEBHOOK_URL"); url != "" {
		dispatchers = append(dispatchers, channels.NewSlackDispatcher(url))
	} else {
		dispatchers = append(dispatchers, channels.NewNoopDispatcher("slack", logger))
	}

	eval := evaluator.New(pg, dispatchers, logger)

	platform.StartService("alerts", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"service":"vigil-alerts","status":"ok"}`))
		})
		r.Post("/v1/notify", func(w http.ResponseWriter, _ *http.Request) {
			_ = eval
			w.WriteHeader(http.StatusAccepted)
		})
	})
}
