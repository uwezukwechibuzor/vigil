// Package vigil is the Go SDK for the Vigil reliability API.
package vigil

import "time"

// Version is the current SDK version.
const Version = "0.1.0"

// Verdict is the result of a flight check.
type Verdict string

const (
	// VerdictPass means the transaction is safe to send.
	VerdictPass Verdict = "pass"
	// VerdictWarn means the transaction may be risky; check the result.
	VerdictWarn Verdict = "warn"
	// VerdictFail means the transaction should NOT be sent.
	VerdictFail Verdict = "fail"
)

// HealthStatus represents the health of a chain or endpoint.
type HealthStatus string

const (
	// HealthStatusHealthy means no issues detected.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusDegraded means partial degradation.
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusDown means the service is unavailable.
	HealthStatusDown HealthStatus = "down"
)

// FeeLevel represents the current gas fee level on a chain.
type FeeLevel string

const (
	// FeeLevelLow is below-normal fees.
	FeeLevelLow FeeLevel = "low"
	// FeeLevelNormal is typical fees.
	FeeLevelNormal FeeLevel = "normal"
	// FeeLevelHigh is elevated fees.
	FeeLevelHigh FeeLevel = "high"
	// FeeLevelExtreme is very high fees.
	FeeLevelExtreme FeeLevel = "extreme"
)

// IncidentStatus represents the lifecycle of an incident.
type IncidentStatus string

const (
	// IncidentStatusOpen is an active incident.
	IncidentStatusOpen IncidentStatus = "open"
	// IncidentStatusAcknowledged means someone has seen it.
	IncidentStatusAcknowledged IncidentStatus = "acknowledged"
	// IncidentStatusResolved means the incident is closed.
	IncidentStatusResolved IncidentStatus = "resolved"
)

// IncidentSeverity represents the impact level of an incident.
type IncidentSeverity string

const (
	// IncidentSeverityCritical is highest impact.
	IncidentSeverityCritical IncidentSeverity = "critical"
	// IncidentSeverityHigh is significant impact.
	IncidentSeverityHigh IncidentSeverity = "high"
	// IncidentSeverityMedium is moderate impact.
	IncidentSeverityMedium IncidentSeverity = "medium"
	// IncidentSeverityLow is minimal impact.
	IncidentSeverityLow IncidentSeverity = "low"
)

// ListMeta holds pagination metadata.
type ListMeta struct {
	Cursor  *string `json:"cursor,omitempty"`
	HasMore bool    `json:"has_more"`
	Total   *int64  `json:"total,omitempty"`
}

// ---- Chains ----

