// Package writer batches probe results to ClickHouse and notable events to Redis Streams.
package writer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	goredis "github.com/redis/go-redis/v9"
	"github.com/uwezukwechibuzor/vigil/services/collector/internal/probe"
	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
)

const (
	batchSize     = 500
	flushInterval = 5 * time.Second
	eventStream   = "events"
)

var (
	writtenTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vigil_collector_written_total",
		Help: "Total probe rows written to ClickHouse.",
	}, []string{"kind"})

	writeLagHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vigil_collector_write_lag_seconds",
		Help:    "Time from probe result creation to ClickHouse write.",
		Buckets: prometheus.DefBuckets,
	}, []string{"kind"})
)

// Writer drains a probe.Result channel, batches rows, and flushes to ClickHouse.
type Writer struct {
	ch     *dbch.Client
	rdb    goredis.UniversalClient
	logger *slog.Logger
}

// New creates a Writer.
func New(ch *dbch.Client, rdb goredis.UniversalClient, logger *slog.Logger) *Writer {
	return &Writer{ch: ch, rdb: rdb, logger: logger}
}

// Run reads from results until ctx is cancelled, batching writes to ClickHouse.
func (w *Writer) Run(ctx context.Context, results <-chan *probe.Result) error {
	var headBatch []dbch.ChainHead
	var probeBatch []dbch.EndpointProbe

	flush := func() {
		if len(headBatch) > 0 {
			if err := w.ch.InsertChainHeads(ctx, headBatch); err != nil {
				w.logger.Error("clickhouse write chain_heads", "err", err)
			} else {
				writtenTotal.WithLabelValues("chain_head").Add(float64(len(headBatch)))
			}
			headBatch = headBatch[:0]
		}
		if len(probeBatch) > 0 {
			if err := w.ch.InsertEndpointProbes(ctx, probeBatch); err != nil {
				w.logger.Error("clickhouse write endpoint_probes", "err", err)
			} else {
				writtenTotal.WithLabelValues("endpoint_probe").Add(float64(len(probeBatch)))
			}
			probeBatch = probeBatch[:0]
		}
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			flush()
			return ctx.Err()

		case <-ticker.C:
			flush()

		case r, ok := <-results:
			if !ok {
				flush()
				return nil
			}

			writeLagHistogram.WithLabelValues(string(r.Kind)).Observe(
				time.Since(r.Ts).Seconds(),
			)

			switch r.Kind {
			case probe.KindRPC, probe.KindHeadLag:
				headBatch = append(headBatch, toChainHead(r))
			default:
				probeBatch = append(probeBatch, toEndpointProbe(r))
			}

			// Publish notable failures to the events stream.
			if !r.OK {
				if err := w.publishEvent(ctx, r); err != nil {
					w.logger.Warn("events stream publish", "err", err)
				}
			}

			if len(headBatch)+len(probeBatch) >= batchSize {
				flush()
			}
		}
	}
}

func (w *Writer) publishEvent(ctx context.Context, r *probe.Result) error {
	fields := map[string]any{
		"endpoint_id": r.EndpointID,
		"kind":        string(r.Kind),
		"chain_id":    fmt.Sprintf("%d", r.ChainID),
		"ts":          r.Ts.Unix(),
	}
	if r.Err != nil {
		fields["error"] = r.Err.Error()
	}
	return w.rdb.XAdd(ctx, &goredis.XAddArgs{
		Stream: eventStream,
		Values: fields,
	}).Err()
}

func toChainHead(r *probe.Result) dbch.ChainHead {
	return dbch.ChainHead{
		Chain:      fmt.Sprintf("%d", r.ChainID),
		EndpointID: r.EndpointID,
		BlockNum:   uint64(r.Value),
		ObservedAt: r.Ts,
		LatencyMs:  uint32(r.LatencyMs),
	}
}

func toEndpointProbe(r *probe.Result) dbch.EndpointProbe {
	status := "ok"
	if !r.OK {
		status = "error"
	}
	errStr := ""
	if r.Err != nil {
		errStr = r.Err.Error()
	}
	return dbch.EndpointProbe{
		EndpointID: r.EndpointID,
		Method:     string(r.Kind),
		Status:     status,
		LatencyMs:  uint32(r.LatencyMs),
		Error:      errStr,
		ObservedAt: r.Ts,
	}
}
