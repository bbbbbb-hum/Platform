package boot

import (
	"fmt"
	"time"

	"github.com/wcs1010270451/helpers/config"
	"github.com/wcs1010270451/helpers/database"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
)

func SetupDB() {
	var dbConfig gorm.Dialector
	switch config.Get("database.connection") {
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
		database := config.Get("database.sqlite.database")
		dbConfig = sqlite.Open(database)

	case "postgres":
		// 构建 DSN 信息
		dsn := fmt.Sprintf("host=%v user=%v password=%v dbname=%v port=%v sslmode=disable TimeZone=Asia/Shanghai",
			config.Get("database.postgres.host"),
			config.Get("database.postgres.username"),
			config.Get("database.postgres.password"),
			config.Get("database.postgres.database"),
			config.Get("database.postgres.port"),
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
	// 测试数据库连接
	sqlDB, err := database.DB.DB()
	if err != nil {
		logger.Error("获取底层数据库连接失败", zap.String("error", err.Error()))
		return
	}

	if err = sqlDB.Ping(); err != nil {
		logger.Error("数据库连接测试失败", zap.String("error", err.Error()))
	}

	logger.Info("数据连接成功！", zap.String("status", "connected successfully"))

	// 检查 database.SQLDB 是否有效
	if database.SQLDB == nil {
		logger.Error("database.SQLDB 为 nil，无法设置连接池参数")
		return
	}

	// 设置最大连接数
	database.SQLDB.SetMaxOpenConns(config.GetInt("database.max_open_connections"))
	// 设置最大空闲连接数
	database.SQLDB.SetMaxIdleConns(config.GetInt("database.max_idle_connections"))
	// 设置每个链接的过期时间
	database.SQLDB.SetConnMaxLifetime(time.Duration(config.GetInt("database.max_life_seconds")) * time.Second)

	logger.Info("数据库连接池配置成功！", zap.String("status", "connection pool configured"))

	// database.DB.AutoMigrate(&user.User{})
}
