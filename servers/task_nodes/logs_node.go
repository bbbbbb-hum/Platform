package task_nodes

import (
	"AgentEarth_AgentPlatform/servers/ctx"
	"AgentEarth_AgentPlatform/servers/tools"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// LoggerNode 日志记录节点 - 只记录，不提供工具
type LoggerNode struct {
	NodeInfo *NodeInfo
}

func (l *LoggerNode) Init(config InitConfig) error {
	logger.Info("初始化日志节点", zap.String("node_id", fmt.Sprint(config.NodeModel.Id)))
	// 初始化日志文件等
	l.NodeInfo = &NodeInfo{
		NodeID:      config.NodeModel.Id,
		NodeType:    config.NodeModel.NodeType,
		NodeName:    config.NodeModel.NodeName,
		Description: config.NodeModel.Description,
	}

	return nil
}

func (l *LoggerNode) GetTools(ctx *ctx.RunningContext) (currentToolList []*mcp.Tool) {
	// 获取工具列表
	toolsMap := tools.GetToolsMap()
	if toolsList, ok := toolsMap.GetServerTools(ctx.ServiceID); ok {
		for _, tool := range toolsList {
			currentToolList = append(currentToolList, tool)
		}
	}
	return
}

func (l *LoggerNode) Process(ctx *ctx.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp map[string]*ctx.CallToolResult) (currentResp map[string]*ctx.CallToolResult, err error) {
	currentResp = lastStepResp
	// 打印当前节点数据
	logger.Info("当前节点数据", zap.String("node_id", fmt.Sprint(ctx.ChainID)), zap.String("user_cmd", userCmd), zap.Any("user_param_map", userParamMap), zap.Any("last_step_resp", currentResp))
	return
}

func (l *LoggerNode) GetNodeInfo() *NodeInfo {
	return l.NodeInfo
}
