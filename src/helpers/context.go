package helpers

// context key 类型定义，避免key冲突
type contextKey string

// ContextKeyApiKeyName 用于在 context 中存储 API Key 名称
const ContextKeyApiKeyName contextKey = "api_key_name"

// ContextKeyUserID 用于在 context 中存储用户 ID
const ContextKeyUserID contextKey = "user_id"
