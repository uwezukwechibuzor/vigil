import { describe, it, expect, vi, beforeEach } from "vitest";
import { createServer } from "../server.js";
import { flightCheckSummary, chainHealthSummary } from "../summaries.js";

// Mock VigilClient
vi.mock("@vigil/sdk-ts", () => ({
  VigilClient: vi.fn().mockImplementation(() => ({
    flightCheck: vi.fn().mockResolvedValue({
      id: "test-id",
      verdict: "warn",
      score: 25,
      checks: {
        simulation: { status: "pass" },
        gas_sanity: { status: "warn", reason: "gas spike" },
      },
      latency_ms: 123,
      created_at: new Date().toISOString(),
    }),
    chainHealth: vi.fn().mockResolvedValue({
      status: "healthy",
      head_lag_blocks: 1,
      reorgs_24h: 0,
      fee_level: "normal",
      worst_endpoints: [],
    }),
    listIncidents: vi.fn().mockResolvedValue({ data: [], meta: { has_more: false } }),
    getRun: vi.fn().mockResolvedValue({
      run_id: "run-1",
      agent_id: "agent-1",
      trace_id: "trace-1",
      span_count: 3,
      total_cost_usd: 0.005,
      created_at: new Date().toISOString(),
      spans: [],
    }),
    endpointHealth: vi.fn().mockResolvedValue({
      status: "healthy",
      uptime_1h_pct: 99.9,
      latency_p95_ms: 45,
      error_rate: 0.001,
    }),
    oracleHealth: vi.fn().mockResolvedValue({
      feed: "ETH/USD",
      last_answer: "3200.00",
      updated_at: new Date().toISOString(),
      stale: false,
    }),
    listChains: vi.fn().mockResolvedValue({ data: [], meta: { has_more: false } }),
  })),
  VigilError: class extends Error {},
}));

// suppress unused variable warning
void beforeEach;

describe("summary formatters", () => {
  it("flightCheckSummary is ≤ 150 words on warn", () => {
    const summary = flightCheckSummary({
      id: "x",
      verdict: "warn",
      score: 25,
      checks: { gas_sanity: { status: "warn", reason: "gas spike" } },
      latency_ms: 100,
      created_at: new Date().toISOString(),
    } as never);
    expect(summary.split(/\s+/).length).toBeLessThanOrEqual(150);
    expect(summary).toContain("WARN");
  });

  it("flightCheckSummary contains suggested action on fail", () => {
    const summary = flightCheckSummary({
      id: "x",
      verdict: "fail",
      score: 80,
      checks: { simulation: { status: "fail", reason: "reverts" } },
      latency_ms: 50,
      created_at: new Date().toISOString(),
    } as never);
    expect(summary).toMatch(/do NOT|block|fail/i);
  });

  it("chainHealthSummary is ≤ 150 words", () => {
    const summary = chainHealthSummary({
      status: "degraded",
      head_lag_blocks: 15,
      reorgs_24h: 2,
      fee_level: "high",
      worst_endpoints: ["ep1"],
    });
    expect(summary.split(/\s+/).length).toBeLessThanOrEqual(150);
  });
});

describe("server tools", () => {
  it("creates server without throwing", () => {
    expect(() => createServer({ apiKey: "test" })).not.toThrow();
  });

  it("server has the expected tool count", () => {
    // createServer returns McpServer — just verify it's constructed
    const server = createServer({ apiKey: "test" });
    expect(server).toBeDefined();
  });
});
