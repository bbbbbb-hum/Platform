package models

import (
	"time"
)

const TableNameAeMcpServicesLimit = "ae_mcp_services_limit"

// mcp服务使用限制配置表
type AeMcpServicesLimit struct {
	Id          int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`          // 自增主键
	ServerId    string    `gorm:"column:server_id;not null" json:"server_id"`            // 服务id
	LimitCalls  int64     `gorm:"column:limit_calls;not null" json:"limit_calls"`        // 限制次数
	LimitTokens int64     `gorm:"column:limit_tokens;not null" json:"limit_tokens"`      // 限制token数
	LimitType   int64     `gorm:"column:limit_type;not null" json:"limit_type"`          // 限制类型：1 天; 2 周；3 月；4 季度；5 年；
	CreateTime  time.Time `gorm:"column:create_time;default:'now()'" json:"create_time"` // 创建时间
	UpdateTime  time.Time `gorm:"column:update_time;default:'now()'" json:"update_time"` // 更新时间
}

func (l *AeMcpServicesLimit) TableName() string {
	return TableNameAeMcpServicesLimit
}

func (l *AeMcpServicesLimit) GetOneByServerId(serverId string) error {
	db := GetDB()
	return db.Where("server_id = ?", serverId).First(l).Error
}
