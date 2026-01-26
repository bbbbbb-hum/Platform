package config

import "AgentEarth_AgentPlatform/src/helpers/config"

func init() {
	config.Add("redis", func() map[string]interface{} {
		return map[string]interface{}{
			"enable":               config.Env("REDIS_ENABLE", false),
			"addr":                 config.Env("REDIS_ADDR", "127.0.0.1:6379"),
			"username":             config.Env("REDIS_USERNAME", ""),
			"password":             config.Env("REDIS_PASSWORD", ""),
			"db":                   config.Env("REDIS_DB", 0),
			"pool_size":            config.Env("REDIS_POOL_SIZE", 20),
			"min_idle_conns":       config.Env("REDIS_MIN_IDLE_CONNS", 5),
			"max_retries":          config.Env("REDIS_MAX_RETRIES", 3),
			"dial_timeout_seconds": config.Env("REDIS_DIAL_TIMEOUT_SECONDS", 5),
			"read_timeout_seconds": config.Env("REDIS_READ_TIMEOUT_SECONDS", 3),
			"write_timeout_seconds": config.Env("REDIS_WRITE_TIMEOUT_SECONDS", 3),
			"pool_timeout_seconds":  config.Env("REDIS_POOL_TIMEOUT_SECONDS", 4),
			"prefix":                config.Env("REDIS_PREFIX", "aeap"),
		}
	})
}
