package cache

import (
	"fmt"

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

// 用户使用次数的key的delta
func KeyUsageUserPeriodDelta(userID, serverID, periodKey string) string {
	return redisHelper.BuildKey("usage", "u", userID, "s", serverID, "p", periodKey, "delta")
}

// 用户key的使用次数的key
func KeyUsageKeyDay(userID string, keyID int32, serverID, dayKey string) string {
	return redisHelper.BuildKey("usage", "u", userID, "k", fmt.Sprintf("%d", keyID), "s", serverID, "d", dayKey)
}

// 用户key的使用次数的key的delta
func KeyUsageDayDirtySet(dayKey string) string {
	return redisHelper.BuildKey("usage", "day", "dirty", dayKey)
}
