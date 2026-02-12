package cache

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"
)

/**
 * 用户使用量递增缓存
 */

var (
	userUsageIncrementTTL = 0 * time.Hour
)

func getUserUsageIncrementKey(userId string) string {
	today := time.Now().Format("2006-01-02-15")
	return redisHelper.BuildKey("user_usage_increment", today, userId)
}

func GetUserUsageIncrement(userId string) float64 {
	key := getUserUsageIncrementKey(userId)
	value, ok, err := redisHelper.GetString(context.Background(), key)
	if err != nil {
		logger.Error("Failed to get user usage increment", zap.Error(err))
		return 0 // 如果获取失败或key不存在，返回默认值0
	}
	if !ok {
		err = SetUserUsageIncrement(userId, 0)
		if err != nil {
			logger.Error("Failed to set user usage increment", zap.Error(err))
			return 0
		}
		return 0
	}
	v, _ := strconv.ParseFloat(value, 64)
	return v
}

func SetUserUsageIncrement(userId string, delta float64) error {
	key := getUserUsageIncrementKey(userId)
	return redisHelper.IncrFloat(context.Background(), key, delta, userUsageIncrementTTL)
}
