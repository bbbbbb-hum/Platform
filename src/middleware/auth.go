package middleware

import (
	"AgentEarth_AgentPlatform/src/helpers"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"

	"go.uber.org/zap"
)

type AuthMiddleware struct {
	Enable    bool   `json:"enable"`
	HeaderKey string `json:"header_key"`
	userCache *UserCache
	limiter   *LimitChecker
}

func NewAuth() *AuthMiddleware {

	m := &AuthMiddleware{
		Enable:    helperConfig.GetBool("server.auth_enable"),
		HeaderKey: helperConfig.GetString("server.auth_key"),
	}

	// 允许通过配置覆盖缓存 TTL（秒），未设置则默认 5 分钟
	cacheTTLSeconds := helperConfig.GetInt("server.auth_cache_ttl")
	userCacheTTL := time.Duration(cacheTTLSeconds) * time.Second
	// Negative cache TTL: 10 minutes
	m.userCache = NewUserCache(userCacheTTL, 10*time.Minute)

	// 每 10 分钟同步一次用量到数据库
	m.limiter = NewLimitChecker(m.userCache)
	m.limiter.StartUsageFlusher(10 * time.Minute)
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

		// 获取apikey名称和userID
		apiKeyName := ""
		userID := ""
		if a.userCache != nil {
			uid, _, keyName, _, _ := a.userCache.Get(apiKey)
			apiKeyName = keyName
			userID = uid
		}

		// 验证API密钥
		if !a.ValidateAPIKey(apiKey) {
			logger.Error(fmt.Sprintf("Authentication failed for request %s %s", r.Method, r.URL.Path))
			http.Error(w, "Unauthorized: API key is invalid or expired. Please use a different API key, or retry after 10 minutes.", http.StatusUnauthorized)
			return
		}

		// APILOG -- 将keyname和userID存入context
		if strings.HasPrefix(r.URL.Path, "/mcp-server/") {
			ctx := r.Context()
			ctx = context.WithValue(ctx, logger.ContextKeyApiKeyName, apiKeyName)
			ctx = context.WithValue(ctx, logger.ContextKeyUserID, userID)
			r = r.WithContext(ctx)
		}

		// 调用次数限制
		serverID := helpers.ExtractServerID(r.URL.Path)
		if serverID != "" && a.limiter != nil && !a.limiter.CheckUsage(apiKey, serverID) {
			logger.Error("Too Many Requests", zap.String("apiKey", apiKey), zap.String("serverID", serverID))
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

	if a.userCache == nil {
		return false
	}
	_, _, _, _, ok := a.userCache.Get(apiKey)
	return ok
}
