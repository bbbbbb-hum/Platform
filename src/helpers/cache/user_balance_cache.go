package cache

import (
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
	"context"
	"errors"
	"strconv"
)

// 获取用户余额key
func GetUserBalanceKey(userId string) string {
	return redisHelper.BuildKey("user_balance", userId)
}

// 获取用户余额
func GetUserBalance(key string) float64 {
	client := redisHelper.Client()
	if client == nil {
		return 0
	}
	val, err := client.Get(context.Background(), key).Result()
	if err != nil {
		return 0
	}
	n, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0
	}
	return n
}

// 缓存用户余额
func SetUserBalance(key string, value float64) error {
	client := redisHelper.Client()
	if client == nil {
		return errors.New("redis client is not initialized")
	}
	return client.Set(context.Background(), key, value, 0).Err()
}
