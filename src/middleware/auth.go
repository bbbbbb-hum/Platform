package middleware

import (
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/models/users"
	"fmt"
	"net/http"
	"time"

	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
)

type AuthMiddleware struct {
	Enable    bool   `json:"enable"`
	ServerKey string `json:"server_key"`
	HeaderKey string `json:"header_key"`
}

func NewAuth() *AuthMiddleware {

	return &AuthMiddleware{
		Enable:    helperConfig.GetBool("server.auth_enable"),
		ServerKey: helperConfig.GetString("server.key"),
		HeaderKey: helperConfig.GetString("server.auth_key"),
	}
}

func (a *AuthMiddleware) Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 如果认证未启用，直接通过
		if !a.Enable {
			next(w, r)
			return
		}

		// 获取API密钥
		apiKey := r.Header.Get(a.HeaderKey)

		// 验证API密钥
		if !a.ValidateAPIKey(apiKey) {
			logger.Error(fmt.Sprintf("Authentication failed for request %s %s", r.Method, r.URL.Path))
			http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
			return
		}
		logger.Info(fmt.Sprintf("Authentication successful for request %s %s", r.Method, r.URL.Path))
		next(w, r)
	}
}

// ValidateAPIKey 验证API密钥
func (a *AuthMiddleware) ValidateAPIKey(apiKey string) bool {
	if !a.Enable {
		return true // 认证未启用，直接通过
	}

	if apiKey == "" {
		logger.Error("API key is empty")
		return false
	}

	// 检查API密钥是否在允许列表中
	userKeysModel := users.McpUserKeys{}
	err := userKeysModel.GetOneByKeyValue(apiKey)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to validate API key: %v", err))
		return false
	}

	if userKeysModel.Id <= 0 {
		logger.Error("API key not found in database")
		return false
	}

	// 检查密钥状态
	if userKeysModel.Status != "active" {
		logger.Error(fmt.Sprintf("API key status is not active: %s", userKeysModel.Status))
		return false
	}

	// 检查密钥是否过期
	if !userKeysModel.ExpiresAt.IsZero() && time.Now().After(userKeysModel.ExpiresAt) {
		logger.Error("API key has expired")
		return false
	}

	// 异步更新使用统计
	go func() {
		updateModel := users.McpUserKeys{}
		if err := updateModel.GetOneByKeyValue(apiKey); err == nil {
			updateModel.LastUsedAt = time.Now()
			updateModel.UsageCount++
			models.GetDB().Save(&updateModel)
		}
	}()

	return true
}
