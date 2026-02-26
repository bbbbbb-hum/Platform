package users

import (
	"time"
)

const TableNameAeUserConsumptionRecord = "ae_user_consumption_record"

// 用户消费记录
type AeUserConsumptionRecord struct {
	Id         int32     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`          // 用户消费记录id
	UserStrId  string    `gorm:"column:user_str_id;not null" json:"user_str_id"`        // 用户uuid
	Amount     float64   `gorm:"column:amount;not null" json:"amount"`                  // 消费金额
	ServiceId  string    `gorm:"column:service_id;not null" json:"service_id"`          // mcp服务id
	CreateTime time.Time `gorm:"column:create_time;default:'now()'" json:"create_time"` // 创建时间
	KeyId      int32     `gorm:"column:key_id;not null" json:"key_id"`                  // 密钥id
}

func (l *AeUserConsumptionRecord) TableName() string {
	return TableNameAeUserConsumptionRecord
}
