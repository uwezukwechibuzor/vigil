# EXP-001 — Onboarding: MCP-config-first vs SDK-install-first

## Status
`Concluded`

## Owner
@uwezukwechibuzor

## Launch date
2026-09-15

## Planned end date
2026-11-10

---

## Hypothesis

> If we show the MCP copy-paste config as the first onboarding step (instead of "install SDK"),
> then 7-day activation will increase by ≥5pp,
> because MCP requires no npm install and lets users reach their first flight check in under 5 minutes.

---

## Variants

| Variant | Description | Assignment |
|---------|-------------|------------|
| Control | Current onboarding: "Install SDK" first | 50% of orgs |
| Treatment | New onboarding: "Copy MCP config" first | 50% of orgs |

**Randomisation unit:** Organisation (`org_id`)

---

## Primary Metric

7-day activation (org runs ≥1 flight check within 7 days of signup)

---

## Guardrails

| Guardrail | Threshold | Result |
|-----------|-----------|--------|
| Flight check p95 latency | ≤ 10% increase | ✅ Unchanged |
| SDK error rate | ≤ 2pp increase | ✅ Within threshold |
| Support ticket volume | ≤ 20% increase | ✅ No increase |

---

## Sample Size

```
Baseline activation rate:  40% (assumed; synthetic data shows ~99% — see note)
MDE:  5pp absolute
n per arm required:  ~1,220
n achieved:  200 control / 200 treatment (synthetic pilot)
```

> ⚠ NOTE: Synthetic seed data has artificially high activation (99%) due to generator logic.
> The experiment below uses a realistically-calibrated subset.

---

## Results (SYNTHETIC DATA — Illustrative Only)

Analysis run with `analytics.experiments.ab_test.analyze_experiment`:

```
=== Experiment Analysis (cuped) ===
  Control:   n=200  mean=0.4050
  Treatment: n=200  mean=0.4700
  Absolute lift:  +0.0650  (+6.50pp)
  Relative lift:  +16.0%
  z=2.197  p=0.0280
  95% CI:  [+0.0072, +0.1228]
  SRM:  p=0.812  OK
  NOTE: CUPED applied (theta=0.0021). Variance reduction achieved.
```

| | Control | Treatment | Lift | p-value | 95% CI |
|--|---------|-----------|------|---------|--------|
| 7d activation | 40.5% | 47.0% | **+6.5pp** | 0.028 | [+0.7pp, +12.3pp] |
| p95 latency (ms) | 142 | 139 | −2% | — | — |
| SDK error rate | 1.2% | 1.4% | +0.2pp | — | — |

**Decision:** ✅ **Ship treatment**

**Rationale:**
- Primary metric: 6.5pp lift exceeds 5pp MDE; p=0.028 < 0.05; CI lower bound = 0.7pp > 0.
- All guardrails pass.
- No SRM detected.
- MCP-first onboarding ships as the new default.

---

## Lessons Learned

1. MCP users reach first flight check ~40 minutes faster than SDK-install users.
2. The "copy-paste config" pattern reduces drop-off at the tool-setup step.
3. Orgs that reach first flight check within 2 hours have significantly higher week-4 retention.

**Next experiment:** Test personalised onboarding (show MCP for Claude/GPT integrations,
SDK for custom agent builders) — expected to add another 3-5pp.

---

> ⚠ All numbers in this writeup are **SYNTHETIC**. Replace with real experiment data
> once Vigil has design partners or beta users.
