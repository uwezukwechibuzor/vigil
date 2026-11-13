---
title: "Quickstart — Python SDK"
description: "Run your first flight check from Python in under 5 minutes."
---

# Quickstart — Python SDK

## Install

```bash
pip install vigil-sdk
```

## Get an API key

Sign in at [vigil.dev](https://vigil.dev) and create an API key from **Settings → API Keys**.

## Run a flight check

```python
import os
from vigil import VigilClient

vigil = VigilClient(api_key=os.environ["VIGIL_API_KEY"])

result = vigil.flight_check(
    chain="ethereum",
    rpc_url=os.environ["ETH_RPC_URL"],
    tx={
        "from": "0xYourWalletAddress",
        "to": "0xContractAddress",
        "value": "0xDE0B6B3A7640000",  # 1 ETH in hex wei
        "data": "0x",
    },
)

if result.risk_score > 0.7:
    raise RuntimeError(f"High risk — aborting: {result.checks}")

print(f"Risk score: {result.risk_score}  Verdict: {result.verdict}")
```

## Async usage

```python
import asyncio
from vigil import AsyncVigilClient

async def main():
    async with AsyncVigilClient(api_key=os.environ["VIGIL_API_KEY"]) as vigil:
        result = await vigil.flight_check(chain="ethereum", ...)
        print(result.verdict)

asyncio.run(main())
```

## Disable telemetry

```bash
VIGIL_TELEMETRY=0 python your_agent.py
```

## Next steps

- [Flight check reference](../reference/flight-check.md)
- [MCP server setup](../guides/mcp-setup.md)
- [Self-hosting guide](../guides/self-hosting.md)
