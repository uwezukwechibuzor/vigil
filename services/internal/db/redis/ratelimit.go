package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RateLimitResult holds the outcome of a rate-limit check.
type RateLimitResult struct {
	Allowed   bool
	Remaining int64
	ResetAt   time.Time
}

// slidingWindowScript is a Lua script for a sliding-window rate limiter.
// Keys: [key]  Args: [limit, windowSeconds, nowUnixMs]
// Use a unique sequence number per request so simultaneous calls don't collapse.
var slidingWindowScript = goredis.NewScript(`
local key    = KEYS[1]
local limit  = tonumber(ARGV[1])
local window = tonumber(ARGV[2]) * 1000
local now    = tonumber(ARGV[3])
local min    = now - window

redis.call('ZREMRANGEBYSCORE', key, '-inf', min)
local count = redis.call('ZCARD', key)

if count < limit then
  local seq = redis.call('INCR', key .. ':seq')
  redis.call('ZADD', key, now, seq)
  redis.call('PEXPIRE', key, window)
  return {1, limit - count - 1, now + window}
else
  local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  local reset = tonumber(oldest[2]) + window
  return {0, 0, reset}
end
`)

// Allow checks whether the given key is within the sliding-window rate limit.
// limit is the max number of requests per window duration.
func (c *Client) Allow(ctx context.Context, key string, limit int, window time.Duration) (*RateLimitResult, error) {
	nowMs := time.Now().UnixMilli()
	res, err := slidingWindowScript.Run(ctx, c.rdb, []string{key}, limit, int(window.Seconds()), nowMs).Int64Slice()
	if err != nil {
		return nil, fmt.Errorf("rate limit script: %w", err)
	}
	if len(res) != 3 {
		return nil, fmt.Errorf("rate limit script: unexpected result length %d", len(res))
	}
	return &RateLimitResult{
		Allowed:   res[0] == 1,
		Remaining: res[1],
		ResetAt:   time.UnixMilli(res[2]),
	}, nil
}
