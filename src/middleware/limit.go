package middleware

import (
	userModels "AgentEarth_AgentPlatform/src/models/users"
	"AgentEarth_AgentPlatform/src/servers"
	"fmt"
	"time"
)

func yyyymmddKey(year, month, day int16) string {
	return fmt.Sprintf("%04d%02d%02d", year, month, day)
}

func dayKeyFromTime(t time.Time) string {
	return yyyymmddKey(int16(t.Year()), int16(t.Month()), int16(t.Day()))
}

func isoWeekStart(year, week int, loc *time.Location) time.Time {
	// ISO week 1 is the week with Jan 4th. Use that to find the Monday of week 1.
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
		periodKey = dayKeyFromTime(start)
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

// 检查调用次数和tokens使用量是否超标
func (a *AuthMiddleware) checkCallsAndTokens(apiKey, serverID string) bool {
	// 获取服务调用限制
	serverInfo := servers.McpServicesMap[serverID]
	if serverInfo == nil {
		return true
	}
	limit := serverInfo.LimitCalls
	if limit <= 0 {
		return true
	}

	now := time.Now()
	periodKey, periodStart, periodEndExcl, ok := periodFor(int(serverInfo.LimitType), now)
	if !ok {
		// 未识别的限制类型，放行
		return true
	}

	userID, keyID, ok := a.getUserInfo(apiKey)
	if !ok || userID == "" || keyID <= 0 {
		// 已经通过 ValidateAPIKey，这里理论上不该失败；失败时放行避免误杀
		return true
	}

	year, month, day := int16(now.Year()), int16(now.Month()), int16(now.Day())
	dayKey := yyyymmddKey(year, month, day)

	// 快路径：内存已有计数
	a.usageMu.Lock()
	if byServer, ok1 := a.usage[userID]; ok1 {
		if byDate, ok2 := byServer[serverID]; ok2 {
			if bucket := byDate[periodKey]; bucket != nil {
				// 用户级限流：bucket.calls 是 user+server+period 的总调用（跨 key 汇总）
				if bucket.calls+1 > limit {
					a.usageMu.Unlock()
					return false
				}
				bucket.calls++
				if bucket.byKeyDay == nil {
					bucket.byKeyDay = make(map[int32]map[string]*usageCounter)
				}
				m := bucket.byKeyDay[keyID]
				if m == nil {
					m = make(map[string]*usageCounter)
					bucket.byKeyDay[keyID] = m
				}
				uc := m[dayKey]
				if uc == nil {
					uc = &usageCounter{
						userID:   userID,
						keyID:    keyID,
						serverID: serverID,
						year:     year,
						month:    month,
						day:      day,
						calls:    0,
						dirty:    0,
					}
					m[dayKey] = uc
				}
				uc.calls++
				uc.dirty++
				a.usageMu.Unlock()
				return true
			}
		}
	}
	a.usageMu.Unlock()

	// 首次命中：从 DB 读取 period 用量作为基线（无记录则按 0）
	existingUserCalls, err := userModels.GetUserCallsSumByUSRange(userID, serverID, periodStart, periodEndExcl)
	if err != nil {
		// DB 异常时放行避免误杀
		return true
	}
	var existingKeyCalls int64
	{
		var row userModels.AeUserRequestServicesLogs
		if err := row.GetOneByUKSDate(userID, keyID, serverID, year, month, day); err == nil {
			existingKeyCalls = int64(row.Calls)
		}
	}

	// 写回内存并计数
	a.usageMu.Lock()
	// double-check
	byServer := a.usage[userID]
	if byServer == nil {
		byServer = make(map[string]map[string]*usageDayBucket)
		a.usage[userID] = byServer
	}
	byDate := byServer[serverID]
	if byDate == nil {
		byDate = make(map[string]*usageDayBucket)
		byServer[serverID] = byDate
	}
	bucket := byDate[periodKey]
	if bucket == nil {
		bucket = &usageDayBucket{
			limitType:     int(serverInfo.LimitType),
			periodKey:     periodKey,
			periodStart:   periodStart,
			periodEndExcl: periodEndExcl,
			calls:         existingUserCalls,
			byKeyDay:      make(map[int32]map[string]*usageCounter),
		}
		byDate[periodKey] = bucket
	}

	m := bucket.byKeyDay[keyID]
	if m == nil {
		m = make(map[string]*usageCounter)
		bucket.byKeyDay[keyID] = m
	}
	uc := m[dayKey]
	if uc == nil {
		uc = &usageCounter{
			userID:   userID,
			keyID:    keyID,
			serverID: serverID,
			year:     year,
			month:    month,
			day:      day,
			calls:    existingKeyCalls,
			dirty:    0,
		}
		m[dayKey] = uc
	}

	if bucket.calls+1 > limit {
		a.usageMu.Unlock()
		return false
	}
	bucket.calls++
	uc.calls++
	uc.dirty++
	a.usageMu.Unlock()
	return true
}

func (a *AuthMiddleware) setUserInfo(apiKey, userID string, keyID int32, ttl time.Duration) {
	a.userMu.Lock()
	a.userInfo[apiKey] = userInfoCacheEntry{
		userID:    userID,
		keyID:     keyID,
		expiresAt: time.Now().Add(ttl),
	}
	a.userMu.Unlock()
}

func (a *AuthMiddleware) getUserInfo(apiKey string) (string, int32, bool) {
	// 1) cache
	a.userMu.RLock()
	entry, ok := a.userInfo[apiKey]
	a.userMu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.userID, entry.keyID, true
	}
	if ok {
		// 过期清理
		a.userMu.Lock()
		delete(a.userInfo, apiKey)
		a.userMu.Unlock()
	}

	// 2) db fallback
	userKeysModel := userModels.McpUserKeys{}
	if err := userKeysModel.GetOneByKeyValue(apiKey); err != nil {
		return "", 0, false
	}
	a.setUserInfo(apiKey, userKeysModel.UserId, userKeysModel.Id, a.cacheTTL)
	return userKeysModel.UserId, userKeysModel.Id, true
}

