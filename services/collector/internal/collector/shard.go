package collector

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	leasePrefix = "vigil:collector:lease:"
	leaseTTL    = 15 * time.Second
	leaseRenew  = 5 * time.Second
)

// ShardManager claims a lease in Redis and uses it to decide which endpoints
// this replica owns via consistent modulo sharding.
type ShardManager struct {
	rdb       goredis.UniversalClient
	leaseKey  string
	replicaID string
	logger    *slog.Logger
}

// NewShardManager creates a shard manager with a random replica ID.
func NewShardManager(rdb goredis.UniversalClient, logger *slog.Logger) (*ShardManager, error) {
	id, err := randomID()
	if err != nil {
		return nil, err
	}
	return &ShardManager{
		rdb:       rdb,
		replicaID: id,
		leaseKey:  leasePrefix + id,
		logger:    logger,
	}, nil
}

// Acquire tries to register this replica and returns a context that renews the
// lease until it is cancelled.
func (s *ShardManager) Acquire(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if err := s.rdb.Set(ctx, s.leaseKey, s.replicaID, leaseTTL).Err(); err != nil {
		return nil, nil, fmt.Errorf("shard acquire: %w", err)
	}
	s.logger.Info("shard lease acquired", "replica_id", s.replicaID)

	leaseCtx, cancel := context.WithCancel(ctx)
	go s.renew(leaseCtx)
	return leaseCtx, cancel, nil
}

// Owns returns true when this replica should collect the endpoint at position
// index out of total (0-based). It enumerates all live replicas to determine
// the shard size.
func (s *ShardManager) Owns(ctx context.Context, endpointIndex, total int) bool {
	if total == 0 {
		return false
	}
	replicas, err := s.liveReplicas(ctx)
	if err != nil || len(replicas) == 0 {
		// If we can't determine the shard, claim everything to avoid gaps.
		return true
	}
	// Find our rank among sorted replica IDs for deterministic assignment.
	rank := sort.SearchStrings(replicas, s.replicaID)
	if rank >= len(replicas) || replicas[rank] != s.replicaID {
		rank = 0
	}
	return endpointIndex%len(replicas) == rank
}

func (s *ShardManager) liveReplicas(ctx context.Context) ([]string, error) {
	keys, err := s.rdb.Keys(ctx, leasePrefix+"*").Result()
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)
	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		id := k[len(leasePrefix):]
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *ShardManager) renew(ctx context.Context) {
	ticker := time.NewTicker(leaseRenew)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = s.rdb.Del(context.Background(), s.leaseKey)
			return
		case <-ticker.C:
			if err := s.rdb.Expire(ctx, s.leaseKey, leaseTTL).Err(); err != nil {
				s.logger.Warn("shard lease renewal failed", "err", err)
			}
		}
	}
}

func randomID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
