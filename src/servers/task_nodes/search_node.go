package task_nodes

import (
	"AgentEarth_AgentPlatform/src/servers/types"
	"github.com/wcs1010270451/helpers/logger"
	_type "github.com/wcs1010270451/helpers/type"
	"go.uber.org/zap"
)

// SearchNode 搜索节点，用于搜索相关功能
type SearchNode struct {
	NodeInfo *NodeInfo //节点信息
}

// Init 初始化搜索节点
func (s *SearchNode) Init(config InitConfig) error {
	logger.Info("初始化搜索节点", zap.String("node_id", string(config.NodeModel.Id)))
	s.NodeInfo = &NodeInfo{
		NodeID:                  config.NodeModel.Id,
		NodeHandle:              config.NodeModel.NodeHandle,
		NodeName:                config.NodeModel.NodeName,
		Description:             config.NodeModel.Description,
		Enabled:                 true,
		ExternalServiceConfigID: config.NodeModel.ExternalServiceId,
	}
	logger.Info("搜索节点初始化完成")
	return nil
}

// GetTools 获取工具列表 - 搜索节点可以添加搜索相关工具
func (s *SearchNode) GetTools(rc *types.RunningContext, lastStepToolList []*ToolDesc) (currentToolList []*ToolDesc) {
	// 过滤同名工具
	logger.Debug("当前工具列表", zap.Int("工具数量", len(lastStepToolList)))
	var currentToolMap = make(map[string]*ToolDesc)
	for _, tool := range lastStepToolList {
		// 将该节点上贡献的工具名称保存到节点信息中
		currentToolMap[tool.ToolName] = tool
	}
	for _, desc := range currentToolMap {
		currentToolList = append(currentToolList, desc)
		s.NodeInfo.ToolNames = append(s.NodeInfo.ToolNames, desc.ToolName)
	}
	logger.Info("搜索节点获取工具列表", zap.Int("工具数量", len(currentToolList)))
	return
}

// Process 处理工具调用 - 搜索节点的业务逻辑
func (s *SearchNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp map[string]*types.CallToolResult) (currentResp map[string]*types.CallToolResult, err error) {
	// 获取上一步的结果
	if lastStepResp != nil {
		currentResp = lastStepResp
	} else {
		currentResp = make(map[string]*types.CallToolResult)
	}
	if len(s.NodeInfo.ToolNames) > 0 && _type.InStrArray(userCmd, s.NodeInfo.ToolNames) {
		logger.Debug("当前节点开始处理...", zap.String("tool_name", userCmd), zap.Int32("node_id", s.NodeInfo.NodeID))
		logger.Info("整合搜索结果...", zap.Any("currentResp", currentResp))
		logger.Debug("当前节点处理完成", zap.String("tool_name", userCmd), zap.Int32("node_id", s.NodeInfo.NodeID))
	} else {
		logger.Debug("当前节点不处理", zap.String("tool_name", userCmd), zap.Int32("node_id", s.NodeInfo.NodeID))
	}
	return
}

// GetNodeInfo 获取节点信息
func (s *SearchNode) GetNodeInfo() *NodeInfo {
	return s.NodeInfo
}
