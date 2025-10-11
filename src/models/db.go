package models

import (
	"AgentEarth_AgentPlatform/src/helpers/database"
	"fmt"
	"gorm.io/gorm"
)

// getDB 获取数据库连接
func GetDB() *gorm.DB {
	db := database.DB
	if db == nil {
		fmt.Printf("警告: database.DB 为 nil\n")
		return nil
	}

	// 检查数据库连接是否有效
	//sqlDB, err := db.DB()
	//if err != nil {
	//	fmt.Printf("警告: 获取底层数据库连接失败: %v\n", err)
	//	return nil
	//}

	//if err = sqlDB.Ping(); err != nil {
	//	fmt.Printf("警告: 数据库连接ping失败: %v\n", err)
	//	return nil
	//}

	return db
}
