package task_nodes

import (
	"AgentEarth_AgentPlatform/servers/types"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// LogsNode 日志记录节点 - 只记录，不提供工具
type LogsNode struct {
	NodeInfo *NodeInfo   //节点信息
	Tools    []*mcp.Tool // 节点支持的工具
}

func (l *LogsNode) Init(config InitConfig) error {
	logger.Info("初始化日志节点", zap.String("node_id", fmt.Sprint(config.NodeModel.Id)))
	// 初始化日志文件等
	l.NodeInfo = &NodeInfo{
		NodeID:      config.NodeModel.Id,
		NodeType:    config.NodeModel.NodeType,
		NodeName:    config.NodeModel.NodeName,
		Description: config.NodeModel.Description,
	}
	//无工具注册
	return nil
}

func (l *LogsNode) GetTools(rc *types.RunningContext) (currentToolList []*mcp.Tool) {
	// 获取上下文中工具列表
	currentToolList = rc.Tools
	// todo 处理上下文中工具，可以增删改查

	// 将当前节点生成的工具列表加入到工具列表中
	currentToolList = append(currentToolList, l.Tools...)
	return
}

func (l *LogsNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp map[string]*types.CallToolResult) (currentResp map[string]*types.CallToolResult, err error) {
	currentResp = lastStepResp
	// 打印当前节点数据
	logger.Info("当前节点数据", zap.String("node_id", fmt.Sprint(rc.ChainID)), zap.String("user_cmd", userCmd), zap.Any("user_param_map", userParamMap), zap.Any("last_step_resp", currentResp))
	return
}

func (l *LogsNode) GetNodeInfo() *NodeInfo {
	return l.NodeInfo
}
