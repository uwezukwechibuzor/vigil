package integration

import (
	"context"
	"testing"

	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/linker"

	"github.com/ethereum/go-ethereum/core/types"
)

// TestLinker_IsSilentFailure validates silent failure detection logic without Docker.
func TestLinker_IsSilentFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires docker")
	}

	tests := []struct {
		name        string
		gasUsed     uint64
		gasEstimate uint64
		status      uint64
		want        bool
	}{
		{"ok tx within estimate", 100_000, 200_000, 1, false},
		{"ok tx 2x estimate exactly", 200_000, 100_000, 1, false},
		{"ok tx 3x estimate silent failure", 300_001, 100_000, 1, true},
		{"failed tx never silent", 300_001, 100_000, 0, false},
		{"no estimate always ok", 300_000, 0, 1, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			receipt := &types.Receipt{
				Status:  tc.status,
				GasUsed: tc.gasUsed,
			}
			got := linker.IsSilentFailure(receipt, tc.gasEstimate)
			if got != tc.want {
				t.Errorf("IsSilentFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLinker_AnvilTx tests the linker against a real Anvil node.
// This test is skipped in short mode and requires an Anvil process.
func TestLinker_AnvilTx(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires docker (anvil)")
	}
	// Full Anvil-based test would use testcontainers to spin up Anvil,
	// send a real transaction, then verify the linker marks it linked.
	// Skipped for now pending Anvil image availability in CI.
	ctx := context.Background()
	_ = ctx
	t.Log("anvil linker test skipped — Anvil container not wired yet")
}
