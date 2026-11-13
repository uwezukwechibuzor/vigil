// Package analytics provides server-side product event tracking via PostHog.
// All events are server-side (never fire for anonymous/unauthenticated requests).
// PII rules: org_id (UUID only), no email, no wallet addresses, no tx payloads.
package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	phEndpoint    = "https://app.posthog.com/capture/"
	batchInterval = 10 * time.Second
	batchMaxSize  = 100
	sendTimeout   = 5 * time.Second
)

var (
	ethAddrRE = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	hex32RE   = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)
	piiFields = map[string]bool{
		"email": true, "private_key": true, "mnemonic": true, "seed": true,
		"address": true, "calldata": true, "tx": true, "secret": true,
	}
)

// Client batches server-side events and delivers them to PostHog.
type Client struct {
	apiKey  string
	logger  *slog.Logger
	mu      sync.Mutex
	queue   []phEvent
	timer   *time.Timer
	hc      *http.Client
	stopped bool
}

type phEvent struct {
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Properties map[string]any `json:"properties"`
}

// New creates an analytics client. If POSTHOG_API_KEY is not set, events are
// logged at debug level only (no network calls).
func New(logger *slog.Logger) *Client {
	return &Client{
		apiKey: os.Getenv("POSTHOG_API_KEY"),
		logger: logger,
		hc:     &http.Client{Timeout: sendTimeout},
	}
}

// Track queues a server-side event for delivery.
// orgID is the PostHog distinct_id (UUID, not email).
func (c *Client) Track(_ context.Context, orgID, event string, props map[string]any) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	c.queue = append(c.queue, phEvent{
		Event:      event,
		DistinctID: orgID,
		Properties: sanitize(props),
	})
	if len(c.queue) >= batchMaxSize {
		c.flush()
		return
	}
	if c.timer == nil {
		c.timer = time.AfterFunc(batchInterval, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.flush()
		})
	}
}

// Flush drains the queue immediately (called on shutdown).
func (c *Client) Flush() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.flush()
	c.stopped = true
}

// flush must be called with c.mu held.
func (c *Client) flush() {
	if len(c.queue) == 0 {
		return
	}
	batch := c.queue
	c.queue = nil
	c.timer = nil
	go c.send(batch)
}

func (c *Client) send(batch []phEvent) {
	if c.apiKey == "" {
		c.logger.Debug("analytics: posthog key not set, dropping batch", "count", len(batch))
		return
	}
	payload := map[string]any{"api_key": c.apiKey, "batch": batch}
	body, err := json.Marshal(payload)
	if err != nil {
		c.logger.Warn("analytics: marshal batch", "error", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout) // #nosec G118 — batch delivery intentionally outlives any request
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, phEndpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		c.logger.Debug("analytics: posthog send failed", "error", err)
		return
	}
	_ = resp.Body.Close()
}

func sanitize(props map[string]any) map[string]any {
	out := make(map[string]any, len(props))
	for k, v := range props {
		if piiFields[strings.ToLower(k)] {
			continue
		}
		if s, ok := v.(string); ok {
			if ethAddrRE.MatchString(s) || hex32RE.MatchString(s) {
				continue
			}
		}
		out[k] = v
	}
	return out
}
