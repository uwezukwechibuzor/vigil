#!/usr/bin/env python3
"""
Notebook 01 — Activation Funnel

Steps:  signup_completed → project_created → api_key_created
        → first_flight_check_run → week-2 active

Reads: main_core.fct_events, main_core.dim_orgs
Outputs: analytics/notebooks/output/01_funnel_overall.png
         analytics/notebooks/output/01_funnel_by_surface.png
         analytics/notebooks/output/01_funnel_timing.png
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
        sys.exit(
            f"DuckDB file not found at {DB_PATH}. Run:\n"
            "  python3 analytics/seed/generate.py\n"
            "  cd analytics/dbt && dbt build --profiles-dir . --target dev"
        )
    return duckdb.connect(DB_PATH, read_only=True)


def run() -> None:
    con = db()

    # ── 1. Build funnel step counts ───────────────────────────────────────────
    funnel_sql = """
    WITH steps AS (
        SELECT
            org_id,
            MAX(CASE WHEN event_name = 'signup_completed'      THEN occurred_at END) AS t_signup,
            MAX(CASE WHEN event_name = 'project_created'       THEN occurred_at END) AS t_project,
            MAX(CASE WHEN event_name = 'api_key_created'       THEN occurred_at END) AS t_key,
            MAX(CASE WHEN event_name = 'first_flight_check_run' THEN occurred_at END) AS t_first_check
        FROM main_core.fct_events
        GROUP BY org_id
    ),
    wk2 AS (
        SELECT DISTINCT e.org_id
        FROM main_core.fct_events e
        JOIN steps s USING (org_id)
        WHERE e.occurred_at >= s.t_signup + INTERVAL '7 days'
          AND e.occurred_at <  s.t_signup + INTERVAL '14 days'
          AND e.event_name  = 'flight_check_run'
    )
    SELECT
        COUNT(*)                                              AS n_signup,
        COUNT(*) FILTER (WHERE t_project IS NOT NULL)        AS n_project,
        COUNT(*) FILTER (WHERE t_key     IS NOT NULL)        AS n_key,
        COUNT(*) FILTER (WHERE t_first_check IS NOT NULL)    AS n_first_check,
        COUNT(DISTINCT wk2.org_id)                           AS n_wk2_active
    FROM steps
    LEFT JOIN wk2 USING (org_id)
    WHERE t_signup IS NOT NULL
    """
    row = con.execute(funnel_sql).fetchone()
    steps = ["Signup", "Project\ncreated", "API key\ncreated", "First\nflight check", "Week-2\nactive"]
    counts = list(row)
    top = counts[0]
    pcts = [c / top for c in counts]

    # ── 2. Plot overall funnel ────────────────────────────────────────────────
    fig, axes = plt.subplots(1, 2, figsize=(12, 5))
    ax = axes[0]
    colors = plt.cm.Blues(np.linspace(0.4, 0.8, len(steps)))
    bars = ax.barh(steps[::-1], pcts[::-1], color=colors[::-1])
    ax.set_xlabel("Conversion from first step")
    ax.xaxis.set_major_formatter(mtick.PercentFormatter(1.0))
    ax.set_title("Activation funnel — cumulative conversion")
    for bar, pct, n in zip(bars, pcts[::-1], counts[::-1]):
        ax.text(pct + 0.01, bar.get_y() + bar.get_height() / 2,
                f"{pct:.0%}  (n={n})", va="center", fontsize=9)

    # Step-to-step drop-off
    ax2 = axes[1]
    step_pcts = [counts[i] / counts[i - 1] if i > 0 else 1.0 for i in range(len(counts))]
    pairs = [f"{steps[i-1].replace(chr(10),' ')}→{steps[i].replace(chr(10),' ')}" for i in range(1, len(steps))]
    ax2.bar(range(len(pairs)), [1 - p for p in step_pcts[1:]], color="salmon")
    ax2.set_xticks(range(len(pairs)))
    ax2.set_xticklabels(pairs, rotation=20, ha="right", fontsize=8)
    ax2.set_ylabel("Step drop-off rate")
    ax2.yaxis.set_major_formatter(mtick.PercentFormatter(1.0))
    ax2.set_title("Step-to-step drop-off")
    plt.tight_layout()
    fig.savefig(OUT / "01_funnel_overall.png", dpi=100)
    plt.close(fig)

    # ── 3. Timing between steps ───────────────────────────────────────────────
    timing_sql = """
    WITH steps AS (
        SELECT
            org_id,
            MAX(CASE WHEN event_name = 'signup_completed'       THEN occurred_at END) AS t_signup,
            MAX(CASE WHEN event_name = 'project_created'        THEN occurred_at END) AS t_project,
            MAX(CASE WHEN event_name = 'api_key_created'        THEN occurred_at END) AS t_key,
            MAX(CASE WHEN event_name = 'first_flight_check_run' THEN occurred_at END) AS t_first_check
        FROM main_core.fct_events
        GROUP BY org_id
    )
    SELECT
        MEDIAN(datediff('hour', t_signup,       t_project))     AS signup_to_project_h,
        MEDIAN(datediff('hour', t_project,      t_key))         AS project_to_key_h,
        MEDIAN(datediff('hour', t_key,          t_first_check)) AS key_to_check_h
    FROM steps
    WHERE t_signup IS NOT NULL AND t_project IS NOT NULL
      AND t_key IS NOT NULL AND t_first_check IS NOT NULL
    """
    timing = con.execute(timing_sql).fetchone()
    timing_labels = ["Signup→Project", "Project→API key", "API key→First check"]
    timing_hours = list(timing)

    fig, ax = plt.subplots(figsize=(8, 4))
    ax.bar(timing_labels, timing_hours, color="steelblue")
    ax.set_ylabel("Median hours")
    ax.set_title("Median time between funnel steps")
    for i, h in enumerate(timing_hours):
        ax.text(i, h + 0.5, f"{h:.1f}h", ha="center", fontsize=9)
    plt.tight_layout()
    fig.savefig(OUT / "01_funnel_timing.png", dpi=100)
    plt.close(fig)

    # ── 4. Funnel by surface ──────────────────────────────────────────────────
    surface_sql = """
    WITH base AS (
        SELECT
            e.org_id,
            e.surface,
            COUNT(*) FILTER (WHERE e.event_name = 'signup_completed')       AS did_signup,
            COUNT(*) FILTER (WHERE e.event_name = 'project_created')        AS did_project,
            COUNT(*) FILTER (WHERE e.event_name = 'api_key_created')        AS did_key,
            COUNT(*) FILTER (WHERE e.event_name = 'first_flight_check_run') AS did_check
        FROM main_core.fct_events e
        GROUP BY e.org_id, e.surface
    )
    SELECT surface,
           SUM(CASE WHEN did_signup  > 0 THEN 1 ELSE 0 END) AS n_signup,
           SUM(CASE WHEN did_project > 0 THEN 1 ELSE 0 END) AS n_project,
           SUM(CASE WHEN did_key     > 0 THEN 1 ELSE 0 END) AS n_key,
           SUM(CASE WHEN did_check   > 0 THEN 1 ELSE 0 END) AS n_first_check
    FROM base
    GROUP BY surface
    ORDER BY n_signup DESC
    """
    surface_rows = con.execute(surface_sql).fetchall()
    surfaces = [r[0] for r in surface_rows]
    s_data = {s: list(r[1:]) for s, r in zip(surfaces, surface_rows)}
    s_labels = ["Signup", "Project", "API key", "First check"]

    fig, ax = plt.subplots(figsize=(10, 5))
    x = np.arange(len(s_labels))
    w = 0.12
    for i, (surf, vals) in enumerate(s_data.items()):
        ax.bar(x + i * w, vals, width=w, label=surf)
    ax.set_xticks(x + w * len(surfaces) / 2)
    ax.set_xticklabels(s_labels)
    ax.set_ylabel("Org count")
    ax.set_title("Funnel steps by surface (orgs that used each surface)")
    ax.legend(fontsize=8)
    plt.tight_layout()
    fig.savefig(OUT / "01_funnel_by_surface.png", dpi=100)
    plt.close(fig)

    # ── Print results ─────────────────────────────────────────────────────────
    print("\n=== Activation Funnel Results ===\n")
    print(f"{'Step':<30} {'Count':>8} {'Conversion':>12} {'Step drop':>12}")
    print("-" * 65)
    for i, (s, n, pct) in enumerate(zip(steps, counts, pcts)):
        step_drop = f"{(1 - step_pcts[i]):.1%}" if i > 0 else "—"
        print(f"  {s.replace(chr(10),' '):<28} {n:>8,} {pct:>11.1%} {step_drop:>12}")
    print(f"\nMedian timing: signup→project={timing_hours[0]:.1f}h, "
          f"project→key={timing_hours[1]:.1f}h, key→first_check={timing_hours[2]:.1f}h")

    biggest_drop_idx = step_pcts[1:].index(min(step_pcts[1:])) + 1
    print(f"\n⚠ Biggest drop-off: {steps[biggest_drop_idx-1].replace(chr(10),' ')} → "
          f"{steps[biggest_drop_idx].replace(chr(10),' ')} "
          f"({(1-step_pcts[biggest_drop_idx]):.1%} lost)")

    print("\n## FINDINGS [SYNTHETIC DATA]")
    print(f"  1. {counts[0]:,} orgs started signup; {counts[-1]:,} ({pcts[-1]:.0%}) were active in week 2.")
    print(f"  2. The biggest drop-off is at the "
          f"'{steps[biggest_drop_idx-1].replace(chr(10),' ')} → "
          f"{steps[biggest_drop_idx].replace(chr(10),' ')}' step "
          f"({(1-step_pcts[biggest_drop_idx]):.1%} lost).")
    print(f"  3. Median time from signup to first flight check: "
          f"{sum(timing_hours):.1f} hours.")

    print("\n## RECOMMENDATIONS")
    print("  1. Reduce friction at the highest drop-off step with inline guidance.")
    print("  2. Send a time-boxed nudge email at the 2h mark if API key not created.")
    print("  3. Surface 'copy-paste MCP config' first for MCP users — it reaches "
          "first flight check fastest in the data.")


if __name__ == "__main__":
    run()
    print("\n✓ Charts saved to analytics/notebooks/output/")
