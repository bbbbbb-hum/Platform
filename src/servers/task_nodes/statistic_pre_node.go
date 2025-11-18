package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/types"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// StatisticPreNode 前置统计节点
type StatisticPreNode struct {
	NodeInfo *types.NodeInfo //节点信息
}

// Init 初始化空节点
func (s *StatisticPreNode) Init(config types.InitConfig) error {
	logger.Info("初始化统计节点", zap.String("node_id", string(config.NodeModel.Id)))
	s.NodeInfo = &types.NodeInfo{
		ServiceID:               config.ServiceID,
		ChainID:                 config.ChainModel.Id,
		NodeID:                  config.NodeModel.Id,
		NodeHandle:              config.NodeModel.NodeHandle,
		NodeName:                config.NodeModel.NodeName,
		Description:             config.NodeModel.Description,
		Enabled:                 true,
		ExternalServiceConfigID: config.NodeModel.ExternalServiceId,
	}
	logger.Info("前置统计节点初始化完成，不提供任何工具")
	return nil
}

// GetTools 获取工具列表 - 统计节点不提供任何工具
func (s *StatisticPreNode) GetTools(rc *types.RunningContext) (currentToolList []*types.ToolDesc) {
	// 统计节点不添加任何工具
	return
}

// Process 处理工具调用 - 空节点什么都不做
func (s *StatisticPreNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (currentResp *mcp.CallToolResult, err error) {
	// 获取上一步的结果
	if lastStepResp != nil {
		currentResp = lastStepResp
	}
	// 统计调用次数
	var serviceModel = &models.AeMcpServices{}
	err = serviceModel.UpdateCallNum(s.NodeInfo.ServiceID)
	if err != nil {
		return
	}
	// 创建统计日志
	var logModel = &models.AeMcpServicesRequestLogs{
		ServerId:     s.NodeInfo.ServiceID,
		ToolName:     userCmd,
		RequestTime:  time.Now(),
		ReturnTime:   time.Now(),
		ResponseTime: 0,
		Status:       0,
		CreateTime:   time.Now(),
		UpdateTime:   time.Now(),
	}
	err = logModel.Create() //todo:数据库操作太多
	if err != nil {
		return
	}
	rc.Stats["log_id"] = logModel.Id
	return
}

// GetNodeInfo 获取节点信息
func (s *StatisticPreNode) GetNodeInfo() *types.NodeInfo {
	return s.NodeInfo
}
