// LangChain JS (langchain/core) tool wrappers for Vigil
// Usage: import { createVigilTools } from "@vigil/plugins/langchain"
import { VigilClient } from "@vigil/sdk-ts";

// Define tool schema compatible with @langchain/core DynamicStructuredTool
export interface LangChainTool {
  name: string;
  description: string;
  schema: object;
  func: (input: unknown) => Promise<string>;
}

export function createVigilTools(client: VigilClient): LangChainTool[] {
  return [
    {
      name: "vigil_flight_check",
      description: "Run a pre-transaction safety check on an EVM transaction before signing it.",
      schema: {
        type: "object",
        properties: {
          chain_id: { type: "number" },
          from: { type: "string" },
          to: { type: "string" },
          data: { type: "string" },
          value: { type: "string" },
          intent: { type: "string" },
        },
        required: ["chain_id", "from", "to", "data", "value"],
      },
      func: async (input: unknown) => {
        const params = input as {
          chain_id: number;
          from: string;
          to: string;
          data: string;
          value: string;
          intent?: string;
        };
        const result = await client.flightCheck(params);
        return `Verdict: ${result.verdict}, Score: ${result.score}/100`;
      },
    },
    {
      name: "vigil_chain_health",
      description: "Check the health of an EVM chain (block lag, reorgs, fee level).",
      schema: {
        type: "object",
        properties: { chain_id: { type: "number" } },
        required: ["chain_id"],
      },
      func: async (input: unknown) => {
        const params = input as { chain_id: number };
        const h = await client.chainHealth(params.chain_id);
        return `Status: ${h.status}, Lag: ${h.head_lag_blocks} blocks, Fee: ${h.fee_level}`;
      },
    },
  ];
}
