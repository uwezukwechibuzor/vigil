# Vigil MCP Server Setup

Vigil gives AI agents onchain safety tools over the Model Context Protocol.

## Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "vigil": {
      "command": "npx",
      "args": ["-y", "@vigil/mcp-server"],
      "env": { "VIGIL_API_KEY": "vgl_live_..." }
    }
  }
}
```

## Claude Code

```bash
claude mcp add vigil -- npx -y @vigil/mcp-server
# Then set: export VIGIL_API_KEY=vgl_live_...
```

Or add to `~/.claude/settings.json`:
```json
{
  "mcpServers": {
    "vigil": {
      "command": "npx",
      "args": ["-y", "@vigil/mcp-server"]
    }
  }
}
```

## Cursor

Add to `~/.cursor/mcp.json`:
```json
{
  "vigil": {
    "command": "npx",
    "args": ["-y", "@vigil/mcp-server"],
    "env": { "VIGIL_API_KEY": "vgl_live_..." }
  }
}
```

## Hosted (Streamable HTTP)

```bash
MCP_PORT=3001 VIGIL_API_KEY=vgl_live_... npx @vigil/mcp-server --http
```

Clients connect to `http://your-host:3001/mcp` with:
```
Authorization: Bearer vgl_live_...
```

## Available Tools

| Tool | Description |
|------|-------------|
| `vigil_flight_check` | Pre-transaction safety check |
| `vigil_chain_health` | Chain block lag, reorgs, gas |
| `vigil_endpoint_health` | RPC/indexer endpoint metrics |
| `vigil_oracle_health` | Oracle freshness check |
| `vigil_explain_incident` | Incident details and remediation |
| `vigil_find_anomalies` | Scan for open incidents |
| `vigil_get_run` | Agent run trace and costs |
| `vigil_list_incidents` | List recent incidents |
