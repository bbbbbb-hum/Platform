package boot

import (
	"AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SetupRedis initializes the global Redis client.
func SetupRedis() error {
	dialTimeout := time.Duration(config.GetInt("redis.dial_timeout_seconds")) * time.Second
	readTimeout := time.Duration(config.GetInt("redis.read_timeout_seconds")) * time.Second
	writeTimeout := time.Duration(config.GetInt("redis.write_timeout_seconds")) * time.Second
	poolTimeout := time.Duration(config.GetInt("redis.pool_timeout_seconds")) * time.Second

	cfg := redisHelper.Config{
		Addr:         config.GetString("redis.addr"),
		Username:     "",
		Password:     config.GetString("redis.password"),
		DB:           config.GetInt("redis.db"),
		PoolSize:     config.GetInt("redis.pool_size"),
		MinIdleConns: config.GetInt("redis.min_idle_conns"),
		MaxRetries:   config.GetInt("redis.max_retries"),
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		PoolTimeout:  poolTimeout,
		Prefix:       config.GetString("redis.prefix"),
	}

	client, err := redisHelper.Init(cfg)
	if err != nil {
		logger.Error("Redis 连接失败", zap.Error(err))
		return fmt.Errorf("Redis connection failed")
	}

	logger.Info("Redis 连接成功",
		zap.String("addr", cfg.Addr),
		zap.Int("db", cfg.DB),
		zap.String("prefix", cfg.Prefix),
		zap.Bool("client_ready", client != nil),
	)
	return nil
}
