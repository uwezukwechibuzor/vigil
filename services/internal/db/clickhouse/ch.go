// Package clickhouse provides a typed ClickHouse client with batched inserts for Vigil telemetry.
package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Client wraps a ClickHouse connection.
type Client struct {
	conn driver.Conn
}

// New opens a ClickHouse connection from a DSN.
func New(ctx context.Context, dsn string) (*Client, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("clickhouse parse dsn: %w", err)
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}
	return &Client{conn: conn}, nil
}

// Close releases the connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// ChainHead is one block-head observation.
type ChainHead struct {
	Chain      string
	EndpointID string
	BlockNum   uint64
	BlockTime  time.Time
	ObservedAt time.Time
	LatencyMs  uint32
}

// InsertChainHeads batch-inserts chain head observations.
func (c *Client) InsertChainHeads(ctx context.Context, rows []ChainHead) error {
	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO chain_heads")
	if err != nil {
		return fmt.Errorf("prepare chain_heads batch: %w", err)
	}
	for _, r := range rows {
		if err := batch.Append(r.Chain, r.EndpointID, r.BlockNum, r.BlockTime, r.ObservedAt, r.LatencyMs); err != nil {
			return fmt.Errorf("append chain_heads: %w", err)
		}
	}
	return batch.Send()
}

// EndpointProbe is one probe result.
type EndpointProbe struct {
	EndpointID string
	Method     string
	Status     string
	LatencyMs  uint32
	Error      string
	ObservedAt time.Time
}

// InsertEndpointProbes batch-inserts probe results.
func (c *Client) InsertEndpointProbes(ctx context.Context, rows []EndpointProbe) error {
	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO endpoint_probes")
	if err != nil {
		return fmt.Errorf("prepare endpoint_probes batch: %w", err)
	}
	for _, r := range rows {
		if err := batch.Append(r.EndpointID, r.Method, r.Status, r.LatencyMs, r.Error, r.ObservedAt); err != nil {
			return fmt.Errorf("append endpoint_probes: %w", err)
		}
	}
	return batch.Send()
}

// Span is an OTel-compatible trace span.
type Span struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Name         string
	Kind         string
	ServiceName  string
	Attrs        map[string]string
	StartTime    time.Time
	EndTime      time.Time
	StatusCode   string
	StatusMsg    string
	GenAISystem  string
	GenAIModel   string
	GenAIOp      string
	InputTokens  uint32
	OutputTokens uint32
}

// InsertSpans batch-inserts trace spans.
func (c *Client) InsertSpans(ctx context.Context, rows []Span) error {
	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO spans")
	if err != nil {
		return fmt.Errorf("prepare spans batch: %w", err)
	}
	for _, r := range rows {
		if err := batch.Append(
			r.TraceID, r.SpanID, r.ParentSpanID, r.Name, r.Kind, r.ServiceName,
			r.Attrs, r.StartTime, r.EndTime, r.StatusCode, r.StatusMsg,
			r.GenAISystem, r.GenAIModel, r.GenAIOp, r.InputTokens, r.OutputTokens,
			time.Now(),
		); err != nil {
			return fmt.Errorf("append span: %w", err)
		}
	}
	return batch.Send()
}
