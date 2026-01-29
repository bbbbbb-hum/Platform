package cache

import (
	"fmt"

	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
)

func KeyUserKey(apiKey string) string {
	return redisHelper.BuildKey("user_key", apiKey)
}

func KeyUserKeyInvalid(apiKey string) string {
	return redisHelper.BuildKey("user_key_invalid", apiKey)
}

func KeyUsageUserPeriod(userID, serverID, periodKey string) string {
	return redisHelper.BuildKey("usage", "u", userID, "s", serverID, "p", periodKey)
}

func KeyUsageUserPeriodDelta(userID, serverID, periodKey string) string {
	return redisHelper.BuildKey("usage", "u", userID, "s", serverID, "p", periodKey, "delta")
}

func KeyUsageKeyDay(userID string, keyID int32, serverID, dayKey string) string {
	return redisHelper.BuildKey("usage", "u", userID, "k", fmt.Sprintf("%d", keyID), "s", serverID, "d", dayKey)
}

func KeyUsageDayDirtySet(dayKey string) string {
	return redisHelper.BuildKey("usage", "day", "dirty", dayKey)
}
