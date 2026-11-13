import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { VigilClient } from "@vigil/sdk-ts";
import * as summaries from "./summaries.js";
import type { ServerOptions } from "./types.js";

export function createServer(opts: ServerOptions = {}): McpServer {
  const apiKey = opts.apiKey ?? process.env["VIGIL_API_KEY"];
  const baseUrl = opts.baseUrl ?? process.env["VIGIL_BASE_URL"];
  const clientOpts: { apiKey?: string; baseUrl?: string } = {};
  if (apiKey !== undefined) clientOpts.apiKey = apiKey;
  if (baseUrl !== undefined) clientOpts.baseUrl = baseUrl;
  const client = new VigilClient(clientOpts);

  const server = new McpServer({
    name: "vigil",
    version: "0.1.0",
  });

  // ---- TOOLS ----

  // vigil_flight_check
  server.tool(
    "vigil_flight_check",
    "Run a pre-transaction safety check. Returns a verdict (pass/warn/fail), risk score 0-100, and a summary of any issues found. Use this before signing any onchain transaction.",
    {
      chain_id: z
        .number()
        .int()
        .positive()
        .describe("EVM chain ID (e.g. 8453 for Base, 1 for Ethereum mainnet)"),
      from: z.string().describe("Sender wallet address (0x...)"),
      to: z.string().describe("Destination address or contract (0x...)"),
      data: z.string().describe("Hex-encoded calldata (use '0x' for plain ETH transfers)"),
      value: z
        .string()
        .describe("Transaction value in hex-encoded wei (use '0x0' for token-only calls)"),
      intent: z
        .string()
        .optional()
        .describe("Human description of the operation, e.g. 'swap 0.1 ETH to USDC'"),
    },
    async (params) => {
      const fcParams: Parameters<typeof client.flightCheck>[0] = {
        chain_id: params.chain_id,
        from: params.from,
        to: params.to,
        data: params.data,
        value: params.value,
      };
      if (params.intent !== undefined) fcParams.intent = params.intent;
      const result = await client.flightCheck(fcParams);
      return {
        content: [
          { type: "text", text: summaries.flightCheckSummary(result) },
          { type: "text", text: "```json\n" + JSON.stringify(result, null, 2) + "\n```" },
        ],
        isError: result.verdict === "fail",
      };
    },
  );

  // vigil_chain_health
  server.tool(
    "vigil_chain_health",
    "Get the current health of an EVM chain: block lag, reorg count in last 24h, gas fee level, and status of the worst-performing RPC endpoints.",
    {
      chain_id: z.number().int().positive().describe("EVM chain ID"),
    },
    async (params) => {
      const h = await client.chainHealth(params.chain_id);
      return {
        content: [
          { type: "text", text: summaries.chainHealthSummary(h) },
          { type: "text", text: "```json\n" + JSON.stringify(h, null, 2) + "\n```" },
        ],
      };
    },
  );

  // vigil_endpoint_health
  server.tool(
    "vigil_endpoint_health",
    "Get the health metrics for a specific RPC, indexer or oracle endpoint: uptime in last hour, p95 latency, and error rate.",
    {
      endpoint_id: z.string().describe("Endpoint UUID from the Vigil dashboard"),
    },
    async (params) => {
      const h = await client.endpointHealth(params.endpoint_id);
      return {
        content: [
          { type: "text", text: summaries.endpointHealthSummary(h) },
          { type: "text", text: "```json\n" + JSON.stringify(h, null, 2) + "\n```" },
        ],
      };
    },
  );

  // vigil_oracle_health
  server.tool(
    "vigil_oracle_health",
    "Check if an on-chain price oracle (e.g. Chainlink ETH/USD) is fresh and returning valid data. Stale oracles can cause incorrect swap prices.",
    {
      feed: z
        .string()
        .describe("Oracle feed identifier, e.g. 'ETH/USD' or '0x...' contract address"),
    },
    async (params) => {
      const h = await client.oracleHealth(params.feed);
      return {
        content: [
          { type: "text", text: summaries.oracleHealthSummary(h) },
          { type: "text", text: "```json\n" + JSON.stringify(h, null, 2) + "\n```" },
        ],
      };
    },
  );

  // vigil_explain_incident
  server.tool(
    "vigil_explain_incident",
    "Get a detailed explanation of a specific incident: what happened, which endpoints were affected, timeline, and suggested remediation.",
    {
      incident_id: z.string().uuid().describe("Incident UUID"),
    },
    async (params) => {
      const incidents = await client.listIncidents({ status: "open" });
      const incident = incidents.data.find((i) => i.id === params.incident_id);
      if (!incident) {
        return {
          content: [{ type: "text", text: `Incident ${params.incident_id} not found.` }],
          isError: true,
        };
      }
      return {
        content: [
          { type: "text", text: summaries.incidentSummary(incident) },
          { type: "text", text: "```json\n" + JSON.stringify(incident, null, 2) + "\n```" },
        ],
      };
    },
  );

  // vigil_find_anomalies
  server.tool(
    "vigil_find_anomalies",
    "Find recent open incidents and health anomalies across a scope. Returns a prioritized list of issues to investigate.",
    {
      scope: z
        .enum(["all", "chains", "endpoints", "oracles"])
        .describe("What to scan for anomalies"),
      window: z.enum(["1h", "6h", "24h", "7d"]).default("24h").describe("Lookback window"),
    },
    async (params) => {
      // list open incidents as a proxy for anomalies
      const incidents = await client.listIncidents({ status: "open" });
      const count = incidents.data.length;
      const summary =
        count === 0
          ? `✅ No open incidents in scope '${params.scope}' for the last ${params.window}.`
          : `⚠️ Found ${count} open incident(s) in scope '${params.scope}' for the last ${params.window}:\n` +
            incidents.data.map((i) => `• [${i.severity.toUpperCase()}] ${i.title}`).join("\n");
      return {
        content: [
          { type: "text", text: summary },
          { type: "text", text: "```json\n" + JSON.stringify(incidents.data, null, 2) + "\n```" },
        ],
      };
    },
  );

  // vigil_get_run
  server.tool(
    "vigil_get_run",
    "Get the full trace of an AI agent run: LLM calls, tool calls, token costs, and the onchain transaction it produced.",
    {
      run_id: z.string().describe("Run ID (vigil_run_id from the trace)"),
    },
    async (params) => {
      const run = await client.getRun(params.run_id);
      return {
        content: [
          { type: "text", text: summaries.runSummary(run) },
          { type: "text", text: "```json\n" + JSON.stringify(run, null, 2) + "\n```" },
        ],
      };
    },
  );

  // vigil_list_incidents
  server.tool(
    "vigil_list_incidents",
    "List recent incidents. Use this to get a quick overview of current problems on the chains and endpoints your project monitors.",
    {
      status: z
        .enum(["open", "acknowledged", "resolved"])
        .optional()
        .describe("Filter by incident status"),
      since: z
        .string()
        .optional()
        .describe("ISO 8601 datetime — only return incidents since this time"),
    },
    async (params) => {
      const incidentParams: { status?: string } = {};
      if (params.status !== undefined) incidentParams.status = params.status;
      const result = await client.listIncidents(incidentParams);
      const summary =
        result.data.length === 0
          ? "No incidents found."
          : `Found ${result.data.length} incident(s):\n` +
            result.data
              .map((i) => `• [${i.severity.toUpperCase()}] ${i.status} — ${i.title}`)
              .join("\n");
      return {
        content: [
          { type: "text", text: summary },
          { type: "text", text: "```json\n" + JSON.stringify(result.data, null, 2) + "\n```" },
        ],
      };
    },
  );

  // ---- RESOURCES ----

  server.resource("chains", "vigil://chains", async (uri) => {
    const chains = await client.listChains();
    return {
      contents: [
        {
          uri: uri.href,
          mimeType: "application/json",
          text: JSON.stringify(chains.data, null, 2),
        },
      ],
    };
  });

  server.resource("conventions", "vigil://docs/conventions", async (uri) => ({
    contents: [
      {
        uri: uri.href,
        mimeType: "text/markdown",
        text: "# Vigil Span Conventions\n\nSee https://docs.vigil.dev/conventions for the full reference.",
      },
    ],
  }));

  // ---- PROMPTS ----

  server.prompt(
    "pre_trade_safety_check",
    "Generate a pre-trade safety checklist for a proposed onchain transaction.",
    {
      chain_id: z.string().describe("EVM chain ID as string"),
      from: z.string().describe("Sender address"),
      to: z.string().describe("Target contract"),
      action: z.string().describe("Human description of the action, e.g. 'swap 0.1 ETH to USDC'"),
    },
    async (params) => ({
      messages: [
        {
          role: "user" as const,
          content: {
            type: "text" as const,
            text: `Use vigil_flight_check with chain_id=${params.chain_id}, from=${params.from}, to=${params.to}, value=0x0, data=0x, intent="${params.action}". Then summarize the verdict and explain any risks. If verdict is "fail", explain why the transaction should not proceed.`,
          },
        },
      ],
    }),
  );

  server.prompt(
    "diagnose_indexer_lag",
    "Diagnose why a subgraph or indexer is lagging behind the chain head.",
    {
      endpoint_id: z.string().describe("Endpoint ID of the indexer"),
    },
    async (params) => ({
      messages: [
        {
          role: "user" as const,
          content: {
            type: "text" as const,
            text: `Use vigil_endpoint_health with endpoint_id=${params.endpoint_id}. If it shows lag, also run vigil_chain_health for the same chain, then vigil_find_anomalies with scope=endpoints. Summarize the root cause and suggest a fix.`,
          },
        },
      ],
    }),
  );

  return server;
}
