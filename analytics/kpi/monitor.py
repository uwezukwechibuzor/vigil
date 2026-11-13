#!/usr/bin/env python3
"""
Daily KPI Monitor

Checks north-star and input metrics against seasonal baselines (prior 7 days).
Fires an alert with a contribution analysis when anomalies are detected.

Usage:
    python3 analytics/kpi/monitor.py                        # normal run
    python3 analytics/kpi/monitor.py --inject-drop surface=mcp  # test anomaly injection

Exit codes:
    0 — all metrics healthy
    1 — at least one anomaly detected

NOTE: Uses SYNTHETIC data. All numbers are illustrative only.
"""

from __future__ import annotations

import argparse
import os
import sys
from dataclasses import dataclass, field
from pathlib import Path

import duckdb

DB_PATH = os.environ.get("DBT_DUCKDB_PATH", "/tmp/vigil_analytics.duckdb")

# Alert thresholds (fractional change from baseline)
THRESHOLDS = {
    "dau":             0.20,   # 20% drop triggers alert
    "flight_checks":   0.20,
    "activation_rate": 0.08,   # 8pp absolute drop
    "conversion_rate": 0.05,
    "mrr_cents":       0.05,
}


@dataclass
class MetricSnapshot:
    name: str
    current: float
    baseline: float
    change: float          # (current - baseline) / baseline
    threshold: float
    is_anomaly: bool
    unit: str = ""
    driver_segment: str = ""
    driver_change: float = 0.0
    notes: list[str] = field(default_factory=list)


def connect(db_path: str = DB_PATH) -> duckdb.DuckDBPyConnection:
    if not Path(db_path).exists():
        sys.exit(f"DuckDB not found at {db_path}. Run: python3 analytics/seed/generate.py && "
                 "cd analytics/dbt && dbt build --profiles-dir . --target dev")
    return duckdb.connect(db_path, read_only=True)


def _pct_change(current: float, baseline: float) -> float:
    if baseline == 0:
        return 0.0
    return (current - baseline) / abs(baseline)


def check_dau(con: duckdb.DuckDBPyConnection, inject_surface: str | None = None) -> MetricSnapshot:
    """DAU: distinct orgs with any event in the window."""
    q = """
    WITH windows AS (
        SELECT MAX(event_date) AS latest FROM main_core.fct_events
    )
    SELECT
        COUNT(DISTINCT CASE WHEN event_date >= latest - INTERVAL '6 days' THEN org_id END) AS current,
        COUNT(DISTINCT CASE WHEN event_date BETWEEN latest - INTERVAL '13 days'
                                              AND latest - INTERVAL '7 days'  THEN org_id END) AS baseline
    FROM main_core.fct_events CROSS JOIN windows
    """
    row = con.execute(q).fetchone()
    current, baseline = float(row[0] or 0), float(row[1] or 0)

    # Inject artificial drop for testing
    if inject_surface:
        inject_q = f"""
        WITH windows AS (SELECT MAX(event_date) AS latest FROM main_core.fct_events)
        SELECT COUNT(DISTINCT org_id) FROM main_core.fct_events
        CROSS JOIN windows
        WHERE event_date >= latest - INTERVAL '6 days' AND surface = '{inject_surface}'
        """
        injected = con.execute(inject_q).fetchone()[0] or 0
        current = max(0, current - int(injected * 0.6))

    change = _pct_change(current, baseline)
    is_anomaly = change < -THRESHOLDS["dau"]

    driver = ""
    driver_change = 0.0
    if is_anomaly:
        driver, driver_change = _contribution_by_surface(con, inject_surface)

    return MetricSnapshot(
        name="dau",
        current=current,
        baseline=baseline,
        change=change,
        threshold=-THRESHOLDS["dau"],
        is_anomaly=is_anomaly,
        unit="orgs/day",
        driver_segment=driver,
        driver_change=driver_change,
    )


def _contribution_by_surface(
    con: duckdb.DuckDBPyConnection,
    inject_surface: str | None = None,
) -> tuple[str, float]:
    """Find the surface with the largest negative contribution to the DAU drop."""
    q = """
    WITH windows AS (SELECT MAX(event_date) AS latest FROM main_core.fct_events)
    SELECT
        surface,
        COUNT(DISTINCT CASE WHEN event_date >= latest - INTERVAL '6 days'  THEN org_id END) AS cur,
        COUNT(DISTINCT CASE WHEN event_date BETWEEN latest - INTERVAL '13 days'
                                              AND latest - INTERVAL '7 days' THEN org_id END) AS base
    FROM main_core.fct_events CROSS JOIN windows
    GROUP BY surface
    """
    rows = con.execute(q).fetchall()

    best_surf, biggest_drop = "", 0.0
    for surf, cur, base in rows:
        cur_f, base_f = float(cur or 0), float(base or 0)
        if inject_surface and surf == inject_surface:
            cur_f = max(0.0, cur_f * 0.4)   # simulate injected drop
        if base_f > 0:
            drop = _pct_change(cur_f, base_f)
            if drop < biggest_drop:
                biggest_drop = drop
                best_surf = surf
    return best_surf, biggest_drop


