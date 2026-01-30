package cache

import (
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
	"context"
	"fmt"
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

	var price float64
	fmt.Sscanf(value, "%f", &price) // 将字符串转换为float64

	return price
}

// 缓存工具价格
func SetToolsPrice(key string, value float64) error {
	return redisHelper.SetString(context.Background(), key, fmt.Sprintf("%f", value), 0)
}
