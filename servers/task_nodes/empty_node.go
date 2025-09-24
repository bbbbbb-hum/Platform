package task_nodes

import (
	"AgentEarth_AgentPlatform/servers/types"

	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// EmptyNode 空节点，啥也不做，用于测试  A
type EmptyNode struct {
	NodeInfo *NodeInfo //节点信息
}

// Init 初始化空节点
func (e *EmptyNode) Init(config InitConfig) error {
	logger.Info("初始化空节点", zap.String("node_id", string(config.NodeModel.Id)))
	e.NodeInfo = &NodeInfo{
		NodeID:      config.NodeModel.Id,
		NodeType:    config.NodeModel.NodeType,
		NodeName:    config.NodeModel.NodeName,
		Description: config.NodeModel.Description,
		Enabled:     true,
	}
	// 空节点不提供任何工具
	logger.Info("空节点初始化完成，不提供任何工具")
	return nil
}

// GetTools 获取工具列表 - 空节点不提供任何工具
func (e *EmptyNode) GetTools(rc *types.RunningContext, lastStepToolList []*ToolDesc) (currentToolList []*ToolDesc) {
	// 空节点不添加任何工具
	// 返回上一步的 工具列表
	currentToolList = lastStepToolList
	return
}

// Process 处理工具调用 - 空节点什么都不做
func (e *EmptyNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp map[string]*types.CallToolResult) (currentResp map[string]*types.CallToolResult, err error) {
	// 空节点什么都不做，直接返回上一步的结果
	currentResp = lastStepResp
	logger.Info("空节点处理完成，无任何操作", zap.Int32("node_id", e.NodeInfo.NodeID))
	return
}

// GetNodeInfo 获取节点信息
func (e *EmptyNode) GetNodeInfo() *NodeInfo {
	return e.NodeInfo
}
