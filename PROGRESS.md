# Progress

## Task 5 — Agent tracing and OTel ingestion ✅

**Completed: 2026-11-13**

### What was built

| Area | Details |
|---|---|
| **Migration 005 update** | Added 11 Vigil agent + cost + linking columns to `spans` CREATE TABLE |
| **Migration 008** | `ALTER TABLE spans ADD COLUMN IF NOT EXISTS` for all 11 new columns (upgrades existing CH installs) |
| **Span struct** | Added `VigilAgentID`, `VigilRunID`, `VigilWallet`, `VigilChainID`, `VigilTxHash`, `VigilFlightCheckID`, `VigilIntent`, `LLMCostUSD`, `GasCostUSD`, `Linked`, `SilentFailure` to `ch.go` |
| **InsertSpans** | Updated column order to match new CREATE TABLE |
| **UnlinkedSpansWithTx** | Queries spans with tx hash but `linked=0` |
| **MarkSpanLinked** | Sets `linked=1`, `silent_failure`, `gas_cost_usd` via ALTER UPDATE |
| **internal/auth** | API key middleware (SHA-256 hash lookup in Postgres; skip if no PG) |
| **internal/otlp/parser** | Converts OTLP ResourceSpans → Span structs; extracts GenAI + Vigil attrs |
| **internal/otlp/scrubber** | Redacts private keys (64-hex), mnemonics (12/24 words), API keys (sk-…); protects addresses |
| **internal/otlp/scrubber_test** | 8 unit tests (no Docker) |
| **internal/otlp/handler** | TracesHandler, MetricsHandler, LogsHandler; back-pressure via XLEN; Prometheus metrics |
| **internal/sampler** | Head-based: always keep errors + tx spans; probabilistic for the rest |
| **internal/writer** | Consumer-group Redis Stream → ClickHouse batch writer; enqueues tx spans for linking |
| **internal/linker** | Fetches onchain receipts; retries 10×; computes gas cost; marks spans linked |
| **internal/cost** | LLMCost (token × price) and GasCostUSD (gas × gas_price × eth_price) |
| **cmd/ingest/main.go** | Full main: optional PG/CH, required Redis, chain clients from env, writer + linker goroutines |
| **scripts/sample_trace** | Standalone script: sends one OTLP protobuf trace to verify end-to-end ingestion |
| **integration/ingest_test.go** | End-to-end test via Redis + ClickHouse testcontainers (Docker-skipped) |
| **integration/linker_test.go** | Silent failure logic tests + Anvil stub (Docker-skipped) |
| **integration/load_test.go** | 2000 spans/s load test against Redis (Docker-skipped) |
| **docs/conventions.md** | Vigil span attribute conventions reference |
| **.env.example** | Added `INGEST_MAX_PAYLOAD_BYTES`, `INGEST_MAX_QUEUE_LEN`, `INGEST_SAMPLE_RATE`, chain RPC examples |

### Decisions

- **Parser uses `fmt.Sprintf` for unknown AnyValue types** — safe fallback; avoids panics on custom attribute types.
- **Scrubber protects addresses** — `0x`+40-hex is a valid wallet address; only 64-hex strings are private keys.
- **Writer uses consumer groups** — multiple replica instances can run without double-processing.
- **Linker queue is non-blocking** — drops spans if full (10k cap) rather than slowing ingestion.
- **MarkSpanLinked uses ALTER UPDATE** — ClickHouse MergeTree doesn't have traditional UPDATE; ALTER TABLE … UPDATE is the idiomatic mutation approach.
- **CH/PG are optional at startup** — the ingest service starts and accepts spans even if ClickHouse is unavailable; the writer will error-log but not crash.

### Open TODOs

- Wire a real `GasPricer` implementation (e.g. CoinGecko API)
- Add per-project sample rate override (store in Postgres, cache in Redis)
- Add Anvil-based linker integration test
- Migrate MarkSpanLinked to use ReplacingMergeTree approach for better performance

## Task 4 — Flight check engine ✅

**Completed: 2026-11-13**

### What was built

