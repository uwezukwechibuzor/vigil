package collector

import (
	"context"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

const (
	endpointsReloadChannel = "vigil:endpoints:reload"
	reloadDebounce         = 2 * time.Second
)

// Loader fetches endpoints from Postgres and reloads them on a Redis pub/sub
// notification without requiring a service restart.
type Loader struct {
	pg     *dbpg.Pool
	rdb    goredis.UniversalClient
	logger *slog.Logger
}

// NewLoader creates an endpoint loader.
func NewLoader(pg *dbpg.Pool, rdb goredis.UniversalClient, logger *slog.Logger) *Loader {
	return &Loader{pg: pg, rdb: rdb, logger: logger}
}

// Watch loads endpoints immediately, then blocks re-loading whenever a message
// arrives on the reload channel. The given onChange callback is invoked each
// time the endpoint list refreshes. Runs until ctx is cancelled.
func (l *Loader) Watch(ctx context.Context, onChange func([]dbpg.Endpoint)) {
	endpoints, err := l.load(ctx)
	if err != nil {
		l.logger.Error("initial endpoint load failed", "err", err)
	} else {
		onChange(endpoints)
	}

	pubsub := l.rdb.Subscribe(ctx, endpointsReloadChannel)
	defer func() { _ = pubsub.Close() }()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			// Debounce: wait a moment then reload once.
			select {
			case <-time.After(reloadDebounce):
			case <-ctx.Done():
				return
			}
			eps, err := l.load(ctx)
			if err != nil {
				l.logger.Error("endpoint reload failed", "err", err)
				continue
			}
			l.logger.Info("endpoints reloaded", "count", len(eps))
			onChange(eps)
		}
	}
}

func (l *Loader) load(ctx context.Context) ([]dbpg.Endpoint, error) {
	return l.pg.ListEnabledEndpoints(ctx)
}
