package middleware

import (
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/models/users"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
)

type AuthMiddleware struct {
	Enable           bool   `json:"enable"`
	ServerKey        string `json:"server_key"`
	HeaderKey        string `json:"header_key"`
	CookieName       string `json:"cookie_name"`
	CookieTTLSeconds int    `json:"cookie_ttl_seconds"`
}

func NewAuth() *AuthMiddleware {

	m := &AuthMiddleware{
		Enable:    helperConfig.GetBool("server.auth_enable"),
		ServerKey: helperConfig.GetString("server.key"),
		HeaderKey: helperConfig.GetString("server.auth_key"),
	}
	// cookie 配置（带默认值）
	cn := helperConfig.GetString("server.auth_cookie_name")
	if cn == "" {
		cn = "mcp_session"
	}
	m.CookieName = cn
	ttl := helperConfig.GetInt("server.auth_cookie_ttl_seconds")
	if ttl <= 0 {
		ttl = 3600 // 30 minutes default
	}
	m.CookieTTLSeconds = ttl
	return m
}

func (a *AuthMiddleware) Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 如果认证未启用，直接通过
		if !a.Enable {
			next(w, r)
			return
		}

		// 优先使用已签名的会话 Cookie（减少读库）
		if a.ServerKey != "" && a.CookieName != "" {
			if c, err := r.Cookie(a.CookieName); err == nil && c != nil && c.Value != "" {
				if ok := a.verifySessionCookie(c.Value); ok {
					logger.Info(fmt.Sprintf("Authentication via cookie for request %s %s", r.Method, r.URL.Path))
					next(w, r)
					return
				}
			}
		}

		// 回退到 Header API Key
		apiKey := r.Header.Get(a.HeaderKey)

		// 验证API密钥
		if !a.ValidateAPIKey(apiKey) {
			logger.Error(fmt.Sprintf("Authentication failed for request %s %s", r.Method, r.URL.Path))
			http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
			return
		}

		// 颁发会话 Cookie，后续请求可绕过读库（需要有 ServerKey 以签名）
		if a.ServerKey != "" && a.CookieName != "" {
			if cookie := a.generateSessionCookie(apiKey); cookie != nil {
				http.SetCookie(w, cookie)
			}
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

// 生成签名会话 Cookie（包含到期时间，HMAC-SHA256 签名）
func (a *AuthMiddleware) generateSessionCookie(apiKey string) *http.Cookie {
	if a.ServerKey == "" || a.CookieName == "" || a.CookieTTLSeconds <= 0 {
		return nil
	}
	exp := time.Now().Add(time.Duration(a.CookieTTLSeconds) * time.Second).Unix()
	apiKeyB64 := base64.RawURLEncoding.EncodeToString([]byte(apiKey))
	payload := strings.Join([]string{"v1", strconv.FormatInt(exp, 10), apiKeyB64}, "|")
	sig := a.hmacSign(payload)
	token := payload + "|" + sig
	return &http.Cookie{
		Name:     a.CookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Unix(exp, 0),
		MaxAge:   a.CookieTTLSeconds,
		HttpOnly: true,
		// 注意：若服务走 HTTPS，建议将 Secure 设为 true
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}
}

// 校验签名会话 Cookie 是否有效且未过期
func (a *AuthMiddleware) verifySessionCookie(token string) bool {
	parts := strings.Split(token, "|")
	if len(parts) != 4 {
		return false
	}
	version := parts[0]
	expStr := parts[1]
	payload := strings.Join(parts[0:3], "|")
	sig := parts[3]
	if version != "v1" {
		return false
	}
	if !hmac.Equal([]byte(sig), []byte(a.hmacSign(payload))) {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > exp {
		return false
	}
	return true
}

func (a *AuthMiddleware) hmacSign(s string) string {
	m := hmac.New(sha256.New, []byte(a.ServerKey))
	m.Write([]byte(s))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}
