package models

import (
	"time"
)

const TableNameAeMcpTaskNode = "ae_mcp_task_node"

// 任务节点表
type AeMcpTaskNode struct {
	Id                int32     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`                   // 自增id
	NodeName          string    `gorm:"column:node_name;not null" json:"node_name"`                     // 节点名称
	NodeHandle        string    `gorm:"column:node_handle;not null" json:"node_handle"`                 // 节点处理(处理函数名称)
	Description       string    `gorm:"column:description;not null" json:"description"`                 // 节点描述
	Enabled           bool      `gorm:"column:enabled;default:false" json:"enabled"`                    // 是否启用
	CreateTime        time.Time `gorm:"column:create_time" json:"create_time"`                          // 创建时间
	UpdateTime        time.Time `gorm:"column:update_time" json:"update_time"`                          // 更新时间
	NodeConfig        string    `gorm:"column:node_config;type:jsonb" json:"node_config"`                // 共表字段：节点配置（含url/protocol/timeout）
}

func (m *AeMcpTaskNode) TableName() string {
	return TableNameAeMcpTaskNode
}

// 获取任务链上所有节点
func (m *AeMcpTaskNode) GetChianNodes(nodeIds []int32) (err error, list []*AeMcpTaskNode) {
	noOrderList := make([]*AeMcpTaskNode, 0, len(nodeIds))
	err = GetDB().Where("id in ?", nodeIds).Find(&noOrderList).Error
	list = sortAeMcpTaskNode(nodeIds, noOrderList)
	return
}

// 按 node name 获取节点列表
func (m *AeMcpTaskNode) GetNodesByNames(nodeNames []string) (error, []*AeMcpTaskNode) {
	noOrderList := make([]*AeMcpTaskNode, 0, len(nodeNames))
	err := GetDB().Where("node_name in ?", nodeNames).Find(&noOrderList).Error
	if err != nil {
		return err, nil
	}
	return nil, sortAeMcpTaskNodeByNames(nodeNames, noOrderList)
}

// 按nodeIds排序任务链节点
func sortAeMcpTaskNode(nodeIds []int32, nodeList []*AeMcpTaskNode) []*AeMcpTaskNode {
	// 创建一个map用于快速查找node
	nodeMap := make(map[int32]*AeMcpTaskNode)
	for _, node := range nodeList {
		nodeMap[node.Id] = node
	}

	// 按照nodeIds的顺序排序
	sortedNodes := make([]*AeMcpTaskNode, 0, len(nodeIds))
	for _, id := range nodeIds {
		if node, exists := nodeMap[id]; exists {
			sortedNodes = append(sortedNodes, node)
		}
	}

	return sortedNodes
}

// 按 node name 排序节点
func sortAeMcpTaskNodeByNames(nodeNames []string, nodeList []*AeMcpTaskNode) []*AeMcpTaskNode {
	nodeMap := make(map[string]*AeMcpTaskNode)
	for _, node := range nodeList {
		nodeMap[node.NodeName] = node
	}

	sortedNodes := make([]*AeMcpTaskNode, 0, len(nodeNames))
	for _, name := range nodeNames {
		if node, exists := nodeMap[name]; exists {
			sortedNodes = append(sortedNodes, node)
		}
	}

	return sortedNodes
}
