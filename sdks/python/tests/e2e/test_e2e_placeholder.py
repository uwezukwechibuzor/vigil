"""End-to-end SDK tests (requires running local stack via `make up`)."""

import pytest


@pytest.mark.skip(reason="Requires local stack: make up")
def test_e2e_placeholder() -> None:
    """Placeholder — real e2e tests run against local docker-compose stack."""
    pass
