package cache

import (
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
	"context"
	"strconv"
)

// 获取工具价格key
func GetToolsPriceKey(serverId, toolName string) string {
	return redisHelper.BuildKey("tools_price", serverId, toolName)
}

// 获取工具价格
func GetToolsPrice(key string) float64 {
	value, err := redisHelper.GetString(context.Background(), key)
	if err != nil {
		return 0 // 如果获取失败，返回默认值0
	}

	price, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}

	return price
}

// 缓存工具价格
func SetToolsPrice(key string, value float64) error {
	// 使用 'f' 格式保持普通小数形式，精度 8 位与数据库一致
	return redisHelper.SetString(context.Background(), key, strconv.FormatFloat(value, 'f', 8, 64), 0)
}
