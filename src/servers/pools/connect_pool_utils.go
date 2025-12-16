package pools

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// 启动本地服务
func (c *ConnectionPool) startLocalService(ctx context.Context, launchInfo *LaunchInfo) (err error) {
	if launchInfo == nil {
		return fmt.Errorf("launchInfo 不能为空")
	}
	if strings.TrimSpace(launchInfo.Command) == "" {
		return fmt.Errorf("launchInfo.command 不能为空")
	}

	// 组装命令
	cmd := exec.CommandContext(ctx, launchInfo.Command, launchInfo.Args...)
	if strings.TrimSpace(launchInfo.Workdir) != "" {
		cmd.Dir = launchInfo.Workdir
	}

	// 环境变量：继承父进程 + 追加自定义 env
	cmd.Env = os.Environ()
	if len(launchInfo.Env) > 0 {
		for k, v := range launchInfo.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, fmt.Sprint(v)))
		}
	}

	// 日志落盘：storage/logs/local_services/<cmd>-<ts>.log
	logDir := filepath.Join("storage", "logs", "local_services")
	_ = os.MkdirAll(logDir, 0o755)
	logName := fmt.Sprintf("%s-%s.log", filepath.Base(launchInfo.Command), time.Now().Format("20060102-150405"))
	logPath := filepath.Join(logDir, logName)
	logFile, ferr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if ferr != nil {
		// 日志文件失败不阻塞启动（仍然启动，只是输出到默认 stderr）
		logger.Warn("本地服务日志文件创建失败，将使用默认输出", zap.String("path", logPath), zap.Error(ferr))
	} else {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	// 启动进程
	if err = cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return fmt.Errorf("启动本地服务失败: %w", err)
	}

	logger.Info("本地服务进程已启动",
		zap.String("command", launchInfo.Command),
		zap.Strings("args", launchInfo.Args),
		zap.String("workdir", cmd.Dir),
		zap.Int("pid", cmd.Process.Pid),
		zap.String("log_path", logPath))

	// 启动后快速失败检测：在短时间内如果进程直接退出，认为启动失败
	// 注意：这里不尝试等待端口/健康检查（LaunchInfo 未提供 URL/端口信息）
	grace := 2 * time.Second
	if launchInfo.LaunchTimeout > 0 {
		t := time.Duration(launchInfo.LaunchTimeout) * time.Millisecond
		if t < grace {
			grace = t
		}
	}
	if dl, ok := ctx.Deadline(); ok {
		remain := time.Until(dl)
		if remain > 0 && remain < grace {
			grace = remain
		}
	}
	if grace <= 0 {
		grace = 1 * time.Second
	}

	done := make(chan error, 1)
	go func() {
		waitErr := cmd.Wait()
		if logFile != nil {
			_ = logFile.Close()
		}
		done <- waitErr
	}()

	timer := time.NewTimer(grace)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case werr := <-done:
		if werr == nil {
			return fmt.Errorf("本地服务进程在启动阶段异常退出（exit=0）")
		}
		return fmt.Errorf("本地服务进程在启动阶段异常退出: %w", werr)
	case <-timer.C:
		return nil
	}
}

// replaceURLAuthPlaceholders 替换URL中 query 参数的占位符：
// - 识别形如：?$(keyX)=$(VALUE)&$(keyY)=$(VALUE)
// - 将其替换为：?keyX=<auth[keyX]>&keyY=<auth[keyY]>
// 仅对存在于 auth 中的 keyX 才会替换；否则保持原样，避免误伤。
func replaceURLAuthPlaceholders(raw string, auth map[string]string) string {
	if strings.TrimSpace(raw) == "" || len(auth) == 0 {
		return raw
	}
	// 快速路径：不包含占位符
	rawLower := strings.ToLower(raw)
	if !strings.Contains(raw, "$(") && !strings.Contains(rawLower, "%24%28") {
		return raw
	}

	u, err := url.Parse(raw)
	if err != nil || u.RawQuery == "" {
		return raw
	}

	parts := strings.Split(u.RawQuery, "&")
	changed := false

	for i, p := range parts {
		if p == "" {
			continue
		}
		kPart, vPart, hasEq := strings.Cut(p, "=")
		if !hasEq {
			continue
		}

		// 尝试对 key/value 做一次解码，兼容少量编码场景
		kDecoded, errK := url.QueryUnescape(kPart)
		if errK != nil {
			kDecoded = kPart
		}
		vDecoded, errV := url.QueryUnescape(vPart)
		if errV != nil {
			vDecoded = vPart
		}

		keyName, ok := unwrapDollarParenPlaceholder(kDecoded)
		if !ok || strings.EqualFold(keyName, "VALUE") {
			continue
		}
		if vDecoded != "$(VALUE)" {
			continue
		}
		authVal, ok := auth[keyName]
		if !ok {
			continue
		}

		// 用真实 key + 对应 value 替换
		parts[i] = url.QueryEscape(keyName) + "=" + url.QueryEscape(authVal)
		changed = true
	}

	if !changed {
		return raw
	}
	u.RawQuery = strings.Join(parts, "&")
	return u.String()
}

