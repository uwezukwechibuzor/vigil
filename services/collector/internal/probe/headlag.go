package probe

import (
	"context"
	"fmt"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/collector/internal/adapter"
)

// HeadLagProbe measures block time age — the difference between now and the
// timestamp of the latest block. High values indicate a stalled chain or RPC.
type HeadLagProbe struct {
	endpointID string
	a          adapter.ChainAdapter
}

// NewHeadLag creates a new head-lag probe.
func NewHeadLag(endpointID string, a adapter.ChainAdapter) *HeadLagProbe {
	return &HeadLagProbe{endpointID: endpointID, a: a}
}

// Kind returns KindHeadLag.
func (p *HeadLagProbe) Kind() Kind { return KindHeadLag }

// Run measures the age of the latest block against wall clock.
func (p *HeadLagProbe) Run(ctx context.Context) (*Result, error) {
	block, err := p.a.HeadBlock(ctx)
	r := &Result{
		EndpointID: p.endpointID,
		Kind:       KindHeadLag,
		Ts:         time.Now().UTC(),
	}

	if err != nil {
		r.OK = false
		r.Err = fmt.Errorf("headlag: %w", err)
		return r, r.Err
	}

	chainID, _ := p.a.ChainID(ctx)
	lag := time.Since(block.Time).Seconds()

	r.OK = true
	r.ChainID = chainID
	r.Value = lag // seconds behind wall clock
	r.Labels = map[string]string{"block": fmt.Sprintf("%d", block.Number)}
	return r, nil
}

// BlockLag returns the lag in seconds between two block timestamps and their
// block numbers. Useful in unit tests and the reorg detector.
func BlockLag(now time.Time, blockTime time.Time) float64 {
	return now.Sub(blockTime).Seconds()
}
