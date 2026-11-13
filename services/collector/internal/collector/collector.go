// Package collector orchestrates probe scheduling, sharding, and ClickHouse writes.
package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	goredis "github.com/redis/go-redis/v9"
	"github.com/uwezukwechibuzor/vigil/services/collector/internal/adapter"
	"github.com/uwezukwechibuzor/vigil/services/collector/internal/probe"
	"github.com/uwezukwechibuzor/vigil/services/collector/internal/writer"
	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

var (
	probesRun = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vigil_collector_probes_run_total",
		Help: "Total number of probe executions.",
	}, []string{"kind"})

	probeFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "vigil_collector_probe_failures_total",
		Help: "Total number of failed probe executions.",
	}, []string{"kind"})
)

// Collector is the top-level service that manages probe goroutines for each endpoint.
type Collector struct {
	pg     *dbpg.Pool
	ch     *dbch.Client
	rdb    goredis.UniversalClient
	logger *slog.Logger

	shard  *ShardManager
	loader *Loader
	writer *writer.Writer
}

// New creates a Collector.
func New(pg *dbpg.Pool, ch *dbch.Client, rdb goredis.UniversalClient, logger *slog.Logger) (*Collector, error) {
	shard, err := NewShardManager(rdb, logger)
	if err != nil {
		return nil, err
	}
	return &Collector{
		pg:     pg,
		ch:     ch,
		rdb:    rdb,
		logger: logger,
		shard:  shard,
		loader: NewLoader(pg, rdb, logger),
		writer: writer.New(ch, rdb, logger),
	}, nil
}

// Run starts the collector. It acquires a shard lease, loads endpoints, and
// launches probe goroutines. It reacts to endpoint list changes via pub/sub.
func (c *Collector) Run(ctx context.Context) error {
	leaseCtx, cancel, err := c.shard.Acquire(ctx)
	if err != nil {
		return err
	}
	defer cancel()

	results := make(chan *probe.Result, 2048)

	// Start the writer goroutine.
	var wg sync.WaitGroup
	writerErrCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		writerErrCh <- c.writer.Run(leaseCtx, results)
	}()

	// Track running probe goroutines so we can stop them on reload.
	var mu sync.Mutex
	cancelFuncs := map[string]context.CancelFunc{}

	stopAll := func() {
		mu.Lock()
		defer mu.Unlock()
		for _, fn := range cancelFuncs {
			fn()
		}
		cancelFuncs = map[string]context.CancelFunc{}
	}
	defer stopAll()

	startEndpoints := func(endpoints []dbpg.Endpoint) {
		stopAll()

		mu.Lock()
		defer mu.Unlock()

		for i, ep := range endpoints {
			if !c.shard.Owns(leaseCtx, i, len(endpoints)) {
				continue
			}

			epCtx, epCancel := context.WithCancel(leaseCtx)
			cancelFuncs[ep.ID.String()] = epCancel

			go c.runProbesForEndpoint(epCtx, ep, results)
		}
		c.logger.Info("probe goroutines started", "owned", len(cancelFuncs), "total", len(endpoints))
	}

	go c.loader.Watch(leaseCtx, startEndpoints)

	// Block until context is cancelled or the writer exits.
	select {
	case <-leaseCtx.Done():
	case err := <-writerErrCh:
		if err != nil && err != context.Canceled {
			c.logger.Error("writer exited", "err", err)
		}
	}

	close(results)
	wg.Wait()
	return nil
}

func (c *Collector) runProbesForEndpoint(ctx context.Context, ep dbpg.Endpoint, out chan<- *probe.Result) {
	a, err := adapter.NewEVM(ctx, ep.URL)
	if err != nil {
		c.logger.Error("adapter dial", "endpoint", ep.Name, "url", ep.URL, "err", err)
		return
	}
	defer a.Close()

	probes := buildProbes(ep, a)
	cfg := probe.DefaultConfig()

	var wg sync.WaitGroup
	for _, p := range probes {
		wg.Add(1)
		go func(pr probe.Probe) {
			defer wg.Done()
			instrumentedLoop(ctx, pr, cfg, out, c.logger)
		}(p)
	}
	wg.Wait()
}

func instrumentedLoop(ctx context.Context, p probe.Probe, cfg probe.Config, out chan<- *probe.Result, logger *slog.Logger) {
	probe.Loop(ctx, instrumentedProbe{p}, cfg, out, logger)
}

// instrumentedProbe wraps a probe to record Prometheus counters.
type instrumentedProbe struct{ probe.Probe }

func (ip instrumentedProbe) Run(ctx context.Context) (*probe.Result, error) {
	r, err := ip.Probe.Run(ctx)
	probesRun.WithLabelValues(string(ip.Kind())).Inc()
	if err != nil || (r != nil && !r.OK) {
		probeFailures.WithLabelValues(string(ip.Kind())).Inc()
	}
	return r, err
}

func buildProbes(ep dbpg.Endpoint, a *adapter.EVMAdapter) []probe.Probe {
	probes := []probe.Probe{
		probe.NewRPC(ep.ID.String(), a),
		probe.NewHeadLag(ep.ID.String(), a),
		probe.NewReorg(ep.ID.String(), a),
		probe.NewGasFee(ep.ID.String(), a),
	}

	if ep.Kind == dbpg.EndpointKindIndexer {
		chainHead := func(ctx context.Context) (uint64, error) {
			b, err := a.HeadBlock(ctx)
			if err != nil {
				return 0, err
			}
			return b.Number, nil
		}
		probes = append(probes, probe.NewIndexer(ep.ID.String(), ep.URL, chainHead))
	}

	// Oracle probe uses the endpoint URL as the contract address when kind=oracle.
	if ep.Kind == dbpg.EndpointKindOracle {
		probes = append(probes, probe.NewOracle(ep.ID.String(), ep.URL, 2*time.Hour, a))
	}

	return probes
}
