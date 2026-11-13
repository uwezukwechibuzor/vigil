---
title: "Quickstart — TypeScript SDK"
description: "Run your first flight check in under 5 minutes."
---

# Quickstart — TypeScript SDK

## Install

```bash
npm install @vigil/sdk
# or
pnpm add @vigil/sdk
```

## Get an API key

Sign in at [vigil.dev](https://vigil.dev) and create an API key from **Settings → API Keys**.

## Run a flight check

```typescript
import { VigilClient } from "@vigil/sdk";

const vigil = new VigilClient({ apiKey: process.env.VIGIL_API_KEY });

const result = await vigil.flightCheck({
  chain: "ethereum",
  rpcUrl: process.env.ETH_RPC_URL,
  tx: {
    from: "0xYourWalletAddress",
    to: "0xContractAddress",
    value: "0xDE0B6B3A7640000", // 1 ETH in hex wei
    data: "0x",
  },
});

if (result.riskScore > 0.7) {
  console.error("High risk — aborting:", result.checks);
  process.exit(1);
}

// Safe to proceed
console.log("Risk score:", result.riskScore, result.verdict);
```

## What the SDK does

1. Sends the transaction intent (not the signed transaction) to Vigil.
2. Vigil checks RPC liveness, gas price, nonce consistency, and optional simulation.
3. Returns a `riskScore` (0–1) and a `verdict` (`safe` / `warning` / `block`).

> **Note**: Vigil never receives or handles private keys. The transaction is not signed or broadcast.

## Disable telemetry

```bash
VIGIL_TELEMETRY=0 node your-agent.js
# or
DO_NOT_TRACK=1 node your-agent.js
```

## Next steps

- [Flight check reference](../reference/flight-check.md)
- [Tracing your agent runs](../guides/tracing.md)
- [MCP server setup](../guides/mcp-setup.md)
