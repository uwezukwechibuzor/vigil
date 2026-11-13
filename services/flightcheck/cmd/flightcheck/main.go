// Package main is the entrypoint for the vigil-flightcheck service.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/v5"
	goredis "github.com/redis/go-redis/v9"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/check"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/engine"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/handler"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/store"
	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
	"github.com/uwezukwechibuzor/vigil/services/internal/platform"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()

	// --- Postgres ---
	var pg *dbpg.Pool
	if dsn := envOrDefault("DATABASE_URL", ""); dsn != "" {
		pool, err := dbpg.New(ctx, dsn)
		if err != nil {
			logger.Error("postgres connect", "error", err)
			os.Exit(1)
		}
		pg = pool
	} else {
		logger.Warn("DATABASE_URL not set; policy and denylist checks disabled")
	}

	// --- ClickHouse ---
	var ch *dbch.Client
	if dsn := envOrDefault("CLICKHOUSE_URL", ""); dsn != "" {
		client, err := dbch.New(ctx, dsn)
		if err != nil {
			logger.Error("clickhouse connect", "error", err)
			os.Exit(1)
		}
		ch = client
		defer func() { _ = ch.Close() }()
	} else {
		logger.Warn("CLICKHOUSE_URL not set; flight check results will not be persisted")
	}

	// --- Redis ---
	var rdb goredis.UniversalClient
	if redisURL := envOrDefault("REDIS_URL", ""); redisURL != "" {
		opts, err := goredis.ParseURL(redisURL)
		if err != nil {
			logger.Error("redis parse url", "error", err)
			os.Exit(1)
		}
		rdb = goredis.NewClient(opts)
		defer func() { _ = rdb.(*goredis.Client).Close() }()
	}

	// --- EVM chain clients ---
	clients := buildChainClients(logger)

	// --- Check registry ---
	registry := check.NewRegistry()
	registry.Register(&check.InfraHealthCheck{})
	registry.Register(&check.SimulationCheck{})
	registry.Register(&check.OracleFreshnessCheck{})
	registry.Register(&check.SlippageCheck{})
	registry.Register(&check.GasSanityCheck{})
	registry.Register(&check.ContractRiskCheck{})
	registry.Register(&check.PolicyCheck{})
	registry.Register(&check.AddressSafetyCheck{Screener: nil}) // wire screener here

	budget := envOrDefaultDuration("FLIGHT_CHECK_BUDGET", 1500*time.Millisecond)
	eng := engine.New(registry, clients, rdb, pg, logger, budget)
	st := store.New(ch, pg, logger)
	h := handler.New(eng, st, logger)

	platform.StartService("flightcheck", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"service":"vigil-flightcheck","status":"ok"}`))
		})
		r.Post("/v1/flight-check", h.FlightCheck)
	})
}

// buildChainClients reads CHAIN_{chainID}_RPC_URL env vars and dials ethclient connections.
func buildChainClients(logger *slog.Logger) []engine.ChainClient {
	// Well-known chain IDs to scan for configured RPC URLs.
	knownChains := []uint64{1, 8453, 42161, 31337} // mainnet, Base, Arbitrum, anvil
	var clients []engine.ChainClient

	for _, chainID := range knownChains {
		key := "CHAIN_" + strconv.FormatUint(chainID, 10) + "_RPC_URL"
		rpcURL := os.Getenv(key)
		if rpcURL == "" {
			continue
		}
		client, err := ethclient.Dial(rpcURL)
		if err != nil {
			logger.Error("dial ethclient", "chain_id", chainID, "url", rpcURL, "error", err)
			continue
		}
		clients = append(clients, engine.ChainClient{ChainID: chainID, Client: client})
		logger.Info("chain client connected", "chain_id", chainID, "url", rpcURL)
	}
	return clients
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrDefaultDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