| Area | Details |
|---|---|
| **DB migration 007** | `services/internal/db/migrations/postgres/007_policies.up.sql` — `policies` and `denylist` tables with indexes |
| **policies.go** | `GetPolicy(ctx, id)` and `IsAddressDenied(ctx, projectID, address)` Postgres repositories |
| **usage.go** | `IncrementFlightChecks(ctx, orgID)` with upsert |
| **clickhouse InsertFlightCheck** | `FlightCheckRow` struct + single-row insert appended to `ch.go` |
| **Pool.QueryRow** | Thin pass-through on `postgres.Pool` needed by the store layer |
| **check/types.go** | `Request`, `Response`, `Result`, `BalanceChange`, `Verdict`, `Status`, `Deps` types |
| **check/registry.go** | `Check` interface + `Registry` |
| **8 check implementations** | `infra_health`, `simulation` (revert decode + ERC-20 balance changes), `oracle_freshness`, `slippage`, `gas_sanity`, `contract_risk` (unlimited approve + denylist), `policy` (chain/contract/value rules), `address_safety` (pluggable screener interface) |
| **engine/engine.go** | Concurrent runner with `sync.WaitGroup`; 1500 ms budget context; verdict + risk-score aggregation; balance change extraction; summary paragraph |
| **store/store.go** | Saves to ClickHouse + increments Postgres usage counter asynchronously |
| **handler/handler.go** | `POST /v1/flight-check`; Prometheus counters + latency histogram |
| **cmd/flightcheck/main.go** | Full main: connects Postgres/ClickHouse/Redis, dials chain clients from `CHAIN_{id}_RPC_URL` env vars, registers all checks, wires engine + handler |
| **check_test.go** | 20 table-driven unit tests covering all 8 checks (no live infra) |
| **integration/flightcheck_test.go** | 4 Anvil-based integration tests (Docker-skipped in -short) |
| **integration/benchmark_test.go** | p95 latency benchmark against Anvil |
| **docs/flight-check.md** | Full API, check descriptions, config table, example |

### Decisions

- **Checks degrade gracefully** — no live client/DB → warn, never panic; keeps latency under budget even when infra is unavailable.
- **Policy UUID validation** happens before the nil-DB check so invalid UUIDs always return `fail`, not `warn`.
- **`expected_out` hex-first** — parsed as hex then decimal so callers may pass either `"0x64"` or `"100"`.
- **`Pool.QueryRow` exposed** — minimal surface; avoids leaking `pgxpool` to external packages.
- **Screener is an interface** — `AddressScreener` wired at startup; ships as nil; Chainalysis/TRM implementations added in a future task.
- **Store runs async** — `go h.store.Save(...)` so storage latency never blocks the HTTP response.
- **`-short` in test-go** — Docker containers require stable port forwarding; `-short` keeps `make test` dependency-free. `make test-integration` runs the full Docker suite (db + collector + flightcheck).

### Open TODOs

- Wire a real `AddressScreener` implementation (Chainalysis / TRM Labs)
- Add Redis-based policy cache to avoid per-request Postgres round-trips
- Add engine-level unit tests with mock deps (stubbed ethclient)

## Task 3 — Chain telemetry collectors ✅

**Completed: 2026-11-13**

### What was built

| Area | Details |
|---|---|
| **Chain adapter interface** | `services/collector/internal/adapter/adapter.go` — `ChainAdapter` interface with types `Block`, `FeeData`, `Log`, `CallMsg`, `FilterQuery`; EVM-agnostic, Solana-pluggable |
| **EVM adapter** | `evm.go` — go-ethereum `ethclient` supporting HTTP + WebSocket; implements all interface methods including `FeeData` (EIP-1559 base fee, tip, `eth_feeHistory` percentiles) |
| **Probe scheduler** | `internal/probe/probe.go` — shared `Probe` interface, `Config`, jitter loop, `Loop()` runner |
| **RPC probe** | Measures round-trip latency by fetching the head block |
| **HeadLag probe** | Measures wall-clock age of the latest block; exposes `BlockLag()` for unit testing |
| **Reorg detector** | Tracks previous head hash; emits alert if same/lower block number has different hash; exposes `DetectReorg()` for unit testing |
| **GasFee probe** | Samples EIP-1559 `BaseFee`, `MaxFee`, `PriorityFee` in gwei |
| **Oracle probe** | Calls Chainlink AggregatorV3 `latestRoundData` via `eth_call`; decodes ABI-encoded int256 answer; flags staleness; exposes `OracleStale()` for unit testing |
| **Indexer probe** | Queries `_meta { block { number } }` GraphQL endpoint; exposes `IndexerLag()` for unit testing |
| **Endpoint loader** | Reads `endpoints` table from Postgres; reloads via Redis pub/sub on `vigil:endpoints:reload` channel with 2s debounce — no restart required |
| **Shard manager** | Registers a lease key per replica in Redis (`vigil:collector:lease:<id>`); renews every 5s; `Owns()` uses sorted live-replica list for deterministic modulo assignment |
| **Writer** | Batches `ChainHead` + `EndpointProbe` rows to ClickHouse (flush every 5s or 500 rows); publishes failure events to Redis Stream `events`; records `written_total` and `write_lag_seconds` Prometheus metrics |
| **Collector orchestrator** | `internal/collector/collector.go` — acquires shard lease, starts writer, watches endpoint reloads, spawns per-endpoint probe goroutines; self-metrics: `probes_run_total`, `probe_failures_total` |
| **main.go** | Connects to Postgres, ClickHouse, Redis from env vars; runs collector + HTTP health/metrics server |
| **Postgres endpoint query** | Added `ListEnabledEndpoints()` to `services/internal/db/postgres/endpoints.go` |
| **Anvil in docker-compose** | `ghcr.io/foundry-rs/foundry:nightly` as `anvil` service; 2s block time, chain-id 31337, port 8545 |
| **Unit tests** | `BlockLag`, `DetectReorg` (5 cases), `OracleStale` (3 cases), `IndexerLag` (4 cases) — all pass |
| **Integration test** | `TestEndpointDownEvent` — spins up Redis via testcontainers, runs failing RPC probe, verifies "endpoint down" event appears on `events` stream within 10s |

