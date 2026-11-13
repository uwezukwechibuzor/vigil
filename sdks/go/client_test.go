package vigil_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

func flightCheckResult(verdict vigil.Verdict) vigil.FlightCheckResult {
	return vigil.FlightCheckResult{
		ID:        "fc_123",
		Verdict:   verdict,
		Score:     0.95,
		Checks:    map[string]interface{}{"slippage": "ok"},
		CreatedAt: time.Now(),
	}
}

func chainHealthResult() vigil.ChainHealth {
	return vigil.ChainHealth{
		Status:         vigil.HealthStatusHealthy,
		HeadLagBlocks:  0,
		Reorgs24h:      0,
		FeeLevel:       vigil.FeeLevelNormal,
		WorstEndpoints: []string{},
	}
}

func jsonHandler(statusCode int, v interface{}) http.HandlerFunc { //nolint:unparam
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(v)
	}
}

func TestFlightCheck_Pass(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(jsonHandler(200, flightCheckResult(vigil.VerdictPass)))
	defer srv.Close()

	client := vigil.New(vigil.WithAPIKey("vgl_test"), vigil.WithBaseURL(srv.URL))
	result, err := client.FlightCheck(context.Background(), vigil.FlightCheckRequest{
		ChainID: 1, From: "0xabc", To: "0xdef", Data: "0x", Value: "0x0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != vigil.VerdictPass {
		t.Errorf("want pass, got %s", result.Verdict)
	}
}

func TestFlightCheck_WarnNoError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(jsonHandler(200, flightCheckResult(vigil.VerdictWarn)))
	defer srv.Close()

	client := vigil.New(vigil.WithAPIKey("vgl_test"), vigil.WithBaseURL(srv.URL))
	result, err := client.FlightCheck(context.Background(), vigil.FlightCheckRequest{
		ChainID: 1, From: "0xabc", To: "0xdef", Data: "0x", Value: "0x0",
	})
	// warn verdict must NOT return an error
	if err != nil {
		t.Fatalf("warn verdict should not return error, got: %v", err)
	}
	if result.Verdict != vigil.VerdictWarn {
		t.Errorf("want warn, got %s", result.Verdict)
	}
}

func TestFlightCheck_RetryOn429(t *testing.T) {
	t.Parallel()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "0.001")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(429)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{"code": "rate_limited", "message": "slow down"},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(flightCheckResult(vigil.VerdictPass))
	}))
	defer srv.Close()

	client := vigil.New(vigil.WithAPIKey("vgl_test"), vigil.WithBaseURL(srv.URL))
	result, err := client.FlightCheck(context.Background(), vigil.FlightCheckRequest{
		ChainID: 1, From: "0xabc", To: "0xdef", Data: "0x", Value: "0x0",
	})
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if result.Verdict != vigil.VerdictPass {
		t.Errorf("want pass, got %s", result.Verdict)
	}
	if callCount != 2 {
		t.Errorf("want 2 calls (1 retry), got %d", callCount)
	}
}

func TestFlightCheck_IdempotencyKey(t *testing.T) {
	t.Parallel()
	var capturedKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedKey = r.Header.Get("X-Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_ = json.NewEncoder(w).Encode(flightCheckResult(vigil.VerdictPass))
	}))
	defer srv.Close()

	client := vigil.New(vigil.WithAPIKey("vgl_test"), vigil.WithBaseURL(srv.URL))
	_, err := client.FlightCheck(context.Background(), vigil.FlightCheckRequest{
		ChainID: 1, From: "0xabc", To: "0xdef", Data: "0x", Value: "0x0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if capturedKey == "" {
		t.Error("X-Idempotency-Key header was not sent")
	}
}

func TestChainHealth(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(jsonHandler(200, chainHealthResult()))
	defer srv.Close()

	client := vigil.New(vigil.WithAPIKey("vgl_test"), vigil.WithBaseURL(srv.URL))
	health, err := client.ChainHealth(context.Background(), 8453)
	if err != nil {
		t.Fatal(err)
	}
	if health.Status != vigil.HealthStatusHealthy {
		t.Errorf("want healthy, got %s", health.Status)
	}
}

func TestListIncidents(t *testing.T) {
	t.Parallel()
	now := time.Now()
	incidents := vigil.IncidentList{
		Data: []vigil.Incident{
			{
				ID:        "inc_1",
				ProjectID: "proj_abc",
				Status:    vigil.IncidentStatusOpen,
				Severity:  vigil.IncidentSeverityHigh,
				Title:     "RPC degraded",
				StartedAt: now,
				CreatedAt: now,
			},
		},
		Meta: vigil.ListMeta{HasMore: false},
	}
	srv := httptest.NewServer(jsonHandler(200, incidents))
	defer srv.Close()

	client := vigil.New(vigil.WithAPIKey("vgl_test"), vigil.WithBaseURL(srv.URL))
	result, err := client.ListIncidents(context.Background(), vigil.IncidentListParams{Status: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Errorf("want 1 incident, got %d", len(result))
	}
	if result[0].Title != "RPC degraded" {
		t.Errorf("unexpected title: %s", result[0].Title)
	}
}

func TestAPIError_On4xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"code": "not_found", "message": "chain not found"},
		})
	}))
	defer srv.Close()

	client := vigil.New(vigil.WithAPIKey("vgl_test"), vigil.WithBaseURL(srv.URL))
	_, err := client.ChainHealth(context.Background(), 99999)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *vigil.APIError
	ok := false
	if ae, isAPIErr := err.(*vigil.APIError); isAPIErr {
		apiErr = ae
		ok = true
	}
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != "not_found" {
		t.Errorf("want not_found, got %s", apiErr.Code)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("want 404, got %d", apiErr.StatusCode)
	}
}
