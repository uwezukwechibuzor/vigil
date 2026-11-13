package clickhouse

import (
	"context"
	"fmt"
	"time"
)

// ChainHealthSummary holds aggregated chain health data.
type ChainHealthSummary struct {
	// Status is one of: healthy, degraded, down.
	Status string
	// HeadLagBlocks is the number of blocks behind the canonical head.
	HeadLagBlocks int64
	// Reorgs24h is the number of reorgs in the past 24 hours.
	Reorgs24h int64
	// FeeLevel is one of: low, normal, high, extreme.
	FeeLevel string
	// WorstEndpoints lists endpoint IDs with the highest error rates.
	WorstEndpoints []string
}

// EndpointHealthSummary holds aggregated endpoint health data.
type EndpointHealthSummary struct {
	// Status is one of: healthy, degraded, down.
	Status string
	// Uptime1hPct is the percentage of successful probes in the past hour.
	Uptime1hPct float64
	// LatencyP95Ms is the 95th percentile latency in milliseconds.
	LatencyP95Ms uint32
	// ErrorRate is the fraction of failed probes (0–1).
	ErrorRate float64
}

// OracleHealthSummary holds the latest oracle feed health data.
type OracleHealthSummary struct {
	// Feed is the oracle feed identifier.
	Feed string
	// LastAnswer is the most recent price answer as a string.
	LastAnswer string
	// UpdatedAt is when the oracle was last updated.
	UpdatedAt time.Time
	// Stale is true when the oracle has not been updated within the expected window.
	Stale bool
}

// RunSummary is a condensed view of one agent run.
type RunSummary struct {
	// RunID is the Vigil run identifier.
	RunID string
	// AgentID is the agent that executed the run.
	AgentID string
	// TraceID is the OTel trace ID.
	TraceID string
	// SpanCount is the number of spans in this run.
	SpanCount int
	// TotalCostUSD is the aggregate LLM + gas cost.
	TotalCostUSD float64
	// TxHash is the onchain transaction hash, if any.
	TxHash string
	// CreatedAt is the run start time.
	CreatedAt time.Time
}

// RunDetail extends RunSummary with the full span tree.
type RunDetail struct {
	RunSummary
	// Spans holds all spans belonging to this run.
	Spans []Span
}

