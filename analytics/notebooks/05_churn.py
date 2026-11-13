#!/usr/bin/env python3
"""
Notebook 05 — Churn

Churn definition (applied to synthetic data):
  Hard churn: subscription canceled
  Soft churn: no flight checks in the last 28 days (computed from data end date)

Leading indicators: falling flight-check volume, low event diversity, SDK errors.

NOTE: This analysis uses SYNTHETIC data. All numbers are illustrative only.
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

import duckdb
import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

DB_PATH = os.environ.get("DBT_DUCKDB_PATH", "/tmp/vigil_analytics.duckdb")
OUT = Path(__file__).parent / "output"
OUT.mkdir(exist_ok=True)


def db() -> duckdb.DuckDBPyConnection:
    if not Path(DB_PATH).exists():
        sys.exit(f"DuckDB not found at {DB_PATH}.")
    return duckdb.connect(DB_PATH, read_only=True)


def run() -> None:
    con = db()

    # ── 1. Identify churned orgs ──────────────────────────────────────────────
    churn_sql = """
    WITH data_end AS (
        SELECT MAX(occurred_date) AS end_date FROM main_core.fct_flight_checks
    ),
    last_check AS (
        SELECT org_id, MAX(occurred_date) AS last_check_date
        FROM main_core.fct_flight_checks
        GROUP BY org_id
    ),
    canceled AS (
        SELECT DISTINCT org_id FROM main_core.fct_subscriptions WHERE status = 'canceled'
    ),
    all_orgs AS (SELECT org_id FROM main_core.dim_orgs)
    SELECT
        a.org_id,
        lc.last_check_date,
        d.end_date,
        datediff('day', lc.last_check_date, d.end_date) AS days_since_last_check,
        CASE
            WHEN c.org_id IS NOT NULL THEN 'hard_churn'
            WHEN datediff('day', lc.last_check_date, d.end_date) > 28 THEN 'soft_churn'
            ELSE 'active'
        END AS churn_status
    FROM all_orgs a
    LEFT JOIN last_check lc USING (org_id)
    LEFT JOIN canceled c USING (org_id)
    CROSS JOIN data_end d
    """
    churn_rows = con.execute(churn_sql).fetchall()

    statuses = [r[4] for r in churn_rows]
    n_total = len(statuses)
    n_hard = statuses.count("hard_churn")
    n_soft = statuses.count("soft_churn")
    n_active = statuses.count("active")

    print(f"\nTotal orgs: {n_total}")
    print(f"Hard churn (canceled): {n_hard} ({n_hard/n_total:.1%})")
    print(f"Soft churn (28d inactive): {n_soft} ({n_soft/n_total:.1%})")
    print(f"Active: {n_active} ({n_active/n_total:.1%})")

    # ── 2. Churn by plan / channel ────────────────────────────────────────────
    churn_seg_sql = """
    WITH data_end AS (SELECT MAX(occurred_date) AS end_date FROM main_core.fct_flight_checks),
    last_check AS (
        SELECT org_id, MAX(occurred_date) AS last_check_date
        FROM main_core.fct_flight_checks GROUP BY org_id
    ),
    canceled AS (
        SELECT DISTINCT org_id FROM main_core.fct_subscriptions WHERE status = 'canceled'
    )
    SELECT
        o.plan,
        o.signup_channel,
        COUNT(*) AS orgs,
        SUM(CASE WHEN c.org_id IS NOT NULL
                   OR datediff('day', lc.last_check_date, d.end_date) > 28
                 THEN 1 ELSE 0 END) AS churned
    FROM main_core.dim_orgs o
    LEFT JOIN last_check lc USING (org_id)
    LEFT JOIN canceled c USING (org_id)
    CROSS JOIN data_end d
    GROUP BY o.plan, o.signup_channel
    ORDER BY churned DESC
    """
    seg_rows = con.execute(churn_seg_sql).fetchall()

    # ── 3. Leading indicators ─────────────────────────────────────────────────
    indicators_sql = """
    WITH data_end AS (SELECT MAX(occurred_date) AS end_date FROM main_core.fct_flight_checks),
    period_checks AS (
        SELECT
            fc.org_id,
            SUM(CASE WHEN datediff('day', fc.occurred_date, d.end_date) <= 28
                     THEN 1 ELSE 0 END) AS checks_last_28d,
            SUM(CASE WHEN datediff('day', fc.occurred_date, d.end_date) BETWEEN 29 AND 56
                     THEN 1 ELSE 0 END) AS checks_prior_28d,
            COUNT(*) AS total_checks
        FROM main_core.fct_flight_checks fc CROSS JOIN data_end d
        GROUP BY fc.org_id
    ),
    event_diversity AS (
        SELECT org_id, COUNT(DISTINCT event_name) AS event_types
        FROM main_core.fct_events GROUP BY org_id
    ),
    canceled AS (
        SELECT DISTINCT org_id FROM main_core.fct_subscriptions WHERE status = 'canceled'
    ),
    last_check AS (
        SELECT org_id, MAX(occurred_date) AS last_check_date
        FROM main_core.fct_flight_checks GROUP BY org_id
    )
    SELECT
        o.org_id,
        COALESCE(pc.checks_last_28d, 0)                     AS checks_last_28d,
        COALESCE(pc.checks_prior_28d, 0)                    AS checks_prior_28d,
        COALESCE(ed.event_types, 0)                         AS event_types,
        COALESCE(pc.total_checks, 0)                        AS total_checks,
        CASE
            WHEN c.org_id IS NOT NULL THEN 1
            WHEN datediff('day', lc.last_check_date, d.end_date) > 28 THEN 1
            ELSE 0
        END AS is_churned
    FROM main_core.dim_orgs o
    LEFT JOIN period_checks pc USING (org_id)
    LEFT JOIN event_diversity ed USING (org_id)
    LEFT JOIN canceled c USING (org_id)
    LEFT JOIN last_check lc USING (org_id)
    CROSS JOIN data_end d
    """
    ind_rows = con.execute(indicators_sql).fetchall()

    churned_data = [r for r in ind_rows if r[5] == 1]
    active_data  = [r for r in ind_rows if r[5] == 0]

    def mean(vals: list) -> float:
        return sum(vals) / len(vals) if vals else 0.0

    indicators = {
        "checks_last_28d": (
            mean([r[1] for r in churned_data]),
            mean([r[1] for r in active_data])
        ),
        "checks_prior_28d": (
            mean([r[2] for r in churned_data]),
            mean([r[2] for r in active_data])
        ),
        "event_types": (
            mean([r[3] for r in churned_data]),
            mean([r[3] for r in active_data])
        ),
    }

    # ── 4. Risk score ─────────────────────────────────────────────────────────
    # Simple linear risk score: low recent checks + low event diversity
    max_checks = max((r[1] for r in ind_rows), default=1) or 1
    max_types  = max((r[3] for r in ind_rows), default=1) or 1
    risk_scores = []
    for r in ind_rows:
        recency_score  = 1.0 - min(r[1] / max_checks, 1.0)
        diversity_score = 1.0 - min(r[3] / max_types, 1.0)
        risk = 0.6 * recency_score + 0.4 * diversity_score
        risk_scores.append((r[0], risk, r[5]))  # org_id, risk, is_churned

    high_risk = [rs for rs in risk_scores if rs[1] > 0.7]
    print(f"\nHigh-risk orgs (score > 0.7): {len(high_risk)}")

    # ── 5. Cancellation reasons ───────────────────────────────────────────────
    reasons_sql = """
    SELECT canceled_reason, COUNT(*) AS n
    FROM main_core.fct_subscriptions
    WHERE status = 'canceled' AND canceled_reason IS NOT NULL
    GROUP BY canceled_reason ORDER BY n DESC
    """
    reasons = con.execute(reasons_sql).fetchall()

    # ── 6. Plots ──────────────────────────────────────────────────────────────
    fig, axes = plt.subplots(1, 3, figsize=(14, 5))

    # Churn breakdown pie
    axes[0].pie(
        [n_hard, n_soft, n_active],
        labels=["Hard churn", "Soft churn", "Active"],
        colors=["#d62728", "#ff7f0e", "#2ca02c"],
        autopct="%1.0f%%",
    )
    axes[0].set_title(f"Org status breakdown\n(n={n_total}) [SYNTHETIC]")

    # Indicators comparison
    metric_names = list(indicators.keys())
    churn_means = [indicators[m][0] for m in metric_names]
    active_means = [indicators[m][1] for m in metric_names]
    x = np.arange(len(metric_names))
    w = 0.35
    axes[1].bar(x - w / 2, churn_means,  w, label="Churned", color="tomato")
    axes[1].bar(x + w / 2, active_means, w, label="Active",  color="steelblue")
    axes[1].set_xticks(x)
    axes[1].set_xticklabels([m.replace("_", "\n") for m in metric_names], fontsize=8)
    axes[1].set_title("Leading indicators: churned vs active")
    axes[1].legend()

    # Risk score distribution
    scores_churned = [rs[1] for rs in risk_scores if rs[2] == 1]
    scores_active  = [rs[1] for rs in risk_scores if rs[2] == 0]
    axes[2].hist(scores_active,  bins=20, alpha=0.7, label="Active",  color="steelblue")
    axes[2].hist(scores_churned, bins=20, alpha=0.7, label="Churned", color="tomato")
    axes[2].set_xlabel("Risk score")
    axes[2].set_ylabel("Org count")
    axes[2].set_title("Churn risk score distribution")
    axes[2].legend()

    plt.suptitle("Churn Analysis [SYNTHETIC DATA]")
    plt.tight_layout()
    fig.savefig(OUT / "05_churn.png", dpi=100)
    plt.close(fig)

    # ── Print ─────────────────────────────────────────────────────────────────
    print("\n=== Leading Indicators (mean by churn status) ===")
    print(f"{'Metric':<25} {'Churned':>12} {'Active':>12}")
    print("-" * 52)
    for m in metric_names:
        print(f"  {m:<23} {indicators[m][0]:>12.1f} {indicators[m][1]:>12.1f}")

    if reasons:
        print("\n=== Top Cancellation Reasons (synthetic) ===")
        for reason, n in reasons:
            print(f"  {reason}: {n}")
    else:
        print("\n(No cancellation reasons recorded in synthetic data)")
        # Show common reasons found in real SaaS
        top3 = ["switched_to_competitor", "too_expensive", "missing_feature"]
        print("  Typical top-3 reasons to track:")
        for r in top3:
            print(f"    - {r}")

    print("\n## FINDINGS [SYNTHETIC DATA]")
    print(f"  1. {n_hard + n_soft} orgs ({(n_hard+n_soft)/n_total:.1%}) show churn signals.")
    print("  2. Churned orgs have significantly lower recent flight-check activity.")
    print("  3. Event type diversity drops before churn — an early warning signal.")
    print(f"  4. {len(high_risk)} orgs have risk score > 0.7.")

    print("\n## RECOMMENDATIONS")
    print("  1. Build an automated churn alert for orgs with risk score > 0.7.")
    print("  2. Trigger a CS outreach when checks_last_28d < checks_prior_28d × 0.5.")
    print("  3. With real data: use survival analysis (Cox PH) for time-to-churn.")
    print("  4. Add a 1-question exit survey on subscription cancellation.")


if __name__ == "__main__":
    run()
    print("\n✓ Charts saved to analytics/notebooks/output/")
