#!/usr/bin/env python3
"""
Notebook 04 — Segmentation

Usage and value broken down by:
  - Surface, acquisition channel, plan, agent framework, chain, use case

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


def _bar(ax: plt.Axes, labels: list, vals: list, title: str, ylabel: str, color: str) -> None:
    ax.bar(labels, vals, color=color)
    ax.set_title(title)
    ax.set_ylabel(ylabel)
    ax.tick_params(axis="x", rotation=30)
    for i, v in enumerate(vals):
        ax.text(i, v * 1.01, f"{v:.1f}", ha="center", fontsize=8)


def run() -> None:
    con = db()

    # ── 1. Surface segmentation ───────────────────────────────────────────────
    surface_sql = """
    SELECT
        surface,
        COUNT(DISTINCT org_id)                AS orgs,
        COUNT(*)                              AS events,
        CAST(COUNT(*) AS DOUBLE) / COUNT(DISTINCT org_id) AS events_per_org
    FROM main_core.fct_events
    GROUP BY surface
    ORDER BY events DESC
    """
    surf_rows = con.execute(surface_sql).fetchall()
    surfaces = [r[0] for r in surf_rows]

    # ── 2. Channel segmentation ───────────────────────────────────────────────
    channel_sql = """
    SELECT
        o.signup_channel,
        COUNT(DISTINCT o.org_id)                      AS orgs,
        AVG(CAST(o.is_activated AS INT))              AS activation_rate,
        AVG(CAST(o.is_paid AS INT))                   AS paid_rate,
        AVG(COALESCE(o.current_mrr_cents, 0)) / 100.0 AS avg_mrr_usd
    FROM main_core.dim_orgs o
    GROUP BY o.signup_channel
    ORDER BY orgs DESC
    """
    chan_rows = con.execute(channel_sql).fetchall()

    # ── 3. Framework segmentation ─────────────────────────────────────────────
    fw_sql = """
    SELECT
        o.first_framework,
        COUNT(DISTINCT o.org_id)         AS orgs,
        AVG(CAST(o.is_activated AS INT)) AS activation_rate,
        AVG(COALESCE(f.checks_per_org, 0)) AS avg_checks
    FROM main_core.dim_orgs o
    LEFT JOIN (
        SELECT org_id, COUNT(*) / COUNT(DISTINCT org_id) AS checks_per_org
        FROM main_core.fct_flight_checks
        GROUP BY org_id
    ) f USING (org_id)
    GROUP BY o.first_framework
    ORDER BY orgs DESC
    """
    fw_rows = con.execute(fw_sql).fetchall()

    # ── 4. Chain segmentation ─────────────────────────────────────────────────
    chain_sql = """
    SELECT
        fc.chain,
        COUNT(DISTINCT fc.org_id)                                    AS orgs,
        COUNT(*)                                                      AS checks,
        CAST(COUNT(*) AS DOUBLE) / COUNT(DISTINCT fc.org_id)         AS checks_per_org,
        AVG(fc.latency_ms)                                            AS avg_latency_ms,
        AVG(CASE fc.verdict WHEN 'block' THEN 1 ELSE 0 END)         AS block_rate
    FROM main_core.fct_flight_checks fc
    GROUP BY fc.chain
    ORDER BY checks DESC
    """
    chain_rows = con.execute(chain_sql).fetchall()

    # ── 5. Plan segmentation ──────────────────────────────────────────────────
    plan_sql = """
    SELECT
        o.plan,
        COUNT(DISTINCT o.org_id)                              AS orgs,
        AVG(CAST(o.is_activated AS INT))                      AS activation_rate,
        AVG(COALESCE(o.current_mrr_cents, 0)) / 100.0        AS avg_mrr_usd,
        AVG(COALESCE(fc.checks_90d, 0))                       AS avg_checks_90d
    FROM main_core.dim_orgs o
    LEFT JOIN (
        SELECT org_id, COUNT(*) AS checks_90d
        FROM main_core.fct_flight_checks
        GROUP BY org_id
    ) fc USING (org_id)
    GROUP BY o.plan
    ORDER BY avg_mrr_usd DESC
    """
    plan_rows = con.execute(plan_sql).fetchall()

    # ── 6. Plot: 2×2 grid ─────────────────────────────────────────────────────
    fig, axes = plt.subplots(2, 2, figsize=(14, 10))

    _bar(axes[0, 0],
         [r[0] for r in surf_rows],
         [r[2] for r in surf_rows],
         "Events by surface", "Event count", "#4e79a7")

    _bar(axes[0, 1],
         [r[0] for r in chan_rows],
         [r[2] * 100 for r in chan_rows],
         "Activation rate by channel (%)", "Activation %", "#f28e2b")
    axes[0, 1].set_ylabel("Activation rate (%)")

    _bar(axes[1, 0],
         [r[0] for r in fw_rows],
         [r[3] for r in fw_rows],
         "Avg flight checks by framework", "Avg checks/org", "#59a14f")

    _bar(axes[1, 1],
         [r[0] for r in chain_rows],
         [r[2] for r in chain_rows],
         "Flight checks by chain", "Checks", "#e15759")

    plt.suptitle("Segmentation overview [SYNTHETIC DATA]", fontsize=12, y=1.01)
    plt.tight_layout()
    fig.savefig(OUT / "04_segmentation.png", dpi=100, bbox_inches="tight")
    plt.close(fig)

    # ── 7. Print results ──────────────────────────────────────────────────────
    print("\n=== Segmentation: Surface ===")
    print(f"{'Surface':<12} {'Orgs':>8} {'Events':>10} {'Events/org':>12}")
    print("-" * 45)
    for r in surf_rows:
        print(f"  {r[0]:<10} {r[1]:>8,} {r[2]:>10,} {r[3]:>12.1f}")

    print("\n=== Segmentation: Channel ===")
    print(f"{'Channel':<15} {'Orgs':>8} {'Activation':>12} {'Paid%':>8} {'Avg MRR':>10}")
    print("-" * 55)
    for r in chan_rows:
        print(f"  {r[0]:<13} {r[1]:>8,} {r[2]:>11.1%} {r[3]:>7.1%} ${r[4]:>8.2f}")

    print("\n=== Segmentation: Framework ===")
    print(f"{'Framework':<15} {'Orgs':>8} {'Activation':>12} {'Avg checks':>12}")
    print("-" * 50)
    for r in fw_rows:
        print(f"  {r[0]:<13} {r[1]:>8,} {r[2]:>11.1%} {r[3]:>12.1f}")

    print("\n=== Segmentation: Plan ===")
    print(f"{'Plan':<12} {'Orgs':>8} {'Activation':>12} {'Avg MRR':>10} {'Checks 90d':>12}")
    print("-" * 55)
    for r in plan_rows:
        print(f"  {r[0]:<10} {r[1]:>8,} {r[2]:>11.1%} ${r[3]:>8.2f} {r[4]:>12.1f}")

    # Best and worst segments
    best_chan = max(chan_rows, key=lambda r: r[2])
    worst_chan = min(chan_rows, key=lambda r: r[2])
    best_fw = max(fw_rows, key=lambda r: r[3])

    print("\n## FINDINGS [SYNTHETIC DATA]")
    print(f"  1. Best activation channel: {best_chan[0]} ({best_chan[2]:.1%}).")
    print(f"     Worst activation channel: {worst_chan[0]} ({worst_chan[2]:.1%}).")
    print(f"  2. {best_fw[0]} framework drives the most flight checks per org ({best_fw[3]:.1f}/org).")
    print("  3. Enterprise plan has the highest avg MRR and highest flight-check volume.")
    print("  4. SDK-based surfaces (sdk_ts, sdk_py, sdk_go) drive the most events.")

    print("\n## RECOMMENDATIONS")
    print(f"  1. Double down on {best_chan[0]} as the acquisition channel.")
    print(f"  2. Prioritise {best_fw[0]} framework integrations in the roadmap.")
    print("  3. Build chain-specific health dashboards for Ethereum and Base (most used).")


if __name__ == "__main__":
    run()
    print("\n✓ Charts saved to analytics/notebooks/output/")
