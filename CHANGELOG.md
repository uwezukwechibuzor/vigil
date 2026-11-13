# Changelog

All notable changes to Vigil follow [Conventional Commits](https://www.conventionalcommits.org/) and [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- Production hardening: Helm chart with HPA, PDB, NetworkPolicy, non-root/read-only containers
- Terraform modules for AWS EKS, RDS Postgres, ElastiCache Redis, S3 backups
- CI/CD: build + cosign image signing, staging auto-deploy, manual production approval, automatic rollback
- External Secrets Operator integration with AWS Secrets Manager
- TLS/HSTS/CSP/X-Frame-Options/Referrer-Policy security headers on all HTTP endpoints
- Per-IP rate limiting on unauthenticated routes
- SSRF protection for user-supplied RPC URLs (`ValidateEndpointURL`)
- Prometheus alert rules (SLOs, service health, ingest backpressure)
- Grafana dashboard (SLO gauges, request rate, error rate, ingest throughput)
- k6 load tests: flight checks at 200 rps, ingest at 5 000 spans/s
- Chaos tests: Redis kill, collector kill, slow ClickHouse scenarios
- STRIDE threat model (`docs/threat-model.md`)
- Runbooks for all alert rules
- `services/insights`: MAD + EWMA anomaly detection, AI root-cause via Anthropic
- `services/alerts`: rule evaluation, Slack/webhook/multi-channel dispatch
- `packages/dashboard`: Next.js 15 App Router, Tailwind, shadcn/ui, Playwright E2E, Lighthouse targets
- Python SDK, TypeScript SDK, MCP server
- OpenAPI-generated Go server stubs for the API
- ClickHouse telemetry storage for spans and traces
- Redis Streams ingest queue with backpressure

## [0.1.0] — 2026-10-01

### Added
- Initial repo scaffold
- Go workspace with per-service modules
- `services/api`: health, auth, flight-check, traces, incidents endpoints
- `services/collector`: OTLP span consumer
- `services/flightcheck`: RPC liveness, gas, nonce checks for Ethereum/Base/Arbitrum
- `services/ingest`: OTLP/HTTP receiver → Redis Streams
- `services/internal/platform`: shared slog, otel, prometheus, redis, postgres helpers
- `services/internal/db`: Postgres migrations (001-009), ClickHouse migrations
- `cli/`: `vigil` CLI with `flight-check` and `traces` subcommands
- `sdks/go`: Go client SDK
- `analytics/tracking-plan`: PostHog event schema
- Docker Compose for local development
