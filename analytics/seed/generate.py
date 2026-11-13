#!/usr/bin/env python3
"""
Synthetic data generator for 90 days × 500 orgs.
Writes CSV files to analytics/dbt/seeds/

Usage:
    python3 analytics/seed/generate.py
    python3 analytics/seed/generate.py --days 30 --orgs 100  (faster CI run)
"""
import argparse
import csv
import os
import random
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Optional

SEED_DIR = Path(__file__).parent.parent / "dbt" / "seeds"

PLANS = ["free", "free", "free", "pro", "pro", "team", "enterprise"]
CHANNELS = ["organic", "organic", "referral", "product_hunt", "twitter", "github", "paid_search"]
FRAMEWORKS = ["langchain", "elizaos", "crewai", "vercel_ai", "none", "none"]
CHAINS = ["ethereum", "base", "arbitrum", "ethereum", "base"]
SURFACES = ["sdk_ts", "sdk_py", "sdk_go", "mcp", "cli", "web"]
VERDICTS = ["safe", "safe", "safe", "warning", "block"]
SIGNUP_METHODS = ["email", "github", "google"]
ENVIRONMENTS = ["production", "staging", "development"]

UTC = timezone.utc


def uid() -> str:
    return str(uuid.uuid4())


def now_offset(days: float, hour: int = 10) -> datetime:
    base = datetime(2026, 8, 15, hour, 0, 0, tzinfo=UTC)
    return base + timedelta(days=days)


@dataclass
class Org:
    org_id: str = field(default_factory=uid)
    name: str = ""
    plan: str = "free"
    created_at: datetime = field(default_factory=datetime.utcnow)
    signup_channel: str = "organic"
    first_framework: str = "none"
    first_chain: str = "ethereum"
    is_internal: bool = False
    deleted_at: Optional[str] = None
    # runtime state (not in CSV)
    projects: list = field(default_factory=list)
    paid_at: Optional[datetime] = None
    paid_plan: str = "free"
    paid_mrr: int = 0
    activated: bool = False
    activation_day: Optional[int] = None


MRR_BY_PLAN = {"pro": 4900, "team": 19900, "enterprise": 99900}


