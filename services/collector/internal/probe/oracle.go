package probe

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/collector/internal/adapter"
)

// latestRoundDataSelector is the ABI selector for Chainlink AggregatorV3
// latestRoundData() → (uint80,int256,uint256,uint256,uint80)
var latestRoundDataSelector = []byte{0xfe, 0xaf, 0x96, 0x8c}

// OracleProbe calls a Chainlink AggregatorV3 latestRoundData and checks staleness.
type OracleProbe struct {
	endpointID   string
	a            adapter.ChainAdapter
	contractAddr string
	maxStaleness time.Duration
}

// NewOracle creates a Chainlink oracle probe.
// contractAddr is the AggregatorV3Interface contract address.
// maxStaleness is the maximum acceptable age of the oracle answer.
func NewOracle(endpointID, contractAddr string, maxStaleness time.Duration, a adapter.ChainAdapter) *OracleProbe {
	return &OracleProbe{
		endpointID:   endpointID,
		a:            a,
		contractAddr: contractAddr,
		maxStaleness: maxStaleness,
	}
}

// Kind returns KindOracle.
func (p *OracleProbe) Kind() Kind { return KindOracle }

// Run calls latestRoundData and checks answer freshness.
func (p *OracleProbe) Run(ctx context.Context) (*Result, error) {
	r := &Result{
		EndpointID: p.endpointID,
		Kind:       KindOracle,
		Ts:         time.Now().UTC(),
		Labels:     map[string]string{"contract": p.contractAddr},
	}

	raw, err := p.a.Call(ctx, adapter.CallMsg{
		To:   p.contractAddr,
		Data: latestRoundDataSelector,
	})
	if err != nil {
		r.OK = false
		r.Err = fmt.Errorf("oracle call: %w", err)
		return r, r.Err
	}

	answer, updatedAt, err := decodeLatestRoundData(raw)
	if err != nil {
		r.OK = false
		r.Err = fmt.Errorf("oracle decode: %w", err)
		return r, r.Err
	}

	chainID, _ := p.a.ChainID(ctx)
	r.ChainID = chainID

	staleness := time.Since(updatedAt)
	stale := staleness > p.maxStaleness

	r.OK = !stale
	r.Value, _ = new(big.Float).SetInt(answer).Float64()
	r.Labels["updated_at"] = updatedAt.UTC().Format(time.RFC3339)
	r.Labels["staleness_s"] = fmt.Sprintf("%.0f", staleness.Seconds())

	if stale {
		r.Err = fmt.Errorf("oracle stale: last update %s ago (max %s)", staleness.Round(time.Second), p.maxStaleness)
	}
	return r, nil
}

// OracleStale returns whether the given updatedAt timestamp exceeds maxStaleness.
// Exposed for unit testing.
func OracleStale(updatedAt time.Time, maxStaleness time.Duration) bool {
	return time.Since(updatedAt) > maxStaleness
}

// decodeLatestRoundData parses the ABI-encoded response from latestRoundData().
// Returns answer (int256) and updatedAt (uint256 unix timestamp).
// ABI layout: 5 × 32-byte slots → roundId, answer, startedAt, updatedAt, answeredInRound
func decodeLatestRoundData(data []byte) (*big.Int, time.Time, error) {
	if len(data) < 5*32 {
		return nil, time.Time{}, fmt.Errorf("latestRoundData response too short: %d bytes", len(data))
	}

	// Slot 1 (bytes 32..64): answer (int256)
	answerBytes := data[32:64]
	answer := new(big.Int).SetBytes(answerBytes)
	// Handle negative int256 (two's complement)
	if answerBytes[0]&0x80 != 0 {
		// negative: answer = answer - 2^256
		maxUint256 := new(big.Int).Lsh(big.NewInt(1), 256)
		answer.Sub(answer, maxUint256)
	}

	// Slot 3 (bytes 96..128): updatedAt (uint256)
	updatedAtBytes := data[96:128]
	updatedAtUnix := int64(binary.BigEndian.Uint64(updatedAtBytes[24:32])) // #nosec G115 -- unix timestamp extracted from ABI-encoded uint256; fits int64 for any real date
	updatedAt := time.Unix(updatedAtUnix, 0).UTC()

	return answer, updatedAt, nil
}
