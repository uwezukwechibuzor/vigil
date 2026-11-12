# Vigil — project context

Vigil is a reliability layer for onchain AI agents and the crypto infra they run on.
Any agent, bot, backend or human can call it to (1) check onchain infra health,
(2) run a pre-transaction "flight check" before signing, and (3) trace agent runs
end to end, from LLM reasoning to the onchain result.
Agent-first, not agent-only: the same core serves SDKs, REST, MCP and a dashboard.

## Stack (do not change without asking)
- Backend services: Go 1.23+ (chi router, pgx, clickhouse-go v2, go-redis, go-ethereum)
- MCP server, TS SDK, dashboard: TypeScript (Node 22, pnpm, Next.js 15, viem)
- Python SDK: Python 3.11+ (httpx, pydantic v2)
- Storage: Postgres 16 (accounts, keys, config), ClickHouse (telemetry, traces),
  Redis 7 (cache, rate limits, Redis Streams queues)
- Telemetry in: OpenTelemetry (OTLP/HTTP), incl. GenAI semantic conventions
- Chains first: Ethereum, Base, Arbitrum (EVM). Design chain adapters so Solana fits later.
- Product analytics: PostHog (events, funnels, feature flags, experiments),
  exported to ClickHouse as the warehouse; dbt (dbt-clickhouse) for models and
  the metrics layer; Python (polars, duckdb, scipy, statsmodels) for analysis;
  Metabase or Grafana for BI dashboards.

## Repo layout
- services/{api,collector,flightcheck,ingest,insights,alerts}  (Go)
- packages/{sdk-ts,mcp-server,dashboard}  (TS)
- sdks/{python,go}
- cli/  (Go, the `vigil` command)
- analytics/{tracking-plan,dbt,notebooks,reports}  (SQL + Python)
- deploy/{docker,k8s,terraform}
- docs/

## Rules
- Production quality: typed errors, context timeouts, structured JSON logs (slog),
  Prometheus metrics on every service at /metrics, health at /healthz and /readyz.
- Config only from env vars (12-factor). Never hardcode secrets. Update .env.example.
- Every feature ships with unit tests; integration tests use docker-compose services.
- Run fmt + lint + tests + build before every commit; fix failures, never
  skip or disable tests. Conventional commits.
- Commits use ONLY the repo's configured git identity (uwezukwechibuzor).
  Never add Co-Authored-By, "Generated with" or any AI attribution lines.
- After each task: push to origin main
  (https://github.com/uwezukwechibuzor/vigil.git). Never force-push.
- Keep PROGRESS.md updated: what was done, decisions, open TODOs.
- Responses to agents must be compact and summarized (no raw dumps).
- Vigil gives risk signals, never guarantees. Never handle or store private keys.
- Product telemetry follows analytics/tracking-plan only: no PII, no wallet
  keys, no transaction payloads; SDK/CLI telemetry is anonymous and can be
  turned off with VIGIL_TELEMETRY=0 or DO_NOT_TRACK=1.

## Go workspace notes
- Each service is its own Go module with a replace directive pointing to
  services/internal/platform for the shared platform library.
- Build individual services: `go build github.com/uwezukwechibuzor/vigil/services/<svc>/cmd/<svc>`
- Build all Go from root Makefile: `make build-go`
- go.work.sum is gitignored (regenerated on each machine).
