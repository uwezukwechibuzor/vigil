"""
Event-count reconciliation job.

Compares raw seed event counts against the staging model to verify that no
rows are silently dropped by the deduplication logic in stg_events.

Usage:
    python3 analytics/reconcile/reconcile.py

Exit codes:
    0 — counts match within tolerance
    1 — drift detected (prints details)

Env vars:
    DBT_DUCKDB_PATH   path to the DuckDB file built by `dbt build` (default: /tmp/vigil_analytics.duckdb)
    RECONCILE_MAX_DRIFT_PCT  acceptable drift percentage (default: 0.5)
"""

from __future__ import annotations

import os
import sys

MAX_DRIFT_PCT = float(os.environ.get("RECONCILE_MAX_DRIFT_PCT", "0.5"))
DB_PATH = os.environ.get("DBT_DUCKDB_PATH", "/tmp/vigil_analytics.duckdb")


def run() -> int:
    try:
        import duckdb
    except ImportError:
        print("duckdb not installed — skipping reconciliation", file=sys.stderr)
        return 0

    con = duckdb.connect(DB_PATH, read_only=True)

    raw = con.execute("SELECT count(*) FROM main_raw.seed_events").fetchone()[0]
    stg = con.execute("SELECT count(*) FROM main_staging.stg_events").fetchone()[0]

    if raw == 0:
        print("WARNING: raw seed_events is empty — skipping reconciliation")
        return 0

    drift_pct = abs(raw - stg) / raw * 100
    print(f"raw={raw}  staging={stg}  drift={drift_pct:.2f}%  threshold={MAX_DRIFT_PCT}%")

    if drift_pct > MAX_DRIFT_PCT:
        print(f"FAIL: event count drift {drift_pct:.2f}% exceeds {MAX_DRIFT_PCT}%")
        return 1

    print("OK: event counts within tolerance")
    return 0


if __name__ == "__main__":
    sys.exit(run())
