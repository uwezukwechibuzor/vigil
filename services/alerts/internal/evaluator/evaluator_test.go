package evaluator_test

import (
	"context"
	"testing"

	"github.com/uwezukwechibuzor/vigil/services/alerts/internal/channels"
)

type captureDispatcher struct {
	kind  string
	calls int
}

func (c *captureDispatcher) Kind() string { return c.kind }

func (c *captureDispatcher) Send(_ context.Context, _ channels.Notification) error {
	c.calls++
	return nil
}

func TestCaptureDispatcher_Kind(t *testing.T) {
	d := &captureDispatcher{kind: "slack"}
	if d.Kind() != "slack" {
		t.Errorf("expected slack, got %s", d.Kind())
	}
}

func TestCaptureDispatcher_Send_Increments(t *testing.T) {
	d := &captureDispatcher{kind: "webhook"}
	_ = d.Send(context.Background(), channels.Notification{Title: "test"})
	if d.calls != 1 {
		t.Errorf("expected 1 call, got %d", d.calls)
	}
}