// Chain represents an EVM chain supported by Vigil.
type Chain struct {
	ID          string    `json:"id"`
	ChainID     int64     `json:"chain_id"`
	Name        string    `json:"name"`
	NativeToken string    `json:"native_token"`
	BlockTimeMS int       `json:"block_time_ms"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChainList is a paginated list of chains.
type ChainList struct {
	Data []Chain  `json:"data"`
	Meta ListMeta `json:"meta"`
}

// ChainHealth is a health snapshot for an EVM chain.
type ChainHealth struct {
	Status         HealthStatus `json:"status"`
	HeadLagBlocks  int64        `json:"head_lag_blocks"`
	Reorgs24h      int64        `json:"reorgs_24h"`
	FeeLevel       FeeLevel     `json:"fee_level"`
	WorstEndpoints []string     `json:"worst_endpoints"`
}

// ---- Endpoints ----

// Endpoint is an RPC/indexer/oracle endpoint registered with Vigil.
type Endpoint struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	ChainID   *string   `json:"chain_id,omitempty"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EndpointList is a paginated list of endpoints.
type EndpointList struct {
	Data []Endpoint `json:"data"`
	Meta ListMeta   `json:"meta"`
}

// EndpointHealth is a health snapshot for an endpoint.
type EndpointHealth struct {
	Status       HealthStatus `json:"status"`
	Uptime1hPct  float64      `json:"uptime_1h_pct"`
	LatencyP95MS int          `json:"latency_p95_ms"`
	ErrorRate    float64      `json:"error_rate"`
}

// OracleHealth is a health snapshot for a price feed.
type OracleHealth struct {
	Feed       string    `json:"feed"`
	LastAnswer string    `json:"last_answer"`
	UpdatedAt  time.Time `json:"updated_at"`
	Stale      bool      `json:"stale"`
}

// ---- Flight Checks ----

// FlightCheckRequest is a pre-transaction check request.
type FlightCheckRequest struct {
	ChainID        int64   `json:"chain_id"`
	From           string  `json:"from"`
	To             string  `json:"to"`
	Data           string  `json:"data"`
	Value          string  `json:"value"`
	Gas            *int64  `json:"gas,omitempty"`
	Intent         *string `json:"intent,omitempty"`
	PolicyID       *string `json:"policy_id,omitempty"`
	MaxSlippageBPS *int    `json:"max_slippage_bps,omitempty"`
	ExpectedOut    *string `json:"expected_out,omitempty"`
	DeadlineS      *int64  `json:"deadline_s,omitempty"`
	ProjectID      *string `json:"project_id,omitempty"`
}

// FlightCheckResult is the result of a flight check.
type FlightCheckResult struct {
	ID        string                 `json:"id"`
	ProjectID *string                `json:"project_id,omitempty"`
	Chain     *string                `json:"chain,omitempty"`
	Verdict   Verdict                `json:"verdict"`
	Score     float64                `json:"score"`
	Checks    map[string]interface{} `json:"checks"`
	LatencyMS *int                   `json:"latency_ms,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// ---- Runs ----

// RunSummary is a summary of a single agent run.
type RunSummary struct {
	RunID        string    `json:"run_id"`
	AgentID      string    `json:"agent_id"`
	TraceID      string    `json:"trace_id"`
	SpanCount    int       `json:"span_count"`
	TotalCostUSD float64   `json:"total_cost_usd"`
	TxHash       *string   `json:"tx_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// RunList is a paginated list of run summaries.
type RunList struct {
	Data []RunSummary `json:"data"`
	Meta ListMeta     `json:"meta"`
}

// Span is an OTel span from a run.
type Span struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID *string           `json:"parent_span_id,omitempty"`
	Name         string            `json:"name"`
	Kind         string            `json:"kind"`
	ServiceName  string            `json:"service_name"`
	Attrs        map[string]string `json:"attrs"`
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time"`
	StatusCode   string            `json:"status_code"`
	StatusMsg    *string           `json:"status_msg,omitempty"`
	GenAIModel   *string           `json:"gen_ai_model,omitempty"`
	InputTokens  *int              `json:"input_tokens,omitempty"`
	OutputTokens *int              `json:"output_tokens,omitempty"`
	LLMCostUSD   *float64          `json:"llm_cost_usd,omitempty"`
}

// RunDetail is a full run with spans.
type RunDetail struct {
	RunSummary
	Spans []Span `json:"spans"`
}

// ---- Incidents ----

// Incident is an active or resolved incident.
type Incident struct {
	ID             string           `json:"id"`
	ProjectID      string           `json:"project_id"`
	RuleID         *string          `json:"rule_id,omitempty"`
	Status         IncidentStatus   `json:"status"`
	Severity       IncidentSeverity `json:"severity"`
	Title          string           `json:"title"`
	StartedAt      time.Time        `json:"started_at"`
	AcknowledgedAt *time.Time       `json:"acknowledged_at,omitempty"`
	ResolvedAt     *time.Time       `json:"resolved_at,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

// IncidentList is a paginated list of incidents.
type IncidentList struct {
	Data []Incident `json:"data"`
	Meta ListMeta   `json:"meta"`
}

// ---- Policies ----

// Policy is a flight check policy.
type Policy struct {
	ID        string                 `json:"id"`
	ProjectID string                 `json:"project_id"`
	Name      string                 `json:"name"`
	Version   int                    `json:"version"`
	Rules     map[string]interface{} `json:"rules"`
	Enabled   bool                   `json:"enabled"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// PolicyList is a list of policies.
type PolicyList struct {
	Data []Policy `json:"data"`
}

// CreatePolicyRequest is a request to create a policy.
type CreatePolicyRequest struct {
	ProjectID string                 `json:"project_id"`
	Name      string                 `json:"name"`
	Rules     map[string]interface{} `json:"rules"`
	Enabled   *bool                  `json:"enabled,omitempty"`
}

// ---- API Keys ----

// APIKey holds metadata about an API key (without plaintext secret).
type APIKey struct {
	ID         string     `json:"id"`
	OrgID      string     `json:"org_id"`
	ProjectID  *string    `json:"project_id,omitempty"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// APIKeyList is a list of API keys.
type APIKeyList struct {
	Data []APIKey `json:"data"`
}

// APIKeyCreateResult holds the new API key metadata and plaintext secret.
type APIKeyCreateResult struct {
	Key    APIKey
	Secret string
}

// CreateAPIKeyRequest is a request to create an API key.
type CreateAPIKeyRequest struct {
	OrgID     string     `json:"org_id"`
	ProjectID *string    `json:"project_id,omitempty"`
	Name      string     `json:"name"`
	Scopes    []string   `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// ---- Usage ----

// UsageRecord is a daily usage counter for an org.
type UsageRecord struct {
	OrgID        string `json:"org_id"`
	Day          string `json:"day"`
	Traces       int64  `json:"traces"`
	FlightChecks int64  `json:"flight_checks"`
	APICalls     int64  `json:"api_calls"`
}

// UsageList is a list of usage records.
type UsageList struct {
	Data []UsageRecord `json:"data"`
}

// ---- Params ----

// RunListParams are query params for listing runs.
type RunListParams struct {
	AgentID   string
	ProjectID string
	Cursor    string
}

// IncidentListParams are query params for listing incidents.
type IncidentListParams struct {
	ProjectID string
	Status    string
}

// UsageParams are query params for fetching usage.
type UsageParams struct {
	OrgID string
	From  string
	To    string
}
