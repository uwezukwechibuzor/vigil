import http from "node:http";
import { randomUUID } from "node:crypto";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { createServer as createMcpServer } from "./server.js";
import type { ServerOptions } from "./types.js";

/** Start an HTTP server that serves MCP over Streamable HTTP at POST /mcp. */
export async function startHttpServer(
  opts: ServerOptions & { port?: number } = {},
): Promise<http.Server> {
  const port = opts.port ?? parseInt(process.env["MCP_PORT"] ?? "3001", 10);

  const httpServer = http.createServer(async (req, res) => {
    if (req.method === "POST" && req.url === "/mcp") {
      // extract auth
      const auth = req.headers["authorization"];
      const apiKey = auth?.startsWith("Bearer ") ? auth.slice(7) : undefined;
      const serverWithKey = createMcpServer({ ...opts, apiKey });

      const transport = new StreamableHTTPServerTransport({
        sessionIdGenerator: () => randomUUID(),
        enableJsonResponse: true,
      });
      await serverWithKey.connect(transport);

      let body = "";
      req.on("data", (chunk: Buffer) => {
        body += chunk.toString();
      });
      req.on("end", () => {
        const parsed = body ? (JSON.parse(body) as unknown) : undefined;
        transport.handleRequest(req, res, parsed).catch((e: unknown) => {
          console.error("mcp handle error", e);
          res.writeHead(500).end();
        });
      });
      return;
    }
    if (req.method === "GET" && req.url === "/healthz") {
      res.writeHead(200, { "Content-Type": "application/json" }).end('{"status":"ok"}');
      return;
    }
    res.writeHead(404).end();
  });

  await new Promise<void>((resolve) => httpServer.listen(port, resolve));
  console.log(`Vigil MCP HTTP server listening on :${port}/mcp`);
  return httpServer;
}
