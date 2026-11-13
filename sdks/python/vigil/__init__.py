"""Vigil Python SDK — reliability layer for onchain AI agents."""

from vigil.client import VigilClient
from vigil.errors import VigilError
from vigil.guarded import guarded_send
from vigil.models import FlightCheckRequest
from vigil.tracing import Run, init_tracing, start_run

__version__ = "0.1.0"
__all__ = [
    "VigilClient",
    "VigilError",
    "FlightCheckRequest",
    "init_tracing",
    "start_run",
    "Run",
    "guarded_send",
]
