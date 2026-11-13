package llm

import (
	"context"
	"fmt"
)

// TemplateProvider is a fallback that generates a template explanation.
type TemplateProvider struct{}

// NewTemplateProvider returns a TemplateProvider.
func NewTemplateProvider() *TemplateProvider { return &TemplateProvider{} }

// Explain returns a templated explanation without calling an LLM.
func (t *TemplateProvider) Explain(_ context.Context, ev Evidence) (string, error) {
	return fmt.Sprintf(
		"%s detected on %s. The %s metric reached %.2f (baseline %.2f, z-score %.1f). "+
			"This may indicate a network partition, provider outage, or unusual traffic spike. "+
			"Suggested actions: (1) check the provider status page, (2) verify endpoint connectivity, "+
			"(3) review recent deployments for this chain.",
		capitalize(ev.Anomalies)+" anomalies", ev.Chain, ev.Metric, ev.PeakValue, ev.Baseline, ev.PeakZScore,
	), nil
}

func capitalize(n int) string {
	if n == 1 {
		return "1 anomaly"
	}
	return fmt.Sprintf("%d anomalies", n)
}
