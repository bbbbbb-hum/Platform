package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/servers/types"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// LogsNode 日志记录节点 - 只记录，不提供工具 C
type LogsNode struct {
	NodeInfo *types.NodeInfo //节点信息
}

func (l *LogsNode) Init(config types.InitConfig) error {
	logger.Info("初始化日志节点", zap.String("node_id", fmt.Sprint(config.NodeModel.Id)))
	// 初始化日志文件等
	l.NodeInfo = &types.NodeInfo{
		ServiceID:               config.ServiceID,
		ChainID:                 config.ChainModel.Id,
		NodeID:                  config.NodeModel.Id,
		NodeHandle:              config.NodeModel.NodeHandle,
		NodeName:                config.NodeModel.NodeName,
		Description:             config.NodeModel.Description,
		Enabled:                 true,
	}
	//无工具注册
	return nil
}

func (l *LogsNode) GetTools(rc *types.RunningContext) (currentToolList []*types.ToolDesc) {
	// todo 处理工具列表，可以增删改查

	// 将当前节点生成的工具列表加入到工具列表中
	logger.Info("当前工具列表", zap.String("node_id", fmt.Sprint(l.NodeInfo.NodeID)), zap.Any("tools", currentToolList))
	return
}

func (l *LogsNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (currentResp *mcp.CallToolResult, err error) {
	// 获取上一步的结果
	if lastStepResp != nil {
		currentResp = lastStepResp
	}
	// 日志节点不用判断，默认所有节点路过都打印日志
	logger.Info("当前节点数据",
		zap.String("node_id", fmt.Sprint(l.NodeInfo.NodeID)),
		zap.String("user_cmd", userCmd),
		zap.Any("user_param_map", userParamMap),
		zap.Any("current_resp", currentResp))
	return
}

// GetNodeInfo 获取节点信息
func (l *LogsNode) GetNodeInfo() *types.NodeInfo {
	return l.NodeInfo
}
