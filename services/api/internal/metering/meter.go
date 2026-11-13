// Package metering provides async API call counting and usage rollup.
package metering

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// DB is the subset of postgres required by the meter.
type DB interface {
	IncrementAPICalls(ctx context.Context, orgID uuid.UUID) error
}

// Meter manages async API call counting and scheduled cleanup.
type Meter struct {
	db     DB
	logger *slog.Logger
}

// New creates a Meter. db may be nil for tests.
func New(db DB, logger *slog.Logger) *Meter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Meter{db: db, logger: logger}
}

// IncrementAPICalls increments the api_calls counter for orgID.
// It fires asynchronously and never blocks the caller.
func (m *Meter) IncrementAPICalls(_ context.Context, orgID uuid.UUID) {
	if m.db == nil {
		return
	}
	go func() { // #nosec G118 -- intentionally outlives the request context; uses its own timeout
		rCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.db.IncrementAPICalls(rCtx, orgID); err != nil {
			m.logger.Warn("increment api calls failed", "org_id", orgID, "error", err)
		}
	}()
}

// StartRollup starts a background goroutine that runs cleanup at midnight daily.
// It stops when ctx is cancelled.
func (m *Meter) StartRollup(ctx context.Context) {
	go func() {
		for {
			next := nextMidnight()
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				m.logger.Info("usage rollup: placeholder (retention managed by DB TTL policy)")
			}
		}
	}()
}

// nextMidnight returns the next midnight in UTC.
func nextMidnight() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}
