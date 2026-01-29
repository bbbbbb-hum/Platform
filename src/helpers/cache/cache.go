package cache

import (
	"context"
	"errors"
	"time"
)

// CheckAndIncrUsage atomically checks limit and increments usage if allowed.
// It expects the base key to be initialized separately.
func CheckAndIncrUsage(ctx context.Context, userID, serverID, periodKey string, limit int64, ttl time.Duration) (bool, error) {
	if userID == "" || serverID == "" || periodKey == "" {
		return false, errors.New("usage key parts are empty")
	}
	if limit <= 0 {
		return true, nil
	}
	baseKey := KeyUsageUserPeriod(userID, serverID, periodKey)
	deltaKey := KeyUsageUserPeriodDelta(userID, serverID, periodKey)
	return evalCheckAndIncrUsage(ctx, baseKey, deltaKey, limit, ttl)
}
