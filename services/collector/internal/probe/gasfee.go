package probe

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/collector/internal/adapter"
)

// GasFeeProbe samples current EIP-1559 fee market data.
type GasFeeProbe struct {
	endpointID string
	a          adapter.ChainAdapter
}

// NewGasFee creates a new gas fee probe.
func NewGasFee(endpointID string, a adapter.ChainAdapter) *GasFeeProbe {
	return &GasFeeProbe{endpointID: endpointID, a: a}
}

// Kind returns KindGasFee.
func (p *GasFeeProbe) Kind() Kind { return KindGasFee }

// Run samples the current EIP-1559 fee market.
func (p *GasFeeProbe) Run(ctx context.Context) (*Result, error) {
	fd, err := p.a.FeeData(ctx)
	r := &Result{
		EndpointID: p.endpointID,
		Kind:       KindGasFee,
		Ts:         time.Now().UTC(),
	}

	if err != nil {
		r.OK = false
		r.Err = fmt.Errorf("gasfee: %w", err)
		return r, r.Err
	}

	chainID, _ := p.a.ChainID(ctx)
	r.OK = true
	r.ChainID = chainID

	labels := make(map[string]string)
	if fd.BaseFee != nil {
		labels["base_fee_gwei"] = weiToGweiStr(fd.BaseFee)
		r.Value = weiToGwei(fd.BaseFee)
	}
	if fd.MaxFee != nil {
		labels["max_fee_gwei"] = weiToGweiStr(fd.MaxFee)
	}
	if fd.PriorityFee != nil {
		labels["priority_fee_gwei"] = weiToGweiStr(fd.PriorityFee)
	}
	r.Labels = labels
	return r, nil
}

func weiToGwei(w *big.Int) float64 {
	if w == nil {
		return 0
	}
	gwei := new(big.Float).Quo(
		new(big.Float).SetInt(w),
		big.NewFloat(1e9),
	)
	f, _ := gwei.Float64()
	return f
}

func weiToGweiStr(w *big.Int) string {
	return fmt.Sprintf("%.4f", weiToGwei(w))
}
