package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AnthropicProvider calls the Anthropic Claude API for root-cause explanations.
type AnthropicProvider struct {
	apiKey     string
	model      string
	maxTokens  int
	httpClient *http.Client
}

// NewAnthropicProvider creates an AnthropicProvider.
// apiKey defaults to ANTHROPIC_API_KEY env var if empty.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	return &AnthropicProvider{
		apiKey:     apiKey,
		model:      model,
		maxTokens:  300,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Explain calls Claude to generate a root-cause explanation. Hard token budget: 300 tokens.
func (a *AnthropicProvider) Explain(ctx context.Context, ev Evidence) (string, error) {
	prompt := fmt.Sprintf(
		"You are an SRE analyzing a Vigil onchain infrastructure alert.\n\n"+
			"Incident: %s\nChain: %s\nEndpoint: %s\nMetric: %s\n"+
			"Anomalies: %d | Peak value: %.2f | Baseline: %.2f | Z-score: %.1f\nStarted: %s\n\n"+
			"In 2-3 sentences: (1) state the most likely root cause, (2) give one concrete next action.\n"+
			"Be specific. Do not use markdown. Plain text only.",
		ev.Title, ev.Chain, ev.Endpoint, ev.Metric,
		ev.Anomalies, ev.PeakValue, ev.Baseline, ev.PeakZScore, ev.StartedAt,
	)

	reqBody, err := json.Marshal(map[string]any{
		"model":      a.model,
		"max_tokens": a.maxTokens,
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic status %d", resp.StatusCode)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in response")
}
