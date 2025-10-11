package config

import (
	"AgentEarth_AgentPlatform/src/models"
	"gorm.io/datatypes"
	"time"
)

const TableNameAeMcpExternalServicesAccount = "ae_mcp_external_services_account"

// 外部服务账号表
type AeMcpExternalServicesAccount struct {
	Id         int32             `gorm:"column:id;primaryKey;autoIncrement" json:"id"` // 自增id
	Name       string            `gorm:"column:name;not null" json:"name"`             // 账号名称
	AuthInfo   datatypes.JSONMap `gorm:"column:auth_info;not null" json:"auth_info"`   // 权限信息
	ConfigId   int32             `gorm:"column:config_id;not null" json:"config_id"`   // 外部服务配置表id
	CreateTime time.Time         `gorm:"column:create_time" json:"create_time"`        // 创建时间
	UpdateTime time.Time         `gorm:"column:update_time" json:"update_time"`        // 更新时间
	Status     string            `gorm:"column:status;default:'unused'" json:"status"` // 使用状态：used-使用中，unused-未使用
}

func (a *AeMcpExternalServicesAccount) TableName() string {
	return TableNameAeMcpExternalServicesAccount
}

func (a *AeMcpExternalServicesAccount) GetOneByConfigId(configId int32) error {
	return models.GetDB().Where("config_id = ?", configId).First(a).Error
}

func (a *AeMcpExternalServicesAccount) GetListByConfigId(configId int32) (list []*AeMcpExternalServicesAccount, err error) {
	err = models.GetDB().Where("config_id = ?", configId).Find(&list).Error
	return
}
