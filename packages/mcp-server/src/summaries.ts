import type {
  FlightCheckResult,
  ChainHealth,
  EndpointHealth,
  OracleHealth,
  Incident,
  RunDetail,
} from "@vigil/sdk-ts";

/** Format a flight check result into an LLM-friendly summary (≤150 words). */
export function flightCheckSummary(r: FlightCheckResult): string {
  const emoji = r.verdict === "pass" ? "✅" : r.verdict === "warn" ? "⚠️" : "🚫";
  const lines = [
    `${emoji} Verdict: ${r.verdict.toUpperCase()} (risk score ${r.score}/100)`,
    `Latency: ${r.latency_ms ?? "?"}ms`,
  ];
  // list failing/warning checks
  const checks = Object.entries(r.checks);
  for (const [name, result] of checks) {
    if (typeof result === "object" && result !== null) {
      const c = result as { status?: string; reason?: string };
      if (c.status && c.status !== "pass") {
        lines.push(`• ${name}: ${c.status} — ${c.reason ?? ""}`);
      }
    }
  }
  if (r.verdict === "pass") lines.push("Suggested action: proceed with the transaction.");
  if (r.verdict === "warn") lines.push("Suggested action: review the warnings before proceeding.");
  if (r.verdict === "fail") lines.push("Suggested action: do NOT send this transaction.");
  return lines.join("\n");
}

/** Format chain health into an LLM-friendly summary (≤150 words). */
export function chainHealthSummary(h: ChainHealth): string {
  const emoji = h.status === "healthy" ? "✅" : h.status === "degraded" ? "⚠️" : "🔴";
  const lines = [
    `${emoji} Chain status: ${h.status.toUpperCase()}`,
    `Block lag: ${h.head_lag_blocks} blocks | Reorgs (24h): ${h.reorgs_24h} | Gas: ${h.fee_level}`,
  ];
  if (h.worst_endpoints.length > 0) {
    lines.push(`Worst endpoints: ${h.worst_endpoints.slice(0, 3).join(", ")}`);
  }
  if (h.status === "healthy") {
    lines.push("Suggested action: chain is healthy, safe to proceed.");
  } else if (h.status === "degraded") {
    lines.push("Suggested action: chain is degraded — monitor closely before transacting.");
  } else {
    lines.push("Suggested action: chain is down — do NOT send transactions.");
  }
  return lines.join("\n");
}

/** Format endpoint health into an LLM-friendly summary (≤150 words). */
export function endpointHealthSummary(h: EndpointHealth): string {
  const emoji = h.status === "healthy" ? "✅" : h.status === "degraded" ? "⚠️" : "🔴";
  const lines = [
    `${emoji} Endpoint status: ${h.status.toUpperCase()}`,
    `Uptime (1h): ${h.uptime_1h_pct.toFixed(2)}% | p95 latency: ${h.latency_p95_ms}ms | Error rate: ${(h.error_rate * 100).toFixed(2)}%`,
  ];
  if (h.status === "healthy") {
    lines.push("Suggested action: endpoint is healthy.");
  } else if (h.status === "degraded") {
    lines.push("Suggested action: endpoint is degraded — consider using a backup RPC.");
  } else {
    lines.push(
      "Suggested action: endpoint is down — switch to a different RPC endpoint immediately.",
    );
  }
  return lines.join("\n");
}

/** Format oracle health into an LLM-friendly summary (≤150 words). */
export function oracleHealthSummary(h: OracleHealth): string {
  const emoji = h.stale ? "⚠️" : "✅";
  const lines = [
    `${emoji} Oracle: ${h.feed}`,
    `Last answer: ${h.last_answer} | Updated: ${h.updated_at}`,
    `Stale: ${h.stale ? "YES" : "no"}`,
  ];
  if (h.stale) {
    lines.push("Suggested action: oracle is stale — do NOT rely on this price feed for swaps.");
  } else {
    lines.push("Suggested action: oracle is fresh, safe to use for pricing.");
  }
  return lines.join("\n");
}

/** Format an incident into an LLM-friendly summary (≤150 words). */
export function incidentSummary(i: Incident): string {
  const severityEmoji: Record<string, string> = {
    critical: "🔴",
    high: "🟠",
    medium: "🟡",
    low: "🔵",
  };
  const emoji = severityEmoji[i.severity] ?? "⚠️";
  const lines = [
    `${emoji} Incident: ${i.title}`,
    `Severity: ${i.severity.toUpperCase()} | Status: ${i.status}`,
    `Started: ${i.started_at}`,
  ];
  if (i.status === "open") {
    lines.push(
      "Suggested action: incident is active — check affected endpoints and consider pausing operations.",
    );
  } else if (i.status === "acknowledged") {
    lines.push("Suggested action: incident is being investigated — monitor for updates.");
  } else {
    lines.push("Suggested action: incident is resolved.");
  }
  return lines.join("\n");
}

/** Format a run detail into an LLM-friendly summary (≤150 words). */
export function runSummary(r: RunDetail): string {
  const lines = [
    `Run: ${r.run_id}`,
    `Agent: ${r.agent_id} | Trace: ${r.trace_id}`,
    `Spans: ${r.span_count} | Total cost: $${r.total_cost_usd.toFixed(4)} USD`,
  ];
  if (r.tx_hash) {
    lines.push(`Transaction: ${r.tx_hash}`);
  }
  lines.push(`Created: ${r.created_at}`);
  if (r.span_count === 0) {
    lines.push("Suggested action: run has no spans — tracing may not be configured.");
  } else {
    lines.push("Suggested action: review spans for latency or cost anomalies.");
  }
  return lines.join("\n");
}
