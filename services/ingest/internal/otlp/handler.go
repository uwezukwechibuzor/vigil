package otlp

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	goredis "github.com/redis/go-redis/v9"
	colpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	ingestsampler "github.com/uwezukwechibuzor/vigil/services/ingest/internal/sampler"
)

var (
	spansReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vigil_ingest_spans_received_total",
		Help: "Total number of spans received by the ingest service.",
	})
	spansDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vigil_ingest_spans_dropped_total",
		Help: "Total number of spans dropped due to sampling or back-pressure.",
	})
	queueLen = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "vigil_ingest_queue_len",
		Help: "Current length of the ingest Redis stream queue.",
	})
)

// Handler handles OTLP/HTTP ingestion for traces, metrics, and logs.
type Handler struct {
	redis      goredis.UniversalClient
	sampler    *ingestsampler.Sampler
	logger     *slog.Logger
	stream     string
	maxBytes   int64
	maxQLen    int64
	sampleRate float64
}

// NewHandler creates a new Handler with the given Redis client.
func NewHandler(rdb goredis.UniversalClient, sampler *ingestsampler.Sampler, logger *slog.Logger, stream string, maxBytes, maxQLen int64, sampleRate float64) *Handler {
	return &Handler{
		redis:      rdb,
		sampler:    sampler,
		logger:     logger,
		stream:     stream,
		maxBytes:   maxBytes,
		maxQLen:    maxQLen,
		sampleRate: sampleRate,
	}
}

// TracesHandler handles POST /v1/traces.
// Accepts both application/x-protobuf and application/json content types.
func (h *Handler) TracesHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, h.maxBytes+1))
	if err != nil {
		h.logger.WarnContext(r.Context(), "read body error", "error", err)
		http.Error(w, `{"error":"read error"}`, http.StatusBadRequest)
		return
	}
	if int64(len(body)) > h.maxBytes {
		http.Error(w, `{"error":"payload too large"}`, http.StatusRequestEntityTooLarge)
		return
	}

	var req colpb.ExportTraceServiceRequest
	ct := r.Header.Get("Content-Type")
	if ct == "application/x-protobuf" {
		if err := proto.Unmarshal(body, &req); err != nil {
			h.logger.WarnContext(r.Context(), "proto unmarshal error", "error", err)
			http.Error(w, `{"error":"invalid protobuf"}`, http.StatusBadRequest)
			return
		}
	} else {
		if err := protojson.Unmarshal(body, &req); err != nil {
			h.logger.WarnContext(r.Context(), "json unmarshal error", "error", err)
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
	}

	spans := ParseResourceSpans(req.ResourceSpans)
	spansReceived.Add(float64(len(spans)))

	// Back-pressure check.
	ql, err := h.redis.XLen(r.Context(), h.stream).Result()
	if err == nil {
		queueLen.Set(float64(ql))
		if ql > h.maxQLen {
			spansDropped.Add(float64(len(spans)))
			http.Error(w, `{"error":"queue full"}`, http.StatusTooManyRequests)
			return
		}
	}

	for i := range spans {
		Scrub(&spans[i])

		if !h.sampler.ShouldKeep(spans[i], h.sampleRate) {
			spansDropped.Inc()
			continue
		}

		b, err := json.Marshal(spans[i])
		if err != nil {
			h.logger.WarnContext(r.Context(), "marshal span error", "error", err)
			continue
		}

		if err := h.redis.XAdd(r.Context(), &goredis.XAddArgs{
			Stream: h.stream,
			Values: map[string]any{"span": string(b)},
		}).Err(); err != nil {
			h.logger.WarnContext(r.Context(), "redis xadd error", "error", err)
		}
	}

	resp := &colpb.ExportTraceServiceResponse{}
	respBytes, _ := proto.Marshal(resp)
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBytes)
}

// MetricsHandler handles POST /v1/metrics — accepts and discards (returns 200).
func (h *Handler) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	w.WriteHeader(http.StatusOK)
}

// LogsHandler handles POST /v1/logs — accepts and discards (returns 200).
func (h *Handler) LogsHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	w.WriteHeader(http.StatusOK)
}
