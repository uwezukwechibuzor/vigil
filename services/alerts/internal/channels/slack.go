package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// SlackDispatcher sends notifications to a Slack incoming webhook.
type SlackDispatcher struct {
	webhookURL string
	hc         *http.Client
}

// NewSlackDispatcher creates a SlackDispatcher.
func NewSlackDispatcher(webhookURL string) *SlackDispatcher {
	return &SlackDispatcher{webhookURL: webhookURL, hc: &http.Client{}}
}

// Kind returns "slack".
func (s *SlackDispatcher) Kind() string { return "slack" }

// Send posts a message to the Slack webhook.
func (s *SlackDispatcher) Send(ctx context.Context, n Notification) error {
	emoji := "🔴"
	switch n.Severity {
	case "high":
		emoji = "🟠"
	case "medium":
		emoji = "🟡"
	case "low":
		emoji = "🟢"
	}
	payload, _ := json.Marshal(map[string]any{
		"text": fmt.Sprintf("%s *%s*\n%s", emoji, n.Title, n.Body),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.hc.Do(req)
	if err != nil {
		return fmt.Errorf("slack send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack status %d", resp.StatusCode)
	}
	return nil
}
