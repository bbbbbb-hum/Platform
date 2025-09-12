package boot

import (
	"github.com/wcs1010270451/helpers/config"
	"github.com/wcs1010270451/helpers/logger"
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
