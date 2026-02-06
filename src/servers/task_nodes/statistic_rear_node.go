package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/helpers/mq"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/types"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// StatisticRearNode 后置统计节点
type StatisticRearNode struct {
	NodeInfo *types.NodeInfo //节点信息
}

// Init 初始化空节点
func (s *StatisticRearNode) Init(config types.InitConfig) error {
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
	logger.Info("后置统计节点初始化完成，不提供任何工具")
	return nil
}

// GetTools 获取工具列表 - 统计节点不提供任何工具
func (s *StatisticRearNode) GetTools(rc *types.RunningContext) (currentToolList []*types.ToolDesc) {
	// 统计节点不添加任何工具
	return
}

// Process 处理工具调用 - 空节点什么都不做
func (s *StatisticRearNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (currentResp *mcp.CallToolResult, err error) {
	// 获取上一步的结果
	if lastStepResp != nil {
		currentResp = lastStepResp
	}

	// 从上下文获取请求日志
	log, ok := rc.Stats["log"]
	if !ok {
		err = fmt.Errorf("log not found in context")
		return
	}
	requestLogModel, ok := log.(*models.AeMcpServicesRequestLogs)
	if !ok {
		err = fmt.Errorf("log is not *models.AeMcpServicesRequestLogs")
		return
	}

	// 更新日志信息
	requestLogModel.ReturnTime = time.Now()
	requestLogModel.ResponseTime = int32(requestLogModel.ReturnTime.UnixMilli() - requestLogModel.RequestTime.UnixMilli())
	requestLogModel.Status = 1
	requestLogModel.UpdateTime = time.Now()

	// 将日志添加到缓冲池，由池统一批量发布到 NATS（或降级写入数据库）
	//pools.GetRequestLogsPool().Add(requestLogModel)
	err = mq.PublishRequestLog(requestLogModel)
	if err != nil {
		logger.Error("发布请求日志到 NATS 失败", zap.Error(err))
		// 发布失败，记录到错误日志便于后续补录
		logger.Error("NATS 发布失败，日志已记录", zap.Any("miss_request_logs", requestLogModel))
	} else {
		logger.Debug("发布请求日志到 NATS 成功")
	}
	return
}

// GetNodeInfo 获取节点信息
func (s *StatisticRearNode) GetNodeInfo() *types.NodeInfo {
	return s.NodeInfo
}