### Decisions

- **Reorg detection** uses a simple hash-compare approach (no deep chain traversal); sufficient for alerting signal without additional RPC calls per cycle.
- **Oracle ABI decoding** is hand-rolled (5-slot, 32-byte each) to avoid a dependency on go-ethereum's ABI package for a single selector.
- **Shard assignment** uses `len(replicas) % index` on sorted lease key IDs, giving zero-coordination consistent assignment. Missing a lease renewal degrades gracefully — the replica claims all endpoints rather than leaving gaps.
- **Writer flush** happens on a 5s ticker or batch-size threshold (500 rows), balancing ClickHouse write overhead vs. data freshness.

---

## Task 2 — Data layer and local infrastructure ✅

**Completed: 2026-11-13**

### Schema overview

**Postgres** (6 migration files, fully reversible):

| Table | Purpose |
|---|---|
| `orgs` | Organisations with plan tier |
| `users` | User accounts |
| `memberships` | Org↔user join table with roles (owner/admin/member/viewer) |
| `projects` | Projects per org with environment (prod/staging/dev) |
| `api_keys` | Key hash + prefix only; plaintext never stored; scoped, expiry-aware |
| `chains` | Supported EVM chains (chain_id, block time, native token) |
| `endpoints` | RPC/indexer/oracle/custom endpoints per project |
| `alert_rules` / `alert_channels` / `incidents` | Alert rules, notification channels, incident tracking |
| `usage_counters` | Daily billing counters (traces, flight checks, API calls) |
| `audit_log` | Immutable audit trail (action, resource, IP, key) |

**ClickHouse** (5 migration files + materialized views):

| Table | TTL | Rollup view |
|---|---|---|
| `chain_heads` | 30 days | `chain_heads_1m` (block max + latency avg per minute) |
| `endpoint_probes` | 30 days | `endpoint_probes_1m` (count + ok-rate + latency per minute) |
| `oracle_updates` | 30 days | — |
| `flight_checks` | 30 days | — |
| `spans` | 30 days | `spans_1m` (count + avg/p95 latency per service+op per minute) |

### What was built

| Area | Details |
|---|---|
| **docker-compose** | Pinned: Postgres 16.9, Redis 7.4.3, ClickHouse 24.12, OTel Collector 0.123.0, Prometheus 3.3.1, Grafana 12.0.1; named volumes, health checks |
| **OTel Collector** | Config in `deploy/otel/config.yaml`: OTLP gRPC+HTTP receivers → batch → Prometheus exporter |
| **Prometheus** | Config in `deploy/prometheus/prometheus.yml` scraping all services + OTel collector |
| **Go module `services/internal/db`** | Shared data-layer module with replace directive wired into go.work |
| **Migration runner** | `migrate.go`: golang-migrate for Postgres (embed FS); custom versioned runner for ClickHouse |
| **Repositories** | `postgres/pg.go` (pool), `orgs.go`, `api_keys.go`, `chains.go`; `clickhouse/ch.go` (batched inserts for chain heads, probes, spans) |
| **Redis helpers** | `redis/cache.go` (JSON TTL cache), `ratelimit.go` (Lua sliding-window), `streams.go` (producer + consumer group + retry/DLQ) |
| **Seed script** | `seed/seed.go` + `cmd/seed/main.go`: demo org, API key (printed once), Ethereum/Base/Arbitrum chains |
| **Makefile** | Added `migrate`, `migrate-down`, `seed`, `test-integration` targets |
| **Integration tests** | testcontainers-go: Postgres migrations, org+API-key CRUD, cache set/get/miss, rate limiter (allow + deny), Streams producer/consumer/ack, dead-letter |

