"""Vigil typed errors."""

from __future__ import annotations


class VigilError(Exception):
    """Raised on API errors from the Vigil service."""

    def __init__(
        self,
        code: str,
        message: str,
        details: dict[str, object] | None = None,
        status_code: int | None = None,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.details: dict[str, object] = details or {}
        self.status_code = status_code

    def __repr__(self) -> str:
        return (
            f"VigilError(code={self.code!r}, message={str(self)!r}, "
            f"status_code={self.status_code!r})"
        )
