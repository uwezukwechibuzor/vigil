/** Vigil API types — generated from openapi.yaml */

export type Verdict = "pass" | "warn" | "fail";
export type HealthStatus = "healthy" | "degraded" | "down";
export type FeeLevel = "low" | "normal" | "high" | "extreme";
export type EndpointKind = "rpc" | "indexer" | "oracle" | "custom";
export type IncidentStatus = "open" | "acknowledged" | "resolved";
export type IncidentSeverity = "critical" | "high" | "medium" | "low";
export type AlertChannelKind = "slack" | "pagerduty" | "webhook" | "email";

export interface ListMeta {
  cursor?: string | null;
  has_more: boolean;
  total?: number;
}

// ---- Chain ----
export interface Chain {
  id: string;
  chain_id: number;
  name: string;
  native_token: string;
  block_time_ms: number;
  enabled: boolean;
  created_at: string;
}

export interface ChainList {
  data: Chain[];
  meta: ListMeta;
}

export interface ChainHealth {
  status: HealthStatus;
  head_lag_blocks: number;
  reorgs_24h: number;
  fee_level: FeeLevel;
  worst_endpoints: string[];
}

// ---- Endpoint ----
export interface Endpoint {
  id: string;
  project_id: string;
  chain_id?: string | null;
  kind: EndpointKind;
  name: string;
  url: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface EndpointList {
  data: Endpoint[];
  meta: ListMeta;
}

export interface EndpointHealth {
  status: HealthStatus;
  uptime_1h_pct: number;
  latency_p95_ms: number;
  error_rate: number;
}

export interface OracleHealth {
  feed: string;
  last_answer: string;
  updated_at: string;
  stale: boolean;
}

// ---- Flight Check ----
export interface FlightCheckRequest {
  chain_id: number | bigint;
  from: string;
  to: string;
  data: string;
  value: string;
  gas?: number | null;
  intent?: string | null;
  policy_id?: string | null;
  max_slippage_bps?: number | null;
  expected_out?: string | null;
  deadline_s?: number | null;
  project_id?: string | null;
}

export interface FlightCheckResult {
  id: string;
  project_id?: string | null;
  chain?: string | null;
  verdict: Verdict;
  score: number;
  checks: Record<string, unknown>;
  latency_ms?: number | null;
  created_at: string;
}

// ---- Runs ----
export interface RunSummary {
  run_id: string;
  agent_id: string;
  trace_id: string;
  span_count: number;
  total_cost_usd: number;
  tx_hash?: string | null;
  created_at: string;
}

export interface RunList {
  data: RunSummary[];
  meta: ListMeta;
}

export interface Span {
  trace_id: string;
  span_id: string;
  parent_span_id?: string | null;
  name: string;
  kind: string;
  service_name: string;
  attrs: Record<string, string>;
  start_time: string;
  end_time: string;
  status_code: string;
  status_msg?: string | null;
  gen_ai_model?: string | null;
  input_tokens?: number | null;
  output_tokens?: number | null;
  llm_cost_usd?: number | null;
}

export interface RunDetail extends RunSummary {
  spans: Span[];
}

// ---- Incidents ----
export interface Incident {
  id: string;
  project_id: string;
  rule_id?: string | null;
  status: IncidentStatus;
  severity: IncidentSeverity;
  title: string;
  started_at: string;
  acknowledged_at?: string | null;
  resolved_at?: string | null;
  created_at: string;
}

export interface IncidentList {
  data: Incident[];
  meta: ListMeta;
}

// ---- Policies ----
export interface Policy {
  id: string;
  project_id: string;
  name: string;
  version: number;
  rules: Record<string, unknown>;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface PolicyList {
  data: Policy[];
}

export interface CreatePolicyRequest {
  project_id: string;
  name: string;
  rules: Record<string, unknown>;
  enabled?: boolean | null;
}

// ---- Alert Rules ----
export interface AlertRule {
  id: string;
  project_id: string;
  name: string;
  description?: string | null;
  severity: IncidentSeverity;
  condition: Record<string, unknown>;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface AlertRuleList {
  data: AlertRule[];
}

// ---- Alert Channels ----
export interface AlertChannel {
  id: string;
  org_id: string;
  kind: AlertChannelKind;
  name: string;
  config: Record<string, unknown>;
  created_at: string;
}

export interface AlertChannelList {
  data: AlertChannel[];
}

// ---- API Keys ----
export interface APIKey {
  id: string;
  org_id: string;
  project_id?: string | null;
  name: string;
  key_prefix: string;
  scopes: string[];
  expires_at?: string | null;
  last_used_at?: string | null;
  revoked_at?: string | null;
  created_at: string;
}

export interface APIKeyList {
  data: APIKey[];
}

export interface APIKeyCreateResult {
  key: APIKey;
  secret: string;
}

export interface CreateAPIKeyRequest {
  org_id: string;
  project_id?: string | null;
  name: string;
  scopes: string[];
  expires_at?: string | null;
}

// ---- Usage ----
export interface UsageRecord {
  org_id: string;
  day: string;
  traces: number;
  flight_checks: number;
  api_calls: number;
}

export interface UsageList {
  data: UsageRecord[];
}
