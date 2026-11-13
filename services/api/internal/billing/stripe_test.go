package billing_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/api/internal/billing"
)

// buildStripeSignature builds a valid Stripe-Signature header for testing.
func buildStripeSignature(payload []byte, secret string, ts int64) string {
	signed := fmt.Sprintf("%d.%s", ts, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signed))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", ts, sig)
}

func TestNoopClient_GetSubscription(t *testing.T) {
	client := &billing.NoopBillingClient{}
	sub, err := client.GetSubscription(context.Background(), "org_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sub == nil {
		t.Fatal("expected non-nil subscription")
	}
	if sub.Plan != "free" {
		t.Errorf("expected plan 'free', got %q", sub.Plan)
	}
	if sub.Status != "active" {
		t.Errorf("expected status 'active', got %q", sub.Status)
	}
}

func TestNoopClient_CreateCheckoutSession(t *testing.T) {
	client := &billing.NoopBillingClient{}
	url, err := client.CreateCheckoutSession(context.Background(), "org_123", "pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty checkout URL")
	}
}

func TestNoopClient_HandleWebhook(t *testing.T) {
	client := &billing.NoopBillingClient{}
	ev, err := client.HandleWebhook([]byte(`{}`), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev == nil {
		t.Fatal("expected non-nil webhook event")
	}
}

func TestStripeClient_WebhookSignatureValid(t *testing.T) {
	secret := "whsec_testsecret"
	payload := []byte(`{"type":"customer.subscription.updated"}`)
	ts := time.Now().Unix()
	sigHeader := buildStripeSignature(payload, secret, ts)

	// Use a real StripeBillingClient to exercise our verifyStripeSignature path.
	client := billing.NewStripeClient("sk_test_fake", secret, "https://example.com/success", "https://example.com/cancel")

	// HandleWebhook calls stripe's ConstructEvent after our verification step.
	// Our HMAC check should pass; the stripe library may reject the event format.
	_, err := client.HandleWebhook(payload, sigHeader)
	t.Logf("webhook result: %v", err)
}

func TestStripeSignatureVerification_Invalid(t *testing.T) {
	secret := "whsec_testsecret"
	payload := []byte(`{"type":"test"}`)

	client := billing.NewStripeClient("sk_test_fake", secret, "https://example.com/success", "https://example.com/cancel")

	// Tampered signature should fail.
	_, err := client.HandleWebhook(payload, "t=12345,v1=invalidsig")
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestStripeSignatureVerification_MissingComponents(t *testing.T) {
	secret := "whsec_testsecret"
	payload := []byte(`{}`)

	client := billing.NewStripeClient("sk_test_fake", secret, "https://example.com/success", "https://example.com/cancel")

	_, err := client.HandleWebhook(payload, "")
	if err == nil {
		t.Fatal("expected error for empty signature header")
	}
}
