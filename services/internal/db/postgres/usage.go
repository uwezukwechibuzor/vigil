package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// IncrementFlightChecks increments the flight_checks counter for an org for today.
// If no row exists for (org_id, today) it inserts one.
func (p *Pool) IncrementFlightChecks(ctx context.Context, orgID uuid.UUID) error {
	_, err := p.db.Exec(ctx, `
		INSERT INTO usage_counters (org_id, day, flight_checks)
		VALUES ($1, current_date, 1)
		ON CONFLICT (org_id, day)
		DO UPDATE SET
			flight_checks = usage_counters.flight_checks + 1,
			updated_at    = now()
	`, orgID)
	if err != nil {
		return fmt.Errorf("increment flight checks: %w", err)
	}
	return nil
}