func (a *AuthMiddleware) startUsageFlusher(interval time.Duration) {
	if interval <= 0 {
		return
	}
	a.usageMu.Lock()
	if a.usageFlushStop != nil {
		a.usageMu.Unlock()
		return
	}
	a.usageFlushStop = make(chan struct{})
	stop := a.usageFlushStop
	a.usageFlushWG.Add(1)
	a.usageMu.Unlock()

	go func() {
		defer a.usageFlushWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				a.flushUsageOnce()
				return
			case <-ticker.C:
				a.flushUsageOnce()
			}
		}
	}()
}

func (a *AuthMiddleware) flushUsageOnce() {
	type flushItem struct {
		userID    string
		keyID     int32
		serverID  string
		periodKey string
		dayKey    string
		year      int16
		month     int16
		day       int16
		calls     int32
	}
	items := make([]flushItem, 0, 64)

	// 提取 dirty 增量（先置 0，失败再回滚）
	a.usageMu.Lock()
	for _, byServer := range a.usage {
		for _, byPeriod := range byServer {
			for _, bucket := range byPeriod {
				if bucket == nil || bucket.byKeyDay == nil {
					continue
				}
				for keyID, byDay := range bucket.byKeyDay {
					for dayKey, uc := range byDay {
						if uc == nil || uc.dirty <= 0 {
							continue
						}
						d := uc.dirty
						uc.dirty = 0
						items = append(items, flushItem{
							userID:    uc.userID,
							keyID:     keyID,
							serverID:  uc.serverID,
							periodKey: bucket.periodKey,
							dayKey:    dayKey,
							year:      uc.year,
							month:     uc.month,
							day:       uc.day,
							calls:     int32(d),
						})
					}
				}
			}
		}
	}
	a.usageMu.Unlock()

	for _, it := range items {
		if err := userModels.AddUsageUpsert(it.userID, it.keyID, it.serverID, it.year, it.month, it.day, it.calls, 0); err != nil {
			// 回滚 dirty
			a.usageMu.Lock()
			if byServer, ok := a.usage[it.userID]; ok {
				if byDate, ok := byServer[it.serverID]; ok {
					if bucket := byDate[it.periodKey]; bucket != nil && bucket.byKeyDay != nil {
						if byDay := bucket.byKeyDay[it.keyID]; byDay != nil {
							if uc := byDay[it.dayKey]; uc != nil {
								uc.dirty += int64(it.calls)
							}
						}
					}
				}
			}
			a.usageMu.Unlock()
		}
	}

	// 清理历史 period bucket：只删除“period 已结束”且所有 key/day 都没有未 flush 的 dirty 的 bucket。
	// 目的：避免 a.usage 随运行时间无限增长。
	now := time.Now()
	a.usageMu.Lock()
	for userID, byServer := range a.usage {
		for serverID, byDate := range byServer {
			for pKey, bucket := range byDate {
				if bucket == nil {
					delete(byDate, pKey)
					continue
				}
				_, curStart, _, ok := periodFor(bucket.limitType, now)
				if !ok {
					// 未识别的 bucket，跳过
					continue
				}
				// 仅清理已结束的 period
				if bucket.periodEndExcl.After(curStart) {
					continue
				}
				canDelete := true
				if bucket.byKeyDay != nil {
					for _, byDay := range bucket.byKeyDay {
						for _, uc := range byDay {
							if uc != nil && uc.dirty > 0 {
								canDelete = false
								break
							}
						}
						if !canDelete {
							break
						}
					}
				}
				if canDelete {
					delete(byDate, pKey)
				}
			}
			if len(byDate) == 0 {
				delete(byServer, serverID)
			}
		}
		if len(byServer) == 0 {
			delete(a.usage, userID)
		}
	}
	a.usageMu.Unlock()
}
