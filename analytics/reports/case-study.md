# Vigil — Product Analytics Case Study

> **Audience:** Technical product managers, data practitioners, and engineering leads
> evaluating reliability tooling for onchain AI agents.
>
> ⚠ All metrics in this case study are **SYNTHETIC** (generated from seeded data).
> They are illustrative only and will be replaced with real numbers from beta users.

---

## The Problem

AI agents operating onchain face a silent failure mode: they submit transactions to degraded
infrastructure — slow RPCs, stale oracles, misconfigured contracts — and either revert
expensively or silently produce wrong results. There is no "flight check" for agents before
they sign.

Teams building agents on Ethereum, Base, and Arbitrum were:

- Discovering problems **after** transactions landed — often after gas was burned.
- Running custom health scripts per chain, with no shared observability layer.
- Unable to attribute failures to the LLM reasoning layer vs. the onchain execution layer.

---

## The Solution

Vigil adds a reliability layer that any agent, bot, backend, or human can call to:

1. **Check onchain infra health** — RPC liveness, gas oracle freshness, contract state.
2. **Run a pre-transaction flight check** — a risk verdict before signing.
3. **Trace agent runs end to end** — LLM reasoning → onchain result, in one timeline.

It is **agent-first, not agent-only**: the same core serves MCP (Claude Desktop / Cursor),
TypeScript/Python/Go SDKs, a REST API, and a dashboard.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│  Agent / Bot / Human                                    │
│  (LangChain · ElizaOS · CrewAI · Vercel AI · custom)   │
└───────────┬─────────────────┬───────────────────────────┘
            │ MCP / SDK / REST│
            ▼                 ▼
┌───────────────────┐  ┌──────────────────┐
│  Flight Check     │  │  Trace Ingest     │
│  Service (Go)     │  │  (OTLP/HTTP, Go)  │
└─────────┬─────────┘  └────────┬─────────┘
          │                     │
          ▼                     ▼
┌─────────────────────────────────────┐
│  Postgres (config, accounts)        │
│  ClickHouse (telemetry, traces)     │
│  Redis (cache, rate limits)         │
└─────────────────────────────────────┘
          │
          ▼
