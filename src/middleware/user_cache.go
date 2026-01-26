package middleware

import (
	userModels "AgentEarth_AgentPlatform/src/models/users"
	"sync"
	"time"
)

// UserProvider resolves apiKey -> (userID, keyID, keyName, userAccount).
type UserProvider interface {
	Get(apiKey string) (userID string, keyID int32, keyName string, userAccount float64, ok bool)
}
type userCacheEntry struct {
	valid       bool
	userID      string
	keyID       int32
	KeyName     string
	userAccount float64 //用户账户余额
	expiresAt   time.Time
}

var globalUserCache *UserCache

// SetGlobalUserCache registers a process-wide user cache.
func SetGlobalUserCache(cache *UserCache) {
	if cache == nil {
		return
	}
	globalUserCache = cache
}

// GetGlobalUserCache returns the process-wide user cache, if set.
func GetGlobalUserCache() *UserCache {
	return globalUserCache
}

// UserCache is a small cache for apiKey -> (userID, keyID).
// It is independent from AuthMiddleware and can be shared by multiple checkers.
type UserCache struct {
	mu     sync.RWMutex
	posTTL time.Duration
	negTTL time.Duration
	m      map[string]userCacheEntry
}

func NewUserCache(posTTL, negTTL time.Duration) *UserCache {
	if posTTL <= 0 {
		posTTL = 5 * time.Minute
	}
	if negTTL <= 0 {
		negTTL = 10 * time.Minute
	}
	return &UserCache{
		posTTL: posTTL,
		negTTL: negTTL,
		m:      make(map[string]userCacheEntry),
	}
}

func (c *UserCache) Get(apiKey string) (string, int32, string, float64, bool) {
	if apiKey == "" {
		return "", 0, "", 0, false
	}
	// 1) cache
	c.mu.RLock()
	entry, ok := c.m[apiKey]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		if !entry.valid {
			return "", 0, "", 0, false
		}
		return entry.userID, entry.keyID, entry.KeyName, entry.userAccount, true
	}
	if ok {
		c.mu.Lock()
		delete(c.m, apiKey)
		c.mu.Unlock()
	}

	// 2) db fallback（后期这里查询DB的时候，可以获取用户的账户余额，写入缓存）
	userKeysModel := userModels.McpUserKeys{}
	if err := userKeysModel.GetOneByKeyValue(apiKey); err != nil {
		// Negative-cache not-found only; avoid poisoning cache on transient DB issues.
		if userModels.IsNotFound(err) {
			c.mu.Lock()
			c.m[apiKey] = userCacheEntry{
				valid:     false,
				expiresAt: time.Now().Add(c.negTTL),
			}
			c.mu.Unlock()
		}
		return "", 0, "", 0, false
	}
	if userKeysModel.Id <= 0 {
		c.mu.Lock()
		c.m[apiKey] = userCacheEntry{
			valid:     false,
			expiresAt: time.Now().Add(c.negTTL),
		}
		c.mu.Unlock()
		return "", 0, "", 0, false
	}
	// status must be active
	if userKeysModel.Status != "active" {
		c.mu.Lock()
		c.m[apiKey] = userCacheEntry{
			valid:     false,
			expiresAt: time.Now().Add(c.negTTL),
		}
		c.mu.Unlock()
		return "", 0, "", 0, false
	}
	// expires_at: zero means no expiry
	if !userKeysModel.ExpiresAt.IsZero() && time.Now().After(userKeysModel.ExpiresAt) {
		c.mu.Lock()
		c.m[apiKey] = userCacheEntry{
			valid:     false,
			expiresAt: time.Now().Add(c.negTTL),
		}
		c.mu.Unlock()
		return "", 0, "", 0, false
	}

	// userAccount 暂时未接真实余额来源（后续可在这里按 user_id 查询用户账户表后填充）
	var userAccount float64 = 0
	c.mu.Lock()
	c.m[apiKey] = userCacheEntry{
		valid:       true,
		userID:      userKeysModel.UserId,
		keyID:       userKeysModel.Id,
		KeyName:     userKeysModel.KeyName,
		userAccount: userAccount,
		expiresAt:   time.Now().Add(c.posTTL),
	}
	c.mu.Unlock()
	return userKeysModel.UserId, userKeysModel.Id, userKeysModel.KeyName, userAccount, true
}

// Put inserts/updates cache entry for apiKey -> (userID, keyID) using the cache TTL.
// This is useful when the caller already queried DB (e.g. ValidateAPIKey) and wants to avoid
// a second DB hit on the next Get().
func (c *UserCache) Put(apiKey, userID string, keyID int32, keyName string, userAccount float64) {
	if c == nil || apiKey == "" || userID == "" || keyID <= 0 {
		return
	}
	c.mu.Lock()
	c.m[apiKey] = userCacheEntry{
		valid:       true,
		userID:      userID,
		keyID:       keyID,
		KeyName:     keyName,
		userAccount: userAccount,
		expiresAt:   time.Now().Add(c.posTTL),
	}
	c.mu.Unlock()
}