// headersContainAuthPlaceholders 检测 headers 中是否存在 $(X) 或 $(VALUE)
func headersContainAuthPlaceholders(headers map[string]string) bool {
	if len(headers) == 0 {
		return false
	}
	for k, v := range headers {
		if strings.Contains(k, "$(") || strings.Contains(v, "$(") || strings.Contains(v, "$(VALUE)") {
			return true
		}
		kl := strings.ToLower(k)
		vl := strings.ToLower(v)
		if strings.Contains(kl, "%24%28") || strings.Contains(vl, "%24%28") {
			return true
		}
	}
	return false
}

// replaceHeaderAuthPlaceholders 按与 URL query 相同的业务规则替换 headers：
// - key 可以是占位符：$(HeaderName)
// - value 可以包含占位符：$(VALUE)
// 替换逻辑：
// - 若 key 是 $(X)，则真实 header key 为 X
// - 若 value 中包含 $(VALUE)，则用 auth[真实key] 替换（存在才替换）
// - 若 auth 中不存在对应 key，则保持原 header（避免误伤）
func replaceHeaderAuthPlaceholders(headers map[string]string, auth map[string]string) map[string]string {
	if len(headers) == 0 || len(auth) == 0 {
		return headers
	}

	newHeaders := make(map[string]string, len(headers))
	for k, v := range headers {
		realKey := k
		keyFromPlaceholder, ok := unwrapDollarParenPlaceholder(k)
		if ok {
			realKey = keyFromPlaceholder
		}

		authVal, hasAuth := auth[realKey]
		if ok && !hasAuth {
			// key 是占位符，但 auth 没对应项：保持原样
			newHeaders[k] = v
			continue
		}

		realVal := v
		if strings.Contains(realVal, "$(VALUE)") && hasAuth {
			realVal = strings.ReplaceAll(realVal, "$(VALUE)", authVal)
		}

		if ok {
			// key 替换为真实 key（同时 value 已处理）
			newHeaders[realKey] = realVal
		} else {
			// key 不变；若 value 含 $(VALUE) 且 auth[key] 存在，会替换掉
			newHeaders[k] = realVal
		}
	}
	return newHeaders
}

// envContainAuthPlaceholders 判断 env 中是否存在 $(X) 或 $(VALUE)
func envContainAuthPlaceholders(env map[string]interface{}) bool {
	if len(env) == 0 {
		return false
	}
	for k, v := range env {
		vs := fmt.Sprint(v)
		if strings.Contains(k, "$(") || strings.Contains(vs, "$(") || strings.Contains(vs, "$(VALUE)") {
			return true
		}
		kl := strings.ToLower(k)
		vl := strings.ToLower(vs)
		if strings.Contains(kl, "%24%28") || strings.Contains(vl, "%24%28") {
			return true
		}
	}
	return false
}

// replaceEnvAuthPlaceholders 按与 header 相同的业务规则替换 env：
// - key 可以是占位符：$(ENV_NAME)
// - value 可以包含占位符：$(VALUE)
// 替换逻辑：
// - 若 key 是 $(X)，则真实 env key 为 X
// - 若 value 中包含 $(VALUE)，则用 auth[真实key] 替换（存在才替换）
// - 若 auth 中不存在对应 key，则保持原 env（避免误伤）
func replaceEnvAuthPlaceholders(env map[string]interface{}, auth map[string]string) map[string]interface{} {
	if len(env) == 0 || len(auth) == 0 {
		return env
	}

	newEnv := make(map[string]interface{}, len(env))
	for k, v := range env {
		realKey := k
		keyFromPlaceholder, ok := unwrapDollarParenPlaceholder(k)
		if ok {
			realKey = keyFromPlaceholder
		}

		authVal, hasAuth := auth[realKey]
		if ok && !hasAuth {
			newEnv[k] = v
			continue
		}

		vs := fmt.Sprint(v)
		changed := false
		if strings.Contains(vs, "$(VALUE)") && hasAuth {
			vs = strings.ReplaceAll(vs, "$(VALUE)", authVal)
			changed = true
		}

		if ok {
			newEnv[realKey] = vs
			continue
		}
		if changed {
			newEnv[k] = vs
		} else {
			newEnv[k] = v
		}
	}
	return newEnv
}

// unwrapDollarParenPlaceholder 解析形如：$(keyX) 的占位符
func unwrapDollarParenPlaceholder(s string) (string, bool) {
	if len(s) < 4 { // "$(" + ")" + at least 1 char
		return "", false
	}
	if !strings.HasPrefix(s, "$(") || !strings.HasSuffix(s, ")") {
		return "", false
	}
	inner := s[2 : len(s)-1]
	if strings.TrimSpace(inner) == "" {
		return "", false
	}
	return inner, true
}
