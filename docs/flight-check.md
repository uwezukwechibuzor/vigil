# Flight Check

Vigil's pre-transaction safety layer. One HTTP call before signing returns `go` / `warn` / `block` with risk signals in under 1500 ms.

> **Note:** Vigil gives risk signals, never guarantees. Never handle or store private keys.

---

## Endpoint

```
POST /v1/flight-check
```

### Request body

| Field             | Type        | Description |
|-------------------|-------------|-------------|
| `chain_id`        | uint64      | EVM chain ID (1 = mainnet, 8453 = Base, 42161 = Arbitrum) |
| `from`            | string      | Sender address |
| `to`              | string      | Destination address or contract |
| `data`            | string      | Hex-encoded calldata (or `"0x"`) |
| `value`           | string      | Hex-encoded wei value (or `"0x0"`) |
| `gas`             | uint64/null | Optional gas limit supplied by the caller |
| `intent`          | string      | Human description of the operation (e.g. "swap 500 USDC to ETH") |
| `policy_id`       | uuid/null   | UUID of a stored policy to enforce |
| `max_slippage_bps`| int         | Maximum acceptable slippage in basis points (0 = disabled) |
| `expected_out`    | string/null | Expected output amount (hex or decimal string) |
| `deadline_s`      | int64/null  | Unix timestamp after which the tx is considered expired |
| `project_id`      | string      | Optional project UUID for policy + usage tracking |

### Response

```json
{
  "check_id":   "uuid",
  "verdict":    "go|warn|block",
  "risk_score": 0,
  "checks": [
    {"name":"infra_health","status":"pass","reason":"RPC healthy, block 1234, latency 12ms","evidence":{}}
  ],
  "balance_changes": [],
  "summary": "All checks passed. Vigil gives risk signals, never guarantees.",
  "latency_ms": 234
}
```

---

## Verdict logic

| Condition | Verdict |
|-----------|---------|
| Any check returns `fail` | `block` |
| Any check returns `warn` (no `fail`) | `warn` |
| All checks return `pass` | `go` |

---

## Risk score (0–100)

Weights per check:

| Check            | fail | warn |
|------------------|------|------|
| infra_health     | 15   | 8    |
| simulation       | 35   | 10   |
| oracle_freshness | 5    | —    |
| slippage         | 10   | 5    |
| gas_sanity       | —    | 5    |
| contract_risk    | 15   | 8    |
| policy           | 15   | —    |
| address_safety   | 10   | 5    |

Scores are summed and capped at 100.

---

## Checks

### 1. `infra_health`

Calls `eth_blockNumber` on the configured RPC endpoint.

- **fail** — no client configured for the requested chain, or the RPC call returns an error, or latency > 500 ms
- **pass** — RPC responded within 500 ms

### 2. `simulation`

Calls `eth_call` to simulate the transaction without submitting it.

- **fail** — the EVM reverts; the revert reason is decoded from the `Error(string)` ABI selector (`0x08c379a0`) when present
- **pass** — simulation completes; ERC-20 `transfer`/`transferFrom` calldata is decoded into `balance_changes`

### 3. `oracle_freshness`

Checks whether `deadline_s` (if supplied) has already passed.

- **fail** — `deadline_s < now()`; includes how many seconds ago it expired
- **pass** — no deadline set, or deadline is in the future

### 4. `slippage`

Checks whether the native value change is within the declared tolerance.

- **fail** — `max_slippage_bps > 0` and `expected_out` is set, and computed slippage exceeds `max_slippage_bps`; also fails if `deadline_s` has expired
- **pass** — within tolerance, or check not applicable

### 5. `gas_sanity`

Calls `eth_gasPrice` via `SuggestGasPrice`.

- **warn** — suggested gas price > 500 Gwei ("gas spike"), or supplied `gas` > 1,000,000 ("high gas usage")
- **pass** — gas price is within normal range

### 6. `contract_risk`

Calls `eth_getCode` to confirm the destination is a contract.

- **warn** — calldata starts with `0x095ea7b3` (ERC-20 `approve`) and the amount is `2^256-1` (unlimited approval); or the address is on the project denylist
- **fail** — address is explicitly denied in the project denylist
- **pass** — no known risks

### 7. `policy`

Loads a named policy from Postgres by `policy_id` and validates three rules:

| Rule | Behaviour |
|------|-----------|
| `allowed_chains` | fail if chain_id not in the list |
| `denied_contracts` | fail if `to` matches a denied address |
| `max_value_wei` | fail if transaction value exceeds the limit |

If no `policy_id` is provided the check passes.

Policies are JSONB documents versioned in the `policies` table. Example:

```json
{
  "allowed_chains": [1, 8453],
  "denied_contracts": ["0xbadcontract"],
  "max_value_wei": "1000000000000000000"
}
```

### 8. `address_safety`

Checks the destination address against:

1. **Project denylist** (`denylist` table in Postgres) — fail if the address is listed
2. **Pluggable screener** — an `AddressScreener` interface can be wired at startup; no implementation is bundled. Risk levels: `none` → pass, `low` → pass, `medium` → warn, `high`/`critical` → fail.

---

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `DATABASE_URL` | — | Postgres DSN; required for policies and denylist |
| `CLICKHOUSE_URL` | — | ClickHouse DSN; required to persist results |
| `REDIS_URL` | — | Redis URL |
| `CHAIN_{id}_RPC_URL` | — | RPC endpoint for a given chain (e.g. `CHAIN_1_RPC_URL`) |
| `FLIGHT_CHECK_BUDGET` | `1500ms` | Maximum wall-clock budget for all checks |
| `PORT` | `8080` | HTTP server port |

---

## Running locally

```sh
CHAIN_1_RPC_URL=https://mainnet.example.com \
FLIGHT_CHECK_BUDGET=1500ms \
go run ./services/flightcheck/cmd/flightcheck
```

```sh
curl -s -X POST localhost:8080/v1/flight-check \
  -H 'Content-Type: application/json' \
  -d '{"chain_id":1,"from":"0xabc...","to":"0xdef...","value":"0x0","data":"0x"}'
```

---

## Persistence

Each result is stored in ClickHouse `flight_checks` (TTL 30 days) and the
org's `usage_counters.flight_checks` counter is incremented in Postgres.
