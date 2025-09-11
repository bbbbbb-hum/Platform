package boot

import (
	"errors"
	"fmt"
	"github.com/wcs1010270451/helpers/config"
	"github.com/wcs1010270451/helpers/database"
	"github.com/wcs1010270451/helpers/logger"
	"gorm.io/gorm"
	"time"

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
		dbConfig = postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true, // disables implicit prepared statement usage
		})
	default:
		panic(errors.New("database connection not supported"))
	}
	// 连接数据库，并设置 GORM 的日志模式
	// database.Connect(dbConfig, logger.Default.LogMode(logger.Info))
	database.Connect(dbConfig, logger.NewGormLogger())
	// 设置最大连接数
	database.SQLDB.SetMaxOpenConns(config.GetInt("database.max_open_connections"))
	// 设置最大空闲连接数
	database.SQLDB.SetMaxIdleConns(config.GetInt("database.max_idle_connections"))
	// 设置每个链接的过期时间
	database.SQLDB.SetConnMaxLifetime(time.Duration(config.GetInt("database.max_life_seconds")) * time.Second)

	// database.DB.AutoMigrate(&user.User{})
}
