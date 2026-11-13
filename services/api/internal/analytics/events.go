package analytics

import (
	"context"

	"github.com/google/uuid"
)

// --- Activation events ---

// APIKeyCreated fires when an API key is generated for an org.
func (c *Client) APIKeyCreated(ctx context.Context, orgID uuid.UUID, isFirst bool, scopeCount int) {
	c.Track(ctx, orgID.String(), "api_key_created", map[string]any{
		"is_first":        isFirst,
		"key_scope_count": scopeCount,
		"surface":         "backend",
	})
}

// ProjectCreated fires when a project is created.
func (c *Client) ProjectCreated(ctx context.Context, orgID uuid.UUID, projectName string, isFirst bool) {
	c.Track(ctx, orgID.String(), "project_created", map[string]any{
		"project_name": projectName,
		"is_first":     isFirst,
		"surface":      "backend",
	})
}

// SignupCompleted fires server-side when account creation is confirmed.
func (c *Client) SignupCompleted(ctx context.Context, orgID uuid.UUID, method, utmSource, utmMedium string) {
	c.Track(ctx, orgID.String(), "signup_completed", map[string]any{
		"signup_method": method,
		"utm_source":    utmSource,
		"utm_medium":    utmMedium,
		"surface":       "backend",
	})
}

// --- Engagement events ---

// FlightCheckRun fires after every flight check response.
func (c *Client) FlightCheckRun(ctx context.Context, orgID uuid.UUID, chain, verdict string, latencyMs, checksFailed int) {
	c.Track(ctx, orgID.String(), "flight_check_run", map[string]any{
		"chain_id":      chain,
		"verdict":       verdict,
		"latency_ms":    latencyMs,
		"checks_failed": checksFailed,
		"surface":       "backend",
	})
}

// RunTraced fires when spans are accepted via OTLP ingest.
func (c *Client) RunTraced(ctx context.Context, orgID uuid.UUID, spanCount, durationMs int) {
	c.Track(ctx, orgID.String(), "run_traced", map[string]any{
		"span_count":  spanCount,
		"duration_ms": durationMs,
		"surface":     "backend",
	})
}

// --- Monetization events ---

// SubscriptionStarted fires when a Stripe webhook confirms a new subscription.
func (c *Client) SubscriptionStarted(ctx context.Context, orgID uuid.UUID, plan, billingPeriod string, mrrCents int) {
	c.Track(ctx, orgID.String(), "subscription_started", map[string]any{
		"plan":           plan,
		"billing_period": billingPeriod,
		"mrr_cents":      mrrCents,
		"surface":        "backend",
	})
}

// SubscriptionCanceled fires when a Stripe webhook confirms a cancellation.
func (c *Client) SubscriptionCanceled(ctx context.Context, orgID uuid.UUID, plan, reason string, mrrCents, monthsActive int) {
	c.Track(ctx, orgID.String(), "subscription_canceled", map[string]any{
		"plan":          plan,
		"mrr_cents":     mrrCents,
		"reason":        reason,
		"months_active": monthsActive,
		"surface":       "backend",
	})
}

// UsageLimitHit fires when an org hits their plan limit.
func (c *Client) UsageLimitHit(ctx context.Context, orgID uuid.UUID, limitType, plan string) {
	c.Track(ctx, orgID.String(), "usage_limit_hit", map[string]any{
		"limit_type": limitType,
		"plan":       plan,
		"surface":    "backend",
	})
}
