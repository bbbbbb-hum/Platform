package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	redisHelper "AgentEarth_AgentPlatform/src/helpers/redis"
	userModels "AgentEarth_AgentPlatform/src/models/users"
)

// GetUserKeyInfo returns user info for the apiKey.
// It reads from Redis first, then falls back to DB and caches the result.
// If not found/invalid, it returns (nil, nil).
func GetUserKeyInfo(ctx context.Context, apiKey string) (*UserKeyInfo, error) {
	if apiKey == "" {
		return nil, errors.New("apiKey is empty")
	}
	// 1) cache
	key := KeyUserKey(apiKey)
	val, ok, err := redisHelper.GetString(ctx, key)
	if err != nil {
		return nil, err
	}
	if ok {
		var info UserKeyInfo
		if jsonErr := json.Unmarshal([]byte(val), &info); jsonErr == nil {
			return &info, nil
		}
	}
	// 2) invalid 负缓存 30 秒 防止短时间无效apikey访问数据库
	invalidKey := KeyUserKeyInvalid(apiKey)
	invalidVal, invalidOk, invalidErr := redisHelper.GetString(ctx, invalidKey)
	if invalidErr != nil {
		return nil, invalidErr
	}
	if invalidOk && invalidVal != "" {
		return nil, nil
	}

	// 3) db fallback
	userKeysModel := userModels.McpUserKeys{}
	if err = userKeysModel.GetOneByKeyValue(apiKey); err != nil {
		_ = redisHelper.SetString(ctx, invalidKey, "1", invalidCacheTTL())
		return nil, nil
	}
	if userKeysModel.Id <= 0 {
		_ = redisHelper.SetString(ctx, invalidKey, "1", invalidCacheTTL())
		return nil, nil
	}
	if userKeysModel.Status != "active" {
		_ = redisHelper.SetString(ctx, invalidKey, "1", invalidCacheTTL())
		return nil, nil
	}
	if !userKeysModel.ExpiresAt.IsZero() && time.Now().After(userKeysModel.ExpiresAt) {
		_ = redisHelper.SetString(ctx, invalidKey, "1", invalidCacheTTL())
		return nil, nil
	}

	info := UserKeyInfo{
		UserID:  userKeysModel.UserId,
		KeyID:   userKeysModel.Id,
		KeyName: userKeysModel.KeyName,
	}
	_ = SetUserKeyInfo(ctx, apiKey, info)
	return &info, nil
}

// SetUserKeyInfo caches user info for the apiKey.
func SetUserKeyInfo(ctx context.Context, apiKey string, info UserKeyInfo) error {
	if apiKey == "" || info.UserID == "" || info.KeyID <= 0 {
		return errors.New("invalid user key info")
	}
	key := KeyUserKey(apiKey)
	payload, err := json.Marshal(info)
	if err != nil {
		return err
	}
	ttl := userCacheTTL()
	return redisHelper.SetString(ctx, key, string(payload), ttl)
}

func userCacheTTL() time.Duration {
	ttlSeconds := helperConfig.GetInt("server.auth_cache_ttl")
	if ttlSeconds <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(ttlSeconds) * time.Second
}

func invalidCacheTTL() time.Duration {
	return 30 * time.Second
}
