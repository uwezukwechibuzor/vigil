"""Tests for anonymous telemetry: opt-out and PII sanitisation."""

from unittest.mock import patch


def _reload_telemetry():
    """Return a fresh _Telemetry instance so env changes take effect."""
    import importlib

    import vigil.telemetry as mod

    importlib.reload(mod)
    return mod._Telemetry()


class TestOptOut:
    def test_disabled_by_vigil_telemetry_zero(self, monkeypatch):
        monkeypatch.setenv("VIGIL_TELEMETRY", "0")
        monkeypatch.delenv("DO_NOT_TRACK", raising=False)
        t = _reload_telemetry()
        assert not t.is_enabled()

    def test_disabled_by_do_not_track(self, monkeypatch):
        monkeypatch.setenv("DO_NOT_TRACK", "1")
        monkeypatch.delenv("VIGIL_TELEMETRY", raising=False)
        t = _reload_telemetry()
        assert not t.is_enabled()

    def test_enabled_by_default(self, monkeypatch):
        monkeypatch.delenv("VIGIL_TELEMETRY", raising=False)
        monkeypatch.delenv("DO_NOT_TRACK", raising=False)
        t = _reload_telemetry()
        assert t.is_enabled()

    def test_track_does_nothing_when_disabled(self, monkeypatch):
        monkeypatch.setenv("VIGIL_TELEMETRY", "0")
        t = _reload_telemetry()
        sent: list = []

        # _send should never be called
        with patch.object(t, "_send", side_effect=lambda b: sent.extend(b)):
            t.track("flight_check_run", {"verdict": "safe"})
            t.flush()

        assert sent == [], "No events should be sent when telemetry is disabled"

    def test_track_queues_event_when_enabled(self, monkeypatch):
        monkeypatch.delenv("VIGIL_TELEMETRY", raising=False)
        monkeypatch.delenv("DO_NOT_TRACK", raising=False)
        t = _reload_telemetry()
        sent: list = []

        with patch.object(t, "_send", side_effect=lambda b: sent.extend(b)):
            t.track(
                "sdk_installed",
                {"sdk_language": "python", "runtime": "cpython", "os_family": "linux"},
            )
            t.flush()

        assert len(sent) == 1
        assert sent[0]["event"] == "sdk_installed"


class TestPIISanitisation:
    """Fuzz PII patterns through the sanitiser — none should appear in output."""

    PII_VALUES = [
        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",  # ETH address
        "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",  # private key
        "0x" + "a" * 64,  # 32-byte hex
    ]
    PII_KEYS = [
        "email",
        "private_key",
        "mnemonic",
        "seed",
        "address",
        "from",
        "to",
        "calldata",
        "tx",
    ]

    def _fresh_telemetry(self, monkeypatch):
        monkeypatch.delenv("VIGIL_TELEMETRY", raising=False)
        monkeypatch.delenv("DO_NOT_TRACK", raising=False)
        return _reload_telemetry()

    def test_pii_values_stripped_from_props(self, monkeypatch):
        t = self._fresh_telemetry(monkeypatch)
        for pii_value in self.PII_VALUES:
            props = {"verdict": "safe", "wallet": pii_value, "extra": pii_value}
            sanitized = t._sanitize(props)
            assert "wallet" not in sanitized or sanitized.get("wallet") != pii_value, (
                f"ETH address leaked through: {pii_value}"
            )
            assert "extra" not in sanitized or sanitized.get("extra") != pii_value, (
                f"PII value leaked through extra: {pii_value}"
            )

    def test_pii_key_names_stripped(self, monkeypatch):
        t = self._fresh_telemetry(monkeypatch)
        for key in self.PII_KEYS:
            props = {key: "some-value"}
            sanitized = t._sanitize(props)
            assert key not in sanitized, f"PII key {key!r} leaked through"

    def test_safe_props_pass_through(self, monkeypatch):
        t = self._fresh_telemetry(monkeypatch)
        props = {"verdict": "safe", "latency_ms": 42, "chain_id": "ethereum"}
        sanitized = t._sanitize(props)
        assert sanitized["verdict"] == "safe"
        assert sanitized["latency_ms"] == 42
        assert sanitized["chain_id"] == "ethereum"

    def test_track_end_to_end_no_pii_sent(self, monkeypatch):
        monkeypatch.delenv("VIGIL_TELEMETRY", raising=False)
        monkeypatch.delenv("DO_NOT_TRACK", raising=False)
        t = _reload_telemetry()
        sent_batches: list = []

        with patch.object(t, "_send", side_effect=lambda b: sent_batches.append(b)):
            t.track(
                "flight_check_run",
                {
                    "verdict": "safe",
                    "from": "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",  # should be stripped
                    "private_key": "0x" + "a" * 64,  # should be stripped
                    "latency_ms": 80,
                },
            )
            t.flush()

        assert sent_batches, "Event should be sent"
        props = sent_batches[0][0]["properties"]
        assert "from" not in props, "wallet 'from' address leaked"
        assert "private_key" not in props, "private_key leaked"
        assert props.get("latency_ms") == 80, "safe prop should pass through"
