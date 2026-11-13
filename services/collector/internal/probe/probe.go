// Package probe defines the shared types and scheduler for all collector probes.
package probe

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"
)

// Kind identifies a probe type.
type Kind string

// Probe kind constants.
const (
	KindRPC     Kind = "rpc"
	KindHeadLag Kind = "head_lag"
	KindReorg   Kind = "reorg"
	KindGasFee  Kind = "gas_fee"
	KindOracle  Kind = "oracle"
	KindIndexer Kind = "indexer"
)

// Result is emitted by every probe after each run.
type Result struct {
	EndpointID string
	ChainID    uint64
	Kind       Kind
	OK         bool
	LatencyMs  float64
	Value      float64 // probe-specific scalar (block number, gas gwei, oracle answer, etc.)
	Labels     map[string]string
	Err        error
	Ts         time.Time
}

// Probe is the interface each probe must implement.
type Probe interface {
	Kind() Kind
	Run(ctx context.Context) (*Result, error)
}

// Config holds scheduling parameters common to all probes.
type Config struct {
	Interval  time.Duration
	Timeout   time.Duration
	JitterPct float64 // fraction of Interval to add as random jitter (0–1)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Interval:  30 * time.Second,
		Timeout:   10 * time.Second,
		JitterPct: 0.2,
	}
}

// jitter returns a jittered sleep duration: interval ± (jitterPct * interval).
func jitter(cfg Config) time.Duration {
	base := cfg.Interval
	delta := float64(base) * cfg.JitterPct
	return base + time.Duration((rand.Float64()*2-1)*delta)
}

// Loop runs p on a jittered schedule, sending results to out until ctx is cancelled.
func Loop(ctx context.Context, p Probe, cfg Config, out chan<- *Result, logger *slog.Logger) {
	for {
		wait := jitter(cfg)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		result, err := p.Run(runCtx)
		cancel()

		if err != nil {
			logger.Warn("probe error", "kind", p.Kind(), "err", err)
			if result == nil {
				result = &Result{Kind: p.Kind(), OK: false, Err: err, Ts: time.Now().UTC()}
			}
		}
		if result != nil {
			select {
			case out <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}
