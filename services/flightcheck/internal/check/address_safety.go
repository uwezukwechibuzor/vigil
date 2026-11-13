package check

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// AddressScreener is a pluggable interface for external address screening services.
// Implementations are wired in at startup; none ship by default.
type AddressScreener interface {
	// Screen returns (riskLevel, reason, error).
	// riskLevel: "none", "low", "medium", "high", "critical".
	Screen(ctx context.Context, address string) (riskLevel string, reason string, err error)
}

// AddressSafetyCheck validates the destination address against the project denylist
// and any registered screener.
type AddressSafetyCheck struct {
	Screener AddressScreener // nil is valid; screening is then skipped
}

// Name returns the check name.
func (c *AddressSafetyCheck) Name() string { return "address_safety" }

// Run checks the `to` address against the Postgres denylist and optional screener.
func (c *AddressSafetyCheck) Run(ctx context.Context, req *Request, deps *Deps) (*Result, error) {
	evidence := map[string]any{
		"address": req.To,
	}

	// 1. Check project-level Postgres denylist.
	if deps.PG != nil && req.ProjectID != "" {
		projectID, err := uuid.Parse(req.ProjectID)
		if err == nil {
			denied, err := deps.PG.IsAddressDenied(ctx, projectID, req.To)
			if err != nil {
				return &Result{
					Name:     c.Name(),
					Status:   StatusWarn,
					Reason:   fmt.Sprintf("denylist lookup error: %v", err),
					Evidence: evidence,
				}, nil
			}
			if denied {
				evidence["denied_by"] = "project_denylist"
				return &Result{
					Name:     c.Name(),
					Status:   StatusFail,
					Reason:   fmt.Sprintf("address %s is on the project denylist", req.To),
					Evidence: evidence,
				}, nil
			}
		}
	}

	// 2. Pluggable screener.
	if c.Screener != nil {
		level, reason, err := c.Screener.Screen(ctx, req.To)
		if err != nil {
			return &Result{
				Name:     c.Name(),
				Status:   StatusWarn,
				Reason:   fmt.Sprintf("screener error: %v", err),
				Evidence: evidence,
			}, nil
		}
		evidence["screener_level"] = level
		evidence["screener_reason"] = reason

		switch level {
		case "critical", "high":
			return &Result{
				Name:     c.Name(),
				Status:   StatusFail,
				Reason:   fmt.Sprintf("address flagged %s by screener: %s", level, reason),
				Evidence: evidence,
			}, nil
		case "medium":
			return &Result{
				Name:     c.Name(),
				Status:   StatusWarn,
				Reason:   fmt.Sprintf("address flagged %s by screener: %s", level, reason),
				Evidence: evidence,
			}, nil
		}
	}

	return &Result{
		Name:     c.Name(),
		Status:   StatusPass,
		Reason:   "address not flagged",
		Evidence: evidence,
	}, nil
}
