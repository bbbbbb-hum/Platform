package cache

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
	"AgentEarth_AgentPlatform/src/models/users"
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 获取用户余额key
func GetUserBalanceKey(userId string) string {
	today := time.Now().Format("2006-01-02")
	return redisHelper.BuildKey("user_balance", userId, today)
}

// GetUserBalance 获取用户余额
// 优先从 Redis 缓存获取，缓存没有则查数据库并回写缓存
func GetUserBalance(key string, userId string) float64 {
	client := redisHelper.Client()

	// 1. 先查 Redis 缓存
	if client != nil {
		val, err := client.Get(context.Background(), key).Result()
		if err == nil {
			n, parseErr := strconv.ParseFloat(val, 64)
			if parseErr == nil {
				return n
			}
		}
		// 如果不是 key 不存在的错误，记录日志
		if err != nil && !errors.Is(err, redis.Nil) {
			logger.Error("获取用户余额缓存失败", zap.String("key", key), zap.Error(err))
		}
	}

	// 2. 缓存没有，查数据库今日余额
	balanceModel := &users.AeUserBalanceStatisticDaily{UserId: userId}
	balance, err := balanceModel.GetTodayBalance()
	if err != nil {
		logger.Error("查询数据库用户余额失败", zap.String("userId", userId), zap.Error(err))
		return 0
	}

	// 3. 如果今日没有记录（返回 0 且无错误），创建今日余额记录
	// 注意：这里无法区分"余额为0"和"记录不存在"，所以每次都尝试写缓存
	if balance == 0 {
		// 尝试创建今日记录（从最新日期复制余额）
		balance, err = balanceModel.AddTodayBalance(userId)
		if err != nil {
			logger.Error("创建今日余额记录失败", zap.String("userId", userId), zap.Error(err))
			return 0
		}
	}

	// 4. 写入 Redis 缓存
	if client != nil {
		// 缓存到当天结束（计算到午夜的剩余时间）
		now := time.Now()
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		ttl := midnight.Sub(now)
		err = client.Set(context.Background(), key, strconv.FormatFloat(balance, 'f', 8, 64), ttl).Err()
		if err != nil {
			logger.Warn("写入用户余额缓存失败", zap.String("key", key), zap.Error(err))
		}
	}

	return balance
}

// DecrUserBalance 累减用户余额（key不存在时创建并设置TTL到当天结束）
func DecrUserBalance(key string, delta float64) error {
	// 缓存到当天结束
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	ttl := midnight.Sub(now)
	return redisHelper.DecrFloat(context.Background(), key, delta, ttl)
}
