"""LangChain tools for Vigil — wrap the vigil-sdk Python client."""
from __future__ import annotations

from typing import Any

try:
    from langchain_core.tools import BaseTool  # type: ignore[import-untyped]
    from pydantic import BaseModel, Field
    HAS_LANGCHAIN = True
except ImportError:
    HAS_LANGCHAIN = False


class FlightCheckInput(BaseModel if HAS_LANGCHAIN else object):  # type: ignore[misc]
    chain_id: int = Field(description="EVM chain ID")
    from_address: str = Field(alias="from", description="Sender address")
    to: str = Field(description="Target address")
    data: str = Field(default="0x", description="Hex calldata")
    value: str = Field(default="0x0", description="Value in hex wei")
    intent: str | None = Field(default=None, description="Human description")


class VigilFlightCheckTool:
    """LangChain-compatible tool for Vigil flight checks."""
    name = "vigil_flight_check"
    description = "Run a pre-transaction safety check using Vigil. Returns verdict (pass/warn/fail) and risk score."

    def __init__(self, client: Any) -> None:
        self._client = client

    def _run(self, **kwargs: Any) -> str:
        result = self._client.flight_check(**kwargs)
        return f"Verdict: {result.verdict}, Score: {result.score}/100"


class VigilChainHealthTool:
    """LangChain-compatible tool for chain health checks."""
    name = "vigil_chain_health"
    description = "Get current health of an EVM chain (block lag, reorgs, fee level)."

    def __init__(self, client: Any) -> None:
        self._client = client

    def _run(self, chain_id: int) -> str:
        h = self._client.chain_health(chain_id)
        return f"Status: {h.status}, Lag: {h.head_lag_blocks} blocks, Fee: {h.fee_level}"


def create_vigil_tools(client: Any) -> list[Any]:
    """Create a list of LangChain-compatible Vigil tools."""
    return [VigilFlightCheckTool(client), VigilChainHealthTool(client)]
