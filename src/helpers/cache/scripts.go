package cache

import (
	"context"
	"errors"
	"time"

	"AgentEarth_AgentPlatform/src/helpers/logger"
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"

	redislib "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	scriptCheckAndIncrUsage = redislib.NewScript(`
-- KEYS[1] = counter key
-- ARGV[1] = limit
-- ARGV[2] = ttlSeconds

local cur = redis.call("GET", KEYS[1])
if not cur then
  cur = 0
else
  cur = tonumber(cur)
end

local limit = tonumber(ARGV[1])
if cur + 1 > limit then
  return 0
end

cur = redis.call("INCR", KEYS[1])
if tonumber(ARGV[2]) > 0 then
  redis.call("EXPIRE", KEYS[1], tonumber(ARGV[2]))
end

return 1
`)
)

func evalCheckAndIncrUsage(ctx context.Context, counterKey string, limit int64, ttl time.Duration) (bool, error) {
	if counterKey == "" {
		return false, errors.New("usage key is empty")
	}
	client := redisHelper.Client()
	if client == nil {
		logger.Error("redis client is not initialized", zap.String("key", counterKey))
		return true, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ttlSeconds := int64(ttl / time.Second)
	res, err := scriptCheckAndIncrUsage.Run(ctx, client, []string{counterKey}, limit, ttlSeconds).Int64()
	if err != nil {
		logger.Error("redis limit check failed", zap.String("key", counterKey), zap.Error(err))
		return true, nil
	}
	return res == 1, nil
}

