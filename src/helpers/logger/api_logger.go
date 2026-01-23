package logger

import (
	"context"

	"go.uber.org/zap"
)

// context key 类型定义，避免key冲突
type contextKey string

// 1. 日志字段key定义 -- 用于context定位
const (
	contextKeyLogType      contextKey = "api_log_log_type"
	contextKeyApiKeyName   contextKey = "api_log_apikey_name"
	contextKeyServiceName  contextKey = "api_log_service_name"
	contextKeyMethod       contextKey = "api_log_method"
	contextKeyParam        contextKey = "api_log_param"
	contextKeyStatusCode   contextKey = "api_log_status_code"
	contextKeyMessage      contextKey = "api_log_message"
	contextKeyResponseData contextKey = "api_log_response_data"
	contextKeyApiLogTime   contextKey = "api_log_duration_ms"
)

// SetLogType 设置日志类型到context
func SetLogType(ctx context.Context, logType string) context.Context {
	return context.WithValue(ctx, contextKeyLogType, logType)
}

// SetApiKeyName 设置apikey名称到context
func SetApiKeyName(ctx context.Context, apiKeyName string) context.Context {
	return context.WithValue(ctx, contextKeyApiKeyName, apiKeyName)
}

// SetServiceName 设置服务名称到context
func SetServiceName(ctx context.Context, serviceName string) context.Context {
	return context.WithValue(ctx, contextKeyServiceName, serviceName)
}

// SetMethod 设置方法名称到context
func SetMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, contextKeyMethod, method)
}

// SetParam 设置参数到context
func SetParam(ctx context.Context, param interface{}) context.Context {
	return context.WithValue(ctx, contextKeyParam, param)
}

// SetStatusCode 设置状态码到context
func SetStatusCode(ctx context.Context, statusCode string) context.Context {
	return context.WithValue(ctx, contextKeyStatusCode, statusCode)
}

// SetMessage 设置消息到context
func SetMessage(ctx context.Context, message string) context.Context {
	return context.WithValue(ctx, contextKeyMessage, message)
}

// SetResponseData 设置响应数据到context
func SetResponseData(ctx context.Context, responseData interface{}) context.Context {
	return context.WithValue(ctx, contextKeyResponseData, responseData)
}

// SetApiLogTime 设置调用耗时到context
func SetApiLogTime(ctx context.Context, durationMs string) context.Context {
	return context.WithValue(ctx, contextKeyApiLogTime, durationMs)
}

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

// 2. 从context中获取不同的字段，并进行日志输出
func LogAPICall(ctx context.Context) {
	method := getStringFromContext(ctx, contextKeyMethod)

	// 只有method字段不为空时，才说明是工具调用，才打印日志
	if method == "" {
		return
	}

	// 获取所有字段值
	logType := getStringFromContext(ctx, contextKeyLogType)
	if logType == "" {
		logType = "AgentGWCall"
	}
	apiKeyName := getStringFromContext(ctx, contextKeyApiKeyName)
	serviceName := getStringFromContext(ctx, contextKeyServiceName)
	param := getInterfaceFromContext(ctx, contextKeyParam)
	statusCode := getStringFromContext(ctx, contextKeyStatusCode)
	message := getStringFromContext(ctx, contextKeyMessage)
	responseData := getInterfaceFromContext(ctx, contextKeyResponseData)
	durationMs := getStringFromContext(ctx, contextKeyApiLogTime)

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
