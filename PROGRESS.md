# Progress

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
