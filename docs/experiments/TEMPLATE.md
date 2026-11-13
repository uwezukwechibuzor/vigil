# Experiment Template

> Copy this file to `docs/experiments/EXP-NNN-title.md` and fill in every section
> **before** launching. The decision rule must be written in advance — never decide after
> looking at the data.

---

## Experiment ID
`EXP-NNN`

## Status
`Draft` / `Running` / `Concluded`

## Owner
@username

## Launch date
YYYY-MM-DD

## Planned end date
YYYY-MM-DD  _(or when target sample size reached — whichever comes first)_

---

## Hypothesis

> **If** we [change X], **then** [primary metric] will [increase/decrease] by [MDE or more],
> **because** [mechanism].

Example:
> If we show the MCP copy-paste config as the first onboarding step (instead of "install SDK"),
> then 7-day activation will increase by ≥5pp,
> because MCP requires fewer dependencies and gets users to their first flight check faster.

---

## Variants

| Variant | Description | Assignment |
|---------|-------------|------------|
| Control   | Current onboarding: "Install SDK" first | 50% of orgs |
| Treatment | New onboarding: "Copy MCP config" first | 50% of orgs |

**Randomisation unit:** Organisation (`org_id`), not individual user.
Rationale: orgs share API keys; user-level randomisation would contaminate results.

---

## Primary Metric

| Metric | Definition | Direction |
|--------|------------|-----------|
| 7-day activation | Org runs ≥1 flight check within 7 days of signup | Increase |

---

## Guardrail Metrics

These must NOT degrade by more than the stated threshold, even if the primary metric improves.

| Guardrail | Threshold | Why |
|-----------|-----------|-----|
| Flight check p95 latency | ≤ 10% increase | A slow check experience harms trust |
| SDK error rate | ≤ 2pp increase | Errors in the new flow harm activation |
| Support ticket volume | ≤ 20% increase | New onboarding must not confuse users |
| Conversion rate (free→paid) | ≤ 2pp decrease | Activation quality must hold |

---

## Sample Size & Duration

```
Baseline activation rate:  40%
MDE (minimum detectable effect):  5pp absolute
Significance level (α):  0.05 (two-sided)
Power (1-β):  80%

Required n per arm:  ~1,220 orgs
Total n required:  ~2,440 orgs

At current signup rate of ~50 orgs/week:
Estimated duration:  ~49 weeks
```

> Re-calculate with `analytics.experiments.ab_test.sample_size_calculator`:
> ```python
> from analytics.experiments.ab_test import sample_size_calculator
> print(sample_size_calculator(baseline_rate=0.40, mde=0.05))
> ```

---

## Assignment Mechanism

1. PostHog feature flag: `onboarding_v1`
2. Flag evaluated **server-side** in the API when an org completes signup.
3. Assignment logged as `fct_experiment_exposures` row (exposure event).
4. Flag payload: `{ "variant": "control" | "treatment" }`.
5. No re-randomisation once assigned.

---

## Exposure Event

```json
{
  "event": "experiment_exposure",
  "properties": {
    "experiment_id": "EXP-001",
    "variant": "treatment",
    "org_id": "<uuid>"
  }
}
```

**When to log:** On the first page load / API call where the flag is evaluated.
Do **not** log on every request — only the first exposure per org.

---

## Decision Rule

Written before launch. The decision will be made strictly by this rule — no exceptions.

> **Ship treatment if:**
> - Primary metric improvement ≥ MDE (5pp)
> - p-value < 0.05 (two-sided)
> - 95% CI lower bound > 0
> - **All** guardrail metrics within threshold
> - No SRM (sample ratio mismatch) detected
>
> **Roll back treatment if:**
> - Any guardrail metric exceeds its threshold, regardless of primary metric
>
> **Call it neutral if:**
> - p-value ≥ 0.05 and 95% CI lower bound < 2pp
>
> **Extend experiment if:**
> - Target sample size not reached
> - SRM detected — investigate before extending

---

## Analysis

Run after reaching target sample size:

```python
import duckdb
import numpy as np
from analytics.experiments.ab_test import analyze_experiment

con = duckdb.connect("/tmp/vigil_analytics.duckdb", read_only=True)

rows = con.execute("""
    SELECT variant, activated_7d, pre_period_checks
    FROM main_core.fct_experiment_exposures
    WHERE experiment_id = 'EXP-001'
""").fetchall()

control   = [(r[1], r[2]) for r in rows if r[0] == "control"]
treatment = [(r[1], r[2]) for r in rows if r[0] == "treatment"]

result = analyze_experiment(
    metric_control=[r[0] for r in control],
    metric_treatment=[r[0] for r in treatment],
    is_binary=True,
    covariate_control=[r[1] for r in control],       # CUPED
    covariate_treatment=[r[1] for r in treatment],
)
print(result)
```

---

## Segment Breakdowns (Exploratory)

The following breakdowns are **exploratory only** — do not use them for the ship decision.
They are hypotheses for the next experiment.

- By plan (free / pro / team / enterprise)
- By surface (MCP / SDK / CLI)
- By framework (LangChain / ElizaOS / CrewAI)
- By signup channel

---

## Results

> Fill in after experiment concludes.

| | Control | Treatment | Lift | p-value | 95% CI |
|--|---------|-----------|------|---------|--------|
| 7d activation | — | — | — | — | — |
| p95 latency | — | — | — | — | — |
| SDK error rate | — | — | — | — | — |

**Decision:** [ Ship / Roll back / Neutral / Extend ]

**Rationale:**

---

## Lessons Learned

> Fill in after experiment concludes.

---
*Template version 1.0 — analytics/kpi framework*
