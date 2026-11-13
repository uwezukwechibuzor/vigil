import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { VigilClient } from "../client.js";
import { VigilError } from "../errors.js";
import type { FlightCheckResult, ChainHealth } from "../types.js";

const baseUrl = "https://api.vigil.dev";

function makeFlightCheckResult(verdict: "pass" | "warn" | "fail"): FlightCheckResult {
  return {
    id: "fc_123",
    verdict,
    score: verdict === "fail" ? 0.1 : verdict === "warn" ? 0.6 : 0.95,
    checks: { slippage: "ok" },
    latency_ms: 42,
    created_at: new Date().toISOString(),
  };
}

function makeChainHealth(): ChainHealth {
  return {
    status: "healthy",
    head_lag_blocks: 0,
    reorgs_24h: 0,
    fee_level: "normal",
    worst_endpoints: [],
  };
}

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("VigilClient", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("flightCheck returns FlightCheckResult on pass verdict", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(makeFlightCheckResult("pass"))));
    const client = new VigilClient({ apiKey: "vgl_test", baseUrl });
    const result = await client.flightCheck({
      chain_id: 1,
      from: "0xabc",
      to: "0xdef",
      data: "0x",
      value: "0x0",
    });
    expect(result.verdict).toBe("pass");
    expect(result.id).toBe("fc_123");
  });

  it("flightCheck does NOT throw on warn verdict", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(makeFlightCheckResult("warn"))));
    const client = new VigilClient({ apiKey: "vgl_test", baseUrl });
    const result = await client.flightCheck({
      chain_id: 1,
      from: "0xabc",
      to: "0xdef",
      data: "0x",
      value: "0x0",
    });
    expect(result.verdict).toBe("warn");
  });

  it("flightCheck calls onWarn callback on warn verdict", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(makeFlightCheckResult("warn"))));
    const client = new VigilClient({ apiKey: "vgl_test", baseUrl });
    const onWarn = vi.fn();
    await client.flightCheck(
      { chain_id: 1, from: "0xabc", to: "0xdef", data: "0x", value: "0x0" },
      { onWarn },
    );
    expect(onWarn).toHaveBeenCalledOnce();
  });

  it("flightCheck throws VigilError on 4xx", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          jsonResponse({ error: { code: "unauthorized", message: "Invalid API key" } }, 401),
        ),
    );
    const client = new VigilClient({ apiKey: "vgl_bad", baseUrl });
    await expect(
      client.flightCheck({ chain_id: 1, from: "0xabc", to: "0xdef", data: "0x", value: "0x0" }),
    ).rejects.toThrow(VigilError);
  });

  it("flightCheck throws VigilError on 5xx after retries", async () => {
    const mockFn = vi
      .fn()
      .mockResolvedValue(jsonResponse({ error: { code: "server_error", message: "oops" } }, 500));
    vi.stubGlobal("fetch", mockFn);
    const client = new VigilClient({ apiKey: "vgl_test", baseUrl });

    // Kick off the promise
    const promise = client
      .flightCheck({
        chain_id: 1,
        from: "0xabc",
        to: "0xdef",
        data: "0x",
        value: "0x0",
      })
      .catch((e: unknown) => e); // catch so it doesn't become unhandled

    // Advance fake timers to skip all retry sleeps
    await vi.runAllTimersAsync();

    const result = await promise;
    expect(result).toBeInstanceOf(VigilError);
    // Initial + 3 retries = 4 attempts
    expect(mockFn).toHaveBeenCalledTimes(4);
  });

  it("retries on 429 and honors Retry-After header", async () => {
    const mockFn = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: { code: "rate_limit", message: "slow down" } }), {
          status: 429,
          headers: { "Content-Type": "application/json", "Retry-After": "0.01" },
        }),
      )
      .mockResolvedValueOnce(jsonResponse(makeFlightCheckResult("pass")));
    vi.stubGlobal("fetch", mockFn);
    const client = new VigilClient({ apiKey: "vgl_test", baseUrl });

    const promise = client.flightCheck({
      chain_id: 1,
      from: "0xabc",
      to: "0xdef",
      data: "0x",
      value: "0x0",
    });
    await vi.runAllTimersAsync();

    const result = await promise;
    expect(result.verdict).toBe("pass");
    expect(mockFn).toHaveBeenCalledTimes(2);
  });

  it("sends X-Idempotency-Key on POST requests", async () => {
    const mockFn = vi.fn().mockResolvedValue(jsonResponse(makeFlightCheckResult("pass")));
    vi.stubGlobal("fetch", mockFn);
    const client = new VigilClient({ apiKey: "vgl_test", baseUrl });
    await client.flightCheck({
      chain_id: 1,
      from: "0xabc",
      to: "0xdef",
      data: "0x",
      value: "0x0",
    });
    const [, init] = mockFn.mock.calls[0] as [string, RequestInit];
    const headers = init.headers as Record<string, string>;
    expect(headers["X-Idempotency-Key"]).toBeTruthy();
  });

  it("chainHealth returns ChainHealth", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse(makeChainHealth())));
    const client = new VigilClient({ apiKey: "vgl_test", baseUrl });
    const health = await client.chainHealth(8453);
    expect(health.status).toBe("healthy");
    expect(health.fee_level).toBe("normal");
  });

  it("VigilError has correct code and statusCode", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          jsonResponse({ error: { code: "not_found", message: "chain not found" } }, 404),
        ),
    );
    const client = new VigilClient({ apiKey: "vgl_test", baseUrl });
    try {
      await client.chainHealth(99999);
      expect.fail("should have thrown");
    } catch (err) {
      expect(err).toBeInstanceOf(VigilError);
      const vigilErr = err as VigilError;
      expect(vigilErr.code).toBe("not_found");
      expect(vigilErr.statusCode).toBe(404);
    }
  });
});
