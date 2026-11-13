// Package main is the entrypoint for the vigil-ingest service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"github.com/ethereum/go-ethereum/ethclient"

	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
	"github.com/uwezukwechibuzor/vigil/services/internal/platform"

	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/auth"
	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/linker"
	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/otlp"
	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/sampler"
	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/writer"
)

const (
	defaultMaxBytes   = int64(4 * 1024 * 1024) // 4 MB
	defaultMaxQLen    = int64(100_000)
	defaultSampleRate = 1.0
	streamName        = "vigil:ingest:spans"
	groupName         = "ingest-writers"
	batchSize         = 200
	flushInterval     = 2 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Postgres (optional) ---
	var pg *dbpg.Pool
	if dsn := envOrDefault("DATABASE_URL", ""); dsn != "" {
		var err error
		pg, err = dbpg.New(ctx, dsn)
		if err != nil {
			logger.Warn("postgres unavailable, auth disabled", "error", err)
		} else {
			defer pg.Close()
		}
	}

	// --- ClickHouse (optional) ---
	var ch *dbch.Client
	if dsn := envOrDefault("CLICKHOUSE_URL", ""); dsn != "" {
		var err error
		ch, err = dbch.New(ctx, dsn)
		if err != nil {
			logger.Warn("clickhouse unavailable, writes disabled", "error", err)
		} else {
			defer func() { _ = ch.Close() }()
		}
	}

	// --- Redis (required) ---
	redisDSN := envOrDefault("REDIS_URL", "redis://localhost:6379")
	redisOpts, err := goredis.ParseURL(redisDSN)
	if err != nil {
		logger.Error("redis parse url", "error", err)
		os.Exit(1)
	}
	rdb := goredis.NewClient(redisOpts)
	defer func() { _ = rdb.Close() }()

	// --- Chain RPC clients ---
	chainClients := buildChainClients(logger)

	// --- Service config ---
	maxBytes := envOrDefaultInt64("INGEST_MAX_PAYLOAD_BYTES", defaultMaxBytes)
	maxQLen := envOrDefaultInt64("INGEST_MAX_QUEUE_LEN", defaultMaxQLen)
	sampleRate := envOrDefaultFloat64("INGEST_SAMPLE_RATE", defaultSampleRate)

	// --- Components ---
	sam := sampler.New(sampleRate)
	lnk := linker.New(ch, chainClients, nil, logger)
	consumerID := fmt.Sprintf("ingest-%s", uuid.New().String()[:8])

	var w *writer.Writer
	if ch != nil {
		w = writer.New(rdb, ch, lnk, logger, streamName, groupName, consumerID, batchSize, flushInterval)
		go func() {
			if runErr := w.Run(ctx); runErr != nil && runErr != context.Canceled {
				logger.Error("writer exited", "error", runErr)
			}
		}()
		go lnk.Run(ctx)
	}

	handler := otlp.NewHandler(rdb, sam, logger, streamName, maxBytes, maxQLen, sampleRate)
	authMiddleware := auth.APIKeyAuth(pg, logger)

	platform.StartService("ingest", func(r chi.Router) {
		r.Get("/", func(wr http.ResponseWriter, _ *http.Request) {
			wr.Header().Set("Content-Type", "application/json")
			_, _ = wr.Write([]byte(`{"service":"vigil-ingest","status":"ok"}`))
		})

		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Post("/v1/traces", handler.TracesHandler)
			r.Post("/v1/metrics", handler.MetricsHandler)
			r.Post("/v1/logs", handler.LogsHandler)
		})
	})
}

// buildChainClients reads CHAIN_{id}_RPC_URL env vars and returns a map of chain clients.
func buildChainClients(logger *slog.Logger) map[uint64]*ethclient.Client {
	clients := make(map[uint64]*ethclient.Client)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, url := parts[0], parts[1]
		if !strings.HasPrefix(key, "CHAIN_") || !strings.HasSuffix(key, "_RPC_URL") {
			continue
		}
		idStr := strings.TrimPrefix(strings.TrimSuffix(key, "_RPC_URL"), "CHAIN_")
		chainID, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}
		c, err := ethclient.DialContext(context.Background(), url)
		if err != nil {
			logger.Warn("failed to connect to chain rpc", "chain_id", chainID, "error", err)
			continue
		}
		clients[chainID] = c
		logger.Info("chain rpc connected", "chain_id", chainID)
	}
	return clients
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrDefaultInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func envOrDefaultFloat64(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}
