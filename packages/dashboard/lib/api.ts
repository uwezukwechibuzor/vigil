const BASE = "/api-proxy";

export async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...((options?.headers as Record<string, string>) ?? {}) },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`${res.status}: ${text}`);
  }
  return res.json() as Promise<T>;
}

export interface Me {
  org_id: string;
  project_id: string | null;
}

export interface Chain {
  id: string;
  chain_id: number;
  name: string;
  native_token: string;
  block_time_ms: number;
  enabled: boolean;
}

export interface ChainHealth {
  status: string;
  head_lag_blocks: number;
  reorgs_24h: number;
  fee_level: string;
  worst_endpoints: string[];
}

export interface Endpoint {
  id: string;
  project_id: string;
  chain_id?: string;
  kind: string;
  name: string;
  url: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface APIKey {
  id: string;
  org_id: string;
  project_id?: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  created_at: string;
  expires_at?: string;
  last_used_at?: string;
  revoked_at?: string;
}

export interface RunSummary {
  run_id: string;
  agent_id: string;
  trace_id: string;
  span_count: number;
  total_cost_usd: number;
  tx_hash?: string;
  created_at: string;
}

export interface Incident {
  id: string;
  project_id: string;
  status: string;
  severity: string;
  title: string;
  started_at: string;
  acknowledged_at?: string;
  resolved_at?: string;
  created_at: string;
}

export interface UsageCounter {
  org_id: string;
  day: string;
  traces: number;
  flight_checks: number;
  api_calls: number;
}

export interface Policy {
  id: string;
  project_id: string;
  name: string;
  version: number;
  rules: Record<string, unknown>;
  enabled: boolean;
  created_at: string;
}

export interface AlertRule {
  id: string;
  project_id: string;
  name: string;
  severity: string;
  condition: Record<string, unknown>;
  enabled: boolean;
  description?: string;
  created_at: string;
}

export function getMe() {
  return apiFetch<Me>("/v1/me");
}
