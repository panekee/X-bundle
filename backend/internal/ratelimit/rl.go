package ratelimit

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	client *redis.Client
	// script SHA cached by client internally via EvalSha; we use Eval for simplicity.
}

func NewRedisLimiter(addr, pass string) (*RedisLimiter, error) {
	if addr == "" { return nil, errors.New("missing redis addr") }
	c := redis.NewClient(&redis.Options{Addr: addr, Password: pass, DB: 0})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil { return nil, err }
	return &RedisLimiter{client: c}, nil
}

// Allow atomically attempts to consume tokens.
// key: string bucket key, capacity: int, refillPerSec: float64, tokensRequested: int
func (r *RedisLimiter) Allow(ctx context.Context, key string, capacity int, refillPerSec float64, tokensRequested int) (bool, int, error) {
	// Lua script token bucket
	script := `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local requested = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])
local refill = tonumber(ARGV[4])
local data = redis.call("HMGET", key, "tokens", "ts")
local tokens = tonumber(data[1]) or capacity
local ts = tonumber(data[2]) or now
local elapsed = now - ts
tokens = math.min(capacity, tokens + elapsed * refill)
if tokens < requested then
  redis.call("HMSET", key, "tokens", tokens, "ts", now)
  redis.call("EXPIRE", key, 3600)
  return {0, tokens}
else
  tokens = tokens - requested
  redis.call("HMSET", key, "tokens", tokens, "ts", now)
  redis.call("EXPIRE", key, 3600)
  return {1, tokens}
end
`
	now := float64(time.Now().UnixNano()) / 1e9
	res, err := r.client.Eval(ctx, script, []string{key}, now, tokensRequested, capacity, refillPerSec).Result()
	if err != nil { return false, 0, err }
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 2 { return false, 0, errors.New("invalid rl response") }
	allowed := arr[0].(int64) == 1
	tokensLeft := int(arr[1].(float64))
	return allowed, tokensLeft, nil
}
