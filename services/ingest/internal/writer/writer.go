// Package writer reads spans from a Redis Stream and batch-writes them to ClickHouse.
package writer

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/linker"
	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
)

var writeDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "vigil_ingest_write_duration_seconds",
	Help:    "Duration of ClickHouse batch writes in seconds.",
	Buckets: prometheus.DefBuckets,
})

// Writer reads spans from a Redis Stream and batch-writes to ClickHouse.
type Writer struct {
	rdb           goredis.UniversalClient
	ch            *dbch.Client
	lnk           *linker.Linker
	logger        *slog.Logger
	stream        string
	group         string
	consumer      string
	batchSize     int
	flushInterval time.Duration
}

// New creates a new Writer.
func New(
	rdb goredis.UniversalClient,
	ch *dbch.Client,
	lnk *linker.Linker,
	logger *slog.Logger,
	stream, group, consumer string,
	batchSize int,
	flushInterval time.Duration,
) *Writer {
	return &Writer{
		rdb:           rdb,
		ch:            ch,
		lnk:           lnk,
		logger:        logger,
		stream:        stream,
		group:         group,
		consumer:      consumer,
		batchSize:     batchSize,
		flushInterval: flushInterval,
	}
}

// Run reads from the Redis Stream until ctx is cancelled.
// Uses consumer groups for horizontal scaling.
// On successful ClickHouse write, ACKs the messages.
func (w *Writer) Run(ctx context.Context) error {
	// Create consumer group if it does not exist.
	if err := w.rdb.XGroupCreateMkStream(ctx, w.stream, w.group, "0").Err(); err != nil {
		// BUSYGROUP means the group already exists — that's fine.
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			w.logger.Warn("xgroup create", "error", err)
		}
	}

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	var batch []dbch.Span
	var msgIDs []string

	flush := func() {
		if len(batch) == 0 {
			return
		}
		start := time.Now()
		if err := w.ch.InsertSpans(ctx, batch); err != nil {
			w.logger.Error("insert spans failed", "count", len(batch), "error", err)
			batch = batch[:0]
			msgIDs = msgIDs[:0]
			return
		}
		writeDuration.Observe(time.Since(start).Seconds())
		w.logger.Info("spans written", "count", len(batch))

		// Enqueue tx spans for onchain linking.
		if w.lnk != nil {
			for _, s := range batch {
				if s.VigilTxHash != "" {
					w.lnk.Enqueue(s)
				}
			}
		}

		// ACK processed messages.
		if err := w.rdb.XAck(ctx, w.stream, w.group, msgIDs...).Err(); err != nil {
			w.logger.Warn("xack failed", "error", err)
		}
		batch = batch[:0]
		msgIDs = msgIDs[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return ctx.Err()
		case <-ticker.C:
			flush()
		default:
		}

		msgs, err := w.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{
			Group:    w.group,
			Consumer: w.consumer,
			Streams:  []string{w.stream, ">"},
			Count:    int64(w.batchSize),
			Block:    time.Second,
		}).Result()
		if err != nil {
			if err == goredis.Nil || err == context.Canceled || err == context.DeadlineExceeded {
				continue
			}
			w.logger.Warn("xreadgroup error", "error", err)
			continue
		}

		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				raw, ok := msg.Values["span"].(string)
				if !ok {
					continue
				}
				var s dbch.Span
				if err := json.Unmarshal([]byte(raw), &s); err != nil {
					w.logger.Warn("span unmarshal error", "error", err)
					continue
				}
				batch = append(batch, s)
				msgIDs = append(msgIDs, msg.ID)

				if len(batch) >= w.batchSize {
					flush()
				}
			}
		}
	}
}
