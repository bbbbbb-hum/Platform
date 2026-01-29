package middleware

import (
	"context"
	"fmt"
	"time"

	cacheHelper "AgentEarth_AgentPlatform/src/helpers/cache"
	userModels "AgentEarth_AgentPlatform/src/models/users"
	"AgentEarth_AgentPlatform/src/servers"
)

// LimitChecker 是限流核心对象：负责 Redis 计数。
type LimitChecker struct{}

func NewLimitChecker() *LimitChecker {
	return &LimitChecker{}
}

func yyyymmddKey(year, month, day int16) string {
	return fmt.Sprintf("%04d%02d%02d", year, month, day)
}

func ttlUntil(end time.Time, buffer time.Duration) time.Duration {
	if end.IsZero() {
		return 0
	}
	ttl := time.Until(end)
	if ttl <= 0 {
		return buffer
	}
	return ttl + buffer
}

func isoWeekStart(year, week int, loc *time.Location) time.Time {
	// ISO 周规则：第 1 周是包含 1 月 4 日的那一周。这里用它来反推该年的第 1 周周一。
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, loc)
	wd := int(jan4.Weekday())
	if wd == 0 {
		wd = 7 // Sunday
	}
	mondayWeek1 := jan4.AddDate(0, 0, -(wd - 1))
	return mondayWeek1.AddDate(0, 0, (week-1)*7)
}

func periodFor(limitType int, now time.Time) (periodKey string, start time.Time, endExcl time.Time, ok bool) {
	loc := now.Location()
	switch limitType {
	case 1: // day
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		endExcl = start.AddDate(0, 0, 1)
		periodKey = yyyymmddKey(int16(start.Year()), int16(start.Month()), int16(start.Day()))
		return periodKey, start, endExcl, true
	case 2: // week (ISO, Monday start)
		y, w := now.ISOWeek()
		start = isoWeekStart(y, w, loc)
		endExcl = start.AddDate(0, 0, 7)
		periodKey = fmt.Sprintf("%04dW%02d", y, w)
		return periodKey, start, endExcl, true
	case 3: // month
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		endExcl = start.AddDate(0, 1, 0)
		periodKey = fmt.Sprintf("%04d%02d", start.Year(), int(start.Month()))
		return periodKey, start, endExcl, true
	case 4: // quarter
		qm := time.Month(((int(now.Month())-1)/3)*3 + 1)
		start = time.Date(now.Year(), qm, 1, 0, 0, 0, 0, loc)
		endExcl = start.AddDate(0, 3, 0)
		q := (int(qm)-1)/3 + 1
		periodKey = fmt.Sprintf("%04dQ%d", start.Year(), q)
		return periodKey, start, endExcl, true
	case 5: // year
		start = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, loc)
		endExcl = start.AddDate(1, 0, 0)
		periodKey = fmt.Sprintf("%04d", start.Year())
		return periodKey, start, endExcl, true
	default:
		return "", time.Time{}, time.Time{}, false
	}
}

// 校验用户的使用量
func (c *LimitChecker) CheckUsage(apiKey, serverID string) bool {
	now := time.Now()
	serverInfo := servers.McpServicesMap[serverID]
	if serverInfo == nil {
		return true
	}
	limit := serverInfo.LimitCalls
	if limit <= 0 {
		return true
	}

	limitType := int(serverInfo.LimitType)
	periodKey, periodStart, periodEndExcl, ok := periodFor(limitType, now)
	if !ok {
		return true
	}
	// 缓存中获取秘钥信息
	userKeyinfo, err := cacheHelper.GetUserKeyInfo(context.Background(), apiKey)
	if err != nil || userKeyinfo == nil || userKeyinfo.UserID == "" || userKeyinfo.KeyID <= 0 {
		return true
	}
	userID := userKeyinfo.UserID
	keyID := userKeyinfo.KeyID

	usageTTL := ttlUntil(periodEndExcl, time.Hour)
	baseKey := cacheHelper.KeyUsageUserPeriod(userID, serverID, periodKey)
	_, exists, err := cacheHelper.GetUsageBase(context.Background(), baseKey)
	if err != nil {
		return true
	}
	if !exists {
		existingUserCalls, err := userModels.GetUserCallsSumByUSRange(userID, serverID, periodStart, periodEndExcl)
		if err != nil {
			return true
		}
		_, _ = cacheHelper.SetUsageBaseIfAbsent(context.Background(), baseKey, existingUserCalls, usageTTL)
	}

	allowed, err := cacheHelper.CheckAndIncrUsage(context.Background(), userID, serverID, periodKey, limit, usageTTL)
	if err != nil {
		return true
	}
	if !allowed {
		return false
	}

	year, month, day := int16(now.Year()), int16(now.Month()), int16(now.Day())
	dayKey := yyyymmddKey(year, month, day)
	_, _ = cacheHelper.IncrUsageDayAndMarkDirty(context.Background(), userID, keyID, serverID, dayKey, 72*time.Hour)
	return true
}

func (c *LimitChecker) StartUsageFlusher(interval time.Duration) {
	// no-op: DB sync handled asynchronously elsewhere
}
