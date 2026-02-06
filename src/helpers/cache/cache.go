package cache

import (
	"context"
	"errors"
	"time"
)

// CheckAndIncrUsage atomically checks limit and increments usage if allowed.
func CheckAndIncrUsage(ctx context.Context, userID, serverID, periodKey string, limit int64, ttl time.Duration) (bool, error) {
	if userID == "" || serverID == "" || periodKey == "" {
		return false, errors.New("usage key parts are empty")
	}
	if limit <= 0 {
		return true, nil
	}
	counterKey := KeyUsageUserPeriod(userID, serverID, periodKey)
	return evalCheckAndIncrUsage(ctx, counterKey, limit, ttl)
}
