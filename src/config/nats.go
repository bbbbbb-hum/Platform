package config

import "AgentEarth_AgentPlatform/src/helpers/config"

func init() {
	config.Add("nats", func() map[string]interface{} {
		return map[string]interface{}{
			// 基础配置
			"enable":      config.Env("NATS_ENABLE", false),
			"url":         config.Env("NATS_URL", "nats://localhost:4222"),
			"stream_name": config.Env("NATS_STREAM_NAME", "REQUEST_LOGS"),
			"subject":     config.Env("NATS_SUBJECT", "logs.request"),

			// 连接配置
			"max_reconnects":         config.Env("NATS_MAX_RECONNECTS", 5),
			"reconnect_wait_seconds": config.Env("NATS_RECONNECT_WAIT_SECONDS", 2),
			"connect_timeout_seconds": config.Env("NATS_CONNECT_TIMEOUT_SECONDS", 10),

			// JetStream 配置
			"stream_max_msgs":        config.Env("NATS_STREAM_MAX_MSGS", 1000000),
			"stream_max_bytes":       config.Env("NATS_STREAM_MAX_BYTES", 1073741824), // 1GB
			"stream_max_age_hours":   config.Env("NATS_STREAM_MAX_AGE_HOURS", 24),
			"stream_replicas":        config.Env("NATS_STREAM_REPLICAS", 1),
		}
	})
}
