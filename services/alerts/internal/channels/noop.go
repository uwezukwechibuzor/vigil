package channels

import (
	"context"
	"log/slog"
)

// NoopDispatcher is a stub for channels not yet implemented (Telegram, Discord, PagerDuty, email).
type NoopDispatcher struct {
	kind   string
	logger *slog.Logger
}

// NewNoopDispatcher creates a NoopDispatcher for the given kind.
func NewNoopDispatcher(kind string, logger *slog.Logger) *NoopDispatcher {
	return &NoopDispatcher{kind: kind, logger: logger}
}

// Kind returns the channel kind.
func (n *NoopDispatcher) Kind() string { return n.kind }

// Send logs and returns nil (no-op).
func (n *NoopDispatcher) Send(_ context.Context, notif Notification) error {
	n.logger.Info("noop channel send", "kind", n.kind, "title", notif.Title)
	return nil
}
