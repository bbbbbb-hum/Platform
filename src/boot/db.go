package boot

import (
	"AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"AgentEarth_AgentPlatform/src/helpers/database"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
)

func SetupDB() {
	var dbConfig gorm.Dialector
	connType := config.Get("database.connection")
	switch connType {
	case "mysql":
		// 构建 DSN 信息
		dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?charset=%v&parseTime=True&multiStatements=true&loc=Local",
			config.Get("database.mysql.username"),
			config.Get("database.mysql.password"),
			config.Get("database.mysql.host"),
			config.Get("database.mysql.port"),
			config.Get("database.mysql.database"),
			config.Get("database.mysql.charset"),
		)
		dbConfig = mysql.New(mysql.Config{
			DSN: dsn,
		})
	case "sqlite":
		// 初始化 sqlite
		db := config.Get("database.sqlite.database")
		dbConfig = sqlite.Open(db)

	case "postgres":
		// 构建 DSN 信息
		dsn := fmt.Sprintf("host=%v user=%v password=%v dbname=%v port=%v sslmode=%v TimeZone=%v",
			config.Get("database.postgres.host"),
			config.Get("database.postgres.username"),
			config.Get("database.postgres.password"),
			config.Get("database.postgres.database"),
			config.Get("database.postgres.port"),
			config.Get("database.postgres.sslmode"),
			config.Get("server.timezone"),
		)
		logger.Info("dsn", zap.String("type", "postgres"), zap.String("dsn", dsn))
		dbConfig = postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true, // disables implicit prepared statement usage
		})
	default:
		logger.Error("没有该配置！", zap.String("error", "database connection not supported"))
		return
	}
	// 连接数据库，并设置 GORM 的日志模式
	logger.Info("连接数据库...", zap.String("action", "connecting"))
	database.Connect(dbConfig, logger.NewGormLogger())

	// 检查数据库连接是否成功
	if database.DB == nil {
		logger.Error("连接失败", zap.String("error", "database.DB is nil"))
		return
	}
	// 测试数据库连接（获取 sql.DB）
	sqlDB, err := database.DB.DB()
	if err != nil {
		logger.Error("获取底层数据库连接失败", zap.String("error", err.Error()))
		return
	}

	// 检查 database.SQLDB 是否有效
	if database.SQLDB == nil {
		logger.Error("database.SQLDB 为 nil，无法设置连接池参数")
		return
	}

	// 基于当前驱动读取连接池参数（避免读取到不存在的顶层键导致无限制）
	var prefix string
	switch connType {
	case "mysql":
		prefix = "database.mysql."
	case "postgres":
		prefix = "database.postgres."
	case "sqlite":
		prefix = "database.sqlite."
	default:
		prefix = "database."
	}
	maxOpen := config.GetInt(prefix + "max_open_connections")
	if maxOpen <= 0 {
		maxOpen = 25
	}
	maxIdle := config.GetInt(prefix + "max_idle_connections")
	if maxIdle < 0 {
		maxIdle = 0
	}
	maxLife := config.GetInt(prefix + "max_life_seconds")
	if maxLife <= 0 {
		maxLife = 300
	}
	// 先应用连接池参数，再进行 Ping，避免启动阶段瞬时超配
	database.SQLDB.SetMaxOpenConns(maxOpen)
	database.SQLDB.SetMaxIdleConns(maxIdle)
	database.SQLDB.SetConnMaxLifetime(time.Duration(maxLife) * time.Second)
	logger.Info("数据库连接池参数已应用",
		zap.Int("max_open_conns", maxOpen),
		zap.Int("max_idle_conns", maxIdle),
		zap.Int("conn_max_life_seconds", maxLife),
	)

	logger.Info("数据库连接池配置成功！", zap.String("status", "connection pools configured"))
	if err = sqlDB.Ping(); err != nil {
		logger.Error("数据库连接测试失败", zap.String("error", err.Error()))
	}
	logger.Info("数据连接成功！", zap.String("status", "connected successfully"))
	// database.DB.AutoMigrate(&user.User{})
}

// GetDB 获取数据库连接实例
func GetDB() *gorm.DB {
	return database.DB
}
