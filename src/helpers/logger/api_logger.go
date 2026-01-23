package logger

// context key 类型定义，避免key冲突
type contextKey string

// apikeyname 和 userid字段输入context/查找定义 -- APILOG，可复用
const (
	ContextKeyApiKeyName contextKey = "api_log_apikey_name"
	ContextKeyUserID     contextKey = "api_log_user_id"
)
