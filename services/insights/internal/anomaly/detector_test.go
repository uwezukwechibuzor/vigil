package anomaly_test

import (
	"math"
	"testing"
	"time"

	"github.com/uwezukwechibuzor/vigil/services/insights/internal/anomaly"
)

func TestDetector_NormalSeries_NoAnomalies(t *testing.T) {
	d := anomaly.NewDetector("latency_ms")
	series := makeSeries(70, 100, 0) // 70 points of value 100
	got := d.Detect(series)
	if len(got) != 0 {
		t.Errorf("expected 0 anomalies, got %d", len(got))
	}
}

func TestDetector_SpikeDetected(t *testing.T) {
	d := anomaly.NewDetector("error_rate")
	series := makeSeries(70, 0.01, 0)
	// inject a spike at index 65
	series[65].Value = 0.95
	got := d.Detect(series)
	if len(got) == 0 {
		t.Error("expected at least one anomaly for 95% error rate spike")
	}
}

func TestDetector_InsufficientData_NoAnomalies(t *testing.T) {
	d := anomaly.NewDetector("head_lag")
	series := makeSeries(10, 1.0, 0) // less than WindowSize/2
	got := d.Detect(series)
	if len(got) != 0 {
		t.Errorf("expected no anomalies with insufficient data, got %d", len(got))
	}
}

func makeSeries(n int, base, noise float64) []anomaly.Point {
	_ = noise
	pts := make([]anomaly.Point, n)
	for i := range pts {
		pts[i] = anomaly.Point{
			At:    time.Now().Add(time.Duration(i) * time.Minute),
			Value: base + math.Sin(float64(i))*0.001,
		}
	}
	return pts
}
