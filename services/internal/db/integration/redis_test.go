package integration_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/uwezukwechibuzor/vigil/services/internal/db/redis"
)

func startRedis(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	url, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("redis url: %v", err)
	}
	return url
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(noopWriter{}, nil))
}

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestCacheSetGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	c, err := redis.New(ctx, startRedis(t))
	if err != nil {
		t.Fatalf("redis new: %v", err)
	}
	defer func() { _ = c.Close() }()

	type payload struct {
		Val string
	}
	if err := c.Set(ctx, "test:key", payload{Val: "hello"}, time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}

	var got payload
	ok, err := c.Get(ctx, "test:key", &got)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Val != "hello" {
		t.Errorf("val = %q, want hello", got.Val)
	}
}

func TestCacheMiss(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	c, err := redis.New(ctx, startRedis(t))
	if err != nil {
		t.Fatalf("redis new: %v", err)
	}
	defer func() { _ = c.Close() }()

	var dst map[string]string
	ok, err := c.Get(ctx, "nonexistent", &dst)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestRateLimiter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	c, err := redis.New(ctx, startRedis(t))
	if err != nil {
		t.Fatalf("redis new: %v", err)
	}
	defer func() { _ = c.Close() }()

	const limit = 3
	key := "rl:test"

	for i := range limit {
		res, err := c.Allow(ctx, key, limit, time.Minute)
		if err != nil {
			t.Fatalf("allow [%d]: %v", i, err)
		}
		if !res.Allowed {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// 4th request must be denied
	res, err := c.Allow(ctx, key, limit, time.Minute)
	if err != nil {
		t.Fatalf("allow over limit: %v", err)
	}
	if res.Allowed {
		t.Error("request over limit should be denied")
	}
}

func TestStreamsProducerConsumer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	c, err := redis.New(ctx, startRedis(t))
	if err != nil {
		t.Fatalf("redis new: %v", err)
	}
	defer func() { _ = c.Close() }()

	stream := "test:stream"
	producer := c.NewProducer(stream)

	id, err := producer.Send(ctx, map[string]any{"event": "ping"})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if id == "" {
		t.Fatal("expected message id")
	}

	consumer, err := c.NewConsumer(ctx, redis.ConsumerConfig{
		Stream:    stream,
		Group:     "test-group",
		Consumer:  "worker-1",
		BatchSize: 10,
		Block:     100 * time.Millisecond,
		MaxRetry:  3,
	}, noopLogger())
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}

	msgs, err := consumer.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if err := consumer.Ack(ctx, msgs[0].ID); err != nil {
		t.Fatalf("ack: %v", err)
	}
}

func TestStreamsDeadLetter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	c, err := redis.New(ctx, startRedis(t))
	if err != nil {
		t.Fatalf("redis new: %v", err)
	}
	defer func() { _ = c.Close() }()

	stream := "test:dlq-stream"
	producer := c.NewProducer(stream)
	if _, err := producer.Send(ctx, map[string]any{"event": "test"}); err != nil {
		t.Fatalf("send: %v", err)
	}

	consumer, err := c.NewConsumer(ctx, redis.ConsumerConfig{
		Stream:    stream,
		Group:     "dlq-group",
		Consumer:  "worker-1",
		BatchSize: 10,
		Block:     100 * time.Millisecond,
		MaxRetry:  0, // immediately move to DLQ on retry
	}, noopLogger())
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}

	// Read but don't ack (simulates failed processing)
	if _, err := consumer.Read(ctx); err != nil {
		t.Fatalf("read: %v", err)
	}

	// Retry should move it to DLQ (MaxRetry=0 means 0 retries allowed)
	// The pending entry has RetryCount=1 after one delivery, so >= MaxRetry(0) → DLQ
	if err := consumer.Retry(ctx); err != nil {
		t.Fatalf("retry: %v", err)
	}
}
