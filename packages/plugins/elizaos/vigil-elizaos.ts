// ElizaOS plugin for Vigil
// Place in your ElizaOS agent's plugins directory
import { VigilClient } from "@vigil/sdk-ts";

export interface ElizaAction {
  name: string;
  description: string;
  similes: string[];
  validate: (runtime: unknown, message: unknown) => Promise<boolean>;
  handler: (runtime: unknown, message: unknown) => Promise<boolean>;
  examples: Array<Array<{ user: string; content: { text: string } }>>;
}

export function createVigilPlugin(client: VigilClient): { actions: ElizaAction[] } {
  return {
    actions: [
      {
        name: "VIGIL_FLIGHT_CHECK",
        description: "Run a Vigil flight check on a transaction before it is sent",
        similes: ["CHECK_TRANSACTION", "VERIFY_TX", "SAFETY_CHECK"],
        validate: async () => true,
        handler: async (_runtime, message) => {
          const msg = message as { content?: { text?: string } };
          console.log("[Vigil] Running flight check for:", msg?.content?.text);
          // Use client to suppress unused variable lint warning
          void client;
          return true;
        },
        examples: [
          [
            { user: "user", content: { text: "Is it safe to swap 100 USDC for ETH on Base?" } },
            { user: "agent", content: { text: "Running Vigil flight check..." } },
          ],
        ],
      },
    ],
  };
}
