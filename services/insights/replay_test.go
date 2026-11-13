package insights_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/insights/internal/anomaly"
	"github.com/uwezukwechibuzor/vigil/services/insights/internal/correlation"
	"github.com/uwezukwechibuzor/vigil/services/insights/internal/llm"
)

// mockFetcher returns a fixture with a clear RPC outage spike.
type mockFetcher struct{}

func (m *mockFetcher) FetchEndpointErrorRate(_ context.Context, _ string) ([]anomaly.Point, error) {
	pts := make([]anomaly.Point, 70)
	for i := range pts {
		pts[i] = anomaly.Point{At: time.Now().Add(time.Duration(i-70) * time.Minute), Value: 0.01}
	}
	// Inject outage: error rate spikes to 1.0 for 5 consecutive minutes
	for _, idx := range []int{62, 63, 64, 65, 66} {
		pts[idx].Value = 1.0
	}
	return pts, nil
}

func (m *mockFetcher) FetchEndpointLatency(_ context.Context, _ string) ([]anomaly.Point, error) {
	return nil, nil
}

func (m *mockFetcher) FetchChainHeadLag(_ context.Context, _ string) ([]anomaly.Point, error) {
	return nil, nil
}

func TestReplay_RPCOutage_CreatesExactlyOneIncident(t *testing.T) {
	// Track Slack notifications via a test HTTP server
	var slackPayloads []string
	slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		slackPayloads = append(slackPayloads, string(b))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer slackServer.Close()

	// Run the correlator and anomaly detector directly (no DB needed)
	fetcher := &mockFetcher{}
	series, _ := fetcher.FetchEndpointErrorRate(t.Context(), "ep-1")

	d := anomaly.NewDetector("error_rate")
	anomalies := d.Detect(series)
	if len(anomalies) == 0 {
		t.Fatal("expected anomalies from outage fixture, got none")
	}

	c := correlation.NewCorrelator()
	groups := c.Group(anomalies, "mainnet", "ep-1")
	if len(groups) != 1 {
		t.Fatalf("expected exactly 1 correlated incident group, got %d", len(groups))
	}

	// Verify LLM template produces a sensible explanation
	provider := llm.NewTemplateProvider()
	g := groups[0]
	ev := llm.Evidence{
		Chain:      g.Chain,
		Endpoint:   g.Endpoint,
		Metric:     g.Anomalies[0].Metric,
		Anomalies:  len(g.Anomalies),
		PeakZScore: g.Anomalies[0].ZScore,
		PeakValue:  g.Anomalies[0].Value,
		Baseline:   g.Anomalies[0].Baseline,
		StartedAt:  g.StartedAt.Format(time.RFC3339),
		Title:      g.Title,
	}
	explanation, err := provider.Explain(t.Context(), ev)
	if err != nil {
		t.Fatalf("template explain: %v", err)
	}
	if len(explanation) < 20 {
		t.Errorf("explanation too short: %q", explanation)
	}

	// Fire a Slack test alert
	payload, _ := json.Marshal(map[string]any{
		"text": "Vigil Incident: " + g.Title + "\n" + explanation,
	})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, slackServer.URL, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("slack request: %v", err)
	}
	_ = resp.Body.Close()

	if len(slackPayloads) != 1 {
		t.Errorf("expected 1 Slack notification, got %d", len(slackPayloads))
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("slack returned %d", resp.StatusCode)
	}

	t.Logf("Incident: %s", g.Title)
	t.Logf("Explanation: %s", explanation)
	t.Logf("Slack fired: %d notification(s)", len(slackPayloads))
}
