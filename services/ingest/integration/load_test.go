package integration

import (
	"bytes"
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	colpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	respb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/otlp"
	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/sampler"

	"log/slog"
	"net/http"
	"os"
)

// TestLoad_2000SpansPerSecond sends 2000 spans/s for 2 seconds and verifies 0 loss.
func TestLoad_2000SpansPerSecond(t *testing.T) {
	if testing.Short() {
		t.Skip("load test requires infra")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Start Redis.
	redisC, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() { _ = redisC.Terminate(ctx) })

	redisAddr, err := redisC.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("redis endpoint: %v", err)
	}

	rdb := goredis.NewClient(&goredis.Options{Addr: redisAddr})
	t.Cleanup(func() { _ = rdb.Close() })

	stream := "vigil:load:spans"
	sam := sampler.New(1.0)
	h := otlp.NewHandler(rdb, sam, logger, stream, 4<<20, 1_000_000, 1.0)

	// Build a reusable OTLP protobuf body.
	now := uint64(time.Now().UnixNano())
	req := &colpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &respb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "load-test-svc"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte("loadtest1234abcd"),
								SpanId:            []byte("loadtest"),
								Name:              "load.span",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: now - uint64(10*time.Millisecond),
								EndTimeUnixNano:   now,
							},
						},
					},
				},
			},
		},
	}
	body, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	const totalSpans = 4000
	const duration = 2 * time.Second

	var wg sync.WaitGroup
	ticker := time.NewTicker(duration / totalSpans)
	defer ticker.Stop()

	sent := 0
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		<-ticker.C
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			r, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/traces", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/x-protobuf")
			h.TracesHandler(rec, r)
		}()
		sent++
	}
	wg.Wait()

	ql, err := rdb.XLen(ctx, stream).Result()
	if err != nil {
		t.Fatalf("xlen: %v", err)
	}

	t.Logf("sent=%d queued=%d", sent, ql)
	if ql < int64(sent) {
		t.Errorf("expected queue length >= %d, got %d", sent, ql)
	}
}
