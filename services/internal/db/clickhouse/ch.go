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

// Span is an OTel-compatible trace span enriched with Vigil agent conventions.
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
	// Vigil agent conventions.
	VigilAgentID       string
	VigilRunID         string
	VigilWallet        string
	VigilChainID       uint64
	VigilTxHash        string
	VigilFlightCheckID string
	VigilIntent        string
	// Cost tracking.
	LLMCostUSD float64
	GasCostUSD float64
	// Onchain linking.
	Linked        uint8
	SilentFailure uint8
}

// FlightCheckRow is one flight check result persisted to ClickHouse.
type FlightCheckRow struct {
	ID        string
	ProjectID string
	Chain     string
	Verdict   string
	Score     float32
	Checks    string // JSON blob
	LatencyMS uint32
}

// InsertFlightCheck stores a single flight check result.
func (c *Client) InsertFlightCheck(ctx context.Context, row FlightCheckRow) error {
	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO flight_checks")
	if err != nil {
		return fmt.Errorf("prepare flight_checks batch: %w", err)
	}
	if err := batch.Append(row.ID, row.ProjectID, row.Chain, row.Verdict, row.Score, row.Checks, row.LatencyMS); err != nil {
		return fmt.Errorf("append flight_check: %w", err)
	}
	return batch.Send()
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
			r.VigilAgentID, r.VigilRunID, r.VigilWallet, r.VigilChainID,
			r.VigilTxHash, r.VigilFlightCheckID, r.VigilIntent,
			r.LLMCostUSD, r.GasCostUSD, r.Linked, r.SilentFailure,
			time.Now(),
		); err != nil {
			return fmt.Errorf("append span: %w", err)
		}
	}
	return batch.Send()
}

// UnlinkedSpansWithTx returns spans that have a tx hash but have not been linked yet.
func (c *Client) UnlinkedSpansWithTx(ctx context.Context, limit int) ([]Span, error) {
	rows, err := c.conn.Query(ctx,
		`SELECT trace_id, span_id, service_name, vigil_tx_hash, vigil_chain_id,
		        gen_ai_model, input_tokens, output_tokens
		   FROM spans
		  WHERE vigil_tx_hash != '' AND linked = 0
		  LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query unlinked spans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var spans []Span
	for rows.Next() {
		var s Span
		if err := rows.Scan(
			&s.TraceID, &s.SpanID, &s.ServiceName, &s.VigilTxHash, &s.VigilChainID,
			&s.GenAIModel, &s.InputTokens, &s.OutputTokens,
		); err != nil {
			return nil, fmt.Errorf("scan unlinked span: %w", err)
		}
		spans = append(spans, s)
	}
	return spans, rows.Err()
}

// MarkSpanLinked sets linked=1 and records silent_failure and gas_cost_usd for a span.
func (c *Client) MarkSpanLinked(ctx context.Context, traceID, spanID string, silentFailure uint8, gasCostUSD float64) error {
	err := c.conn.Exec(ctx,
		`ALTER TABLE spans UPDATE linked = 1, silent_failure = ?, gas_cost_usd = ?
		  WHERE trace_id = ? AND span_id = ?`,
		silentFailure, gasCostUSD, traceID, spanID,
	)
	if err != nil {
		return fmt.Errorf("mark span linked: %w", err)
	}
	return nil
}
