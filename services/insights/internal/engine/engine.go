// Package engine runs the anomaly detection cycle and creates incidents.
package engine

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/uwezukwechibuzor/vigil/services/insights/internal/anomaly"
	"github.com/uwezukwechibuzor/vigil/services/insights/internal/correlation"
	"github.com/uwezukwechibuzor/vigil/services/insights/internal/llm"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// MetricFetcher fetches time-series metric data for anomaly detection.
type MetricFetcher interface {
	// FetchEndpointErrorRate returns the error rate time series for an endpoint (last hour, 1-min buckets).
	FetchEndpointErrorRate(ctx context.Context, endpointID string) ([]anomaly.Point, error)
	// FetchEndpointLatency returns the p95 latency time series.
	FetchEndpointLatency(ctx context.Context, endpointID string) ([]anomaly.Point, error)
	// FetchChainHeadLag returns the head lag time series.
	FetchChainHeadLag(ctx context.Context, chain string) ([]anomaly.Point, error)
}

// Engine runs anomaly detection on a schedule.
type Engine struct {
	pg         *dbpg.Pool
	fetcher    MetricFetcher
	llm        llm.Provider
	correlator *correlation.Correlator
	logger     *slog.Logger
	projectID  uuid.UUID
}

// New creates an Engine.
func New(pg *dbpg.Pool, fetcher MetricFetcher, provider llm.Provider, projectID uuid.UUID, logger *slog.Logger) *Engine {
	return &Engine{
		pg:         pg,
		fetcher:    fetcher,
		llm:        provider,
		correlator: correlation.NewCorrelator(),
		logger:     logger,
		projectID:  projectID,
	}
}

// RunOnce performs one anomaly detection cycle for the given endpoints.
// Returns the number of incidents created.
func (e *Engine) RunOnce(ctx context.Context, endpointIDs []string, chains []string) (int, error) {
	_ = chains // chains processed in a future iteration
	created := 0

	for _, eid := range endpointIDs {
		// Fetch error rate
		errSeries, err := e.fetcher.FetchEndpointErrorRate(ctx, eid)
		if err != nil {
			e.logger.Warn("fetch error rate", "endpoint", eid, "err", err)
			continue
		}

		d := anomaly.NewDetector("error_rate")
		anomalies := d.Detect(errSeries)
		groups := e.correlator.Group(anomalies, "", eid)

		for _, g := range groups {
			n, err := e.createIncident(ctx, g)
			if err != nil {
				e.logger.Error("create incident", "err", err)
				continue
			}
			created += n
		}
	}

	return created, nil
}

func (e *Engine) createIncident(ctx context.Context, g correlation.Group) (int, error) {
	if len(g.Anomalies) == 0 {
		return 0, nil
	}

	// Build evidence
	evidence := map[string]any{
		"metric":        g.Anomalies[0].Metric,
		"anomaly_count": len(g.Anomalies),
		"peak_z_score":  g.Anomalies[0].ZScore,
		"peak_value":    g.Anomalies[0].Value,
		"baseline":      g.Anomalies[0].Baseline,
		"chain":         g.Chain,
		"endpoint":      g.Endpoint,
	}
	evidenceJSON, _ := json.Marshal([]any{evidence})

	ev := llm.Evidence{
		Chain:      g.Chain,
		Endpoint:   g.Endpoint,
		Metric:     g.Anomalies[0].Metric,
		Anomalies:  len(g.Anomalies),
		PeakZScore: g.Anomalies[0].ZScore,
		PeakValue:  g.Anomalies[0].Value,
		Baseline:   g.Anomalies[0].Baseline,
		StartedAt:  g.StartedAt.Format(time.RFC3339),
		Title:      g.Title,
	}

	explanation, err := e.llm.Explain(ctx, ev)
	if err != nil {
		e.logger.Warn("llm explain failed, using template", "err", err)
		explanation = "Anomaly detected on " + ev.Chain + " endpoint. Check provider status and connectivity."
	}

	summary := g.Title + ": " + g.Anomalies[0].Metric + " anomaly detected"

	_, err = e.pg.CreateIncident(ctx, e.projectID, nil, g.Severity, g.Title, summary, explanation, evidenceJSON)
	if err != nil {
		return 0, err
	}
	return 1, nil
}
