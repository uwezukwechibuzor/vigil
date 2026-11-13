// Package correlation groups anomalies into incidents.
package correlation

import (
	"time"

	"github.com/uwezukwechibuzor/vigil/services/insights/internal/anomaly"
)

// Group represents correlated anomalies.
type Group struct {
	Anomalies []anomaly.Anomaly
	Chain     string
	Endpoint  string
	Severity  string
	Title     string
	StartedAt time.Time
}

// Correlator groups anomalies into incidents.
type Correlator struct {
	// Window is the max time gap between anomalies to still group them.
	Window time.Duration
}

// NewCorrelator returns a Correlator with a 15-minute window.
func NewCorrelator() *Correlator {
	return &Correlator{Window: 15 * time.Minute}
}

// Group clusters anomalies by proximity in time and assigns a title.
func (c *Correlator) Group(anomalies []anomaly.Anomaly, chain, endpoint string) []Group {
	if len(anomalies) == 0 {
		return nil
	}
	var groups []Group
	cur := Group{
		Anomalies: []anomaly.Anomaly{anomalies[0]},
		Chain:     chain,
		Endpoint:  endpoint,
		StartedAt: anomalies[0].At,
		Severity:  anomalies[0].Severity,
	}
	for _, a := range anomalies[1:] {
		if a.At.Sub(cur.Anomalies[len(cur.Anomalies)-1].At) <= c.Window {
			cur.Anomalies = append(cur.Anomalies, a)
			if severityRank(a.Severity) > severityRank(cur.Severity) {
				cur.Severity = a.Severity
			}
		} else {
			cur.Title = buildTitle(cur)
			groups = append(groups, cur)
			cur = Group{
				Anomalies: []anomaly.Anomaly{a},
				Chain:     chain,
				Endpoint:  endpoint,
				StartedAt: a.At,
				Severity:  a.Severity,
			}
		}
	}
	cur.Title = buildTitle(cur)
	groups = append(groups, cur)
	return groups
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func buildTitle(g Group) string {
	metric := ""
	if len(g.Anomalies) > 0 {
		metric = g.Anomalies[0].Metric
	}
	if g.Chain != "" {
		return "Anomaly on " + g.Chain + ": " + metric
	}
	if g.Endpoint != "" {
		return "Anomaly on endpoint " + g.Endpoint + ": " + metric
	}
	return "Anomaly detected: " + metric
}
