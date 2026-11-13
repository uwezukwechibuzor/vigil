package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// UsageCounter holds daily usage metrics for an org.
type UsageCounter struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	Day          time.Time
	Traces       int64
	FlightChecks int64
	APICalls     int64
	UpdatedAt    time.Time
}

// GetUsageByOrgAndDateRange returns daily usage counters between from and to (inclusive).
func (p *Pool) GetUsageByOrgAndDateRange(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]UsageCounter, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, org_id, day, traces, flight_checks, api_calls, updated_at
		FROM usage_counters
		WHERE org_id = $1 AND day >= $2 AND day <= $3
		ORDER BY day ASC
	`, orgID, from, to)
	if err != nil {
		return nil, fmt.Errorf("get usage: %w", err)
	}
	defer rows.Close()

	var out []UsageCounter
	for rows.Next() {
		var u UsageCounter
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Day, &u.Traces, &u.FlightChecks, &u.APICalls, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan usage: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// IncrementAPICalls increments the api_calls counter for an org for today.
func (p *Pool) IncrementAPICalls(ctx context.Context, orgID uuid.UUID) error {
	_, err := p.db.Exec(ctx, `
		INSERT INTO usage_counters (org_id, day, api_calls)
		VALUES ($1, current_date, 1)
		ON CONFLICT (org_id, day)
		DO UPDATE SET
			api_calls  = usage_counters.api_calls + 1,
			updated_at = now()
	`, orgID)
	if err != nil {
		return fmt.Errorf("increment api calls: %w", err)
	}
	return nil
}
