"""Tests for the Vigil Python client."""

from __future__ import annotations

from datetime import UTC, datetime

import pytest
from pytest_httpx import HTTPXMock

from vigil.client import VigilClient
from vigil.errors import VigilError
from vigil.models import FlightCheckRequest


def _now() -> str:
    return datetime.now(UTC).isoformat()


def _flight_check_response(verdict: str = "pass") -> dict:
    return {
        "id": "fc_123",
        "verdict": verdict,
        "score": 0.95 if verdict == "pass" else 0.6 if verdict == "warn" else 0.1,
        "checks": {"slippage": "ok"},
        "latency_ms": 42,
        "created_at": _now(),
    }


def _chain_health_response() -> dict:
    return {
        "status": "healthy",
        "head_lag_blocks": 0,
        "reorgs_24h": 0,
        "fee_level": "normal",
        "worst_endpoints": [],
    }


def _make_request() -> FlightCheckRequest:
    return FlightCheckRequest(
        **{"from": "0xabc", "chain_id": 1, "to": "0xdef", "data": "0x", "value": "0x0"}
    )


@pytest.fixture
def client() -> VigilClient:
    return VigilClient(api_key="vgl_test_xxx", base_url="https://api.vigil.dev")


def test_flight_check_pass(httpx_mock: HTTPXMock, client: VigilClient) -> None:
    httpx_mock.add_response(json=_flight_check_response("pass"))
    result = client.flight_check(_make_request())
    assert result.verdict == "pass"
    assert result.id == "fc_123"


def test_flight_check_does_not_raise_on_warn(httpx_mock: HTTPXMock, client: VigilClient) -> None:
    httpx_mock.add_response(json=_flight_check_response("warn"))
    # Should NOT raise — warn is not an error
    result = client.flight_check(_make_request())
    assert result.verdict == "warn"


def test_flight_check_calls_on_warn_callback(
    httpx_mock: HTTPXMock, client: VigilClient
) -> None:
    httpx_mock.add_response(json=_flight_check_response("warn"))
    called_with = []
    client.flight_check(_make_request(), on_warn=lambda r: called_with.append(r))
    assert len(called_with) == 1
    assert called_with[0].verdict == "warn"


def test_vigil_error_on_4xx(httpx_mock: HTTPXMock, client: VigilClient) -> None:
    httpx_mock.add_response(
        status_code=401,
        json={"error": {"code": "unauthorized", "message": "Invalid API key"}},
    )
    with pytest.raises(VigilError) as exc_info:
        client.flight_check(_make_request())
    err = exc_info.value
    assert err.code == "unauthorized"
    assert err.status_code == 401


def test_retry_on_429(httpx_mock: HTTPXMock, client: VigilClient) -> None:
    httpx_mock.add_response(
        status_code=429,
        json={"error": {"code": "rate_limited", "message": "slow down"}},
        headers={"Retry-After": "0.01"},
    )
    httpx_mock.add_response(json=_flight_check_response("pass"))
    result = client.flight_check(_make_request())
    assert result.verdict == "pass"


def test_idempotency_key_sent(httpx_mock: HTTPXMock, client: VigilClient) -> None:
    httpx_mock.add_response(json=_flight_check_response("pass"))
    client.flight_check(_make_request())
    request = httpx_mock.get_requests()[0]
    assert "X-Idempotency-Key" in request.headers


def test_chain_health(httpx_mock: HTTPXMock, client: VigilClient) -> None:
    httpx_mock.add_response(json=_chain_health_response())
    health = client.chain_health(8453)
    assert health.status == "healthy"
    assert health.fee_level == "normal"


def test_chain_health_error(httpx_mock: HTTPXMock, client: VigilClient) -> None:
    httpx_mock.add_response(
        status_code=404,
        json={"error": {"code": "not_found", "message": "chain not found"}},
    )
    with pytest.raises(VigilError) as exc_info:
        client.chain_health(99999)
    assert exc_info.value.code == "not_found"
    assert exc_info.value.status_code == 404


def test_max_retries_raises(httpx_mock: HTTPXMock, client: VigilClient) -> None:
    for _ in range(4):  # MAX_RETRIES + 1
        httpx_mock.add_response(
            status_code=500,
            json={"error": {"code": "server_error", "message": "oops"}},
        )
    with pytest.raises(VigilError):
        client.flight_check(_make_request())
