# Product Analytics

Vigil's analytics stack gives full visibility into product health, feature adoption,
and revenue — without storing any PII or sensitive onchain data.

## Architecture overview

```
SDK/CLI/MCP  ──► PostHog (capture)
Dashboard    ──► PostHog (web)
API server   ──► PostHog (server-side)

PostHog warehouse export
        │
        ▼
  ClickHouse (raw events)
        │
        ▼
  dbt (staging → core → marts)
        │
        ▼
  Grafana / Metabase dashboards
```

## Tracking plan

The single source of truth is `analytics/tracking-plan/tracking-plan.yaml`.
It defines all 28 events across five lifecycle stages:

| Stage | Events |
|---|---|
| Acquisition | `api_accessed`, `docs_viewed`, `sdk_installed` |
| Activation | `signup_completed`, `project_created`, `api_key_created`, `first_flight_check`, `first_run_traced`, `integration_connected`, `onboarding_completed` |
| Engagement | `flight_check_run`, `run_traced`, `dashboard_viewed`, `policy_created`, `alert_rule_created`, `mcp_tool_called`, `cli_command_run`, `api_called` |
| Monetization | `trial_started`, `subscription_started`, `subscription_upgraded`, `subscription_downgraded`, `subscription_canceled`, `usage_limit_hit`, `payment_failed` |
| Health | `error_occurred`, `rate_limit_hit`, `latency_spike_detected` |

### Generating typed helpers

```bash
python3 analytics/tracking-plan/codegen.py
```

Outputs to `analytics/tracking-plan/generated/`:
- `tracking.ts` — TypeScript interfaces + `track*()` functions
- `tracking.py` — Python dataclasses + `track_*()` functions
- `tracking.go` — Go structs + `Track*()` functions

### CI enforcement

```bash
python3 analytics/tracking-plan/codegen.py --validate /path/to/source/root
```

Exits 1 if any `.track("event_name")` call references an undeclared event.

## Privacy rules

| Allowed | Blocked |
|---|---|
| `org_id` (UUID only) | email addresses |
| `plan`, `surface`, `verdict` | wallet addresses (`0x` + 40 hex) |
| `latency_ms`, `chain_id` | 32-byte hex keys |
| `sdk_version`, `os_family` | `private_key`, `calldata`, `mnemonic`, `from`, `to` |

The sanitizer runs in all four surfaces (TS SDK, Python SDK, Go SDK, API server).
Any value matching `^0x[0-9a-fA-F]{40}$` or `^0x[0-9a-fA-F]{64}$` is dropped,
regardless of key name.

## Opt-out

Set either env var before process start:

```bash
VIGIL_TELEMETRY=0   # SDK/CLI opt-out
DO_NOT_TRACK=1      # universal standard
```

Server-side events (from the API service) are not sent if `POSTHOG_API_KEY` is unset.

## dbt project

Located at `analytics/dbt/`. Uses **dbt-duckdb** for local dev and CI,
**dbt-clickhouse** for production.

```bash
# Dev (DuckDB — no Docker required)
cd analytics/dbt
dbt build --profiles-dir . --target dev

# Production (ClickHouse)
dbt build --profiles-dir . --target prod
```

### Model layers

| Layer | Materialization | Description |
|---|---|---|
| `staging` | view | Deduplication, type casting, enum validation |
| `core` | table | Business entities (`dim_orgs`, `fct_events`, `fct_flight_checks`, `fct_subscriptions`) |
| `marts` | table | Metric surfaces for BI tools |

### Mart models

| Model | Grain | Key metrics |
|---|---|---|
| `mrt_daily_active_orgs` | calendar day | DAU, WAU, MAU, stickiness |
| `mrt_weekly_retention` | cohort × week | retention rate, retained/churned counts |
| `mrt_activation` | signup week | activation rate, median days-to-value |
| `mrt_mrr` | month × movement type | new MRR, expansion, contraction, churn, net MRR |
| `mrt_feature_adoption` | event name × week | weekly active orgs per feature |

### Activation macro

An org is "activated" if its first flight check occurs within
`var('activation_days', 7)` days of signup:

```sql
{{ is_activated('o.created_at', 'fc.first_check_at') }}
```

## Seed data

```bash
# Generate 90 days × 500 orgs of synthetic data
python3 analytics/seed/generate.py --days 90 --orgs 500

# Seeds land in analytics/dbt/seeds/ and are loaded by `dbt seed`
```

## Data quality

- **62 schema tests** (unique, not_null, accepted_values, relationships) run on every `dbt build`
- **Event volume assertion** (`tests/assert_daily_event_volume.sql`): fails if any day has 0 DAU in the last 30 days
- **Reconciliation job** (`analytics/reconcile/reconcile.py`): compares raw seed counts to staging, fails if drift > 0.5%

```bash
python3 analytics/reconcile/reconcile.py
```

## BI dashboard

A Grafana dashboard JSON is at `analytics/bi/grafana_dashboard.json`.
Import it via Grafana → Dashboards → Import. It requires the
**ClickHouse datasource plugin** (grafana-clickhouse-datasource v4+).

Panels:
- DAU / WAU / MAU timeseries
- Stickiness (DAU/MAU ratio)
- Net MRR
- MRR movement waterfall
- Weekly activation rate
- Cohort retention heatmap
- Flight check volume by verdict
- Flight check latency (p50/p95/p99)
