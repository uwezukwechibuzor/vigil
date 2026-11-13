"""
PostHog survey definitions + LLM theme tagging + behavioural join.

Two surveys are defined:
  1. post_first_check  — shown after the first flight check succeeds.
  2. cancellation      — shown when a subscription is canceled.

Theme tagging uses an LLM (via the Anthropic SDK) to cluster free-text
responses into a small set of labelled themes. The tagged responses are
then joined to org-level behaviour data from DuckDB.

NOTE: Survey responses may contain PII (email, company names). This module
strips or anonymises before writing to any persistent store.
No wallet keys, private keys, or transaction payloads are ever collected.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from typing import Optional

# PostHog survey definitions (send to PostHog API or embed via JS snippet)
SURVEYS: dict[str, dict] = {
    "post_first_check": {
        "name": "Post first flight check",
        "description": "Shown once after org runs its first successful flight check",
        "type": "popover",
        "trigger": {"type": "event", "event": "first_flight_check_run"},
        "conditions": {"seenSurveyCount": {"operator": "lt", "value": 1}},
        "questions": [
            {
                "type": "rating",
                "question": "How easy was it to run your first flight check?",
                "scale": 5,
                "display": "emoji",
                "lower_bound_label": "Very hard",
                "upper_bound_label": "Very easy",
            },
            {
                "type": "open",
                "question": "What almost stopped you from reaching this point?",
                "optional": True,
            },
        ],
        "appearance": {"backgroundColor": "#1a1a2e", "submitButtonColor": "#6C63FF"},
    },
    "cancellation": {
        "name": "Cancellation reason",
        "description": "Shown when a subscription is canceled",
        "type": "popover",
        "trigger": {"type": "event", "event": "subscription_canceled"},
        "questions": [
            {
                "type": "single_choice",
                "question": "Why are you canceling?",
                "choices": [
                    "Price / budget",
                    "Missing feature I need",
                    "Switched to another tool",
                    "Not using it enough",
                    "Technical issues",
                    "Other",
                ],
            },
            {
                "type": "open",
                "question": "What would have made you stay?",
                "optional": True,
            },
        ],
    },
}


# ── Theme taxonomy ────────────────────────────────────────────────────────────

THEMES = {
    "pricing":          "Cost or pricing concern",
    "onboarding":       "Friction in setup or onboarding",
    "missing_feature":  "Missing or incomplete feature",
    "reliability":      "Reliability, latency, or errors",
    "value":            "Unclear value proposition or ROI",
    "positive":         "Positive sentiment or compliment",
    "other":            "Does not fit other categories",
}


@dataclass
class SurveyResponse:
    response_id: str
    org_id: str
    survey_id: str
    occurred_at: str
    rating: Optional[int] = None
    choice: Optional[str] = None
    open_text: Optional[str] = None
    theme: Optional[str] = None
    theme_confidence: float = 0.0
    notes: list[str] = field(default_factory=list)


# ── PII scrubber ─────────────────────────────────────────────────────────────

_EMAIL_RE = re.compile(r"[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+")
_URL_RE   = re.compile(r"https?://\S+")


def scrub_pii(text: str) -> str:
    """Replace emails and URLs before storing or sending to LLM."""
    text = _EMAIL_RE.sub("[EMAIL]", text)
    text = _URL_RE.sub("[URL]", text)
    return text


# ── LLM theme tagger ─────────────────────────────────────────────────────────

def tag_theme(response: SurveyResponse, model: str = "claude-haiku-4-5-20251001") -> SurveyResponse:
    """
    Tag an open-text response with a theme using an LLM.
    Requires ANTHROPIC_API_KEY env var.
    Strips PII before sending to the API.
    Returns the response with theme and theme_confidence set.
    """
    if not response.open_text:
        response.theme = "other"
        response.theme_confidence = 1.0
        return response

    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        response.notes.append("ANTHROPIC_API_KEY not set — theme tagging skipped")
        response.theme = "other"
        return response

    try:
        import anthropic  # optional dependency
    except ImportError:
        response.notes.append("anthropic package not installed — theme tagging skipped")
        response.theme = "other"
        return response

    cleaned = scrub_pii(response.open_text)
    theme_list = "\n".join(f"  {k}: {v}" for k, v in THEMES.items())
    prompt = (
        f"Classify the following user feedback into exactly ONE theme from this list:\n"
        f"{theme_list}\n\n"
        f"Feedback: {cleaned}\n\n"
        f"Respond with ONLY the theme key (e.g. 'pricing'). No explanation."
    )

    client = anthropic.Anthropic(api_key=api_key)
    msg = client.messages.create(
        model=model,
        max_tokens=16,
        messages=[{"role": "user", "content": prompt}],
    )
    raw = msg.content[0].text.strip().lower()
    if raw in THEMES:
        response.theme = raw
        response.theme_confidence = 0.9
    else:
        response.theme = "other"
        response.theme_confidence = 0.5
        response.notes.append(f"LLM returned unknown theme '{raw}'; falling back to 'other'")
    return response


# ── Behaviour join ────────────────────────────────────────────────────────────

def join_to_behaviour(responses: list[SurveyResponse], db_path: str | None = None) -> list[dict]:
    """
    Join tagged survey responses to org-level behaviour data from DuckDB.
    Returns a list of dicts with response fields + behaviour signals.
    """
    import duckdb

    db = db_path or os.environ.get("DBT_DUCKDB_PATH", "/tmp/vigil_analytics.duckdb")
    con = duckdb.connect(db, read_only=True)

    org_ids = list({r.org_id for r in responses})
    placeholders = ", ".join(f"'{oid}'" for oid in org_ids)
    behaviour = con.execute(f"""
        SELECT
            o.org_id,
            o.plan,
            o.signup_channel,
            o.first_framework,
            COUNT(fc.check_id) AS total_checks,
            o.is_activated,
            o.is_paid
        FROM main_core.dim_orgs o
        LEFT JOIN main_core.fct_flight_checks fc USING (org_id)
        WHERE o.org_id IN ({placeholders})
        GROUP BY o.org_id, o.plan, o.signup_channel, o.first_framework,
                 o.is_activated, o.is_paid
    """).fetchall()
    con.close()

    beh_map = {row[0]: row for row in behaviour}
    results = []
    for r in responses:
        beh = beh_map.get(r.org_id)
        results.append({
            "response_id":       r.response_id,
            "org_id":            r.org_id,
            "survey_id":         r.survey_id,
            "occurred_at":       r.occurred_at,
            "rating":            r.rating,
            "choice":            r.choice,
            "theme":             r.theme,
            "plan":              beh[1] if beh else None,
            "signup_channel":    beh[2] if beh else None,
            "framework":         beh[3] if beh else None,
            "total_checks":      int(beh[4]) if beh else 0,
            "is_activated":      bool(beh[5]) if beh else False,
            "is_paid":           bool(beh[6]) if beh else False,
        })
    return results


# ── Theme summary ─────────────────────────────────────────────────────────────

def theme_summary(joined: list[dict]) -> dict[str, int]:
    """Count responses per theme."""
    counts: dict[str, int] = {}
    for row in joined:
        t = row.get("theme") or "other"
        counts[t] = counts.get(t, 0) + 1
    return dict(sorted(counts.items(), key=lambda x: -x[1]))


if __name__ == "__main__":
    print("Survey definitions loaded.")
    for survey_id, cfg in SURVEYS.items():
        print(f"  {survey_id}: {cfg['name']} — {len(cfg['questions'])} questions")
    print("\nTheme taxonomy:")
    for k, v in THEMES.items():
        print(f"  {k}: {v}")
