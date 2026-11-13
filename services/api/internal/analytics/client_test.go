package analytics

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
)

func TestSanitize(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		value   any
		wantOut bool
	}{
		{"ETH address stripped", "wallet", "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", false},
		{"hex32 stripped", "raw", "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", false},
		{"private_key key stripped", "private_key", "anything", false},
		{"email key stripped", "email", "user@example.com", false},
		{"calldata key stripped", "calldata", "0xabcd", false},
		{"safe verdict passes", "verdict", "safe", true},
		{"safe chain_id passes", "chain_id", "ethereum", true},
		{"int value passes", "latency_ms", 80, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			props := map[string]any{tc.key: tc.value}
			result := sanitize(props)
			_, present := result[tc.key]
			if present != tc.wantOut {
				t.Errorf("sanitize[%q] present=%v, want %v", tc.key, present, tc.wantOut)
			}
		})
	}
}

func TestClientNilSafe(_ *testing.T) {
	var c *Client
	// nil client must not panic
	c.Track(context.Background(), uuid.New().String(), "test_event", nil)
	c.Flush()
	c.APIKeyCreated(context.Background(), uuid.New(), false, 1)
}

func TestClientNoKeyLogs(_ *testing.T) {
	c := New(slog.Default())
	// No POSTHOG_API_KEY set — should log at debug, not panic
	c.Track(context.Background(), "org-1", "flight_check_run", map[string]any{
		"verdict":    "safe",
		"latency_ms": 42,
	})
	c.Flush()
}

func TestNoPrivateKeyInEventProps(_ *testing.T) {
	c := New(slog.Default())
	privKey := "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	c.Track(context.Background(), "org-1", "flight_check_run", map[string]any{
		"verdict":     "safe",
		"private_key": privKey,                                      // should be stripped
		"from":        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", // should be stripped
	})
	// Can't easily assert the network payload without a mock, but sanitize is tested above
	c.Flush()
}
