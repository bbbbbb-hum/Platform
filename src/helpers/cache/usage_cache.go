package cache

import (
	"context"
	"errors"
	"strconv"
	"time"

	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"

	redislib "github.com/redis/go-redis/v9"
)

// GetUsageBase 获取基线计数（数据库加载值），并返回是否存在。
func GetUsageBase(ctx context.Context, key string) (int64, bool, error) {
	if key == "" {
		return 0, false, errors.New("usage base key is empty")
	}
	val, err := redisHelper.GetString(ctx, key)
	if err != nil {
		if err == redislib.Nil {
			return 0, false, nil
		}
		return 0, false, err
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}

// GetUsageDelta 获取增量计数（消息队列同步用），并返回是否存在。
func GetUsageDelta(ctx context.Context, key string) (int64, bool, error) {
	if key == "" {
		return 0, false, errors.New("usage delta key is empty")
	}
	val, err := redisHelper.GetString(ctx, key)
	if err != nil {
		if err == redislib.Nil {
			return 0, false, nil
		}
		return 0, false, err
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}

// SetUsageBaseIfAbsent 仅在基线不存在时写入（避免并发重复覆盖）。
func SetUsageBaseIfAbsent(ctx context.Context, key string, value int64, ttl time.Duration) (bool, error) {
	if key == "" {
		return false, errors.New("usage base key is empty")
	}
	client := redisHelper.Client()
	if client == nil {
		return false, errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ok, err := client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

// GetAndResetUsageDelta 读取后清空增量计数（用于同步后清零）。
func GetAndResetUsageDelta(ctx context.Context, key string) (int64, bool, error) {
	if key == "" {
		return 0, false, errors.New("usage delta key is empty")
	}
	client := redisHelper.Client()
	if client == nil {
		return 0, false, errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	val, err := client.GetDel(ctx, key).Result()
	if err != nil {
		if err == redislib.Nil {
			return 0, false, nil
		}
		return 0, false, err
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}

// IncrUsageDayAndMarkDirty 当天计数 +1，并将该 key 标记为“已变化”。
func IncrUsageDayAndMarkDirty(ctx context.Context, userID string, keyID int32, serverID, dayKey string, ttl time.Duration) (int64, error) {
	if userID == "" || keyID <= 0 || serverID == "" || dayKey == "" {
		return 0, errors.New("invalid usage day params")
	}
	client := redisHelper.Client()
	if client == nil {
		return 0, errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	usageKey := KeyUsageKeyDay(userID, keyID, serverID, dayKey)
	dirtyKey := KeyUsageDayDirtySet(dayKey)

	pipe := client.TxPipeline()
	incrCmd := pipe.Incr(ctx, usageKey)
	pipe.Expire(ctx, usageKey, ttl)
	pipe.SAdd(ctx, dirtyKey, usageKey)
	pipe.Expire(ctx, dirtyKey, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incrCmd.Val(), nil
}

// GetUsageDayDirtyKeys 获取当天发生变化的 dayKey 列表。
func GetUsageDayDirtyKeys(ctx context.Context, dayKey string) ([]string, error) {
	if dayKey == "" {
		return nil, errors.New("dayKey is empty")
	}
	client := redisHelper.Client()
	if client == nil {
		return nil, errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dirtyKey := KeyUsageDayDirtySet(dayKey)
	return client.SMembers(ctx, dirtyKey).Result()
}

// ClearUsageDayDirty 清理已同步的 dirty 标记。
func ClearUsageDayDirty(ctx context.Context, dayKey string, keys []string) error {
	if dayKey == "" {
		return errors.New("dayKey is empty")
	}
	if len(keys) == 0 {
		return nil
	}
	client := redisHelper.Client()
	if client == nil {
		return errors.New("redis client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dirtyKey := KeyUsageDayDirtySet(dayKey)
	members := make([]interface{}, 0, len(keys))
	for _, k := range keys {
		members = append(members, k)
	}
	return client.SRem(ctx, dirtyKey, members...).Err()
}
