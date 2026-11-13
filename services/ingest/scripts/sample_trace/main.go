// Package main sends a sample OTLP trace to the vigil-ingest service to verify ingestion.
// Usage: INGEST_URL=http://localhost:8080 go run ./scripts/sample_trace
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net/http"
	"os"
	"time"

	colpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	respb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

func main() {
	ingestURL := os.Getenv("INGEST_URL")
	if ingestURL == "" {
		ingestURL = "http://localhost:8080"
	}

	now := uint64(time.Now().UnixNano())
	traceID := makeID(16)
	spanID := makeID(8)

	req := &colpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &respb.Resource{
					Attributes: []*commonpb.KeyValue{
						strAttr("service.name", "sample-agent"),
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           traceID,
								SpanId:            spanID,
								Name:              "agent.run",
								Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
								StartTimeUnixNano: now - uint64(500*time.Millisecond),
								EndTimeUnixNano:   now,
								Status: &tracepb.Status{
									Code: tracepb.Status_STATUS_CODE_OK,
								},
								Attributes: []*commonpb.KeyValue{
									strAttr("gen_ai.system", "anthropic"),
									strAttr("gen_ai.request.model", "claude-sonnet-4-6"),
									intAttr("gen_ai.usage.input_tokens", 100),
									intAttr("gen_ai.usage.output_tokens", 50),
									strAttr("vigil.agent.id", "test-agent-1"),
									strAttr("vigil.run.id", "run-001"),
									strAttr("vigil.intent", "swap 0.1 ETH to USDC"),
								},
							},
						},
					},
				},
			},
		},
	}

	body, err := proto.Marshal(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal error: %v\n", err)
		os.Exit(1)
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		ingestURL+"/v1/traces", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "build request error: %v\n", err)
		os.Exit(1)
	}
	httpReq.Header.Set("Content-Type", "application/x-protobuf")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "http error: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unexpected status: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	traceHex := fmt.Sprintf("%x", traceID)
	fmt.Printf("OK: trace_id=%s\n", traceHex)
}

// strAttr builds a string KeyValue.
func strAttr(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}},
	}
}

// intAttr builds an integer KeyValue.
func intAttr(key string, value int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: value}},
	}
}

// makeID generates a pseudo-random byte slice of the given length using time-based seeding.
func makeID(n int) []byte {
	b := make([]byte, n)
	v := uint64(time.Now().UnixNano())
	for i := 0; i < n; i += 8 {
		rem := n - i
		if rem > 8 {
			rem = 8
		}
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], v+uint64(i))
		copy(b[i:], buf[:rem])
	}
	return b
}
