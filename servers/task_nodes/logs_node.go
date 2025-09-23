package task_nodes

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/servers/tools"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// LoggerNode 日志记录节点 - 只记录，不提供工具
type LoggerNode struct {
	NodeInfo *NodeInfo `json:"info"`
}

func (l *LoggerNode) Init(ctx *NodeContext, node *models.AeMcpTaskNode) error {
	logger.Info("初始化日志节点", zap.String("node_id", fmt.Sprint(ctx.ChainID)))
	// 初始化日志文件等
	l.NodeInfo = &NodeInfo{
		NodeID:      node.Id,
		NodeType:    node.NodeType,
		NodeName:    node.NodeName,
		Description: node.Description,
	}
	// 获取节点顺序
	if nodeStats, ok := ctx.Stats[node.NodeType].(map[string]interface{}); ok {
		if order := nodeStats["node_order"]; order != nil {
			l.NodeInfo.Order = order.(int)
		}
	}
	return nil
}

func (l *LoggerNode) GetTools(ctx *NodeContext, lastStepToolList []*mcp.Tool) (currentToolList []*mcp.Tool, err error) {
	// 获取工具列表
	toolsMap := tools.GetToolsMap()
	currentToolList = lastStepToolList
	if toolsList, ok := toolsMap.GetServerTools(ctx.ServiceID); ok {
		for _, tool := range toolsList {
			currentToolList = append(currentToolList, tool)
		}
	}
	return
}

func (l *LoggerNode) Process(ctx *NodeContext, userCmd string, userParamMap any, lastStepResp map[string]*CallToolResult) (currentResp map[string]*CallToolResult, err error) {
	currentResp = lastStepResp
	// 打印当前节点数据
	logger.Info("当前节点数据", zap.String("node_id", fmt.Sprint(ctx.ChainID)), zap.String("user_cmd", userCmd), zap.Any("user_param_map", userParamMap), zap.Any("last_step_resp", currentResp))
	return
}
