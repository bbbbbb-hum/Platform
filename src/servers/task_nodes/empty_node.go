package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/servers/types"

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
		NodeHandle:  config.NodeModel.NodeHandle,
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
	// 获取上一步的结果
	if lastStepResp != nil {
		currentResp = lastStepResp
	} else {
		currentResp = make(map[string]*types.CallToolResult)
	}
	// 判断当前节点是否处理
	if len(e.NodeInfo.ToolNames) > 0 && helpers.InStrArray(userCmd, e.NodeInfo.ToolNames) {
		logger.Debug("当前节点开始处理...", zap.String("tool_name", userCmd), zap.Int32("node_id", e.NodeInfo.NodeID))
		//无处理
		logger.Debug("当前节点处理完成", zap.String("tool_name", userCmd), zap.Int32("node_id", e.NodeInfo.NodeID))
	} else {
		logger.Debug("当前节点不处理", zap.String("tool_name", userCmd), zap.Int32("node_id", e.NodeInfo.NodeID))
	}
	return
}

// GetNodeInfo 获取节点信息
func (e *EmptyNode) GetNodeInfo() *NodeInfo {
	return e.NodeInfo
}
