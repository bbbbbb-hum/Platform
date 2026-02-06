package config

import "AgentEarth_AgentPlatform/src/helpers/config"

func init() {
	config.Add("server", func() map[string]interface{} {
		return map[string]interface{}{
			// 应用名称
			"name": config.Env("SERVER_NAME", "AgentPlatform"),
			// 当前环境，用以区分多环境，一般为 local, stage, production, test
			"env": config.Env("SERVER_ENV", "production"),
			// 是否进入调试模式
			"debug": config.Env("SERVER_DEBUG", false),
			// 应用服务端口
			"host": config.Env("SERVER_HOST", "0.0.0.0"),
			"port": config.Env("SERVER_PORT", "9001"),
			// 加密会话、JWT 加密
			//"key": config.Env("SERVER_KEY", "33446a9dcf9ea060a0a6532b166da32f304af0de"),
			// 用以生成链接
			"url": config.Env("SERVER_URL", "http://0.0.0.0:9001"),
			// 设置时区，JWT 里会使用，日志记录里也会使用到
			"timezone": config.Env("TIMEZONE", "Asia/Shanghai"),

			"auth_enable":    config.Env("SERVER_AUTH_ENABLE", false),
			"auth_key":       config.Env("SERVER_AUTH_KEY", ""),
			"auth_cache_ttl": config.Env("SERVER_AUTH_CACHE_TTL", 300),

			"namespace": config.Env("SERVER_NAMESPACE", "local-dev"),
		}
	})
}
