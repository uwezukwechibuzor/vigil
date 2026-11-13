"""
Anonymous, opt-out telemetry for the Vigil Python SDK.

What is sent:  surface, version, OS, runtime, method name, success/fail, latency bucket
What is NEVER sent:  wallet addresses, private keys, calldata, API keys, file paths, emails
Opt out:  VIGIL_TELEMETRY=0  or  DO_NOT_TRACK=1
"""

from __future__ import annotations

import os
import platform
import queue
import re
import sys
import threading
from typing import Any

SDK_VERSION = "0.1.0"
BATCH_INTERVAL_S = 60.0
ENDPOINT = "https://app.posthog.com/capture/"
PH_KEY = "phc_vigil_sdk_anonymous"

_ETH_ADDR = re.compile(r"^0x[0-9a-fA-F]{40}$")
_HEX32 = re.compile(r"^0x[0-9a-fA-F]{64}$")
_PII_KEYS = frozenset(
    {
        "email",
        "password",
        "private_key",
        "mnemonic",
        "seed",
        "address",
        "from",
        "to",
        "data",
        "calldata",
        "tx",
        "key",
        "secret",
    }
)


class _Telemetry:
    def __init__(self) -> None:
        self._queue: queue.SimpleQueue[dict[str, Any]] = queue.SimpleQueue()
        self._timer: threading.Timer | None = None
        self._lock = threading.Lock()
        self._notified = False

    def is_enabled(self) -> bool:
        if os.environ.get("VIGIL_TELEMETRY") == "0":
            return False
        if os.environ.get("DO_NOT_TRACK") == "1":
            return False
        return True

    def track(self, event: str, props: dict[str, Any]) -> None:
        if not self.is_enabled():
            return
        if not self._notified:
            self._notified = True
            print(
                "[vigil] Anonymous telemetry is enabled. Disable with VIGIL_TELEMETRY=0.",
                file=sys.stderr,
            )
        self._queue.put({"event": event, "properties": self._sanitize(props)})
        self._schedule_flush()

    def flush(self) -> None:
        """Flush immediately (used in tests)."""
        with self._lock:
            if self._timer is not None:
                self._timer.cancel()
                self._timer = None
        self._do_flush()

    def _sanitize(self, props: dict[str, Any]) -> dict[str, Any]:
        out: dict[str, Any] = {}
        for k, v in props.items():
            if k.lower() in _PII_KEYS:
                continue
            if isinstance(v, str) and (_ETH_ADDR.match(v) or _HEX32.match(v)):
                continue
            out[k] = v
        return out

    def _schedule_flush(self) -> None:
        with self._lock:
            if self._timer is not None:
                return
            t = threading.Timer(BATCH_INTERVAL_S, self._do_flush)
            t.daemon = True
            t.start()
            self._timer = t

    def _do_flush(self) -> None:
        with self._lock:
            self._timer = None
        batch = []
        try:
            while True:
                batch.append(self._queue.get_nowait())
        except queue.Empty:
            pass
        if not batch:
            return
        self._send(batch)

    def _send(self, batch: list[dict[str, Any]]) -> None:
        try:
            import httpx  # already a project dependency
        except ImportError:
            return
        payload = {
            "api_key": PH_KEY,
            "batch": [
                {
                    "event": item["event"],
                    "properties": {
                        **item["properties"],
                        "$lib": "vigil-sdk-py",
                        "$lib_version": SDK_VERSION,
                        "surface": "sdk_py",
                        "distinct_id": self._anon_id(),
                        "python_version": platform.python_version(),
                        "os_family": platform.system().lower(),
                    },
                }
                for item in batch
            ],
        }
        try:
            httpx.post(ENDPOINT, json=payload, timeout=3.0)
        except Exception:  # noqa: BLE001
            pass  # telemetry must never break the caller

    @staticmethod
    def _anon_id() -> str:
        return f"sdk-py-{platform.system().lower()}-{platform.python_version()}"


telemetry = _Telemetry()
