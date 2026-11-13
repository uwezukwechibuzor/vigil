---
title: "MCP Server Setup"
description: "Connect Vigil to any MCP-compatible AI client."
---

# MCP Server Setup

The Vigil MCP server exposes flight checks, trace queries, and incident lookups as tools that any MCP-compatible client (Claude Desktop, Cursor, etc.) can call.

## Install

```bash
npm install -g @vigil/mcp-server
```

## Configure your MCP client

### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "vigil": {
      "command": "vigil-mcp",
      "env": {
        "VIGIL_API_KEY": "your-api-key",
        "VIGIL_BASE_URL": "https://api.vigil.dev"
      }
    }
  }
}
```

Restart Claude Desktop. You will see Vigil tools in the tool picker.

### Cursor / other clients

Follow the same pattern — point the MCP client at `vigil-mcp` with the same env vars.

## Available tools

| Tool | Description |
|------|-------------|
| `vigil_flight_check` | Run a pre-transaction flight check |
| `vigil_chain_health` | Get current health of a chain (RPC liveness, gas, block lag) |
| `vigil_list_incidents` | List open incidents across all monitored chains |
| `vigil_get_trace` | Retrieve an agent run trace by trace ID |

## Self-hosted MCP server

```bash
docker run -p 3100:3100 \
  -e VIGIL_API_KEY=your-key \
  -e VIGIL_BASE_URL=https://your-vigil-instance \
  ghcr.io/uwezukwechibuzor/vigil-mcp:latest
```

Then point your MCP client at `http://localhost:3100/sse`.
