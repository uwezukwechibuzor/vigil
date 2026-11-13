package channels

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
)

// WebhookDispatcher sends HMAC-signed webhook notifications.
type WebhookDispatcher struct {
	hc *http.Client
}

// NewWebhookDispatcher creates a WebhookDispatcher.
func NewWebhookDispatcher() *WebhookDispatcher {
	return &WebhookDispatcher{hc: &http.Client{}}
}

// Kind returns "webhook".
func (w *WebhookDispatcher) Kind() string { return "webhook" }

// Send POSTs a JSON notification to the webhook URL, signed with HMAC-SHA256.
func (w *WebhookDispatcher) Send(ctx context.Context, n Notification) error {
	body, _ := json.Marshal(map[string]any{
		"incident_id": n.IncidentID,
		"title":       n.Title,
		"body":        n.Body,
		"severity":    n.Severity,
		"chain":       n.Chain,
	})
	sig := ""
	if n.SigningKey != "" {
		mac := hmac.New(sha256.New, []byte(n.SigningKey))
		mac.Write(body)
		sig = hex.EncodeToString(mac.Sum(nil))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if sig != "" {
		req.Header.Set("X-Vigil-Signature", "sha256="+sig)
	}
	resp, err := w.hc.Do(req)
	if err != nil {
		return fmt.Errorf("webhook send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}
