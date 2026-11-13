package check

import (
	"context"
	"fmt"
	"time"
)

// OracleFreshnessCheck verifies the transaction deadline has not already passed.
type OracleFreshnessCheck struct{}

// Name returns the check name.
func (c *OracleFreshnessCheck) Name() string { return "oracle_freshness" }

// Run checks whether the deadline_s field (if set) is in the future.
func (c *OracleFreshnessCheck) Run(_ context.Context, req *Request, _ *Deps) (*Result, error) {
	if req.DeadlineS == nil {
		return &Result{
			Name:   c.Name(),
			Status: StatusPass,
			Reason: "no deadline set",
		}, nil
	}

	now := time.Now().Unix()
	if *req.DeadlineS < now {
		return &Result{
			Name:   c.Name(),
			Status: StatusFail,
			Reason: fmt.Sprintf("transaction deadline expired %ds ago", now-*req.DeadlineS),
			Evidence: map[string]any{
				"deadline_s": *req.DeadlineS,
				"now_s":      now,
				"expired_s":  now - *req.DeadlineS,
			},
		}, nil
	}

	remaining := *req.DeadlineS - now
	return &Result{
		Name:   c.Name(),
		Status: StatusPass,
		Reason: fmt.Sprintf("deadline valid, %ds remaining", remaining),
		Evidence: map[string]any{
			"deadline_s":  *req.DeadlineS,
			"now_s":       now,
			"remaining_s": remaining,
		},
	}, nil
}
