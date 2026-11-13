import { describe, it, expect, vi } from "vitest";

// Demo: "is it safe to swap 0.1 ETH to USDC on Base Sepolia right now?"
describe("demo: pre-trade safety check", () => {
  it("swap 0.1 ETH to USDC on Base Sepolia returns a verdict with summary", async () => {
    // Arrange: mock VigilClient returns "warn" for high gas
    const mockFlightCheck = vi.fn().mockResolvedValue({
      id: "demo-check-1",
      verdict: "warn",
      score: 20,
      checks: {
        gas_sanity: {
          status: "warn",
          reason: "Gas price is elevated: 180 Gwei (threshold 150 Gwei)",
        },
        simulation: { status: "pass" },
        oracle_freshness: { status: "pass" },
        contract_risk: { status: "pass" },
      },
      latency_ms: 210,
      created_at: new Date().toISOString(),
    });

    // Import dynamically so mock is set up first
    const { flightCheckSummary } = await import("../summaries.js");
    const result = await mockFlightCheck({
      chain_id: 84532, // Base Sepolia
      from: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
      to: "0x4200000000000000000000000000000000000006", // WETH on Base Sepolia
      data: "0x",
      value: "0x16345785d8a0000", // 0.1 ETH
      intent: "swap 0.1 ETH to USDC on Base Sepolia",
    });

    const summary = flightCheckSummary(result as Parameters<typeof flightCheckSummary>[0]);

    expect(result.verdict).toMatch(/^(pass|warn|fail)$/);
    expect(summary).toContain("WARN");
    expect(summary.split(/\s+/).length).toBeLessThanOrEqual(150);
    // LLM gets actionable guidance
    expect(summary).toMatch(/proceed|review|do NOT/i);

    console.log("\n=== Demo: Pre-trade safety check ===");
    console.log("Query: is it safe to swap 0.1 ETH to USDC on Base Sepolia right now?");
    console.log("\nVigil response:");
    console.log(summary);
  });
});
