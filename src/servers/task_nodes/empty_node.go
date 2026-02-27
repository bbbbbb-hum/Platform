package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/servers/types"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// EmptyNode 空节点，啥也不做，用于测试  A
type EmptyNode struct {
	NodeInfo *types.NodeInfo //节点信息
}

// Init 初始化空节点
func (e *EmptyNode) Init(config types.InitConfig) error {
	logger.Info("初始化空节点", zap.String("node_id", string(config.NodeModel.Id)))
	e.NodeInfo = &types.NodeInfo{
		ServiceID:               config.ServiceID,
		ChainID:                 config.ChainModel.Id,
		NodeID:                  config.NodeModel.Id,
		NodeHandle:              config.NodeModel.NodeHandle,
		NodeName:                config.NodeModel.NodeName,
		Description:             config.NodeModel.Description,
		Enabled:                 true,
	}
	// 空节点不提供任何工具
	logger.Info("空节点初始化完成，不提供任何工具")
	return nil
}

// GetTools 获取工具列表 - 空节点不提供任何工具
func (e *EmptyNode) GetTools(rc *types.RunningContext) (currentToolList []*types.ToolDesc) {
	// 空节点不添加任何工具
	return
}

// Process 处理工具调用 - 空节点什么都不做
func (e *EmptyNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (currentResp *mcp.CallToolResult, err error) {
	// 获取上一步的结果
	if lastStepResp != nil {
		currentResp = lastStepResp
	}
	return
}

// GetNodeInfo 获取节点信息
func (e *EmptyNode) GetNodeInfo() *types.NodeInfo {
	return e.NodeInfo
}
