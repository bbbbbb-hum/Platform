package config

import (
	"AgentEarth_AgentPlatform/src/models"
	"gorm.io/datatypes"
	"time"
)

const TableNameAeMcpExternalServicesConfig = "ae_mcp_external_services_config"

// 外部服务配置表
type AeMcpExternalServicesConfig struct {
	Id                int32             `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                                    // 自增id
	Name              string            `gorm:"column:name;not null" json:"name"`                                                // 服务名称
	Type              string            `gorm:"column:type;not null" json:"type"`                                                // 服务类型：stdio-标准输入输出，sse-sse连接
	LaunchInfo        datatypes.JSONMap `gorm:"column:launch_info;not null" json:"launch_info"`                                  // 启动信息
	CreateTime        time.Time         `gorm:"column:create_time" json:"create_time"`                                           // 创建时间
	UpdateTime        time.Time         `gorm:"column:update_time" json:"update_time"`                                           // 更新时间
	ConnectInfo       datatypes.JSONMap `gorm:"column:connect_info;not null" json:"connect_info"`                                //连接信息
	ExternalServiceId string            `gorm:"column:external_service_id;default:gen_random_uuid()" json:"external_service_id"` // 外部服务唯一标识
	MaxInstance       int32             `gorm:"column:max_instance;default:1" json:"max_instance"`                               // 最大实例数
}

func (c *AeMcpExternalServicesConfig) TableName() string {
	return TableNameAeMcpExternalServicesConfig
}

func (c *AeMcpExternalServicesConfig) GetOne(id int32) error {
	return models.GetDB().First(c, id).Error
}

func (c *AeMcpExternalServicesConfig) GetOneByExternalServiceId(externalServiceId string) error {
	return models.GetDB().Where("external_service_id = ?", externalServiceId).First(c).Error
}
func (c *AeMcpExternalServicesConfig) GetList() (list []*AeMcpExternalServicesConfig, err error) {
	err = models.GetDB().Find(&list).Error
	return
}
