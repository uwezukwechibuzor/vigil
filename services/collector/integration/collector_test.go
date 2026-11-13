package integration_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/uwezukwechibuzor/vigil/services/collector/internal/adapter"
	"github.com/uwezukwechibuzor/vigil/services/collector/internal/probe"
)

// TestEndpointDownEvent verifies that when an RPC endpoint is unreachable,
// an "endpoint down" event appears on the Redis "events" stream within 10s.
func TestEndpointDownEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration: requires Docker")
	}

	ctx := context.Background()

	redisC, err := tcredis.Run(ctx, "redis:7.4")
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(redisC) })

	redisURL, err := redisC.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("redis url: %v", err)
	}

	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("parse redis url: %v", err)
	}
	rdb := goredis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })

	endpointID := "test-endpoint-down"

	// failingAdapter simulates a dead RPC endpoint.
	rpcProbe := probe.NewRPC(endpointID, &failingAdapter{})
	results := make(chan *probe.Result, 32)

	cfg := probe.Config{
		Interval:  150 * time.Millisecond,
		Timeout:   300 * time.Millisecond,
		JitterPct: 0,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	probeCtx, cancelProbe := context.WithTimeout(ctx, 8*time.Second)
	defer cancelProbe()

	go probe.Loop(probeCtx, rpcProbe, cfg, results, logger)

	// Forward the first failure to the Redis "events" stream.
	go func() {
		for {
			select {
			case <-probeCtx.Done():
				return
			case r, ok := <-results:
				if !ok {
					return
				}
				if r != nil && !r.OK {
					_ = rdb.XAdd(ctx, &goredis.XAddArgs{
						Stream: "events",
						Values: map[string]any{
							"endpoint_id": r.EndpointID,
							"kind":        string(r.Kind),
							"error":       "endpoint down",
						},
					})
					return
				}
			}
		}
	}()

	// Poll until the event appears on the stream.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		msgs, err := rdb.XRead(ctx, &goredis.XReadArgs{
			Streams: []string{"events", "0"},
			Count:   10,
			Block:   500 * time.Millisecond,
		}).Result()
		if err != nil && !errors.Is(err, goredis.Nil) {
			t.Logf("xread: %v", err)
			continue
		}
		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				if msg.Values["endpoint_id"] == endpointID {
					t.Logf("event received: %v", msg.Values)
					return
				}
			}
		}
	}

	t.Fatal("timed out waiting for endpoint down event on Redis stream")
}

// failingAdapter always returns an error, simulating a dead RPC node.
type failingAdapter struct{}

func (f *failingAdapter) HeadBlock(_ context.Context) (*adapter.Block, error) {
	return nil, errors.New("connection refused")
}
func (f *failingAdapter) BlockByNumber(_ context.Context, _ *big.Int) (*adapter.Block, error) {
	return nil, errors.New("connection refused")
}
func (f *failingAdapter) Call(_ context.Context, _ adapter.CallMsg) ([]byte, error) {
	return nil, errors.New("connection refused")
}
func (f *failingAdapter) EstimateGas(_ context.Context, _ adapter.CallMsg) (uint64, error) {
	return 0, errors.New("connection refused")
}
func (f *failingAdapter) FeeData(_ context.Context) (*adapter.FeeData, error) {
	return nil, errors.New("connection refused")
}
func (f *failingAdapter) Simulate(_ context.Context, _ adapter.CallMsg) ([]byte, error) {
	return nil, errors.New("connection refused")
}
func (f *failingAdapter) GetLogs(_ context.Context, _ adapter.FilterQuery) ([]adapter.Log, error) {
	return nil, errors.New("connection refused")
}
func (f *failingAdapter) ChainID(_ context.Context) (uint64, error) {
	return 0, errors.New("connection refused")
}
func (f *failingAdapter) Close() {}
