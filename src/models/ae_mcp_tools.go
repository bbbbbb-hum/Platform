package models

import (
	"time"

	"gorm.io/datatypes"
)

const TableNameAeMcpTools = "ae_mcp_tools"

type AeMcpTools struct {
	Id            int32             `gorm:"column:id;primaryKey;autoIncrement" json:"id"`     // 主键
	ServiceId     int32             `gorm:"column:service_id;not null" json:"service_id"`     // 服务id
	Name          string            `gorm:"column:name;not null" json:"name"`                 // 工具名称
	Description   string            `gorm:"column:description;not null" json:"description"`   // 工具描述
	ArgsSchema    datatypes.JSONMap `gorm:"column:args_schema;default:{}" json:"args_schema"` // 参数
	CreateTime    time.Time         `gorm:"column:create_time" json:"create_time"`            // 创建时间
	UpdateTime    time.Time         `gorm:"column:update_time" json:"update_time"`            // 更新时间
	XlcreditPrice float64           `gorm:"column:xlcredit_price" json:"xlcredit_price"`      // XLCredit价格
}

func (t *AeMcpTools) TableName() string {
	return TableNameAeMcpTools
}

// BatchDeleteByServiceId 按服务ID删除工具
func (t *AeMcpTools) BatchDeleteByServiceId(ServiceId int32) error {
	return GetDB().Where("service_id=?", ServiceId).Delete(&AeMcpTools{}).Error
}

// Create 新增工具
func (t *AeMcpTools) Create() error {
	return GetDB().Create(t).Error
}
