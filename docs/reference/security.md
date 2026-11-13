---
title: "Security & Responsible Use"
description: "Vigil's security model, data practices, and responsible-use policy."
---

# Security & Responsible Use

## What Vigil does and does not do

Vigil is a **reliability layer** for onchain AI agents:

- ✅ Checks RPC liveness, gas prices, block lag
- ✅ Runs pre-transaction risk analysis on **unsigned** transaction intent
- ✅ Traces agent runs (LLM calls → onchain results)
- ✅ Detects anomalies in onchain metrics and fires alerts
- ❌ **Never** handles, stores, or transmits private keys
- ❌ **Never** signs or broadcasts transactions
- ❌ **Never** guarantees transaction safety — Vigil gives **risk signals**, not guarantees

## Privacy and telemetry

**What we collect** (SDK telemetry):
- Anonymous event counts (flight checks run, verdict distribution)
- SDK version, language runtime, OS family
- Anonymous session ID (rotating daily)

**What we never collect**:
- Wallet addresses
- Transaction payloads or calldata
- Private keys or mnemonics
- PII of any kind

**Opt-out**:
```bash
VIGIL_TELEMETRY=0 your-agent
# or
DO_NOT_TRACK=1 your-agent
```

## Data retention

| Data type | Retention |
|-----------|-----------|
| Flight check results | 90 days |
| Agent traces (spans) | 30 days |
| Incidents | 1 year |
| Audit logs | 7 years |

You may request export or deletion of your data at any time via the API:

```
GET  /v1/data/export   — export all your tenant's data as JSON
POST /v1/data/delete   — schedule deletion (confirmed by email)
```

## Security certifications and practices

- **TLS everywhere**: All endpoints require TLS 1.2+; HSTS enforced.
- **Non-root containers**: All pods run as `nobody` (UID 65534) with read-only root filesystem.
- **Secrets management**: AWS Secrets Manager via External Secrets Operator; no secrets in code or CI.
- **Dependency scanning**: `trivy` and `gosec` run on every PR; no HIGH/CRITICAL findings permitted.
- **Image signing**: All production images are signed with `cosign` keyless OIDC.
- **Threat model**: See [docs/threat-model.md](../threat-model.md).

## Responsible use policy

By using Vigil you agree not to:

1. Submit transaction intents for the purpose of reverse-engineering Vigil's risk model.
2. Use Vigil as a sole safety mechanism without independent verification for high-value transactions.
3. Send personal data, private keys, or secrets in any API field.
4. Attempt to identify or enumerate other users' data.

## Reporting vulnerabilities

See [SECURITY.md](../../SECURITY.md) for the responsible disclosure process.

Contact: **uwezukwechibuzor@yahoo.com**
