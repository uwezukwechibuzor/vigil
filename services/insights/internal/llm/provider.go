// Package llm provides LLM-powered root-cause analysis for incidents.
package llm

import "context"

// Evidence is structured context for root-cause analysis.
type Evidence struct {
	Chain      string
	Endpoint   string
	Metric     string
	Anomalies  int
	PeakZScore float64
	PeakValue  float64
	Baseline   float64
	StartedAt  string
	Title      string
}

// Provider writes a root-cause explanation from structured evidence.
type Provider interface {
	// Explain generates a root-cause explanation from structured evidence.
	Explain(ctx context.Context, ev Evidence) (explanation string, err error)
}
