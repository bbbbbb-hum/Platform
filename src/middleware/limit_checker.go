package middleware

import (
	userModels "AgentEarth_AgentPlatform/src/models/users"
	"AgentEarth_AgentPlatform/src/servers"
	"fmt"
	"sync"
	"time"
)

type UsageCounter struct {
	userID   string
	keyID    int32
	serverID string
	year     int16
	month    int16
	day      int16

	calls  int64
	change bool // 是否有变化（calls 是否增长）；用于决定是否需要同步到 DB
}

// usageDayBucket 用于按「用户 + 服务 + 周期」聚合计数（用于用户级限流），同时保留
// 「key + 天」的计数器用于 DB upsert（日志表按 key_id + day 存储）。
type usageDayBucket struct {
	limitType     int
	periodKey     string
	periodStart   time.Time
	periodEndExcl time.Time

	// calls 是 user+server 在不同周期内的总调用（跨 key 汇总），根据 limitType 选择使用哪一个字段判断上限。
	dayCalls        int64 // 日
	weekCalls       int64 // 周
	monthCalls      int64 // 月
	threeMonthCalls int64 // 季度
	yearCalls       int64 // 年

	// byKeyDay: keyID -> dayKey(yyyymmdd) -> counter
	byKeyDay map[int32]map[string]*UsageCounter
}

func (b *usageDayBucket) callsPtr(limitType int) *int64 {
	switch limitType {
	case 1:
		return &b.dayCalls
	case 2:
		return &b.weekCalls
	case 3:
		return &b.monthCalls
	case 4:
		return &b.threeMonthCalls
	case 5:
		return &b.yearCalls
	default:
		// 未识别类型：默认按天
		return &b.dayCalls
	}
}

// LimitChecker 是限流核心对象：持有内存用量 map、互斥锁，以及定时 flush 到数据库的协程控制信息。
type LimitChecker struct {
	userCache *UserCache

	usageMu        sync.Mutex
	usage          map[string]map[string]map[string]*usageDayBucket // userID -> serverID -> periodKey -> bucket
	usageFlushStop chan struct{}
	usageFlushWG   sync.WaitGroup
}

func NewLimitChecker(userCache *UserCache) *LimitChecker {
	return &LimitChecker{
		userCache: userCache,
		usage:     make(map[string]map[string]map[string]*usageDayBucket),
	}
}

func yyyymmddKey(year, month, day int16) string {
	return fmt.Sprintf("%04d%02d%02d", year, month, day)
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

	if c.userCache == nil {
		return true
	}
	userID, keyID, _, _, ok := c.userCache.Get(apiKey)
	if !ok || userID == "" || keyID <= 0 {
		return true
	}

	year, month, day := int16(now.Year()), int16(now.Month()), int16(now.Day())
	dayKey := yyyymmddKey(year, month, day)

	// 快路径：命中内存 bucket 直接判断 + 自增；未命中则走慢路径（DB 基线）
	c.usageMu.Lock()
	handled, allowed := c.tryFastPathLocked(userID, keyID, serverID, periodKey, dayKey, limitType, limit)
	c.usageMu.Unlock()
	if handled {
		return allowed
	}

	// 慢路径：内存不存在，先查 DB 得到 period 基线，再写入内存并计数
	existingUserCalls, err := userModels.GetUserCallsSumByUSRange(userID, serverID, periodStart, periodEndExcl)
	if err != nil {
		return true
	}
	var existingKeyCalls int64
	{
		var urs userModels.AeUserRequestServicesLogs
		if err = urs.GetOneByUKSDate(userID, keyID, serverID, year, month, day); err == nil {
			existingKeyCalls = int64(urs.Calls)
		}
	}

	c.usageMu.Lock()
	allowed = c.applySlowPathLocked(
		userID, keyID, serverID,
		periodKey, dayKey,
		periodStart, periodEndExcl,
		year, month, day,
		limitType,
		limit,
		existingUserCalls,
		existingKeyCalls,
	)
	c.usageMu.Unlock()
	return allowed
}

// tryFastPathLocked 尝试快路径（必须在已持有 c.usageMu 的前提下调用）。
// 返回 handled=true 表示已命中内存 bucket 并完成判断/自增；allowed 表示是否放行。
func (c *LimitChecker) tryFastPathLocked(
	userID string,
	keyID int32,
	serverID string,
	periodKey string,
	dayKey string,
	limitType int,
	limit int64,
) (handled bool, allowed bool) {
	byServer, ok := c.usage[userID]
	if !ok {
		return false, false
	}
	byPeriod, ok := byServer[serverID]
	if !ok {
		return false, false
	}
	bucket := byPeriod[periodKey]
	if bucket == nil {
		return false, false
	}

	callsPtr := bucket.callsPtr(limitType)
	if *callsPtr+1 > limit {
		return true, false
	}

	if bucket.byKeyDay == nil {
		bucket.byKeyDay = make(map[int32]map[string]*UsageCounter)
	}
	usageCounterMap := bucket.byKeyDay[keyID]
	if usageCounterMap == nil {
		usageCounterMap = make(map[string]*UsageCounter)
		bucket.byKeyDay[keyID] = usageCounterMap
	}
	usageCounter := usageCounterMap[dayKey]
	if usageCounter == nil {
		// 全量同步模式下，UsageCounter.calls 必须有 DB 基线，否则会把 DB 覆盖成过小的值。
		// key/day counter 缺失时走慢路径查询 DB 后再创建。
		return false, false
	}
	*callsPtr++
	usageCounter.calls++
	usageCounter.change = true
	return true, true
}

