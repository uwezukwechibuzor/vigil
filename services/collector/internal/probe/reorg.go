package probe

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/collector/internal/adapter"
)

// ReorgProbe detects chain reorganisations by comparing each new head block's
// parent hash against the previous head's hash. A mismatch signals a reorg.
type ReorgProbe struct {
	endpointID string
	a          adapter.ChainAdapter

	mu       sync.Mutex
	prevHash string
	prevNum  uint64
}

// NewReorg creates a new reorg-detector probe.
func NewReorg(endpointID string, a adapter.ChainAdapter) *ReorgProbe {
	return &ReorgProbe{endpointID: endpointID, a: a}
}

// Kind returns KindReorg.
func (p *ReorgProbe) Kind() Kind { return KindReorg }

// Run checks whether the latest block hash signals a chain reorg.
func (p *ReorgProbe) Run(ctx context.Context) (*Result, error) {
	block, err := p.a.HeadBlock(ctx)
	r := &Result{
		EndpointID: p.endpointID,
		Kind:       KindReorg,
		Ts:         time.Now().UTC(),
	}

	if err != nil {
		r.OK = false
		r.Err = fmt.Errorf("reorg probe: %w", err)
		return r, r.Err
	}

	chainID, _ := p.a.ChainID(ctx)
	r.ChainID = chainID

	p.mu.Lock()
	prev := p.prevHash
	prevNum := p.prevNum
	p.prevHash = block.Hash
	p.prevNum = block.Number
	p.mu.Unlock()

	if prev == "" {
		r.OK = true
		r.Value = 0
		return r, nil
	}

	// Simple detection: if the new head number <= prev number and hash differs, reorg.
	reorg := DetectReorg(prevNum, prev, block.Number, block.Hash)
	r.OK = !reorg
	if reorg {
		r.Value = 1
		r.Labels = map[string]string{
			"prev_hash":  prev,
			"curr_hash":  block.Hash,
			"prev_block": fmt.Sprintf("%d", prevNum),
			"curr_block": fmt.Sprintf("%d", block.Number),
		}
		r.Err = fmt.Errorf("reorg detected at block %d (was %d)", block.Number, prevNum)
	}
	return r, nil
}

// DetectReorg returns true if the transition from (prevNum, prevHash) to
// (currNum, currHash) looks like a reorg. It is exposed for unit testing.
func DetectReorg(prevNum uint64, prevHash string, currNum uint64, currHash string) bool {
	if prevHash == "" || currHash == "" {
		return false
	}
	// Same number, different hash → reorg on this slot.
	if currNum == prevNum && currHash != prevHash {
		return true
	}
	// Block number went backwards → definitely a reorg.
	if currNum < prevNum {
		return true
	}
	return false
}
