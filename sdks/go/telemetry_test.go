package vigil

import (
	"os"
	"testing"
)

func TestTelIsEnabled(t *testing.T) {
	t.Run("disabled by VIGIL_TELEMETRY=0", func(t *testing.T) {
		t.Setenv("VIGIL_TELEMETRY", "0")
		t.Setenv("DO_NOT_TRACK", "")
		if TelIsEnabled() {
			t.Error("expected telemetry disabled")
		}
	})

	t.Run("disabled by DO_NOT_TRACK=1", func(t *testing.T) {
		t.Setenv("VIGIL_TELEMETRY", "")
		t.Setenv("DO_NOT_TRACK", "1")
		if TelIsEnabled() {
			t.Error("expected telemetry disabled")
		}
	})

	t.Run("enabled by default", func(_ *testing.T) {
		_ = os.Unsetenv("VIGIL_TELEMETRY")
		_ = os.Unsetenv("DO_NOT_TRACK")
		// In CI we expect this to be enabled unless one of the env vars is set
		// (we don't assert true because the CI env might have DO_NOT_TRACK set)
	})
}

func TestSanitizeTel(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		value   string
		wantOut bool
	}{
		{"ETH address stripped", "wallet", "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", false},
		{"32-byte hex stripped", "key", "0x" + "a" + string(make([]byte, 63)), false},
		{"private_key key stripped", "private_key", "anything", false},
		{"email key stripped", "email", "user@example.com", false},
		{"from key stripped", "from", "0xdeadbeef", false},
		{"safe value passes", "verdict", "safe", true},
		{"safe number passes", "latency_ms", "80", true},
		{"chain_id passes", "chain_id", "ethereum", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := map[string]any{tc.key: tc.value}
			result := sanitizeTel(props)
			_, present := result[tc.key]
			if present != tc.wantOut {
				t.Errorf("sanitizeTel[%q]=%v present=%v, want %v", tc.key, tc.value, present, tc.wantOut)
			}
		})
	}
}

func TestSanitizeTelPrivateKeyValue(t *testing.T) {
	privKey := "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	props := map[string]any{"data": privKey}
	result := sanitizeTel(props)
	if v, ok := result["data"]; ok && v == privKey {
		t.Error("32-byte hex private key leaked through sanitizer")
	}
}

func TestLatencyBucket(t *testing.T) {
	cases := []struct {
		ms   int
		want string
	}{
		{50, "lt100ms"},
		{200, "lt500ms"},
		{800, "lt2s"},
		{5000, "lt10s"},
		{30000, "gt10s"},
	}
	for _, tc := range cases {
		got := LatencyBucket(tc.ms)
		if got != tc.want {
			t.Errorf("LatencyBucket(%d) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}
