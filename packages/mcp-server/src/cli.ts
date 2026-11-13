import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { createServer } from "./server.js";

async function main() {
  const server = createServer();
  const transport = new StdioServerTransport();
  await server.connect(transport);
  // keep alive
  process.on("SIGINT", () => process.exit(0));
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
