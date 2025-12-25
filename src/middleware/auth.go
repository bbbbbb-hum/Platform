package middleware

import (
	"AgentEarth_AgentPlatform/src/helpers"
	"AgentEarth_AgentPlatform/src/models/users"
	"fmt"
	"net/http"
	"sync"
	"time"

	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
)

type AuthMiddleware struct {
	Enable    bool   `json:"enable"`
	HeaderKey string `json:"header_key"`

	cacheMu  sync.RWMutex
	cache    map[string]apiKeyCacheEntry
	cacheTTL time.Duration

	userMu   sync.RWMutex
	userInfo map[string]userInfoCacheEntry // key: apiKey

	usageMu        sync.Mutex
	usage          map[string]map[string]map[string]*usageDayBucket // userID -> serverID -> yyyymmdd -> bucket
	usageFlushStop chan struct{}
	usageFlushWG   sync.WaitGroup
}

type apiKeyCacheEntry struct {
	valid     bool
	expiresAt time.Time
}

type userInfoCacheEntry struct {
	userID    string
	keyID     int32
	expiresAt time.Time
}

type usageCounter struct {
	userID   string
	keyID    int32
	serverID string
	year     int16
	month    int16
	day      int16

	calls int64
	dirty int64
}

// usageDayBucket holds per-user aggregation for enforcing user-level limits over a time period
// (day/week/month/quarter/year), while still keeping per-key-per-day counters for DB upsert
// (logs table is keyed by key_id + day).
type usageDayBucket struct {
	limitType     int
	periodKey     string
	periodStart   time.Time
	periodEndExcl time.Time

	// calls is the total calls for this user+server+period (summed across all keys).
	calls int64

	// byKeyDay: keyID -> dayKey(yyyymmdd) -> counter
	byKeyDay map[int32]map[string]*usageCounter
}

func NewAuth() *AuthMiddleware {

	m := &AuthMiddleware{
		Enable:    helperConfig.GetBool("server.auth_enable"),
		HeaderKey: helperConfig.GetString("server.auth_key"),
	}
	m.cache = make(map[string]apiKeyCacheEntry)
	m.userInfo = make(map[string]userInfoCacheEntry)
	m.usage = make(map[string]map[string]map[string]*usageDayBucket)
	// 允许通过配置覆盖缓存 TTL（秒），未设置则默认 5 分钟
	cacheTTLSeconds := helperConfig.GetInt("server.auth_cache_ttl")
	m.cacheTTL = time.Duration(cacheTTLSeconds) * time.Second

	// 每 10 分钟同步一次用量到数据库
	m.startUsageFlusher(10 * time.Minute)
	return m
}

func (a *AuthMiddleware) Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 如果认证未启用，直接通过
		if !a.Enable {
			logger.Info("Authentication disabled")
			next(w, r)
			return
		}

		// 使用 Header API Key
		apiKey := r.Header.Get(a.HeaderKey)

		// 验证API密钥
		if !a.ValidateAPIKey(apiKey) {
			logger.Error(fmt.Sprintf("Authentication failed for request %s %s", r.Method, r.URL.Path))
			http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
			return
		}

		// 调用次数限制（先只做按天 CallsLimit）
		serverID := helpers.ExtractServerID(r.URL.Path)
		if serverID != "" && !a.checkCallsAndTokens(apiKey, serverID) {
			http.Error(w, "Too Many Requests: Calls limit exceeded", http.StatusTooManyRequests)
			return
		}

		logger.Info(fmt.Sprintf("Authentication successful for request %s %s", r.Method, r.URL.Path))
		next(w, r)
	}
}

// ValidateAPIKey 验证API密钥
func (a *AuthMiddleware) ValidateAPIKey(apiKey string) bool {
	if apiKey == "" {
		logger.Error("API key is empty")
		return false
	}

	// 1) 先查缓存
	if isExpired, hit := a.getFromCache(apiKey); hit {
		return isExpired
	}

	// 检查API密钥是否在允许列表中
	userKeysModel := users.McpUserKeys{}
	err := userKeysModel.GetOneByKeyValue(apiKey)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to validate API key: %v", err))
		// 负缓存，短 TTL，避免打爆数据库
		a.setCache(apiKey, false, a.cacheTTL)
		return false
	}

	if userKeysModel.Id <= 0 {
		logger.Error("API key not found in database")
		a.setCache(apiKey, false, a.cacheTTL)
		return false
	}

	// 检查密钥状态
	if userKeysModel.Status != "active" {
		logger.Error(fmt.Sprintf("API key status is not active: %s", userKeysModel.Status))
		a.setCache(apiKey, false, a.cacheTTL)
		return false
	}

	// 缓存写入
	a.setCache(apiKey, true, a.cacheTTL)
	a.setUserInfo(apiKey, userKeysModel.UserId, userKeysModel.Id, a.cacheTTL)

	return true
}

// 缓存读：命中且未过期则返回（valid, true）；否则返回（false, false）
func (a *AuthMiddleware) getFromCache(apiKey string) (bool, bool) {
	a.cacheMu.RLock()
	entry, ok := a.cache[apiKey]
	a.cacheMu.RUnlock()
	if !ok {
		return false, false
	}
	if time.Now().After(entry.expiresAt) {
		// 过期，清理
		a.cacheMu.Lock()
		delete(a.cache, apiKey)
		a.cacheMu.Unlock()
		return false, false
	}
	return entry.valid, true
}

// 缓存写：设置指定 TTL
func (a *AuthMiddleware) setCache(apiKey string, valid bool, ttl time.Duration) {
	a.cacheMu.Lock()
	a.cache[apiKey] = apiKeyCacheEntry{
		valid:     valid,
		expiresAt: time.Now().Add(ttl),
	}
	a.cacheMu.Unlock()
}
