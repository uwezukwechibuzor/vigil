"""Vigil API models — Pydantic v2."""

from __future__ import annotations

from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, Field

# ---- Common ----


class ListMeta(BaseModel):
    """Pagination metadata."""

    cursor: str | None = None
    has_more: bool
    total: int | None = None


# ---- Chains ----


class Chain(BaseModel):
    """An EVM chain supported by Vigil."""

    id: str
    chain_id: int
    name: str
    native_token: str
    block_time_ms: int
    enabled: bool
    created_at: datetime


class ChainList(BaseModel):
    """Paginated list of chains."""

    data: list[Chain]
    meta: ListMeta


class ChainHealth(BaseModel):
    """Health snapshot for an EVM chain."""

    status: Literal["healthy", "degraded", "down"]
    head_lag_blocks: int
    reorgs_24h: int
    fee_level: Literal["low", "normal", "high", "extreme"]
    worst_endpoints: list[str]


# ---- Endpoints ----


class Endpoint(BaseModel):
    """An RPC/indexer/oracle endpoint registered with Vigil."""

    id: str
    project_id: str
    chain_id: str | None = None
    kind: Literal["rpc", "indexer", "oracle", "custom"]
    name: str
    url: str
    enabled: bool
    created_at: datetime
    updated_at: datetime


class EndpointList(BaseModel):
    """Paginated list of endpoints."""

    data: list[Endpoint]
    meta: ListMeta


class EndpointHealth(BaseModel):
    """Health snapshot for an endpoint."""

    status: Literal["healthy", "degraded", "down"]
    uptime_1h_pct: float
    latency_p95_ms: int
    error_rate: float


# ---- Oracles ----


class OracleHealth(BaseModel):
    """Health snapshot for a Chainlink-style price feed."""

    feed: str
    last_answer: str
    updated_at: datetime
    stale: bool


# ---- Flight Checks ----


class FlightCheckRequest(BaseModel):
    """Pre-transaction flight check request."""

    chain_id: int
    from_: str = Field(alias="from")
    to: str
    data: str
    value: str
    gas: int | None = None
    intent: str | None = None
    policy_id: str | None = None
    max_slippage_bps: int | None = None
    expected_out: str | None = None
    deadline_s: int | None = None
    project_id: str | None = None

    model_config = {"populate_by_name": True}

    def to_api_dict(self) -> dict[str, Any]:
        """Serialize to dict with 'from' (not 'from_') for the API."""
        d = self.model_dump(by_alias=True, exclude_none=True)
        return d


class FlightCheckResult(BaseModel):
    """Result of a pre-transaction flight check."""

    id: str
    project_id: str | None = None
    chain: str | None = None
    verdict: Literal["pass", "warn", "fail"]
    score: float
    checks: dict[str, Any]
    latency_ms: int | None = None
    created_at: datetime


# ---- Runs ----


class RunSummary(BaseModel):
    """Summary of a single agent run."""

    run_id: str
    agent_id: str
    trace_id: str
    span_count: int
    total_cost_usd: float
    tx_hash: str | None = None
    created_at: datetime


class RunList(BaseModel):
    """Paginated list of run summaries."""

    data: list[RunSummary]
    meta: ListMeta


class Span(BaseModel):
    """An OTel span from a run."""

    trace_id: str
    span_id: str
    parent_span_id: str | None = None
    name: str
    kind: str
    service_name: str
    attrs: dict[str, str] = Field(default_factory=dict)
    start_time: datetime
    end_time: datetime
    status_code: str
    status_msg: str | None = None
    gen_ai_model: str | None = None
    input_tokens: int | None = None
    output_tokens: int | None = None
    llm_cost_usd: float | None = None


class RunDetail(RunSummary):
    """Full run detail with spans."""

    spans: list[Span]


# ---- Incidents ----


class Incident(BaseModel):
    """An active or resolved incident."""

    id: str
    project_id: str
    rule_id: str | None = None
    status: Literal["open", "acknowledged", "resolved"]
    severity: Literal["critical", "high", "medium", "low"]
    title: str
    started_at: datetime
    acknowledged_at: datetime | None = None
    resolved_at: datetime | None = None
    created_at: datetime


class IncidentList(BaseModel):
    """Paginated list of incidents."""

    data: list[Incident]
    meta: ListMeta


# ---- Policies ----


class Policy(BaseModel):
    """A flight check policy."""

    id: str
    project_id: str
    name: str
    version: int
    rules: dict[str, Any]
    enabled: bool
    created_at: datetime
    updated_at: datetime


class PolicyList(BaseModel):
    """List of policies."""

    data: list[Policy]


class CreatePolicyRequest(BaseModel):
    """Request to create a policy."""

    project_id: str
    name: str
    rules: dict[str, Any]
    enabled: bool | None = None


# ---- Alert Rules ----


class AlertRule(BaseModel):
    """An alert rule."""

    id: str
    project_id: str
    name: str
    description: str | None = None
    severity: Literal["critical", "high", "medium", "low"]
    condition: dict[str, Any]
    enabled: bool
    created_at: datetime
    updated_at: datetime


class AlertRuleList(BaseModel):
    """List of alert rules."""

    data: list[AlertRule]


# ---- Alert Channels ----


class AlertChannel(BaseModel):
    """A notification channel."""

    id: str
    org_id: str
    kind: Literal["slack", "pagerduty", "webhook", "email"]
    name: str
    config: dict[str, Any]
    created_at: datetime


class AlertChannelList(BaseModel):
    """List of alert channels."""

    data: list[AlertChannel]


# ---- API Keys ----


class APIKey(BaseModel):
    """An API key (without plaintext secret)."""

    id: str
    org_id: str
    project_id: str | None = None
    name: str
    key_prefix: str
    scopes: list[str]
    expires_at: datetime | None = None
    last_used_at: datetime | None = None
    revoked_at: datetime | None = None
    created_at: datetime


class APIKeyList(BaseModel):
    """List of API keys."""

    data: list[APIKey]


class APIKeyCreateResult(BaseModel):
    """Result of creating an API key — includes plaintext secret shown once."""

    key: APIKey
    secret: str


class CreateAPIKeyRequest(BaseModel):
    """Request to create an API key."""

    org_id: str
    project_id: str | None = None
    name: str
    scopes: list[str]
    expires_at: datetime | None = None


# ---- Usage ----


class UsageRecord(BaseModel):
    """Daily usage counter for an org."""

    org_id: str
    day: str
    traces: int
    flight_checks: int
    api_calls: int


class UsageList(BaseModel):
    """List of usage records."""

    data: list[UsageRecord]
