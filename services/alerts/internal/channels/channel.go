// Package channels defines the Dispatcher interface and all channel implementations.
package channels

import "context"

// Notification is an alert notification to dispatch.
type Notification struct {
	Title      string
	Body       string
	Severity   string
	IncidentID string
	Chain      string
	WebhookURL string
	SigningKey string
}

// Dispatcher sends notifications to a channel.
type Dispatcher interface {
	// Kind returns the channel kind string (e.g. "slack").
	Kind() string
	// Send dispatches a notification. Returns error only on hard failure.
	Send(ctx context.Context, n Notification) error
}