// applySlowPathLocked 慢路径：把 DB 基线写入内存并执行本次 +1 计数（必须在已持有 c.usageMu 的前提下调用）。
// 返回 true 表示放行，false 表示超过上限。
func (c *LimitChecker) applySlowPathLocked(
	userID string,
	keyID int32,
	serverID string,
	periodKey string,
	dayKey string,
	periodStart time.Time,
	periodEndExcl time.Time,
	year, month, day int16,
	limitType int,
	limit int64,
	existingUserCalls int64,
	existingKeyCalls int64,
) bool {
	byServer := c.usage[userID]
	if byServer == nil {
		byServer = make(map[string]map[string]*usageDayBucket)
		c.usage[userID] = byServer
	}
	byPeriod := byServer[serverID]
	if byPeriod == nil {
		byPeriod = make(map[string]*usageDayBucket)
		byServer[serverID] = byPeriod
	}
	bucket := byPeriod[periodKey]
	if bucket == nil {
		bucket = &usageDayBucket{
			limitType:     limitType,
			periodKey:     periodKey,
			periodStart:   periodStart,
			periodEndExcl: periodEndExcl,
			byKeyDay:      make(map[int32]map[string]*UsageCounter),
		}
		// 初始化当前 limitType 对应的 period 基线
		*bucket.callsPtr(limitType) = existingUserCalls
		byPeriod[periodKey] = bucket
	}
	m := bucket.byKeyDay[keyID]
	if m == nil {
		m = make(map[string]*UsageCounter)
		bucket.byKeyDay[keyID] = m
	}
	usageCounter := m[dayKey]
	if usageCounter == nil {
		usageCounter = &UsageCounter{
			userID:   userID,
			keyID:    keyID,
			serverID: serverID,
			year:     year,
			month:    month,
			day:      day,
			calls:    existingKeyCalls,
			change:   false,
		}
		m[dayKey] = usageCounter
	}

	callsPtr := bucket.callsPtr(limitType)
	if *callsPtr+1 > limit {
		return false
	}
	*callsPtr++
	usageCounter.calls++
	usageCounter.change = true
	return true
}

func (c *LimitChecker) StartUsageFlusher(interval time.Duration) {
	if interval <= 0 {
		return
	}
	c.usageMu.Lock()
	if c.usageFlushStop != nil {
		c.usageMu.Unlock()
		return
	}
	c.usageFlushStop = make(chan struct{})
	stop := c.usageFlushStop
	c.usageFlushWG.Add(1)
	c.usageMu.Unlock()

	go func() {
		defer c.usageFlushWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				c.flushUsageOnce()
				return
			case <-ticker.C:
				c.flushUsageOnce()
			}
		}
	}()
}

func (c *LimitChecker) flushUsageOnce() {
	type flushItem struct {
		userID    string
		keyID     int32
		serverID  string
		periodKey string
		dayKey    string
		year      int16
		month     int16
		day       int16
		callsAbs  int32
	}
	items := make([]flushItem, 0, 64)

	// 提取需要同步的计数器（先置 change=false，写库失败再回滚为 true）
	c.usageMu.Lock()
	for _, byServer := range c.usage {
		for _, byPeriod := range byServer {
			for _, bucket := range byPeriod {
				if bucket == nil || bucket.byKeyDay == nil {
					continue
				}
				for keyID, byDay := range bucket.byKeyDay {
					for dayKey, usageCounter := range byDay {
						if usageCounter == nil || !usageCounter.change {
							continue
						}
						callsAbs := usageCounter.calls
						usageCounter.change = false
						items = append(items, flushItem{
							userID:    usageCounter.userID,
							keyID:     keyID,
							serverID:  usageCounter.serverID,
							periodKey: bucket.periodKey,
							dayKey:    dayKey,
							year:      usageCounter.year,
							month:     usageCounter.month,
							day:       usageCounter.day,
							callsAbs:  int32(callsAbs),
						})
					}
				}
			}
		}
	}
	c.usageMu.Unlock()

	for _, it := range items {
		if err := userModels.SetUsageUpsert(it.userID, it.keyID, it.serverID, it.year, it.month, it.day, it.callsAbs, 0); err != nil {
			// 写库失败：回滚 change=true（不回滚 calls，calls 是内存权威总量）
			c.usageMu.Lock()
			if byServer, ok := c.usage[it.userID]; ok {
				if byDate, ok := byServer[it.serverID]; ok {
					if bucket := byDate[it.periodKey]; bucket != nil && bucket.byKeyDay != nil {
						if byDay := bucket.byKeyDay[it.keyID]; byDay != nil {
							if usageCounter := byDay[it.dayKey]; usageCounter != nil {
								usageCounter.change = true
							}
						}
					}
				}
			}
			c.usageMu.Unlock()
		}
	}

	// 清理已结束的 period bucket（只有全部 change==false 才会删除，避免丢同步）
	now := time.Now()
	c.usageMu.Lock()
	for userID, byServer := range c.usage {
		for serverID, byDate := range byServer {
			for pKey, bucket := range byDate {
				if bucket == nil {
					delete(byDate, pKey)
					continue
				}
				_, curStart, _, ok := periodFor(bucket.limitType, now)
				if !ok {
					continue
				}
				if bucket.periodEndExcl.After(curStart) {
					continue
				}
				canDelete := true
				if bucket.byKeyDay != nil {
					for _, byDay := range bucket.byKeyDay {
						for _, usageCounter := range byDay {
							if usageCounter != nil && usageCounter.change {
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
			delete(c.usage, userID)
		}
	}
	c.usageMu.Unlock()
}
