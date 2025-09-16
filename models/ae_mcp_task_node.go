package models

import (
	"time"
)

const TableNameAeMcpTaskNode = "ae_mcp_task_node"

// 任务节点表
type AeMcpTaskNode struct {
	Id         int32     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`   // 自增id
	NodeName   string    `gorm:"column:node_name;not null" json:"node_name"`     // 节点名称
	NodeHandle string    `gorm:"column:node_handle;not null" json:"node_handle"` // 节点处理(处理函数名称)
	Enabled    bool      `gorm:"column:enabled;default:false" json:"enabled"`    // 是否启用
	CreateTime time.Time `gorm:"column:create_time" json:"create_time"`          // 创建时间
	UpdateTime time.Time `gorm:"column:update_time" json:"update_time"`          // 更新时间
	ServiceId  int32     `gorm:"column:service_id;not null" json:"service_id"`   // 外部服务配置id
}

func (m *AeMcpTaskNode) TableName() string {
	return TableNameAeMcpTaskNode
}

// 获取任务链上所有节点
func (m *AeMcpTaskNode) GetChianNodes(nodeIds []int32) (err error, list []*AeMcpTaskNode) {
	err = GetDB().Where("id in ?", nodeIds).Find(&list).Error
	return
}
