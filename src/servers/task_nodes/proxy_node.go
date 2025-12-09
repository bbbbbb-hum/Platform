package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/servers/pools"
	"AgentEarth_AgentPlatform/src/servers/types"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// ProxyNode SSE代理节点，用于代理SSE类型的MCP服务
type ProxyNode struct {
	NodeInfo *types.NodeInfo //节点信息
}

// Init 初始化SSE代理节点
func (p *ProxyNode) Init(config types.InitConfig) error {
	logger.Info("初始化代理节点", zap.String("node_id", string(config.NodeModel.Id)))
	p.NodeInfo = &types.NodeInfo{
		ServiceID:               config.ServiceID,
		ChainID:                 config.ChainModel.Id,
		NodeID:                  config.NodeModel.Id,
		NodeHandle:              config.NodeModel.NodeHandle,
		NodeName:                config.NodeModel.NodeName,
		Description:             config.NodeModel.Description,
		Enabled:                 true,
		ExternalServiceConfigID: config.NodeModel.ExternalServiceId,
	}

	// 初始化外部MCP服务
	if config.NodeModel.ExternalServiceId != "" {
		err := pools.GetConnectPool().InitializeService(config.NodeModel.ExternalServiceId)
		if err != nil {
			logger.Error("初始化外部 MCP服务失败", zap.Error(err))
			return err
		}
	}

	logger.Info("代理节点初始化完成")
	return nil
}

// GetTools 获取工具列表 - 从外部MCP服务获取工具
func (p *ProxyNode) GetTools(rc *types.RunningContext) (currentToolList []*types.ToolDesc) {

	// 从连接池获取外部服务的工具
	if p.NodeInfo.ExternalServiceConfigID != "" {
		externalTools := pools.GetConnectPool().GetServiceTools(p.NodeInfo.ExternalServiceConfigID)
		logger.Debug("工具列表", zap.Int("工具数量", len(externalTools)))
		for _, tool := range externalTools {
			logger.Debug("工具", zap.String("工具名称", tool.Name), zap.Any("工具输入参数", tool.InputSchema))
			// 将该节点上贡献的工具名称保存到节点信息中
			p.NodeInfo.ToolNames = append(p.NodeInfo.ToolNames, tool.Name)
			// 修改工具输入参数的schema的版本
			if tool.InputSchema != nil && tool.InputSchema.Schema != "https://json-schema.org/draft/2020-12/schema" {
				tool.InputSchema.Schema = "https://json-schema.org/draft/2020-12/schema"
			}
			currentToolList = append(currentToolList, &types.ToolDesc{
				ToolDesc:        tool.Description,
				ToolInputSchema: tool.InputSchema,
				ToolName:        tool.Name,
			})
		}
	}

	logger.Info("代理节点获取工具列表", zap.Int("工具数量", len(currentToolList)))
	return
}

// Process 处理工具调用 - 调用外部MCP服务
func (p *ProxyNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (currentResp *mcp.CallToolResult, err error) {
	// 获取上一步的结果
	if lastStepResp != nil {
		currentResp = lastStepResp
	}

	// 调用必应的MCP服务的工具
	currentResp, err = pools.GetConnectPool().CallTool(p.NodeInfo.ExternalServiceConfigID, userCmd, userParamMap)
	if err != nil {
		return
	}
	logger.Debug("代理工具结果", zap.Any("result", currentResp))
	return
}

// GetNodeInfo 获取节点信息
func (p *ProxyNode) GetNodeInfo() *types.NodeInfo {
	return p.NodeInfo
}
