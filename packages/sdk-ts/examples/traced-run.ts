// Example: startRun + tracedLLMCall + end
import { VigilClient, startRun, initTracing } from "../src/index.js";

initTracing({ serviceName: "my-agent", apiKey: process.env["VIGIL_API_KEY"] });

const _vigil = new VigilClient();
const run = startRun({ agentId: "agent_abc123", intent: "swap USDC on Base" });

const answer = await run.tracedLLMCall(
  { model: "claude-3-5-sonnet-20241022", maxTokens: 1024 },
  async (span) => {
    // Call your LLM here...
    span.setAttribute("gen_ai.usage.input_tokens", 150);
    span.setAttribute("gen_ai.usage.output_tokens", 80);
    return "I will swap 100 USDC to ETH on Base";
  },
);

console.log("LLM answer:", answer);

await run.tracedToolCall("execute_swap", async (span) => {
  span.setAttribute("tool.chain_id", "8453");
  // Execute the swap here...
  run.attachTx("0xsomeTxHash");
});

run.end("ok");
console.log("Run complete. run_id:", run.runId, "trace_id:", run.traceId);
