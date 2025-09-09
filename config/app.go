package config

import "github.com/wcs1010270451/helpers/config"

func init() {
	config.Add("app", func() map[string]interface{} {
		return map[string]interface{}{

			// 应用名称
			"name": config.Env("APP_NAME", "iaiapi-proxy"),

			// 当前环境，用以区分多环境，一般为 local, stage, production, test
			"env": config.Env("APP_ENV", "production"),

			// 是否进入调试模式
			"debug": config.Env("APP_DEBUG", false),

			// 应用服务端口
			"port": config.Env("APP_PORT", "3001"),

			// 加密会话、JWT 加密
			"secret_key": config.Env("APP_KEY", "33446a9dcf9ea060a0a6532b166da32f304af0de"),

			// 用以生成链接
			"url": config.Env("APP_URL", "http://localhost:3000"),

			// 设置时区，JWT 里会使用，日志记录里也会使用到
			"timezone": config.Env("TIMEZONE", "Asia/Shanghai"),
		}
	})
	config.Add("proxy", func() map[string]interface{} {
		return map[string]interface{}{
			"url40": config.Env("PROXY_URL_FOUR", ""),
			"url35": config.Env("PROXY_URL_THREE", ""),
		}
	})
	config.Add("flow", func() map[string]interface{} {
		return map[string]interface{}{
			"threshold": config.Env("FLOW_THRESHOLD", 0),
		}
	})
}
