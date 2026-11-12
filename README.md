# Vigil

Vigil is a reliability layer for onchain AI agents and the crypto infra they run on.

## What it does

| Feature | Description |
|---|---|
| Infra health | Check RPC nodes, bridges, oracles and sequencers before trusting them |
| Flight check | Pre-sign validation of transactions — gas, nonce, allowance, risk signals |
| Agent tracing | End-to-end trace from LLM reasoning → onchain result with full audit log |

> Vigil gives risk signals, never guarantees. It never handles or stores private keys.

## Quickstart

### Prerequisites

- Go 1.23+
- Node 22, pnpm 9+
- Python 3.11+, uv
- Docker & Docker Compose

### Local dev

```bash
# 1. Copy env
cp .env.example .env

# 2. Start infrastructure (Postgres, Redis, ClickHouse)
make up

# 3. Format, lint, test, build everything
make ci

# 4. Run a service (example)
PORT=8080 go run ./services/api/cmd/api
curl http://localhost:8080/healthz
```

### Project structure

```
services/           Go microservices (chi, pgx, otel)
  api/              REST + WebSocket gateway
  collector/        Telemetry & trace ingestion
  flightcheck/      Pre-transaction validation engine
  ingest/           Raw event ingestion pipeline
  insights/         Aggregation & metrics computation
  alerts/           Alert rule evaluation & dispatch
  internal/platform Shared: config, logger, server, middleware, tracing

packages/           TypeScript packages (pnpm workspace + Turborepo)
  sdk-ts/           TypeScript SDK
  mcp-server/       MCP server for AI agent integration
  dashboard/        Next.js 15 dashboard

sdks/
  python/           Python SDK (httpx + pydantic v2)
  go/               Go SDK

cli/                `vigil` CLI binary
analytics/          dbt models, notebooks, tracking plan
deploy/             Docker, Kubernetes, Terraform
```

## Contributing

```bash
# Install pre-commit hooks
npx lefthook install

# CI checks
make fmt lint test build
```

Commits must follow [Conventional Commits](https://www.conventionalcommits.org/).