┌──────────────────────────────────────────────────────────┐
│  Analytics Layer                                          │
│  dbt (staging → core → marts) → DuckDB (local dev)       │
│  PostHog (product events, feature flags, surveys)         │
│  Metabase / Grafana (BI dashboards)                       │
└──────────────────────────────────────────────────────────┘
```

---

## Key Metrics (SYNTHETIC — 90-day pilot cohort)

| Metric | Value |
|--------|-------|
| Total orgs onboarded | 495 |
| 7-day activation rate | ~99%* |
| Avg flight checks / active org / week | ~270 |
| Paid conversion (free → any paid plan) | ~25% |
| Active MRR | ~$4,158 |
| Churn rate (subscription canceled) | ~10% of paid |

> *Synthetic seed data generates near-perfect activation. Real cohorts are expected
> at 35–45% based on comparable onboarding benchmarks.

---

## Activation Funnel

Steps from signup to week-2 active:

| Step | Conversion |
|------|-----------|
| Signup → Project created | ~100% (synthetic) |
| Project created → API key created | ~100% (synthetic) |
| API key created → First flight check | ~99% (synthetic) |
| First flight check → Week-2 active | Varies by plan |

**Key finding:** The biggest real-world drop-off is expected at the
"API key → first flight check" step. MCP users have a shorter path
(copy-paste config vs. npm install) and are hypothesised to convert faster.

---

## Experiment: MCP-First Onboarding (EXP-001)

**Hypothesis:** Showing the MCP copy-paste config as the first onboarding step
(instead of "install SDK") increases 7-day activation by ≥5pp.

**Method:** Org-level random assignment (50/50); exposure logged server-side;
CUPED variance reduction using pre-period flight checks as covariate.

**Result (SYNTHETIC):**

| Arm | n | 7-day activation |
|-----|---|-----------------|
| Control (SDK-first) | 200 | 40.5% |
| Treatment (MCP-first) | 200 | 47.0% |

- Lift: **+6.5pp** (p=0.028; 95% CI [+0.7pp, +12.3pp])
- All guardrails passed.
- **Decision: Shipped.**

**Interpretation:** MCP users reach their first flight check ~40 minutes faster.
The "copy-paste config" pattern eliminates the tool-setup step that caused the
largest drop-off in SDK-based onboarding.

> See full writeup: `docs/experiments/EXP-001-onboarding-mcp-first.md`

---

## Retention Analysis

Logistic regression (week-1 behaviours → week-4 retention):

| Predictor | Coefficient | Direction |
|-----------|------------|-----------|
| `checks_wk1` (flight checks in week 1) | Largest positive | ↑ Retention |
| `events_wk1` (any events) | Positive | ↑ Retention |
| Paid plan | Positive | ↑ Retention |
| Free plan (no upgrade) | Negative | ↓ Retention |

**Aha behaviour:** Running ≥3 flight checks in week 1 is the strongest predictor
of week-4 retention. This is a correlation, not a causal claim — it likely reflects
orgs that already had a working agent (and thus had something to check).

---

## Churn Analysis

**Churn definition:** Subscription canceled (hard churn) or no flight checks in
28 days (soft churn).

**Top cancellation reasons (synthetic):**
1. Missing feature (35%)
2. Price / budget (28%)
3. Switched to another tool (22%)
4. Not using it enough (15%)

**Risk score model:** `0.6 × recency_score + 0.4 × diversity_score`
(higher = higher churn risk). Orgs with risk > 0.7 are flagged for proactive outreach.

---

## KPI Monitor

A daily automated monitor checks five metrics against 7-day rolling baselines:

| Metric | Threshold |
|--------|-----------|
| DAU | >20% drop |
| Flight checks | >20% drop |
| Activation rate | >8pp drop |
| Free-to-paid conversion | >5% drop |
| Active MRR | >5% drop |

When an anomaly is detected, the monitor runs a contribution analysis to identify
the driving segment (surface, channel, framework). It fires an exit-code-1 alert
suitable for integration with PagerDuty, Slack, or CI.

**Validated:** Injecting a 60% artificial drop on the `mcp` surface correctly
triggers the alert and names `mcp` as the driver.

---

## Product Decisions Made from This Analysis

1. **MCP-first onboarding shipped** (EXP-001): +6.5pp activation (synthetic).
2. **"3 checks in week 1" becomes an activation milestone**: Added to onboarding
   checklist and displayed as a progress indicator in the dashboard.
3. **Churn risk score surfaced in dashboard**: Orgs with score >0.7 are flagged
   for the CS team.
4. **LangChain + Base combination**: Highest flight-check volume per org — a
   priority segment for case studies and documentation.

---

## What We Would Measure Next (with Real Users)

1. **Time-to-first-value**: Exact time from signup to first successful flight check,
   broken down by onboarding surface.
2. **Experiment EXP-002**: Personalised onboarding (MCP for Claude/GPT integrations,
   SDK for custom builders) — expected +3–5pp on top of EXP-001.
3. **Qualitative survey data**: Tag responses from `post_first_check` survey
   to identify friction in the "almost didn't make it" step.
4. **NRR by cohort**: Track net revenue retention month-over-month once paid cohorts
   mature (need 3+ months of real subscription data).

---

## Technical Stack (Analytics Layer)

| Component | Technology |
|-----------|-----------|
| Event ingestion | PostHog (JS SDK, server-side Python SDK) |
| Warehouse | ClickHouse (telemetry) + DuckDB (local analytics) |
| Transformation | dbt (staging → core → marts) |
| Experiment framework | PostHog feature flags (org-level assignment) |
| Statistical analysis | Python: scipy, scikit-learn, statsmodels |
| Notebook format | Python scripts (`.py`) — version-controllable, diff-friendly |
| Visualisation | matplotlib (headless, PNG output) |
| BI dashboards | Metabase / Grafana |
| KPI monitoring | Python script, cron-triggered, Slack/PagerDuty webhook |

---

## Code

All analysis code is in `analytics/` within the Vigil monorepo:

```
analytics/
  notebooks/        # 6 analysis notebooks (activation, retention, paths,
  │                 # segmentation, churn, monetization)
  experiments/      # Experiment framework (ab_test.py)
  tests/            # Unit tests for experiment analysis (19 tests)
  kpi/              # Daily monitor + weekly report generator
  dbt/              # dbt models (staging → core → marts)
  seed/             # Synthetic data generator
  surveys/          # PostHog survey definitions + LLM theme tagging
  reports/          # Case study and weekly report output
```

---

*Case study version 1.0 — Vigil Analytics · 2026*
*Contact: uwezukwechibuzor@yahoo.com*
