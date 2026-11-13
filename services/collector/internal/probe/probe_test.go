package probe_test

import (
	"testing"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/collector/internal/probe"
)

// --- HeadLag tests ----------------------------------------------------------

func TestBlockLag(t *testing.T) {
	now := time.Now()

	t.Run("fresh block", func(t *testing.T) {
		lag := probe.BlockLag(now, now.Add(-2*time.Second))
		if lag < 1.9 || lag > 2.1 {
			t.Errorf("expected ~2s lag, got %f", lag)
		}
	})

	t.Run("very old block", func(t *testing.T) {
		lag := probe.BlockLag(now, now.Add(-10*time.Minute))
		if lag < 599 {
			t.Errorf("expected >599s lag, got %f", lag)
		}
	})

	t.Run("future block is non-negative", func(t *testing.T) {
		lag := probe.BlockLag(now, now.Add(5*time.Second))
		if lag > 0 {
			t.Errorf("future block should report non-positive lag, got %f", lag)
		}
	})
}

// --- ReorgDetection tests ---------------------------------------------------

func TestDetectReorg(t *testing.T) {
	cases := []struct {
		name     string
		prevNum  uint64
		prevHash string
		currNum  uint64
		currHash string
		want     bool
	}{
		{
			name:    "normal progression",
			prevNum: 100, prevHash: "0xaaa",
			currNum: 101, currHash: "0xbbb",
			want: false,
		},
		{
			name:    "same block same hash",
			prevNum: 100, prevHash: "0xaaa",
			currNum: 100, currHash: "0xaaa",
			want: false,
		},
		{
			name:    "same block different hash — reorg",
			prevNum: 100, prevHash: "0xaaa",
			currNum: 100, currHash: "0xccc",
			want: true,
		},
		{
			name:    "block went backwards — reorg",
			prevNum: 100, prevHash: "0xaaa",
			currNum: 99, currHash: "0xddd",
			want: true,
		},
		{
			name:    "empty prev hash — no detection yet",
			prevNum: 0, prevHash: "",
			currNum: 1, currHash: "0xeee",
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := probe.DetectReorg(tc.prevNum, tc.prevHash, tc.currNum, tc.currHash)
			if got != tc.want {
				t.Errorf("DetectReorg(%d,%s,%d,%s) = %v, want %v",
					tc.prevNum, tc.prevHash, tc.currNum, tc.currHash, got, tc.want)
			}
		})
	}
}

// --- OracleStaleness tests --------------------------------------------------

func TestOracleStale(t *testing.T) {
	now := time.Now()

	t.Run("fresh update", func(t *testing.T) {
		if probe.OracleStale(now.Add(-1*time.Hour), 2*time.Hour) {
			t.Error("expected not stale")
		}
	})

	t.Run("stale update", func(t *testing.T) {
		if !probe.OracleStale(now.Add(-3*time.Hour), 2*time.Hour) {
			t.Error("expected stale")
		}
	})

	t.Run("exactly at boundary is fresh", func(t *testing.T) {
		if probe.OracleStale(now.Add(-2*time.Hour+time.Second), 2*time.Hour) {
			t.Error("expected not stale at boundary")
		}
	})
}

// --- IndexerLag tests -------------------------------------------------------

func TestIndexerLag(t *testing.T) {
	cases := []struct {
		chain   uint64
		indexed uint64
		want    uint64
	}{
		{chain: 100, indexed: 95, want: 5},
		{chain: 100, indexed: 100, want: 0},
		{chain: 100, indexed: 101, want: 0}, // indexer ahead — treat as 0
		{chain: 0, indexed: 0, want: 0},
	}
	for _, tc := range cases {
		got := probe.IndexerLag(tc.chain, tc.indexed)
		if got != tc.want {
			t.Errorf("IndexerLag(%d,%d) = %d, want %d", tc.chain, tc.indexed, got, tc.want)
		}
	}
}
