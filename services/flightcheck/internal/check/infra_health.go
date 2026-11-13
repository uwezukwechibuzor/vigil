package check

import (
	"context"
	"fmt"
	"time"
)

// InfraHealthCheck verifies the RPC endpoint is live and responsive.
type InfraHealthCheck struct{}

// Name returns the check name.
func (c *InfraHealthCheck) Name() string { return "infra_health" }

// Run calls eth_blockNumber and fails if no client is configured or latency > 500 ms.
func (c *InfraHealthCheck) Run(ctx context.Context, req *Request, deps *Deps) (*Result, error) {
	if deps.Client == nil {
		return &Result{
			Name:   c.Name(),
			Status: StatusFail,
			Reason: fmt.Sprintf("no RPC client configured for chain %d", req.ChainID),
		}, nil
	}

	const latencyLimit = 500 * time.Millisecond
	callCtx, cancel := context.WithTimeout(ctx, latencyLimit)
	defer cancel()

	start := time.Now()
	blockNum, err := deps.Client.BlockNumber(callCtx)
	elapsed := time.Since(start)

	if err != nil {
		return &Result{
			Name:   c.Name(),
			Status: StatusFail,
			Reason: fmt.Sprintf("eth_blockNumber failed: %v", err),
			Evidence: map[string]any{
				"latency_ms": elapsed.Milliseconds(),
			},
		}, nil
	}

	if elapsed > latencyLimit {
		return &Result{
			Name:   c.Name(),
			Status: StatusFail,
			Reason: fmt.Sprintf("RPC latency %dms exceeds 500ms limit", elapsed.Milliseconds()),
			Evidence: map[string]any{
				"latency_ms": elapsed.Milliseconds(),
				"block":      blockNum,
			},
		}, nil
	}

	return &Result{
		Name:   c.Name(),
		Status: StatusPass,
		Reason: fmt.Sprintf("RPC healthy, block %d, latency %dms", blockNum, elapsed.Milliseconds()),
		Evidence: map[string]any{
			"latency_ms": elapsed.Milliseconds(),
			"block":      blockNum,
		},
	}, nil
}
