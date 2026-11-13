// Package sampler implements head-based span sampling for the ingest service.
package sampler

import (
	"math/rand"

	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
)

// Sampler decides which spans to keep based on configurable rates.
type Sampler struct {
	defaultRate float64 // 0.0–1.0; 1.0 keeps all spans
}

// New creates a Sampler with the given default rate.
// A rate of 1.0 keeps all spans; 0.0 drops all spans.
func New(defaultRate float64) *Sampler {
	if defaultRate < 0 {
		defaultRate = 0
	}
	if defaultRate > 1 {
		defaultRate = 1
	}
	return &Sampler{defaultRate: defaultRate}
}

// ShouldKeep returns true if the span should be ingested.
// Always keeps spans with status_code == "error", a vigil_tx_hash, or a vigil_flight_check_id.
// Otherwise keeps with probability rate (1.0 = keep all, 0.5 = keep 50%).
func (s *Sampler) ShouldKeep(span dbch.Span, rate float64) bool {
	// Always keep error spans.
	if span.StatusCode == "error" {
		return true
	}
	// Always keep spans with an onchain tx hash.
	if span.VigilTxHash != "" {
		return true
	}
	// Always keep spans tied to a flight check.
	if span.VigilFlightCheckID != "" {
		return true
	}
	// Apply sampling rate.
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	return rand.Float64() < rate //nolint:gosec // non-cryptographic sampling is intentional
}
