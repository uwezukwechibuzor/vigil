"""Vigil API client."""

from __future__ import annotations

import httpx
from pydantic import BaseModel


class ClientOptions(BaseModel):
    base_url: str
    api_key: str | None = None
    timeout: float = 30.0


class VigilClient:
    """Synchronous Vigil API client."""

    def __init__(self, base_url: str, *, api_key: str | None = None) -> None:
        self._opts = ClientOptions(base_url=base_url, api_key=api_key)
        headers: dict[str, str] = {"User-Agent": "vigil-python/0.1.0"}
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"
        self._http = httpx.Client(base_url=base_url, headers=headers, timeout=self._opts.timeout)

    def healthz(self) -> dict[str, str]:
        resp = self._http.get("/healthz")
        resp.raise_for_status()
        return resp.json()  # type: ignore[no-any-return]

    def close(self) -> None:
        self._http.close()

    def __enter__(self) -> VigilClient:
        return self

    def __exit__(self, *_: object) -> None:
        self.close()
