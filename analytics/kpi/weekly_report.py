#!/usr/bin/env python3
"""
Weekly Product Review Report Generator

Generates a markdown report with the 5 key metrics, WoW change,
the top anomaly with its likely driver, and open questions.

Usage:
    python3 analytics/kpi/weekly_report.py > analytics/reports/weekly_report.md
    python3 analytics/kpi/weekly_report.py --output analytics/reports/weekly_report.md

NOTE: Uses SYNTHETIC data. All numbers are illustrative only.
"""

from __future__ import annotations

import argparse
import os
import sys
from datetime import date, timedelta
from pathlib import Path

import duckdb

DB_PATH = os.environ.get("DBT_DUCKDB_PATH", "/tmp/vigil_analytics.duckdb")


def connect() -> duckdb.DuckDBPyConnection:
    if not Path(DB_PATH).exists():
        sys.exit(f"DuckDB not found at {DB_PATH}.")
    return duckdb.connect(DB_PATH, read_only=True)


def _pct(a: float, b: float) -> str:
    if b == 0:
        return "N/A"
    return f"{(a - b) / b:+.1%}"


def _arrow(a: float, b: float) -> str:
    if a > b * 1.02:
        return "↑"
    if a < b * 0.98:
        return "↓"
    return "→"


def generate_report() -> str:
    con = connect()

    # ── Metric 1: WAU ────────────────────────────────────────────────────────
    wau_sql = """
    WITH w AS (SELECT MAX(event_date) AS latest FROM main_core.fct_events)
    SELECT
        COUNT(DISTINCT CASE WHEN event_date >= latest - INTERVAL '6 days'  THEN org_id END) AS wau_cur,
        COUNT(DISTINCT CASE WHEN event_date BETWEEN latest - INTERVAL '13 days'
                                              AND latest - INTERVAL '7 days' THEN org_id END) AS wau_prev
    FROM main_core.fct_events CROSS JOIN w
    """
    r = con.execute(wau_sql).fetchone()
    wau_cur, wau_prev = float(r[0] or 0), float(r[1] or 0)

    # ── Metric 2: Flight checks per active org ───────────────────────────────
    fc_sql = """
    WITH w AS (SELECT MAX(occurred_date) AS latest FROM main_core.fct_flight_checks)
    SELECT
        COUNT(*) FILTER (WHERE occurred_date >= latest - INTERVAL '6 days')      AS fc_cur,
        COUNT(*) FILTER (WHERE occurred_date BETWEEN latest - INTERVAL '13 days'
                                                AND latest - INTERVAL '7 days')  AS fc_prev,
        COUNT(DISTINCT org_id) FILTER (WHERE occurred_date >= latest - INTERVAL '6 days') AS active_orgs
    FROM main_core.fct_flight_checks CROSS JOIN w
    """
    r = con.execute(fc_sql).fetchone()
    fc_cur, fc_prev, fc_active = float(r[0] or 0), float(r[1] or 0), float(r[2] or 1)
    fc_per_org_cur  = fc_cur  / max(fc_active, 1)
    fc_prev_active = con.execute("""
        WITH w AS (SELECT MAX(occurred_date) AS latest FROM main_core.fct_flight_checks)
        SELECT COUNT(DISTINCT org_id) FROM main_core.fct_flight_checks CROSS JOIN w
        WHERE occurred_date BETWEEN latest - INTERVAL '13 days' AND latest - INTERVAL '7 days'
    """).fetchone()[0] or 1
    fc_per_org_prev = fc_prev / float(fc_prev_active)

    # ── Metric 3: Activation rate ─────────────────────────────────────────────
    act_sql = """
    SELECT AVG(CAST(is_activated AS DOUBLE)) AS activation_rate
    FROM main_core.dim_orgs
    """
    activation_rate = float(con.execute(act_sql).fetchone()[0] or 0)

    # ── Metric 4: Paid conversion ─────────────────────────────────────────────
    conv_sql = "SELECT AVG(CAST(is_paid AS DOUBLE)) FROM main_core.dim_orgs"
    conversion_rate = float(con.execute(conv_sql).fetchone()[0] or 0)

    # ── Metric 5: MRR ─────────────────────────────────────────────────────────
    mrr_sql = """
    SELECT SUM(mrr_cents) FROM main_core.fct_subscriptions WHERE status = 'active'
    """
    mrr_cents = float(con.execute(mrr_sql).fetchone()[0] or 0)

    # ── Top anomaly (contribution analysis) ──────────────────────────────────
    contrib_sql = """
    WITH w AS (SELECT MAX(event_date) AS latest FROM main_core.fct_events)
    SELECT
        surface,
        COUNT(DISTINCT CASE WHEN event_date >= latest - INTERVAL '6 days'  THEN org_id END) AS cur,
        COUNT(DISTINCT CASE WHEN event_date BETWEEN latest - INTERVAL '13 days'
                                              AND latest - INTERVAL '7 days' THEN org_id END) AS prev
    FROM main_core.fct_events CROSS JOIN w
    GROUP BY surface
    ORDER BY (CAST(cur AS DOUBLE) - prev) / GREATEST(prev, 1) ASC
    LIMIT 1
    """
    r = con.execute(contrib_sql).fetchone()
    top_anomaly_surface = r[0] if r else "unknown"
    top_anomaly_cur     = float(r[1] or 0) if r else 0
    top_anomaly_prev    = float(r[2] or 0) if r else 0

    # ── Channel breakdown ─────────────────────────────────────────────────────
    chan_sql = """
    SELECT signup_channel, COUNT(*) AS orgs, AVG(CAST(is_activated AS DOUBLE)) AS act_rate
    FROM main_core.dim_orgs GROUP BY signup_channel ORDER BY orgs DESC LIMIT 3
    """
    top_channels = con.execute(chan_sql).fetchall()

    con.close()

    # ── Build report ──────────────────────────────────────────────────────────
    today = date.today()
    week_start = today - timedelta(days=6)

    lines = [
        f"# Vigil — Weekly Product Review",
        f"",
        f"> **Period:** {week_start} → {today}  ·  *[SYNTHETIC DATA — illustrative only]*",
        f"",
        f"## 5 Key Metrics",
        f"",
        f"| Metric | This Week | WoW | Signal |",
        f"|--------|-----------|-----|--------|",
        f"| WAU (weekly active orgs) | {wau_cur:,.0f} | {_pct(wau_cur, wau_prev)} | {_arrow(wau_cur, wau_prev)} |",
        f"| Flight checks / active org | {fc_per_org_cur:.1f} | {_pct(fc_per_org_cur, fc_per_org_prev)} | {_arrow(fc_per_org_cur, fc_per_org_prev)} |",
        f"| Activation rate | {activation_rate:.1%} | — | — |",
        f"| Free-to-paid conversion | {conversion_rate:.1%} | — | — |",
        f"| Active MRR | ${mrr_cents / 100:,.0f} | — | — |",
        f"",
        f"## Top Anomaly",
        f"",
    ]

    wow_change = _pct(top_anomaly_cur, top_anomaly_prev)
    if top_anomaly_prev > 0 and top_anomaly_cur < top_anomaly_prev * 0.85:
        lines += [
            f"🔴 **WAU for surface `{top_anomaly_surface}`** dropped {wow_change} WoW "
            f"({top_anomaly_cur:.0f} vs {top_anomaly_prev:.0f} orgs).",
            f"",
            f"**Likely driver:** Activity on the `{top_anomaly_surface}` surface fell. "
            f"Check for SDK errors, breaking changes, or deployment issues.",
            f"",
        ]
    else:
        lines += [
            f"✅ No significant anomalies detected this week.",
            f"",
        ]

    lines += [
        f"## Segment Highlights",
        f"",
        f"| Channel | Orgs | Activation Rate |",
        f"|---------|------|----------------|",
    ]
    for ch_name, ch_orgs, ch_act in top_channels:
        lines.append(f"| {ch_name} | {ch_orgs:,} | {ch_act:.1%} |")

    lines += [
        f"",
        f"## Open Questions",
        f"",
        f"1. What drove the change in `{top_anomaly_surface}` WAU?",
        f"2. Are the orgs that activated in the last 7 days using flight checks within 24h?",
        f"3. Is the free-to-paid conversion rate improving or plateauing by cohort?",
        f"4. Which framework is converting to paid fastest?",
        f"",
        f"## Action Items",
        f"",
        f"- [ ] Review `{top_anomaly_surface}` SDK error logs for the week.",
        f"- [ ] Run activation funnel analysis (notebook 01) for latest cohort.",
        f"- [ ] Schedule check-in with orgs at risk of soft churn (risk score > 0.7).",
        f"",
        f"---",
        f"*Generated by `analytics/kpi/weekly_report.py` — data as of {today}.*",
    ]

    return "\n".join(lines)


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate weekly product review report")
    parser.add_argument("--output", metavar="PATH", help="Write report to file (default: stdout)")
    args = parser.parse_args()

    report = generate_report()

    if args.output:
        Path(args.output).parent.mkdir(parents=True, exist_ok=True)
        Path(args.output).write_text(report)
        print(f"✓ Report written to {args.output}")
    else:
        print(report)


if __name__ == "__main__":
    main()
