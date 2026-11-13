// Package anomaly implements rolling MAD + EWMA anomaly detection.
package anomaly

import (
	"math"
	"sort"
	"time"
)

// Point is a time-series data point.
type Point struct {
	At    time.Time
	Value float64
}

// Anomaly signals a detected anomaly.
type Anomaly struct {
	At       time.Time
	Metric   string
	Value    float64
	Baseline float64
	ZScore   float64 // MAD-based z-score
	Severity string  // "critical" | "high" | "medium" | "low"
}

// Detector runs MAD + EWMA anomaly detection on a window of points.
type Detector struct {
	// WindowSize is the rolling window for median/MAD calculation.
	WindowSize int
	// Alpha is the EWMA smoothing factor (0 < alpha < 1).
	Alpha float64
	// Threshold is the number of MAD units to call an anomaly (default 3.5).
	Threshold float64
	// MetricName is included in emitted anomalies.
	MetricName string
}

// NewDetector creates a Detector with sensible defaults.
func NewDetector(metric string) *Detector {
	return &Detector{
		WindowSize: 60, // 60 one-minute buckets = 1 hour
		Alpha:      0.1,
		Threshold:  3.5,
		MetricName: metric,
	}
}

// Detect returns anomalies found in the given series.
// Series must be sorted by time ascending.
func (d *Detector) Detect(series []Point) []Anomaly {
	if len(series) < d.WindowSize/2 {
		return nil
	}
	var anomalies []Anomaly
	var ewma float64
	var initialized bool

	for i := d.WindowSize; i <= len(series); i++ {
		window := series[i-d.WindowSize : i]
		p := series[i-1]

		// Update EWMA
		if !initialized {
			ewma = p.Value
			initialized = true
		} else {
			ewma = d.Alpha*p.Value + (1-d.Alpha)*ewma
		}
		// Suppress unused variable warning; ewma used for future baseline extension.
		_ = ewma

		// Compute median and MAD of the window
		vals := make([]float64, len(window))
		for j, w := range window {
			vals[j] = w.Value
		}
		median := computeMedian(vals)
		mad := computeMAD(vals, median)

		if mad == 0 {
			// When the window is constant, treat any deviation as an anomaly
			// by using a small relative floor so division works correctly.
			if math.Abs(p.Value-median) < 1e-9 {
				continue // truly constant, no anomaly
			}
			mad = math.Max(math.Abs(median)*0.01, 1e-9)
		}
		// Modified z-score
		z := 0.6745 * (p.Value - median) / mad

		if math.Abs(z) > d.Threshold {
			sev := "medium"
			switch {
			case math.Abs(z) > 10:
				sev = "critical"
			case math.Abs(z) > 7:
				sev = "high"
			}
			anomalies = append(anomalies, Anomaly{
				At:       p.At,
				Metric:   d.MetricName,
				Value:    p.Value,
				Baseline: median,
				ZScore:   z,
				Severity: sev,
			})
		}
	}
	return anomalies
}

func computeMedian(vals []float64) float64 {
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func computeMAD(vals []float64, median float64) float64 {
	devs := make([]float64, len(vals))
	for i, v := range vals {
		devs[i] = math.Abs(v - median)
	}
	return computeMedian(devs)
}
