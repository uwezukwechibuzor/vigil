package check

import (
	"context"
	"fmt"
	"math/big"
)

const (
	gweiPerEth        = 1e9
	gasSpikeLimitGwei = 500
	highGasUsageLimit = 1_000_000
)

// GasSanityCheck warns when gas price is unusually high or estimated usage is very large.
type GasSanityCheck struct{}

// Name returns the check name.
func (c *GasSanityCheck) Name() string { return "gas_sanity" }

// Run queries the current gas price and optionally estimates gas usage.
func (c *GasSanityCheck) Run(ctx context.Context, req *Request, deps *Deps) (*Result, error) {
	if deps.Client == nil {
		return &Result{
			Name:   c.Name(),
			Status: StatusWarn,
			Reason: "no RPC client; gas sanity skipped",
		}, nil
	}

	suggestedPrice, err := deps.Client.SuggestGasPrice(ctx)
	if err != nil {
		return &Result{
			Name:   c.Name(),
			Status: StatusWarn,
			Reason: fmt.Sprintf("could not fetch gas price: %v", err),
		}, nil
	}

	// Convert to Gwei.
	gwei := new(big.Float).Quo(
		new(big.Float).SetInt(suggestedPrice),
		big.NewFloat(gweiPerEth),
	)
	gweiF64, _ := gwei.Float64()

	evidence := map[string]any{
		"gas_price_gwei": gweiF64,
		"gas_price_wei":  suggestedPrice.String(),
	}

	var warnings []string

	if gweiF64 > gasSpikeLimitGwei {
		warnings = append(warnings, fmt.Sprintf("gas spike: %.1f Gwei > 500 Gwei limit", gweiF64))
	}

	// Estimate gas if the user did not supply one.
	if req.Gas != nil {
		evidence["gas_limit"] = *req.Gas
		if *req.Gas > highGasUsageLimit {
			warnings = append(warnings, fmt.Sprintf("high gas usage: %d > 1,000,000 limit", *req.Gas))
		}
	}

	if len(warnings) > 0 {
		reason := warnings[0]
		if len(warnings) > 1 {
			reason = fmt.Sprintf("%s; %s", warnings[0], warnings[1])
		}
		return &Result{
			Name:     c.Name(),
			Status:   StatusWarn,
			Reason:   reason,
			Evidence: evidence,
		}, nil
	}

	return &Result{
		Name:     c.Name(),
		Status:   StatusPass,
		Reason:   fmt.Sprintf("gas price %.1f Gwei is within normal range", gweiF64),
		Evidence: evidence,
	}, nil
}
