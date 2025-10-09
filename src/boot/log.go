package boot

import (
	"AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
)

// SetupLogger 初始化 Logger
func SetupLogger() {
	var isLocal bool
	if config.GetString("SERVER_ENV") == "local" {
		isLocal = true
	}
	logger.InitLogger(
		config.GetString("log.filename"),
		config.GetInt("log.max_size"),
		config.GetInt("log.max_backup"),
		config.GetInt("log.max_age"),
		config.GetBool("log.compress"),
		config.GetString("log.type"),
		config.GetString("log.level"),
		isLocal,
	)
}
