# Vigil — STRIDE Threat Model

> Version: 1.0 — 2026-11-13  
> Scope: API, FlightCheck, Ingest, Collector, Insights, Alerts services + dashboard

---

## System overview

Vigil is a reliability layer that:
1. Checks onchain infra health (RPC liveness, gas, nonce).
2. Runs pre-transaction flight checks for agents.
3. Traces agent runs via OTLP ingestion.

Trust boundaries:
- **External callers** (SDK/REST/MCP clients, agents) → API over TLS
- **API** → internal services (gRPC/HTTP, within cluster)
- **Internal services** → Postgres, ClickHouse, Redis (private subnet)
- **Dashboard** → API (same boundary as external callers)

---

## STRIDE Analysis

### S — Spoofing

| ID | Threat | Affected component | Mitigation | Status |
|----|--------|--------------------|------------|--------|
| S1 | Caller forges API key to access another tenant's data | API | JWT signed with HS256 (secret in Secrets Manager); per-tenant row-level isolation | Mitigated |
| S2 | Attacker replays captured webhook | Alerts → customer webhook | HMAC-SHA256 `X-Vigil-Signature` on every webhook delivery; nonce in payload | Mitigated |
| S3 | Pod spoofs another service's identity within cluster | All services | Kubernetes NetworkPolicy restricts pod-to-pod traffic to named labels; mTLS planned | Partial |

### T — Tampering

| ID | Threat | Affected component | Mitigation | Status |
|----|--------|--------------------|------------|--------|
| T1 | Attacker modifies span data in transit | Ingest | TLS on all external endpoints; internal traffic on private subnet | Mitigated |
| T2 | SQL injection via span attributes stored to Postgres | Ingest → Postgres | pgx parameterised queries; no string interpolation into SQL | Mitigated |
| T3 | ClickHouse injection via OTLP attribute values | Ingest → ClickHouse | `clickhouse-go` v2 uses typed parameters; no raw string interpolation | Mitigated |
| T4 | Helm chart values injection via CI | Deploy | Values are pinned in `values.yaml`; CD workflow uses `--set` only for image tag | Mitigated |

### R — Repudiation

| ID | Threat | Affected component | Mitigation | Status |
|----|--------|--------------------|------------|--------|
| R1 | Caller denies making a flight-check request | FlightCheck | Structured JSON access logs (slog) with request ID, tenant ID, timestamp to CloudWatch | Mitigated |
| R2 | Internal service denies processing a span | Ingest → ClickHouse | Spans stored with ingestion timestamp; Redis Streams provides durable queue with ACKs | Mitigated |

### I — Information Disclosure

| ID | Threat | Affected component | Mitigation | Status |
|----|--------|--------------------|------------|--------|
| I1 | Error messages leak stack traces | API | `production` mode returns generic error messages; details only in structured logs | Mitigated |
| I2 | Webhook URL leaks in logs | Alerts | Webhook URLs stored in Secrets Manager; never logged | Mitigated |
| I3 | SSRF — attacker supplies internal RPC URL | FlightCheck | `ValidateEndpointURL` blocks all RFC-1918 / loopback / link-local ranges and `localhost` | Mitigated |
| I4 | Telemetry contains PII / wallet keys | Ingest | Tracking plan enforces no PII / no wallet keys; payload not stored; sanitisation in SDK | Mitigated |
| I5 | Postgres credentials in environment leak via `/metrics` | All | `/metrics` requires internal-only network access (NetworkPolicy); env vars never logged | Mitigated |
| I6 | Private key material handled by Vigil | All | Architecture explicitly prohibits private key handling; flight-check only receives tx intent, not signed tx | Mitigated |

### D — Denial of Service

| ID | Threat | Affected component | Mitigation | Status |
|----|--------|--------------------|------------|--------|
| D1 | Flood unauthenticated endpoints | API | IP-based rate limiting (`IPMiddleware`, sliding window in Redis) | Mitigated |
| D2 | Large span payloads exhaust ingest memory | Ingest | Max request body 4 MB enforced in HTTP middleware | Mitigated |
| D3 | Slow RPC upstream causes flight-check goroutine leak | FlightCheck | Per-request `context.WithTimeout` (3 s default) on all outbound RPC calls | Mitigated |
| D4 | ClickHouse slow queries starve ingest | Ingest | Async writer with bounded queue; backpressure alert at depth > 50 k | Mitigated |
| D5 | HPA unable to scale fast enough | All | PDB ensures at least 1 pod available; HPA min=2, scale-up stabilisation=30 s | Mitigated |

### E — Elevation of Privilege

| ID | Threat | Affected component | Mitigation | Status |
|----|--------|--------------------|------------|--------|
| E1 | Container escape via writable root filesystem | All | `readOnlyRootFilesystem: true`; only `/tmp` writable via emptyDir | Mitigated |
| E2 | Container runs as root | All | `runAsNonRoot: true`, `runAsUser: 65534` (nobody) | Mitigated |
| E3 | Pod gains kernel capabilities | All | `capabilities.drop: [ALL]`; no capabilities added | Mitigated |
| E4 | Tenant A reads Tenant B's data | API | All queries include `tenant_id = $1`; enforced at ORM layer | Mitigated |
| E5 | CI token used for lateral movement | CI/CD | GitHub Actions OIDC — short-lived token scoped to this repo; no long-lived secrets in CI | Mitigated |

---

## Residual Risks

| ID | Description | Likelihood | Impact | Accepted by |
|----|-------------|-----------|--------|------------|
| S3 | mTLS not yet enforced between pods | Low | Medium | Engineering — planned for v1.1 |
| I7 | Third-party AI provider (Anthropic) processes span data for root-cause | Low | Low | Disclosed in privacy policy; no PII sent |

---

## Out of Scope

- End-user browser security (phishing, XSS in user-controlled content)
- Cryptographic key management on the caller's side
- Chain-level protocol vulnerabilities (Ethereum consensus, smart contract bugs)

---

## Review cadence

This document is reviewed:
- Before each major release
- After any security incident
- When a new external integration is added
