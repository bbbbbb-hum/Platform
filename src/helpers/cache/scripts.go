package cache

import (
	"context"
	"errors"
	"time"

	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"

	redislib "github.com/redis/go-redis/v9"
)

var (
	scriptCheckAndIncrUsage = redislib.NewScript(`
-- KEYS[1] = base counter key
-- KEYS[2] = delta counter key
-- ARGV[1] = limit
-- ARGV[2] = ttlSeconds

local base = redis.call("GET", KEYS[1])
if not base then
  base = 0
else
  base = tonumber(base)
end

local delta = redis.call("GET", KEYS[2])
if not delta then
  delta = 0
else
  delta = tonumber(delta)
end

local limit = tonumber(ARGV[1])
if base + delta + 1 > limit then
  return 0
end

delta = redis.call("INCR", KEYS[2])
if tonumber(ARGV[2]) > 0 then
  redis.call("EXPIRE", KEYS[2], tonumber(ARGV[2]))
end

return 1
`)

	scriptIncrKeyDay = redislib.NewScript(`
-- KEYS[1] = key/day counter
-- ARGV[1] = ttlSeconds

local cur = redis.call("INCR", KEYS[1])
if tonumber(ARGV[1]) > 0 then
  redis.call("EXPIRE", KEYS[1], tonumber(ARGV[1]))
end
return cur
`)
)

func evalCheckAndIncrUsage(ctx context.Context, baseKey, deltaKey string, limit int64, ttl time.Duration) (bool, error) {
	if baseKey == "" || deltaKey == "" {
		return false, errors.New("usage keys are empty")
	}
	client := redisHelper.Client()
	if client == nil {
		return false, errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ttlSeconds := int64(ttl / time.Second)
	res, err := scriptCheckAndIncrUsage.Run(ctx, client, []string{baseKey, deltaKey}, limit, ttlSeconds).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func evalIncrKeyDay(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if key == "" {
		return 0, errors.New("key/day cache key is empty")
	}
	client := redisHelper.Client()
	if client == nil {
		return 0, errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ttlSeconds := int64(ttl / time.Second)
	return scriptIncrKeyDay.Run(ctx, client, []string{key}, ttlSeconds).Int64()
}
