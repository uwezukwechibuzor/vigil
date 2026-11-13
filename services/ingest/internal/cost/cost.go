// Package cost computes LLM and gas costs for agent spans.
package cost

import (
	"context"
	"math/big"
)

// ModelPrice holds per-token pricing for an LLM model (USD per million tokens).
type ModelPrice struct {
	InputPerMToken  float64 // USD per million input tokens
	OutputPerMToken float64 // USD per million output tokens
}

// DefaultPrices is the built-in model price table (approximate, as of 2025).
var DefaultPrices = map[string]ModelPrice{
	"gpt-4o":            {5.0, 15.0},
	"gpt-4o-mini":       {0.15, 0.6},
	"gpt-4":             {30.0, 60.0},
	"claude-opus-4":     {15.0, 75.0},
	"claude-sonnet-4-6": {3.0, 15.0},
	"claude-haiku-4-5":  {0.8, 4.0},
	"gemini-1.5-pro":    {3.5, 10.5},
	"gemini-1.5-flash":  {0.075, 0.3},
}

// fallbackModel is used when the requested model is not in DefaultPrices.
const fallbackModel = "claude-sonnet-4-6"

// LLMCost computes USD cost from token counts and model name.
// Falls back to claude-sonnet-4-6 pricing if model not found.
func LLMCost(model string, inputTokens, outputTokens uint32) float64 {
	p, ok := DefaultPrices[model]
	if !ok {
		p = DefaultPrices[fallbackModel]
	}
	inputCost := float64(inputTokens) / 1_000_000 * p.InputPerMToken
	outputCost := float64(outputTokens) / 1_000_000 * p.OutputPerMToken
	return inputCost + outputCost
}

// GasCostUSD computes USD cost from gas usage.
// Returns 0 if ethPriceUSD is 0 (unknown) or gasPriceWei is nil.
func GasCostUSD(gasUsed uint64, gasPriceWei *big.Int, ethPriceUSD float64) float64 {
	if ethPriceUSD == 0 || gasPriceWei == nil {
		return 0
	}
	gasCostWei := new(big.Float).Mul(
		new(big.Float).SetUint64(gasUsed),
		new(big.Float).SetInt(gasPriceWei),
	)
	// Convert from wei to ETH (divide by 1e18).
	weiPerETH := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	gasCostETH, _ := new(big.Float).Quo(gasCostWei, weiPerETH).Float64()
	return gasCostETH * ethPriceUSD
}

// GasPricer is the interface for fetching the current ETH price.
// No implementation is wired in the ingest service; callers may pass nil (cost = 0).
type GasPricer interface {
	ETHPriceUSD(ctx context.Context) (float64, error)
}
