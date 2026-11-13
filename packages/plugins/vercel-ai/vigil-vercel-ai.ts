// Vercel AI SDK tool wrappers for Vigil
// Usage: import { vigilTools } from "@vigil/plugins/vercel-ai"
// Requires: ai ^4.0
import { VigilClient } from "@vigil/sdk-ts";

// Return type matches Vercel AI SDK's tool() shape (no hard dep)
export interface VercelTool {
  description: string;
  parameters: { type: "object"; properties: Record<string, unknown>; required?: string[] };
  execute: (params: unknown) => Promise<unknown>;
}

export function createVigilTools(client: VigilClient): Record<string, VercelTool> {
  return {
    vigil_flight_check: {
      description:
        "Run a pre-transaction safety check. Returns verdict (pass/warn/fail) and risk score.",
      parameters: {
        type: "object",
        properties: {
          chain_id: { type: "number", description: "EVM chain ID" },
          from: { type: "string", description: "Sender address" },
          to: { type: "string", description: "Target contract or address" },
          data: { type: "string", description: "Hex calldata" },
          value: { type: "string", description: "Value in hex wei" },
          intent: { type: "string", description: "Human description of the action" },
        },
        required: ["chain_id", "from", "to", "data", "value"],
      },
      execute: async (params) => {
        return await client.flightCheck(params as Parameters<typeof client.flightCheck>[0]);
      },
    },
    vigil_chain_health: {
      description: "Get chain health metrics.",
      parameters: {
        type: "object",
        properties: { chain_id: { type: "number" } },
        required: ["chain_id"],
      },
      execute: async (params) => {
        const p = params as { chain_id: number };
        return await client.chainHealth(p.chain_id);
      },
    },
  };
}
