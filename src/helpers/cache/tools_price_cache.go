package cache

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
	"AgentEarth_AgentPlatform/src/models"
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// 缓存工具价格

var (
	toolsPriceCacheTTL = 0 * time.Minute
)

// 获取工具价格key
func getToolsPriceKey(serverId, toolName string) string {
	return redisHelper.BuildKey("tools_price", serverId, toolName)
}

// 获取工具价格
func GetToolsPrice(serverId, toolName string) float64 {
	key := getToolsPriceKey(serverId, toolName)
	value, ok, err := redisHelper.GetString(context.Background(), key)
	if err != nil {
		return 0
	}
	// 如果获取失败或key不存在，查库来同步redis缓存
	if !ok {
		// 从数据库中获取 (先不查tools表，先用services表价格)
		// toolModel := &models.AeMcpTools{}
		// tool, err := toolModel.GetToolByServiceIdAndName(serverId, toolName)
		serviceModel := &models.AeMcpServices{}
		service, err := serviceModel.GetOneByServerId(serverId)
		if err != nil {
			logger.Error("获取服务价格失败", zap.String("server_id", serverId), zap.Error(err))
			return 0
		}
		err = redisHelper.SetString(context.Background(), key, strconv.FormatFloat(service.XlcreditPrice, 'f', 8, 64), toolsPriceCacheTTL)
		if err != nil {
			logger.Error("设置服务价格失败", zap.String("server_id", serverId), zap.Error(err))
			return 0
		}
		return service.XlcreditPrice
	}
	price, err := strconv.ParseFloat(value, 64)
	if err != nil {
		logger.Error("解析服务价格失败", zap.String("server_id", serverId), zap.Error(err))
		return 0
	}
	return price
}

// 缓存工具价格
func SetToolsPrice(serverId, toolName string, value float64) error {
	key := getToolsPriceKey(serverId, toolName)
	// 使用 'f' 格式保持普通小数形式，精度 8 位与数据库一致
	return redisHelper.SetString(context.Background(), key, strconv.FormatFloat(value, 'f', 8, 64), toolsPriceCacheTTL)
}
