package cache

import (
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
)

func KeyUserKey(apiKey string) string {
	return redisHelper.BuildKey("user_key", apiKey)
}

// 无效UserKey 的key
func KeyUserKeyInvalid(apiKey string) string {
	return redisHelper.BuildKey("user_key_invalid", apiKey)
}

// 用户使用次数的key
func KeyUsageUserPeriod(userID, serverID, periodKey string) string {
	return redisHelper.BuildKey("usage", "u", userID, "s", serverID, "p", periodKey)
}

