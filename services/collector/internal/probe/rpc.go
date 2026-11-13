package probe

import (
	"context"
	"fmt"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/collector/internal/adapter"
)

// RPCProbe measures round-trip latency to an RPC endpoint by fetching the head block.
type RPCProbe struct {
	endpointID string
	a          adapter.ChainAdapter
}

// NewRPC creates a new RPC latency probe.
func NewRPC(endpointID string, a adapter.ChainAdapter) *RPCProbe {
	return &RPCProbe{endpointID: endpointID, a: a}
}

// Kind returns KindRPC.
func (p *RPCProbe) Kind() Kind { return KindRPC }

// Run measures RPC latency by fetching the head block.
func (p *RPCProbe) Run(ctx context.Context) (*Result, error) {
	start := time.Now()
	block, err := p.a.HeadBlock(ctx)
	latency := time.Since(start)

	r := &Result{
		EndpointID: p.endpointID,
		Kind:       KindRPC,
		LatencyMs:  float64(latency.Milliseconds()),
		Ts:         time.Now().UTC(),
	}

	if err != nil {
		r.OK = false
		r.Err = fmt.Errorf("rpc probe: %w", err)
		return r, r.Err
	}

	chainID, _ := p.a.ChainID(ctx)
	r.OK = true
	r.ChainID = chainID
	r.Value = float64(block.Number)
	return r, nil
}
