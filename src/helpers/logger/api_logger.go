package logger

import (
	"context"

	"go.uber.org/zap"
)

// context key 类型定义，避免key冲突
type contextKey string

// 日志字段key定义（导出，供其他包直接使用）
const (
	ContextKeyLogType      contextKey = "api_log_log_type"
	ContextKeyApiKeyName   contextKey = "api_log_apikey_name"
	ContextKeyServiceName  contextKey = "api_log_service_name"
	ContextKeyMethod       contextKey = "api_log_method"
	ContextKeyParam        contextKey = "api_log_param"
	ContextKeyStatusCode   contextKey = "api_log_status_code"
	ContextKeyMessage      contextKey = "api_log_message"
	ContextKeyResponseData contextKey = "api_log_response_data"
	ContextKeyApiLogTime   contextKey = "api_log_duration_ms"
)

// getStringFromContext 从context获取string值
func getStringFromContext(ctx context.Context, key contextKey) string {
	if val, ok := ctx.Value(key).(string); ok {
		return val
	}
	return ""
}

// getInterfaceFromContext 从context获取interface{}值
func getInterfaceFromContext(ctx context.Context, key contextKey) interface{} {
	return ctx.Value(key)
}

// LogAPICall 从context获取所有字段并打印API调用日志
func LogAPICall(ctx context.Context) {
	method := getStringFromContext(ctx, ContextKeyMethod)

	// 只有method字段不为空时，才说明是工具调用，才打印日志
	if method == "" {
		return
	}

	// 获取所有字段值
	logType := getStringFromContext(ctx, ContextKeyLogType)
	if logType == "" {
		logType = "AgentGWCall"
	}
	apiKeyName := getStringFromContext(ctx, ContextKeyApiKeyName)
	serviceName := getStringFromContext(ctx, ContextKeyServiceName)
	param := getInterfaceFromContext(ctx, ContextKeyParam)
	statusCode := getStringFromContext(ctx, ContextKeyStatusCode)
	message := getStringFromContext(ctx, ContextKeyMessage)
	responseData := getInterfaceFromContext(ctx, ContextKeyResponseData)
	durationMs := getStringFromContext(ctx, ContextKeyApiLogTime)

	// 按照顺序组合日志，并进行日志输出
	Logger.Info("MCP服务日志",
		zap.String("log_type", logType),         // 0. 必须存在的字段 -- "AgentGWCall"
		zap.String("apikey_name", apiKeyName),   // 1. apikey的名称
		zap.String("service_name", serviceName), // 2. 访问的服务名称
		zap.String("method", method),            // 3. 访问的服务中的工具名称
		zap.Any("param", param),                 // 4. 访问服务需要的参数列表
		zap.String("status_code", statusCode),   // 5. 返回给用户的状态码
		zap.String("msg", message),              // 6. 返回给用户的消息
		zap.Any("response_data", responseData),  // 7. 返回给用户的内容
		zap.String("duration_ms", durationMs),   // 8. 调用时间 -- ms
	)
}
