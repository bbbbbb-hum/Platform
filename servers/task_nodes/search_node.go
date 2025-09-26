package task_nodes

import (
	"AgentEarth_AgentPlatform/servers/pools"
	"AgentEarth_AgentPlatform/servers/types"
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
	// 启动必应的MCP服务
	err := pools.GetSSEPool().InitializeSSEPool(config.NodeModel.ExternalServiceId)
	if err != nil {
		logger.Error("初始化必应MCP服务失败", zap.Error(err))
		return err
	}

	logger.Info("搜索节点初始化完成")
	return nil
}

// GetTools 获取工具列表 - 搜索节点可以添加搜索相关工具
func (s *SearchNode) GetTools(rc *types.RunningContext, lastStepToolList []*ToolDesc) (currentToolList []*ToolDesc) {
	// 获取上一步的工具列表
	currentToolList = lastStepToolList

	// TODO: 在这里添加搜索相关的工具
	// 例如：搜索工具、过滤工具等
	// 从连接池中获取必应的MCP服务下的工具列表
	biyingTools := pools.GetSSEPool().GetServiceTools(s.NodeInfo.ExternalServiceConfigID)
	logger.Debug("必应工具列表", zap.Int("工具数量", len(biyingTools)))
	for _, tool := range biyingTools {
		// 将该节点上贡献的工具名称保存到节点信息中
		s.NodeInfo.ToolNames = append(s.NodeInfo.ToolNames, tool.Name)
		if tool.InputSchema != nil && tool.InputSchema.Schema != "https://json-schema.org/draft/2020-12/schema" {
			tool.InputSchema.Schema = "https://json-schema.org/draft/2020-12/schema"
		}
		currentToolList = append(currentToolList, &ToolDesc{
			ToolDesc:        tool.Description,
			ToolInputSchema: tool.InputSchema,
			ToolName:        tool.Name,
		})

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
		// 调用必应的MCP服务的工具
		callToolResult, structuredResult, err1 := pools.GetSSEPool().CallTool(s.NodeInfo.ExternalServiceConfigID, userCmd, userParamMap)
		if err1 != nil {
			err = err1
			return
		}
		logger.Debug("必应工具结果", zap.Any("result", callToolResult))
		//在搜索结果中加一条记录
		//callToolResult.Content = append(callToolResult.Content, &mcp.TextContent{
		//	Text: "这条是我额外增加的结果",
		//})
		currentResp[userCmd] = &types.CallToolResult{
			Result:           callToolResult,
			StructuredResult: structuredResult,
		}
		// 将结果保存到节点上下文
		rc.ResultMap[s.NodeInfo.NodeID] = currentResp
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
