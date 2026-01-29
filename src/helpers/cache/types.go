package cache

// UserKeyInfo stores apiKey-bound user info for auth and limits.
type UserKeyInfo struct {
	UserID  string `json:"user_id"`
	KeyID   int32  `json:"key_id"`
	KeyName string `json:"key_name"`
}

// UsageCheckResult represents limit check result.
type UsageCheckResult struct {
	Allowed bool
	Current int64
	Limit   int64
}