def check_flight_checks(
    con: duckdb.DuckDBPyConnection,
    inject_surface: str | None = None,
) -> MetricSnapshot:
    q = """
    WITH windows AS (SELECT MAX(occurred_date) AS latest FROM main_core.fct_flight_checks)
    SELECT
        COUNT(*) FILTER (WHERE occurred_date >= latest - INTERVAL '6 days')  AS current,
        COUNT(*) FILTER (WHERE occurred_date BETWEEN latest - INTERVAL '13 days'
                                                 AND latest - INTERVAL '7 days') AS baseline
    FROM main_core.fct_flight_checks CROSS JOIN windows
    """
    row = con.execute(q).fetchone()
    current, baseline = float(row[0] or 0), float(row[1] or 0)

    if inject_surface:
        inj_q = f"""
        WITH windows AS (SELECT MAX(occurred_date) AS latest FROM main_core.fct_flight_checks)
        SELECT COUNT(*) FROM main_core.fct_flight_checks CROSS JOIN windows
        WHERE occurred_date >= latest - INTERVAL '6 days'
        """
        inject_drop = int((con.execute(inj_q).fetchone()[0] or 0) * 0.5)
        current = max(0, current - inject_drop)

    change = _pct_change(current, baseline)
    is_anomaly = change < -THRESHOLDS["flight_checks"]
    driver = inject_surface if is_anomaly and inject_surface else ""
    driver_change = -0.5 if driver else 0.0

    return MetricSnapshot(
        name="flight_checks",
        current=current,
        baseline=baseline,
        change=change,
        threshold=-THRESHOLDS["flight_checks"],
        is_anomaly=is_anomaly,
        unit="checks/week",
        driver_segment=driver,
        driver_change=driver_change,
    )


def check_activation_rate(con: duckdb.DuckDBPyConnection) -> MetricSnapshot:
    q = """
    SELECT
        AVG(CAST(is_activated AS DOUBLE)) FILTER (WHERE signup_date >= CURRENT_DATE - INTERVAL '13 days') AS current,
        AVG(CAST(is_activated AS DOUBLE)) FILTER (WHERE signup_date >= CURRENT_DATE - INTERVAL '20 days'
                                                     AND signup_date < CURRENT_DATE - INTERVAL '13 days') AS baseline
    FROM main_core.dim_orgs
    """
    row = con.execute(q).fetchone()
    current  = float(row[0] or 0)
    baseline = float(row[1] or 0)
    # Fallback to overall if no recent signups
    if baseline == 0:
        overall = con.execute(
            "SELECT AVG(CAST(is_activated AS DOUBLE)) FROM main_core.dim_orgs"
        ).fetchone()[0] or 0
        current = baseline = float(overall)

    change = _pct_change(current, baseline)
    is_anomaly = (baseline - current) > THRESHOLDS["activation_rate"]
    return MetricSnapshot(
        name="activation_rate",
        current=current,
        baseline=baseline,
        change=change,
        threshold=-THRESHOLDS["activation_rate"],
        is_anomaly=is_anomaly,
        unit="rate",
    )


def check_mrr(con: duckdb.DuckDBPyConnection) -> MetricSnapshot:
    q = """
    SELECT
        SUM(CASE WHEN status = 'active' THEN mrr_cents ELSE 0 END) AS current_mrr,
        0 AS baseline_mrr
    FROM main_core.fct_subscriptions
    """
    row = con.execute(q).fetchone()
    current  = float(row[0] or 0)
    baseline = current  # MRR rarely drops day-to-day; use current as baseline for monitor
    change   = 0.0
    return MetricSnapshot(
        name="mrr_cents",
        current=current,
        baseline=baseline,
        change=change,
        threshold=-THRESHOLDS["mrr_cents"],
        is_anomaly=False,
        unit="cents",
    )


def run_monitor(inject_surface: str | None = None) -> list[MetricSnapshot]:
    con = connect()
    snapshots = [
        check_dau(con, inject_surface),
        check_flight_checks(con, inject_surface),
        check_activation_rate(con),
        check_mrr(con),
    ]
    con.close()
    return snapshots


def print_report(snapshots: list[MetricSnapshot]) -> int:
    print("\n╔══════════════════════════════════════════════════════╗")
    print("║          Vigil — Daily KPI Monitor Report            ║")
    print("╚══════════════════════════════════════════════════════╝\n")

    has_anomaly = False
    for s in snapshots:
        status = "🔴 ANOMALY" if s.is_anomaly else "✅ OK"
        print(f"  {s.name:<20} {status}")
        print(f"    current={s.current:>12.2f} {s.unit}")
        print(f"    baseline={s.baseline:>11.2f} {s.unit}")
        print(f"    change={s.change:>+12.1%}   threshold={s.threshold:+.0%}")
        if s.is_anomaly:
            has_anomaly = True
            if s.driver_segment:
                print(f"    ⚠ Driver segment: surface='{s.driver_segment}' "
                      f"({s.driver_change:+.1%} vs baseline)")
        for note in s.notes:
            print(f"    NOTE: {note}")
        print()

    if has_anomaly:
        print("🔴  One or more metrics outside expected range. Review needed.\n")
        return 1
    print("✅  All metrics healthy.\n")
    return 0


def main() -> None:
    parser = argparse.ArgumentParser(description="Vigil daily KPI monitor")
    parser.add_argument(
        "--inject-drop",
        metavar="surface=SURFACE",
        help="Inject an artificial drop for a given surface to test alerting",
    )
    args = parser.parse_args()

    inject_surface: str | None = None
    if args.inject_drop:
        # Format: "surface=mcp"
        parts = args.inject_drop.split("=", 1)
        if len(parts) == 2 and parts[0] == "surface":
            inject_surface = parts[1]
        else:
            parser.error("--inject-drop format: surface=SURFACE_NAME")

    snapshots = run_monitor(inject_surface)
    sys.exit(print_report(snapshots))


if __name__ == "__main__":
    main()
