"""CrewAI tools for Vigil."""
from __future__ import annotations

from typing import Any

try:
    from crewai_tools import BaseTool  # type: ignore[import-untyped]
    HAS_CREWAI = True
except ImportError:
    HAS_CREWAI = False


class VigilFlightCheckTool:
    """CrewAI tool for Vigil flight checks."""
    name: str = "Vigil Flight Check"
    description: str = "Run a pre-transaction safety check on an EVM transaction."

    def __init__(self, client: Any) -> None:
        self._client = client

    def _run(self, chain_id: int, from_address: str, to: str,
             data: str = "0x", value: str = "0x0", intent: str | None = None) -> str:
        result = self._client.flight_check(
            chain_id=chain_id, from_address=from_address, to=to,
            data=data, value=value, intent=intent,
        )
        return f"Verdict: {result.verdict}, Score: {result.score}/100"
