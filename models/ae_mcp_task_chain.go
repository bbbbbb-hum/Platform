package models

import (
	"fmt"
	"github.com/lib/pq"
	"time"
)

const TableNameAeMcpTaskChain = "ae_mcp_task_chain"

// 任务链表
type AeMcpTaskChain struct {
	Id     int32  `gorm:"column:id;primaryKey;autoIncrement" json:"id"` // 自增id，主键
	Name   string `gorm:"column:name;not null" json:"name"`             // 任务链名称
	Status string `gorm:"column:status;default:'create'" json:"status"` // 任务链状态：create 创建中;used 使用中; stop 已停用;
	//NodeIds    int32         `gorm:"column:node_ids;default:{}" json:"node_ids"`   // 节点id数组
	NodeIds    pq.Int32Array `gorm:"column:node_ids;type:integer[];default:'{}'" json:"node_ids"`
	CreateTime time.Time     `gorm:"column:create_time" json:"create_time"` // 创建时间
	UpdateTime time.Time     `gorm:"column:update_time" json:"update_time"` // 更新时间
}

func (m *AeMcpTaskChain) TableName() string {
	return TableNameAeMcpTaskChain
}

// GetOneById 通过id获取一条数据
func (m *AeMcpTaskChain) GetOneById(id int32) error {
	db := getDB()
	if db == nil {
		return fmt.Errorf("数据库连接未初始化")
	}
	return db.Where("id = ?", id).First(m).Error
}
