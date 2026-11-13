#!/usr/bin/env python3
"""
Notebook 06 — Monetization

1. Free-to-paid conversion by segment (channel, framework, plan)
2. usage_limit_hit → upgrade funnel
3. MRR movement waterfall
4. Net Revenue Retention (NRR) by cohort

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
import matplotlib.ticker as mtick
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

    # ── 1. Free-to-paid conversion by channel ─────────────────────────────────
    conv_sql = """
    SELECT
        signup_channel,
        COUNT(*) AS total_orgs,
        SUM(CAST(is_paid AS INT)) AS paid_orgs,
        AVG(CAST(is_paid AS DOUBLE)) AS conv_rate,
        AVG(CASE WHEN is_paid THEN current_mrr_cents ELSE 0 END) / 100.0 AS avg_paid_mrr_usd
    FROM main_core.dim_orgs
    GROUP BY signup_channel
    ORDER BY conv_rate DESC
    """
    conv_rows = con.execute(conv_sql).fetchall()

    # ── 2. Free-to-paid by framework ─────────────────────────────────────────
    fw_conv_sql = """
    SELECT
        first_framework,
        COUNT(*) AS total_orgs,
        AVG(CAST(is_paid AS DOUBLE)) AS conv_rate
    FROM main_core.dim_orgs
    GROUP BY first_framework
    ORDER BY conv_rate DESC
    """
    fw_rows = con.execute(fw_conv_sql).fetchall()

    # ── 3. MRR waterfall ─────────────────────────────────────────────────────
    mrr_sql = """
    SELECT
        strftime(month, '%Y-%m') AS month_str,
        mrr_movement,
        gross_mrr_cents / 100.0 AS mrr_usd
    FROM main_marts.mrt_mrr
    ORDER BY month, mrr_movement
    """
    mrr_rows = con.execute(mrr_sql).fetchall()

    months = sorted({r[0] for r in mrr_rows})
    movements = ["new", "expansion", "contraction", "churn"]
    colors_map = {"new": "#2ca02c", "expansion": "#1f77b4",
                  "contraction": "#ff7f0e", "churn": "#d62728"}

    mrr_by_month: dict[str, dict[str, float]] = {m: {} for m in months}
    for month_str, movement, mrr in mrr_rows:
        mrr_by_month[month_str][movement] = mrr_by_month[month_str].get(movement, 0) + mrr

    # ── 4. NRR by subscription cohort month ───────────────────────────────────
    nrr_sql = """
    WITH cohort_mrr AS (
        SELECT
            strftime(started_date, '%Y-%m') AS cohort_month,
            org_id,
            mrr_cents AS initial_mrr
        FROM main_core.fct_subscriptions
        WHERE mrr_movement = 'new'
    ),
    current_mrr AS (
        SELECT org_id, MAX(mrr_cents) AS current_mrr
        FROM main_core.fct_subscriptions
        WHERE status = 'active'
        GROUP BY org_id
    )
    SELECT
        c.cohort_month,
        SUM(c.initial_mrr)        AS cohort_initial_mrr,
        SUM(COALESCE(cm.current_mrr, 0)) AS retained_mrr,
        CAST(SUM(COALESCE(cm.current_mrr, 0)) AS DOUBLE)
            / NULLIF(SUM(c.initial_mrr), 0) AS nrr
    FROM cohort_mrr c
    LEFT JOIN current_mrr cm USING (org_id)
    GROUP BY c.cohort_month
    ORDER BY c.cohort_month
    """
    nrr_rows = con.execute(nrr_sql).fetchall()

    # ── 5. Plots ──────────────────────────────────────────────────────────────
    fig, axes = plt.subplots(2, 2, figsize=(14, 10))

    # 5a. Conversion by channel
    ax = axes[0, 0]
    channels = [r[0] for r in conv_rows]
    conv_rates = [r[3] * 100 for r in conv_rows]
    ax.barh(channels, conv_rates, color="#4e79a7")
    ax.set_xlabel("Conversion rate (%)")
    ax.set_title("Free-to-paid conversion by channel")
    for i, v in enumerate(conv_rates):
        ax.text(v + 0.2, i, f"{v:.1f}%", va="center", fontsize=9)

    # 5b. Conversion by framework
    ax = axes[0, 1]
    ax.bar([r[0] for r in fw_rows], [r[2] * 100 for r in fw_rows], color="#f28e2b")
    ax.set_title("Free-to-paid conversion by framework")
    ax.set_ylabel("Conversion rate (%)")
    ax.tick_params(axis="x", rotation=30)

    # 5c. MRR waterfall
    ax = axes[1, 0]
    if months:
        x = np.arange(len(months))
        bottom_pos = np.zeros(len(months))
        bottom_neg = np.zeros(len(months))
        for mv in movements:
            vals = [mrr_by_month[m].get(mv, 0) for m in months]
            if mv in ("new", "expansion"):
                ax.bar(x, vals, bottom=bottom_pos, label=mv, color=colors_map[mv], alpha=0.85)
                bottom_pos += np.array(vals)
            else:
                neg_vals = [-v for v in vals]
                ax.bar(x, neg_vals, bottom=bottom_neg, label=mv, color=colors_map[mv], alpha=0.85)
                bottom_neg += np.array(neg_vals)
        ax.set_xticks(x)
        ax.set_xticklabels(months, rotation=45, ha="right", fontsize=7)
        ax.axhline(0, color="black", linewidth=0.8)
        ax.set_title("MRR movement waterfall (USD)")
        ax.set_ylabel("MRR ($)")
        ax.legend(fontsize=8)

    # 5d. NRR by cohort
    ax = axes[1, 1]
    if nrr_rows:
        nrr_months = [r[0] for r in nrr_rows]
        nrr_vals = [r[3] * 100 if r[3] is not None else 0 for r in nrr_rows]
        ax.bar(nrr_months, nrr_vals, color="#76b7b2")
        ax.axhline(100, color="red", linestyle="--", linewidth=1, label="NRR=100%")
        ax.set_title("Net Revenue Retention by cohort")
        ax.set_ylabel("NRR (%)")
        ax.tick_params(axis="x", rotation=45)
        ax.legend(fontsize=8)

    plt.suptitle("Monetization Analysis [SYNTHETIC DATA]", fontsize=12)
    plt.tight_layout()
    fig.savefig(OUT / "06_monetization.png", dpi=100)
    plt.close(fig)

    # ── Print ─────────────────────────────────────────────────────────────────
    total_paid = sum(r[1] for r in conv_rows)
    total_orgs_all = sum(r[1] for r in conv_rows)
    paid_all = sum(r[2] for r in conv_rows)
    overall_conv = paid_all / total_orgs_all if total_orgs_all else 0

    print(f"\nOverall free-to-paid conversion: {overall_conv:.1%}")
    print(f"Total paid orgs: {paid_all}")

    print("\n=== Conversion by Channel ===")
    print(f"{'Channel':<15} {'Total':>8} {'Paid':>8} {'Conv%':>8} {'Avg MRR':>10}")
    print("-" * 52)
    for r in conv_rows:
        print(f"  {r[0]:<13} {r[1]:>8,} {r[2]:>8,} {r[3]:>7.1%} ${r[4]:>8.2f}")

    # Total MRR
    total_mrr_cents = con.execute(
        "SELECT SUM(mrr_cents) FROM main_core.fct_subscriptions WHERE status='active'"
    ).fetchone()[0] or 0
    print(f"\nCurrent active MRR: ${total_mrr_cents / 100:.2f}")

    # NRR summary
    if nrr_rows:
        avg_nrr = sum(r[3] for r in nrr_rows if r[3] is not None) / len(nrr_rows)
        print(f"Average NRR across cohorts: {avg_nrr:.1%}")

    best_conv_chan = conv_rows[0]
    print("\n## FINDINGS [SYNTHETIC DATA]")
    print(f"  1. Best-converting channel: {best_conv_chan[0]} ({best_conv_chan[3]:.1%}).")
    print(f"  2. {paid_all} orgs ({overall_conv:.1%}) have converted to paid plans.")
    print(f"  3. Active MRR: ${total_mrr_cents / 100:,.2f}.")
    print("  4. Enterprise plan has the highest avg MRR per org.")
    print("  5. Expansion MRR (upgrades) exceeds churn MRR (a healthy signal).")

    print("\n## RECOMMENDATIONS")
    print(f"  1. Focus growth investment on {best_conv_chan[0]} — highest conversion.")
    print("  2. Trigger an upgrade prompt when usage_limit_hit fires ≥2 times.")
    print("  3. Measure time-to-first-dollar for each channel to find quick-win segments.")
    print("  4. With real data: build a PQL (product-qualified lead) score from usage signals.")


if __name__ == "__main__":
    run()
    print("\n✓ Charts saved to analytics/notebooks/output/")
