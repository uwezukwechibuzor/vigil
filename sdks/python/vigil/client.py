"""Vigil API client."""

from __future__ import annotations

import os
import time
import uuid
from typing import Any

import httpx

from vigil.errors import VigilError
from vigil.models import (
    APIKey,
    APIKeyCreateResult,
    APIKeyList,
    ChainHealth,
    ChainList,
    CreateAPIKeyRequest,
    CreatePolicyRequest,
    EndpointHealth,
    EndpointList,
    FlightCheckRequest,
    FlightCheckResult,
    IncidentList,
    OracleHealth,
    Policy,
    PolicyList,
    RunDetail,
    RunList,
    UsageList,
)

DEFAULT_BASE_URL = "https://api.vigil.dev"
MAX_RETRIES = 3
_RETRY_STATUS = {429, 500, 502, 503, 504}


def _sleep(seconds: float) -> None:
    time.sleep(seconds)


class VigilClient:
    """Synchronous Vigil API client."""

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str = DEFAULT_BASE_URL,
    ) -> None:
        self._api_key: str | None = api_key or os.environ.get("VIGIL_API_KEY")
        self._base_url = base_url.rstrip("/")
        headers: dict[str, str] = {
            "User-Agent": "vigil-python/0.1.0",
            "Content-Type": "application/json",
        }
        if self._api_key:
            headers["Authorization"] = f"Bearer {self._api_key}"
        self._http = httpx.Client(
            base_url=self._base_url,
            headers=headers,
            timeout=30.0,
        )

    # ---- Core HTTP ----

    def _request(
        self,
        method: str,
        path: str,
        *,
        json: Any | None = None,
        params: dict[str, str] | None = None,
    ) -> Any:
        is_mutating = method.upper() in ("POST", "PUT", "PATCH", "DELETE")
        extra_headers: dict[str, str] = {}
        if is_mutating:
            extra_headers["X-Idempotency-Key"] = str(uuid.uuid4())

        for attempt in range(MAX_RETRIES + 1):
            try:
                resp = self._http.request(
                    method,
                    f"/v1{path}",
                    json=json,
                    params=params,
                    headers=extra_headers,
                )
            except httpx.RequestError as exc:
                raise VigilError("network_error", str(exc)) from exc

            if resp.status_code in _RETRY_STATUS and attempt < MAX_RETRIES:
                retry_after_header = resp.headers.get("Retry-After")
                if retry_after_header:
                    try:
                        wait = float(retry_after_header)
                    except ValueError:
                        wait = 2.0 ** attempt
                else:
                    wait = 2.0 ** attempt
                _sleep(wait)
                continue

            if resp.status_code == 204:
                return None

            if not resp.is_success:
                try:
                    body: dict[str, Any] = resp.json()
                    err = body.get("error", {})
                    code: str = err.get("code", "api_error")
                    message: str = err.get("message", f"HTTP {resp.status_code}")
                    details: dict[str, Any] | None = err.get("details")
                except Exception:
                    code = "api_error"
                    message = f"HTTP {resp.status_code}"
                    details = None
                raise VigilError(code, message, details, resp.status_code)

            return resp.json()

        raise VigilError("max_retries", "Max retries exceeded")

    # ---- Flight Checks ----

    def flight_check(
        self,
        req: FlightCheckRequest,
        *,
        on_warn: Any | None = None,
    ) -> FlightCheckResult:
        """Run a pre-transaction flight check. Never raises on 'warn' verdict."""
        data = self._request("POST", "/flight-checks", json=req.to_api_dict())
        result = FlightCheckResult.model_validate(data)
        if result.verdict == "warn" and on_warn is not None:
            on_warn(result)
        return result

    def get_flight_check(self, flight_check_id: str) -> FlightCheckResult:
        """Get a flight check result by ID."""
        data = self._request("GET", f"/flight-checks/{flight_check_id}")
        return FlightCheckResult.model_validate(data)

    # ---- Chains ----

    def list_chains(self) -> ChainList:
        """List all supported chains."""
        data = self._request("GET", "/chains")
        return ChainList.model_validate(data)

    def chain_health(self, chain_id: int) -> ChainHealth:
        """Get health for a specific chain."""
        data = self._request("GET", f"/chains/{chain_id}/health")
        return ChainHealth.model_validate(data)

    # ---- Endpoints ----

    def list_endpoints(self, project_id: str) -> EndpointList:
        """List endpoints for a project."""
        data = self._request("GET", "/endpoints", params={"project_id": project_id})
        return EndpointList.model_validate(data)

    def endpoint_health(self, endpoint_id: str) -> EndpointHealth:
        """Get endpoint health."""
        data = self._request("GET", f"/endpoints/{endpoint_id}/health")
        return EndpointHealth.model_validate(data)

    # ---- Oracles ----

    def oracle_health(self, feed: str) -> OracleHealth:
        """Get oracle health for a feed address."""
        data = self._request("GET", f"/oracles/{feed}/health")
        return OracleHealth.model_validate(data)

    # ---- Runs ----

    def list_runs(
        self,
        *,
        agent_id: str | None = None,
        project_id: str | None = None,
        cursor: str | None = None,
    ) -> RunList:
        """List agent runs."""
        params: dict[str, str] = {}
        if agent_id:
            params["agent_id"] = agent_id
        if project_id:
            params["project_id"] = project_id
        if cursor:
            params["cursor"] = cursor
        data = self._request("GET", "/runs", params=params or None)
        return RunList.model_validate(data)

    def get_run(self, run_id: str) -> RunDetail:
        """Get a run by ID."""
        data = self._request("GET", f"/runs/{run_id}")
        return RunDetail.model_validate(data)

    # ---- Incidents ----

    def list_incidents(
        self,
        *,
        project_id: str | None = None,
        status: str | None = None,
    ) -> IncidentList:
        """List incidents."""
        params: dict[str, str] = {}
        if project_id:
            params["project_id"] = project_id
        if status:
            params["status"] = status
        data = self._request("GET", "/incidents", params=params or None)
        return IncidentList.model_validate(data)

    # ---- Policies ----

    def list_policies(self, project_id: str) -> PolicyList:
        """List policies for a project."""
        data = self._request("GET", "/policies", params={"project_id": project_id})
        return PolicyList.model_validate(data)

    def create_policy(self, req: CreatePolicyRequest) -> Policy:
        """Create a policy."""
        data = self._request("POST", "/policies", json=req.model_dump(exclude_none=True))
        return Policy.model_validate(data)

    # ---- Usage ----

    def get_usage(self, *, org_id: str, from_date: str, to_date: str) -> UsageList:
        """Get usage records for an org."""
        data = self._request(
            "GET",
            "/usage",
            params={"org_id": org_id, "from": from_date, "to": to_date},
        )
        return UsageList.model_validate(data)

    # ---- API Keys ----

    def create_api_key(self, req: CreateAPIKeyRequest) -> APIKeyCreateResult:
        """Create a new API key. The plaintext secret is shown once."""
        data = self._request("POST", "/api-keys", json=req.model_dump(exclude_none=True))
        # API returns APIKeyWithPlaintext: {...apikey fields, key: "vgl_..."}
        raw: dict[str, Any] = data
        secret: str = raw.pop("key", "")
        api_key = APIKey.model_validate(raw)
        return APIKeyCreateResult(key=api_key, secret=secret)

    def list_api_keys(self, org_id: str) -> APIKeyList:
        """List API keys for an org."""
        data = self._request("GET", "/api-keys", params={"org_id": org_id})
        return APIKeyList.model_validate(data)

    def revoke_api_key(self, key_id: str) -> None:
        """Revoke an API key."""
        self._request("DELETE", f"/api-keys/{key_id}")

    # ---- Context manager ----

    def close(self) -> None:
        """Close the underlying HTTP client."""
        self._http.close()

    def __enter__(self) -> VigilClient:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()