def generate(days: int, n_orgs: int, seed: int = 42) -> None:
    random.seed(seed)
    SEED_DIR.mkdir(parents=True, exist_ok=True)

    # Build org cohorts: orgs sign up over the 90-day window
    orgs: list[Org] = []
    for i in range(n_orgs):
        signup_day = random.uniform(0, days * 0.9)
        plan = random.choice(PLANS)
        framework = random.choice(FRAMEWORKS)
        chain = random.choice(CHAINS)
        org = Org(
            name=f"org-{i:04d}",
            plan=plan,
            created_at=now_offset(signup_day),
            signup_channel=random.choice(CHANNELS),
            first_framework=framework,
            first_chain=chain,
            is_internal=(i < 5),  # first 5 orgs are internal
        )
        # 25% of orgs eventually upgrade to paid
        if random.random() < 0.25 and plan == "free":
            upgrade_day = signup_day + random.uniform(2, 20)
            if upgrade_day < days:
                org.paid_at = now_offset(upgrade_day)
                org.paid_plan = random.choice(["pro", "pro", "team"])
                org.paid_mrr = MRR_BY_PLAN[org.paid_plan]
        orgs.append(org)

    events: list[dict] = []
    flight_checks: list[dict] = []
    projects_rows: list[dict] = []
    sub_rows: list[dict] = []

    def add_event(org: Org, event_name: str, occurred_at: datetime,
                  surface: str = "backend", props: dict = None) -> None:
        events.append({
            "event_id": uid(),
            "org_id": org.org_id,
            "project_id": org.projects[0] if org.projects else "",
            "user_id": org.org_id,  # simplified 1:1 user/org for seed
            "anonymous_id": uid(),
            "session_id": uid() if surface == "web" else "",
            "occurred_at": occurred_at.isoformat(),
            "received_at": (occurred_at + timedelta(seconds=random.uniform(0.1, 2.0))).isoformat(),
            "surface": surface,
            "event_name": event_name,
            "app_version": "0.1.0",
            "plan": org.plan,
            "environment": random.choice(["production", "production", "staging"]),
            "framework": org.first_framework,
            "chain_id": org.first_chain if "flight_check" in event_name else "",
            "properties": str(props or {}),
        })

    # Pass 1: signups, projects, keys
    for org in orgs:
        if org.is_internal:
            continue
        t = org.created_at

        add_event(org, "signup_completed", t, "web",
                  {"signup_method": random.choice(SIGNUP_METHODS)})

        # Create a project 1–3 hours after signup
        project_id = uid()
        org.projects.append(project_id)
        pt = t + timedelta(hours=random.uniform(0.5, 3))
        projects_rows.append({
            "project_id": project_id,
            "org_id": org.org_id,
            "name": f"project-{org.name}",
            "created_at": pt.isoformat(),
            "deleted_at": "",
        })
        add_event(org, "project_created", pt, "web", {"is_first": True, "project_name": f"project-{org.name}"})
        add_event(org, "api_key_created", pt + timedelta(minutes=5), "web",
                  {"is_first": True, "key_scope_count": 2})

        if random.random() < 0.6:
            add_event(org, "endpoint_added",
                      pt + timedelta(minutes=10), "web",
                      {"endpoint_kind": "rpc", "chain_id": org.first_chain, "is_first": True})

    # Pass 2: flight checks (ongoing engagement)
    for org in orgs:
        if org.is_internal or not org.projects:
            continue
        signup_ts = org.created_at
        days_active = (now_offset(days) - signup_ts).days
        if days_active < 1:
            continue

        # Avg checks per active day varies by plan
        avg_per_day = {"free": 3, "pro": 20, "team": 60, "enterprise": 200}.get(org.plan, 3)
        surface = random.choice(["sdk_ts", "sdk_py", "sdk_go", "mcp"])

        for day_offset in range(min(days_active, days)):
            # engagement drops off for free tier
            if org.plan == "free" and random.random() > 0.4:
                continue
            n_checks = max(0, int(random.gauss(avg_per_day, avg_per_day * 0.3)))
            if n_checks == 0:
                continue
            if not org.activated and n_checks > 0:
                org.activated = True
                org.activation_day = day_offset
                add_event(org, "first_flight_check_run",
                          signup_ts + timedelta(days=day_offset),
                          "backend",
                          {"days_since_signup": day_offset,
                           "chain_id": org.first_chain,
                           "verdict": random.choice(VERDICTS)})

            for _ in range(n_checks):
                ts = signup_ts + timedelta(
                    days=day_offset,
                    hours=random.uniform(0, 23),
                    minutes=random.uniform(0, 59),
                )
                verdict = random.choice(VERDICTS)
                latency_ms = int(random.gauss(80, 30))
                latency_ms = max(5, latency_ms)
                checks_failed = 0 if verdict == "safe" else random.randint(1, 3)
                risk = 0.05 if verdict == "safe" else (0.5 if verdict == "warning" else 0.9)
                risk += random.uniform(-0.05, 0.05)
                risk = min(1.0, max(0.0, risk))

                flight_checks.append({
                    "check_id": uid(),
                    "org_id": org.org_id,
                    "project_id": org.projects[0],
                    "chain": org.first_chain,
                    "verdict": verdict,
                    "risk_score": round(risk, 4),
                    "latency_ms": latency_ms,
                    "checks_failed": checks_failed,
                    "occurred_at": ts.isoformat(),
                })

                # 10% of flight checks also fire an event
                if random.random() < 0.1:
                    add_event(org, "flight_check_run", ts, surface, {
                        "verdict": verdict,
                        "latency_ms": latency_ms,
                        "checks_failed": checks_failed,
                        "chain_id": org.first_chain,
                    })

    # Pass 3: subscriptions
    for org in orgs:
        if org.paid_at is None:
            continue
        sub_id = uid()
        sub_rows.append({
            "sub_id": sub_id,
            "org_id": org.org_id,
            "plan": org.paid_plan,
            "status": "active",
            "started_at": org.paid_at.isoformat(),
            "ended_at": "",
            "mrr_cents": org.paid_mrr,
            "canceled_reason": "",
        })
        add_event(org, "subscription_started", org.paid_at, "backend", {
            "plan": org.paid_plan,
            "billing_period": "monthly",
            "mrr_cents": org.paid_mrr,
        })
        # 10% churn within 60 days
        if random.random() < 0.1:
            churn_at = org.paid_at + timedelta(days=random.uniform(10, 60))
            if (churn_at - org.created_at).days < days:
                sub_rows[-1]["status"] = "canceled"
                sub_rows[-1]["ended_at"] = churn_at.isoformat()
                sub_rows[-1]["canceled_reason"] = random.choice(
                    ["price", "feature_missing", "switched", "not_using"])
                add_event(org, "subscription_canceled", churn_at, "backend", {
                    "plan": org.paid_plan,
                    "mrr_cents": org.paid_mrr,
                    "reason": sub_rows[-1]["canceled_reason"],
                    "months_active": max(1, int((churn_at - org.paid_at).days / 30)),
                })

    # Pass 4: dashboard page views, CLI events
    for org in orgs:
        if org.is_internal or not org.activated:
            continue
        pages = ["overview", "flight-checks", "runs", "incidents", "settings"]
        for _ in range(random.randint(2, 15)):
            day_offset = random.uniform(0, days)
            ts = org.created_at + timedelta(days=day_offset)
            if ts > now_offset(days):
                continue
            add_event(org, "dashboard_page_viewed", ts, "web", {
                "page_name": random.choice(pages),
                "load_time_ms": random.randint(80, 600),
            })
        if random.random() < 0.4:
            ts = org.created_at + timedelta(days=random.uniform(1, 10))
            add_event(org, "cli_command_run", ts, "cli", {
                "command": "flight-check",
                "subcommand": "run",
                "success": True,
                "latency_bucket": "lt500ms",
            })

    # Write CSVs
    _write_csv("seed_orgs.csv",
               ["org_id", "name", "plan", "created_at", "signup_channel",
                "first_framework", "first_chain", "is_internal", "deleted_at"],
               [{
                   "org_id": o.org_id,
                   "name": o.name,
                   "plan": o.plan,
                   "created_at": o.created_at.isoformat(),
                   "signup_channel": o.signup_channel,
                   "first_framework": o.first_framework,
                   "first_chain": o.first_chain,
                   "is_internal": o.is_internal,
                   "deleted_at": o.deleted_at or "",
               } for o in orgs])

    _write_csv("seed_projects.csv",
               ["project_id", "org_id", "name", "created_at", "deleted_at"],
               projects_rows)

    _write_csv("seed_events.csv",
               ["event_id", "org_id", "project_id", "user_id", "anonymous_id",
                "session_id", "occurred_at", "received_at", "surface", "event_name",
                "app_version", "plan", "environment", "framework", "chain_id", "properties"],
               events)

    _write_csv("seed_flight_checks.csv",
               ["check_id", "org_id", "project_id", "chain", "verdict",
                "risk_score", "latency_ms", "checks_failed", "occurred_at"],
               flight_checks)

    _write_csv("seed_subscriptions.csv",
               ["sub_id", "org_id", "plan", "status", "started_at",
                "ended_at", "mrr_cents", "canceled_reason"],
               sub_rows)

    # Summary
    print(f"✓ seed_orgs.csv          {len(orgs):,} rows")
    print(f"✓ seed_projects.csv      {len(projects_rows):,} rows")
    print(f"✓ seed_events.csv        {len(events):,} rows")
    print(f"✓ seed_flight_checks.csv {len(flight_checks):,} rows")
    print(f"✓ seed_subscriptions.csv {len(sub_rows):,} rows")


def _write_csv(name: str, fieldnames: list, rows: list) -> None:
    path = SEED_DIR / name
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fieldnames, extrasaction="ignore")
        w.writeheader()
        w.writerows(rows)


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate synthetic Vigil analytics seed data")
    parser.add_argument("--days", type=int, default=90, help="Days of history to generate")
    parser.add_argument("--orgs", type=int, default=500, help="Number of orgs")
    parser.add_argument("--seed", type=int, default=42, help="Random seed")
    args = parser.parse_args()
    print(f"Generating {args.days} days × {args.orgs} orgs (seed={args.seed}) …")
    generate(args.days, args.orgs, args.seed)
    print(f"✓ Seed data written to {SEED_DIR}")


if __name__ == "__main__":
    main()
