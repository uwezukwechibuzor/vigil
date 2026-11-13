// Package main is the entrypoint for the vigil-insights service.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/uwezukwechibuzor/vigil/services/insights/internal/engine"
	"github.com/uwezukwechibuzor/vigil/services/insights/internal/llm"
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

	var provider llm.Provider
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		provider = llm.NewAnthropicProvider(key, "")
	} else {
		logger.Warn("ANTHROPIC_API_KEY not set; using template fallback")
		provider = llm.NewTemplateProvider()
	}

	projectID := uuid.Nil
	if s := os.Getenv("INSIGHTS_PROJECT_ID"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			projectID = id
		}
	}

	var eng *engine.Engine
	if pg != nil {
		eng = engine.New(pg, nil, provider, projectID, logger) // fetcher wired in a future task
	}

	platform.StartService("insights", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"service":"vigil-insights","status":"ok"}`))
		})
		r.Post("/v1/scan", func(w http.ResponseWriter, req *http.Request) {
			if eng == nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			n, err := eng.RunOnce(req.Context(), nil, nil)
			if err != nil {
				logger.Error("scan", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"incidents_created":` + strconv.Itoa(n) + `}`))
		})
	})
}
