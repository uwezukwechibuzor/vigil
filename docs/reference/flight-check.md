---
title: "Flight Check Reference"
description: "Complete reference for the pre-transaction flight check API."
---

# Flight Check Reference

The flight check evaluates a transaction intent before signing and returns a risk score and per-check results.

> **Important**: Vigil never receives a signed transaction or private key. The `tx` object contains the unsigned intent only.

## Endpoint

```
POST /v1/flight-check
Authorization: Bearer <api-key>
Content-Type: application/json
```

## Request

```json
{
  "chain": "ethereum",
  "rpc_url": "https://mainnet.infura.io/v3/YOUR_PROJECT_ID",
  "tx": {
    "from": "0x742d35Cc6634C0532925a3b8D4C9b51A",
    "to": "0xA0b86a4D92c8c5D2F87E5b57f7c6B9d3e0c4f8A",
    "value": "0xDE0B6B3A7640000",
    "data": "0x",
    "gas": "0x5208",
    "gas_price": "0x3B9ACA00"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `chain` | string | ✅ | Chain identifier: `ethereum`, `base`, `arbitrum` |
| `rpc_url` | string | ✅ | RPC endpoint URL. Must be `https://` or `http://` to a non-private address. |
| `tx.from` | string | ✅ | Sender address (EIP-55 checksum optional) |
| `tx.to` | string | ✅ | Recipient address |
| `tx.value` | string | | Hex-encoded wei amount |
| `tx.data` | string | | Hex-encoded calldata |
| `tx.gas` | string | | Hex-encoded gas limit |
| `tx.gas_price` | string | | Hex-encoded gas price in wei |

## Response

```json
{
  "verdict": "safe",
  "risk_score": 0.12,
  "checks": {
    "rpc_liveness": { "ok": true, "latency_ms": 42 },
    "gas_price": { "ok": true, "current_gwei": 15.2, "tx_gwei": 10.0 },
    "nonce": { "ok": true, "expected": 7, "tx_nonce": null },
    "block_lag": { "ok": true, "lag_seconds": 3 }
  },
  "request_id": "req_01HXZ..."
}
```

| Field | Type | Description |
|-------|------|-------------|
| `verdict` | string | `safe` / `warning` / `block` |
| `risk_score` | float | 0.0 (no risk) to 1.0 (blocked) |
| `checks` | object | Per-check results |
| `checks.rpc_liveness.ok` | bool | RPC endpoint responded within 3 s |
| `checks.gas_price.ok` | bool | TX gas price ≥ 90% of current network gas price |
| `checks.nonce.ok` | bool | Nonce matches account's current nonce (if provided) |
| `checks.block_lag.ok` | bool | Latest block is < 60 s old |

## Verdict mapping

| Condition | Verdict |
|-----------|---------|
| All checks pass, risk_score < 0.3 | `safe` |
| Any check warns, risk_score 0.3–0.7 | `warning` |
| Risk score > 0.7 or critical check failed | `block` |

## Error responses

| Status | Code | Description |
|--------|------|-------------|
| 400 | `invalid_request` | Missing required field or invalid chain |
| 401 | `unauthorized` | Missing or invalid API key |
| 422 | `invalid_rpc_url` | RPC URL resolves to private/loopback address (SSRF protection) |
| 429 | `rate_limit_exceeded` | Per-IP or per-plan limit exceeded |
| 503 | `upstream_unavailable` | RPC endpoint did not respond |

## Rate limits

| Plan | Requests/min |
|------|-------------|
| Free | 100 |
| Pro | 1 000 |
| Team | 5 000 |
| Enterprise | Unlimited |

## Supported chains

| Chain | Identifier |
|-------|------------|
| Ethereum Mainnet | `ethereum` |
| Base | `base` |
| Arbitrum One | `arbitrum` |
