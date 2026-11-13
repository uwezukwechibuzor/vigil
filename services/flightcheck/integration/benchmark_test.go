package integration

import (
	"context"
	"sort"
	"testing"

	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/check"
)

// BenchmarkFlightCheck measures p95 latency for a simple ETH transfer against
// the shared Anvil instance started in TestMain.
func BenchmarkFlightCheck(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping Docker-based benchmark in -short mode")
	}

	// Wrap b in a minimal testing.T-compatible helper so buildEngine can log.
	eng := buildEngineB(b)

	req := &check.Request{
		ChainID: anvilChainID,
		From:    anvilFunded,
		To:      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		Value:   "0x64",
		Data:    "0x",
	}

	var latencies []int64
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		resp := eng.Run(context.Background(), req)
		latencies = append(latencies, resp.LatencyMS)
		if resp.LatencyMS > 1500 {
			b.Fatalf("latency %dms over 1500ms budget", resp.LatencyMS)
		}
	}

	b.StopTimer()
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p95idx := int(float64(len(latencies)) * 0.95)
		if p95idx >= len(latencies) {
			p95idx = len(latencies) - 1
		}
		p95 := latencies[p95idx]
		b.Logf("p95 latency: %dms (n=%d)", p95, len(latencies))
		if p95 > 1500 {
			b.Fatalf("p95 latency %dms exceeds 1500ms budget", p95)
		}
	}
}