### Decisions

- **golang-migrate** uses embedded SQL files via `embed.FS`; Postgres DSN uses `pgx5://` scheme to match the pgx/v5 driver.
- **ClickHouse migrations** use a custom runner (no golang-migrate driver stable for CH); files sorted by name, up/down directions, semicolon-split statements.
- **Rate limiter** uses a Lua script with INCR-based unique member so concurrent requests at the same millisecond don't collapse into a single sorted-set entry.

---

## Task 1 — Monorepo scaffold, tooling and CI ✅

**Completed: 2026-11-13**

### What was built

| Area | Details |
|---|---|
| **Directory layout** | Full structure per CLAUDE.md: services/{api,collector,flightcheck,ingest,insights,alerts}, packages/{sdk-ts,mcp-server,dashboard}, sdks/{python,go}, cli, analytics, deploy, docs |
| **Go workspace** | go.work at root; each service is its own Go module; shared `services/internal/platform` module; replace directives in service go.mods pointing to local platform module |
| **Platform package** | `services/internal/platform`: `config.go` (12-factor env loader), `logger.go` (slog JSON), `server.go` (chi router, graceful shutdown, /healthz /readyz /metrics, OTel middleware), `middleware.go` (request ID, panic recovery, request logger), `telemetry.go` (OTLP/HTTP tracer provider) |
| **Service mains** | Minimal `main.go` for api, collector, flightcheck, ingest, insights, alerts — all call `platform.StartService` and answer /healthz=200, /readyz=200, /metrics=200 |
| **CLI** | Minimal `cli/cmd/vigil/main.go` |
| **TypeScript** | pnpm workspace + Turborepo; @vigil/sdk-ts, @vigil/mcp-server build via tsc; @vigil/dashboard (Next.js 15) builds; strict tsconfig; ESLint v10 flat config + Prettier |
| **Python SDK** | `sdks/python`: httpx + pydantic v2 client, uv + pyproject.toml, ruff + mypy pass |
| **Tooling** | golangci-lint v2 (`.golangci.yml`), ESLint v10 (`eslint.config.js`), Prettier, ruff, mypy, `.editorconfig` |
| **Makefile** | Targets: `fmt`, `lint`, `test`, `build`, `docker-build`, `up`, `down`, `ci` |
| **Dockerfiles** | Multi-stage distroless/nonroot Dockerfile per service in `deploy/docker/` |
| **docker-compose** | `deploy/docker/docker-compose.yml` with Postgres 16, Redis 7, ClickHouse 24 |
| **GitHub Actions** | `.github/workflows/ci.yml`: go-lint, go-test, go-build, govulncheck, ts-lint, python-lint, docker-build (matrix); actionlint passes |
| **Pre-commit** | `lefthook.yml`: go-fmt, go-lint, ts-fmt, py-fmt, py-lint, conventional-commit-msg |
| **Configs** | `.env.example`, `.gitignore`, LICENSE (Apache-2.0), README, CLAUDE.md |
| **Settings** | `.claude/settings.json` with bypassPermissions for Claude Code sessions |

### Decisions

- **Go workspace + replace directives**: Go 1.26 rejects null pseudo-versions (`v0.0.0-00010101000000-000000000000`) in workspace mode unless the module has a real git hash. Added `replace` directives in each service go.mod pointing to `../internal/platform`. The go.work `use` directives are still present for tooling discovery.
- **`go.work.sum` gitignored**: Regenerated per-machine; commit it only when pinning exact checksums.
- **golangci-lint v2**: Uses `default: standard` preset plus a small extra set. `gofmt`/`goimports` are formatters (separate section) in v2.
- **ESLint v10**: Flat config (`eslint.config.js`) required; `.eslintrc.json` is deprecated in v9+.
- **Dashboard tsconfig**: Uses `moduleResolution: bundler` to match Next.js 15 requirements; doesn't extend the base tsconfig to avoid NodeNext/bundler conflicts.

### Open TODOs

- Lefthook not installed as git hooks yet (run `npx lefthook install` after cloning)
- Docker builds not verified locally (requires Docker daemon) — CI matrix verifies them
- govulncheck not run locally (CI handles it)
- Python test suite is placeholder (no test files yet — added in later tasks)
