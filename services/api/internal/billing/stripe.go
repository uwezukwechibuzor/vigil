// Package billing provides billing and subscription management via Stripe.
package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/subscription"
	"github.com/stripe/stripe-go/v82/webhook"
)

// Subscription represents an org's billing subscription.
type Subscription struct {
	OrgID            string
	Plan             string
	Status           string
	CurrentPeriodEnd time.Time
}

// WebhookEvent is a parsed Stripe webhook event.
type WebhookEvent struct {
	Type  string
	OrgID string
	Plan  string
}

// Client abstracts Stripe operations for testability.
type Client interface {
	// CreateCheckoutSession creates a Stripe checkout session URL.
	CreateCheckoutSession(ctx context.Context, orgID, plan string) (url string, err error)
	// GetSubscription retrieves the current subscription for an org.
	GetSubscription(ctx context.Context, orgID string) (*Subscription, error)
	// HandleWebhook verifies the Stripe-Signature and parses the event.
	HandleWebhook(payload []byte, sig string) (*WebhookEvent, error)
}

// StripeBillingClient implements BillingClient using the Stripe API.
type StripeBillingClient struct {
	secretKey     string
	webhookSecret string
	successURL    string
	cancelURL     string
}

// NewStripeClient creates a StripeBillingClient from env-sourced config.
func NewStripeClient(secretKey, webhookSecret, successURL, cancelURL string) *StripeBillingClient {
	stripe.Key = secretKey
	return &StripeBillingClient{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		successURL:    successURL,
		cancelURL:     cancelURL,
	}
}

// CreateCheckoutSession creates a Stripe hosted checkout session.
func (s *StripeBillingClient) CreateCheckoutSession(_ context.Context, orgID, plan string) (string, error) {
	priceID := priceIDForPlan(plan)
	if priceID == "" {
		return "", fmt.Errorf("unknown plan: %s", plan)
	}

	params := &stripe.CheckoutSessionParams{
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL:        stripe.String(s.successURL),
		CancelURL:         stripe.String(s.cancelURL),
		ClientReferenceID: stripe.String(orgID),
	}
	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("create checkout session: %w", err)
	}
	return sess.URL, nil
}

// GetSubscription retrieves the subscription metadata for an org.
func (s *StripeBillingClient) GetSubscription(_ context.Context, orgID string) (*Subscription, error) {
	params := &stripe.SubscriptionListParams{}
	params.Filters.AddFilter("limit", "", "1")
	iter := subscription.List(params)
	for iter.Next() {
		sub := iter.Subscription()
		if sub.Metadata["org_id"] == orgID {
			return &Subscription{
				OrgID:            orgID,
				Plan:             sub.Metadata["plan"],
				Status:           string(sub.Status),
				CurrentPeriodEnd: time.Now().Add(30 * 24 * time.Hour),
			}, nil
		}
	}
	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	return &Subscription{OrgID: orgID, Plan: "free", Status: "active"}, nil
}

// HandleWebhook verifies the Stripe-Signature header using HMAC-SHA256.
func (s *StripeBillingClient) HandleWebhook(payload []byte, sig string) (*WebhookEvent, error) {
	if err := verifyStripeSignature(payload, sig, s.webhookSecret); err != nil {
		return nil, err
	}
	event, err := webhook.ConstructEvent(payload, sig, s.webhookSecret)
	if err != nil {
		return nil, fmt.Errorf("construct event: %w", err)
	}
	ev := &WebhookEvent{Type: string(event.Type)}
	return ev, nil
}

// verifyStripeSignature performs manual HMAC-SHA256 verification of the Stripe-Signature header.
func verifyStripeSignature(payload []byte, sigHeader, secret string) error {
	parts := strings.Split(sigHeader, ",")
	var timestamp, receivedSig string
	for _, p := range parts {
		if strings.HasPrefix(p, "t=") {
			timestamp = strings.TrimPrefix(p, "t=")
		}
		if strings.HasPrefix(p, "v1=") {
			receivedSig = strings.TrimPrefix(p, "v1=")
		}
	}
	if timestamp == "" || receivedSig == "" {
		return errors.New("missing stripe signature components")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	// Reject events older than 5 minutes.
	if time.Since(time.Unix(ts, 0)) > 5*time.Minute {
		return errors.New("stripe event too old")
	}

	signed := fmt.Sprintf("%s.%s", timestamp, string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signed))
	expected := hex.EncodeToString(mac.Sum(nil))

	if subtle := hmacEqual(expected, receivedSig); !subtle {
		return errors.New("stripe signature mismatch")
	}
	return nil
}

// hmacEqual performs a constant-time string comparison.
func hmacEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// priceIDForPlan maps plan names to Stripe price IDs.
// In production these come from env vars; here we stub them.
func priceIDForPlan(plan string) string {
	prices := map[string]string{
		"pro":        "price_pro",
		"team":       "price_team",
		"enterprise": "price_enterprise",
	}
	return prices[plan]
}

// NoopBillingClient is a stub that returns safe defaults for tests.
type NoopBillingClient struct{}

// CreateCheckoutSession returns a stub checkout URL.
func (n *NoopBillingClient) CreateCheckoutSession(_ context.Context, _, _ string) (string, error) {
	return "https://checkout.stripe.com/stub", nil
}

// GetSubscription returns a free tier stub subscription.
func (n *NoopBillingClient) GetSubscription(_ context.Context, orgID string) (*Subscription, error) {
	return &Subscription{
		OrgID:            orgID,
		Plan:             "free",
		Status:           "active",
		CurrentPeriodEnd: time.Now().Add(30 * 24 * time.Hour),
	}, nil
}

// HandleWebhook always returns a stub event.
func (n *NoopBillingClient) HandleWebhook(_ []byte, _ string) (*WebhookEvent, error) {
	return &WebhookEvent{Type: "noop"}, nil
}
