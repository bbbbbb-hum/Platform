package middleware

import (
	"AgentEarth_AgentPlatform/src/helpers"
	cacheHelper "AgentEarth_AgentPlatform/src/helpers/cache"
	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type AuthMiddleware struct {
	Enable    bool   `json:"enable"`
	HeaderKey string `json:"header_key"`
	limiter   *LimitChecker
}

func NewAuth() *AuthMiddleware {

	m := &AuthMiddleware{
		Enable:    helperConfig.GetBool("server.auth_enable"),
		HeaderKey: helperConfig.GetString("server.auth_key"),
	}

	// 每 10 分钟同步一次用量到数据库
	m.limiter = NewLimitChecker()
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
		info, err := cacheHelper.GetUserKeyInfo(r.Context(), apiKey)
		if err != nil || info == nil {
			logger.Error(fmt.Sprintf("Authentication failed for request %s %s", r.Method, r.URL.Path))
			http.Error(w, "Unauthorized: API key is invalid or expired. Please use a different API key, or retry after 10 minutes.", http.StatusUnauthorized)
			return
		}

		// APILOG -- 将keyname和userID存入context
		ctx := r.Context()
		ctx = context.WithValue(ctx, helpers.ContextKeyApiKeyName, info.KeyName)
		ctx = context.WithValue(ctx, helpers.ContextKeyUserID, info.UserID)
		ctx = context.WithValue(ctx, "key_id", int64(info.KeyID))
		r = r.WithContext(ctx)

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
	info, err := cacheHelper.GetUserKeyInfo(context.Background(), apiKey)
	return err == nil && info != nil
}
