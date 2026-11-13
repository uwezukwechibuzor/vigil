// Package handler provides the HTTP layer for the flight check service.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/check"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/engine"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/store"
)

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "vigil",
		Subsystem: "flightcheck",
		Name:      "requests_total",
		Help:      "Total flight-check requests by verdict.",
	}, []string{"verdict"})

	latencyHistogram = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "vigil",
		Subsystem: "flightcheck",
		Name:      "latency_ms",
		Help:      "Flight-check end-to-end latency in milliseconds.",
		Buckets:   []float64{50, 100, 250, 500, 750, 1000, 1500, 2000},
	})
)

// Handler holds the dependencies for the HTTP layer.
type Handler struct {
	engine *engine.Engine
	store  *store.Store
	logger *slog.Logger
}

// New creates a new Handler.
func New(eng *engine.Engine, st *store.Store, logger *slog.Logger) *Handler {
	return &Handler{engine: eng, store: st, logger: logger}
}

// FlightCheck handles POST /v1/flight-check.
func (h *Handler) FlightCheck(w http.ResponseWriter, r *http.Request) {
	var req check.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	resp := h.engine.Run(r.Context(), &req)

	// Persist asynchronously so we don't block the response.
	go h.store.Save(r.Context(), &req, resp)

	requestsTotal.WithLabelValues(string(resp.Verdict)).Inc()
	latencyHistogram.Observe(float64(resp.LatencyMS))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("encode flight check response", "error", err)
	}
}