// GetFlightCheck retrieves a single flight check result by ID.
func (c *Client) GetFlightCheck(ctx context.Context, id string) (*FlightCheckRow, error) {
	rows, err := c.conn.Query(ctx,
		`SELECT id, project_id, chain, verdict, score, checks, latency_ms
		   FROM flight_checks
		  WHERE id = ?
		  LIMIT 1`, id)
	if err != nil {
		return nil, fmt.Errorf("get flight check: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return nil, fmt.Errorf("flight check not found: %s", id)
	}
	var r FlightCheckRow
	if err := rows.Scan(&r.ID, &r.ProjectID, &r.Chain, &r.Verdict, &r.Score, &r.Checks, &r.LatencyMS); err != nil {
		return nil, fmt.Errorf("scan flight check: %w", err)
	}
	return &r, rows.Err()
}

// GetChainHealth returns aggregated health metrics for a chain.
func (c *Client) GetChainHealth(ctx context.Context, chainID string) (*ChainHealthSummary, error) {
	// Query max block lag and p95 latency over the past 5 minutes.
	var lagBlocks int64
	var latencyP95 float64

	lagRows, err := c.conn.Query(ctx,
		`SELECT max(block_num) FROM chain_heads WHERE chain = ? AND observed_at > now() - INTERVAL 5 MINUTE`,
		chainID)
	if err != nil {
		return nil, fmt.Errorf("chain health lag query: %w", err)
	}
	defer func() { _ = lagRows.Close() }()
	if lagRows.Next() {
		_ = lagRows.Scan(&lagBlocks)
	}

	probeRows, err := c.conn.Query(ctx,
		`SELECT quantile(0.95)(latency_ms) FROM endpoint_probes WHERE endpoint_id = ? AND observed_at > now() - INTERVAL 1 HOUR`,
		chainID)
	if err != nil {
		return nil, fmt.Errorf("chain health probe query: %w", err)
	}
	defer func() { _ = probeRows.Close() }()
	if probeRows.Next() {
		_ = probeRows.Scan(&latencyP95)
	}

	status := "healthy"
	feeLevel := "normal"
	if latencyP95 > 2000 {
		status = "degraded"
		feeLevel = "high"
	}

	return &ChainHealthSummary{
		Status:         status,
		HeadLagBlocks:  lagBlocks,
		Reorgs24h:      0,
		FeeLevel:       feeLevel,
		WorstEndpoints: []string{},
	}, nil
}

// GetEndpointHealth returns aggregated health metrics for an endpoint.
func (c *Client) GetEndpointHealth(ctx context.Context, endpointID string) (*EndpointHealthSummary, error) {
	rows, err := c.conn.Query(ctx,
		`SELECT
			countIf(status = 'ok') / count() * 100 as uptime_pct,
			quantile(0.95)(latency_ms)              as p95_ms,
			countIf(status != 'ok') / count()       as error_rate
		 FROM endpoint_probes
		 WHERE endpoint_id = ?
		   AND observed_at > now() - INTERVAL 1 HOUR`,
		endpointID)
	if err != nil {
		return nil, fmt.Errorf("endpoint health query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sum := &EndpointHealthSummary{Status: "healthy"}
	if rows.Next() {
		var p95 float64
		if err := rows.Scan(&sum.Uptime1hPct, &p95, &sum.ErrorRate); err != nil {
			return nil, fmt.Errorf("scan endpoint health: %w", err)
		}
		sum.LatencyP95Ms = uint32(p95)
	}

	switch {
	case sum.ErrorRate > 0.5:
		sum.Status = "down"
	case sum.ErrorRate > 0.1 || sum.Uptime1hPct < 90:
		sum.Status = "degraded"
	}
	return sum, rows.Err()
}

// GetOracleHealth returns the latest oracle feed health data.
func (c *Client) GetOracleHealth(ctx context.Context, feed string) (*OracleHealthSummary, error) {
	rows, err := c.conn.Query(ctx,
		`SELECT feed, answer, updated_at, stale
		   FROM oracle_updates
		  WHERE feed = ?
		  ORDER BY updated_at DESC
		  LIMIT 1`, feed)
	if err != nil {
		return nil, fmt.Errorf("oracle health query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return nil, fmt.Errorf("oracle feed not found: %s", feed)
	}
	var h OracleHealthSummary
	var staleUint8 uint8
	if err := rows.Scan(&h.Feed, &h.LastAnswer, &h.UpdatedAt, &staleUint8); err != nil {
		return nil, fmt.Errorf("scan oracle health: %w", err)
	}
	h.Stale = staleUint8 == 1
	return &h, rows.Err()
}

// ListRuns returns a paginated list of agent run summaries.
func (c *Client) ListRuns(ctx context.Context, agentID, projectID, cursor string, limit int) ([]RunSummary, error) {
	if limit <= 0 {
		limit = 20
	}

	var rows interface {
		Next() bool
		Scan(...any) error
		Close() error
		Err() error
	}
	var err error

	switch {
	case agentID != "" && projectID != "":
		rows, err = c.conn.Query(ctx,
			`SELECT vigil_run_id, vigil_agent_id, trace_id, count() as span_count,
				sum(llm_cost_usd + gas_cost_usd) as total_cost, any(vigil_tx_hash), min(start_time)
			 FROM spans
			 WHERE vigil_agent_id = ? AND project_id = ?
			 GROUP BY vigil_run_id, vigil_agent_id, trace_id
			 ORDER BY min(start_time) DESC
			 LIMIT ?`, agentID, projectID, limit)
	case agentID != "":
		rows, err = c.conn.Query(ctx,
			`SELECT vigil_run_id, vigil_agent_id, trace_id, count() as span_count,
				sum(llm_cost_usd + gas_cost_usd) as total_cost, any(vigil_tx_hash), min(start_time)
			 FROM spans
			 WHERE vigil_agent_id = ?
			 GROUP BY vigil_run_id, vigil_agent_id, trace_id
			 ORDER BY min(start_time) DESC
			 LIMIT ?`, agentID, limit)
	case projectID != "":
		rows, err = c.conn.Query(ctx,
			`SELECT vigil_run_id, vigil_agent_id, trace_id, count() as span_count,
				sum(llm_cost_usd + gas_cost_usd) as total_cost, any(vigil_tx_hash), min(start_time)
			 FROM spans
			 WHERE project_id = ?
			 GROUP BY vigil_run_id, vigil_agent_id, trace_id
			 ORDER BY min(start_time) DESC
			 LIMIT ?`, projectID, limit)
	default:
		rows, err = c.conn.Query(ctx,
			`SELECT vigil_run_id, vigil_agent_id, trace_id, count() as span_count,
				sum(llm_cost_usd + gas_cost_usd) as total_cost, any(vigil_tx_hash), min(start_time)
			 FROM spans
			 GROUP BY vigil_run_id, vigil_agent_id, trace_id
			 ORDER BY min(start_time) DESC
			 LIMIT ?`, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list runs query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []RunSummary
	for rows.Next() {
		var s RunSummary
		if err := rows.Scan(&s.RunID, &s.AgentID, &s.TraceID, &s.SpanCount, &s.TotalCostUSD, &s.TxHash, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan run summary: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetRun returns the full trace detail for one agent run.
func (c *Client) GetRun(ctx context.Context, id string) (*RunDetail, error) {
	spanRows, err := c.conn.Query(ctx,
		`SELECT trace_id, span_id, parent_span_id, name, kind, service_name,
		        start_time, end_time, status_code, status_msg,
		        gen_ai_model, input_tokens, output_tokens, llm_cost_usd,
		        vigil_run_id, vigil_agent_id, vigil_tx_hash
		 FROM spans
		 WHERE vigil_run_id = ?
		 ORDER BY start_time ASC`, id)
	if err != nil {
		return nil, fmt.Errorf("get run spans query: %w", err)
	}
	defer func() { _ = spanRows.Close() }()

	var detail RunDetail
	detail.RunID = id
	var totalCost float64
	seen := map[string]bool{}

	for spanRows.Next() {
		var s Span
		var statusMsg, genAIModel string
		if err := spanRows.Scan(
			&s.TraceID, &s.SpanID, &s.ParentSpanID, &s.Name, &s.Kind, &s.ServiceName,
			&s.StartTime, &s.EndTime, &s.StatusCode, &statusMsg,
			&genAIModel, &s.InputTokens, &s.OutputTokens, &s.LLMCostUSD,
			&s.VigilRunID, &s.VigilAgentID, &s.VigilTxHash,
		); err != nil {
			return nil, fmt.Errorf("scan run span: %w", err)
		}
		s.StatusMsg = statusMsg
		s.GenAIModel = genAIModel
		detail.Spans = append(detail.Spans, s)
		totalCost += s.LLMCostUSD
		if !seen[s.TraceID] {
			detail.TraceID = s.TraceID
			seen[s.TraceID] = true
		}
		if s.VigilAgentID != "" {
			detail.AgentID = s.VigilAgentID
		}
		if s.VigilTxHash != "" {
			detail.TxHash = s.VigilTxHash
		}
	}
	if err := spanRows.Err(); err != nil {
		return nil, fmt.Errorf("scan run spans: %w", err)
	}

	detail.SpanCount = len(detail.Spans)
	detail.TotalCostUSD = totalCost
	if len(detail.Spans) > 0 {
		detail.CreatedAt = detail.Spans[0].StartTime
	}
	return &detail, nil
}
