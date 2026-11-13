"""Guarded transaction send helper."""

from __future__ import annotations

from collections.abc import Callable
from typing import Any

from vigil.client import VigilClient
from vigil.errors import VigilError
from vigil.models import FlightCheckRequest, FlightCheckResult
from vigil.tracing import Run


def guarded_send(
    client: VigilClient,
    send_fn: Callable[[dict[str, Any]], str],
    tx: FlightCheckRequest,
    *,
    on_warn: Callable[[FlightCheckResult], None] | None = None,
    run: Run | None = None,
) -> dict[str, str]:
    """
    Run a flight check then send the transaction if the verdict is not 'fail'.

    Args:
        client: VigilClient instance.
        send_fn: Callable that takes a tx dict and returns a tx hash string.
                 For web3.py: lambda tx: w3.eth.send_transaction(tx).hex()
        tx: FlightCheckRequest to check and send.
        on_warn: Optional callback called with the result on 'warn' verdict.
        run: Optional Run to attach the tx hash to.

    Returns:
        dict with 'tx_hash' and 'flight_check_id'.

    Raises:
        VigilError: on 'fail' verdict or API errors.
    """
    result = client.flight_check(tx, on_warn=on_warn)

    if result.verdict == "fail":
        raise VigilError(
            "flight_check_failed",
            f"Flight check failed with score {result.score}",
            {"checks": result.checks, "flight_check_id": result.id},
        )

    tx_dict: dict[str, Any] = {
        "from": tx.from_,
        "to": tx.to,
        "data": tx.data,
        "value": int(tx.value, 16) if tx.value.startswith("0x") else int(tx.value),
    }
    if tx.gas is not None:
        tx_dict["gas"] = tx.gas

    tx_hash = send_fn(tx_dict)

    if run is not None:
        run.attach_tx(tx_hash, result.id)

    return {"tx_hash": tx_hash, "flight_check_id": result.id}
