package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
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

	// 更新请求日志
	logId, ok := rc.Stats["log_id"]
	if !ok {
		err = fmt.Errorf("log_id not found")
		return
	}
	logIdInt, ok := logId.(int32)
	if !ok {
		err = fmt.Errorf("log_id is not int32")
		return
	}
	var requestLogModel = &models.AeMcpServicesRequestLogs{
		Id: logIdInt,
	}
	//todo:数据库操作太多
	err = requestLogModel.GetOne()
	if err != nil {
		err = fmt.Errorf("get request log failed: %w", err)
		return
	}
	requestLogModel.ReturnTime = time.Now()
	requestLogModel.ResponseTime = int32(requestLogModel.ReturnTime.UnixMilli() - requestLogModel.RequestTime.UnixMilli())
	requestLogModel.Status = 1
	err = requestLogModel.Update()
	if err != nil {
		return
	}
	// 更新调用成功次数
	var serviceModel = &models.AeMcpServices{}
	err = serviceModel.UpdateCallSuccess(s.NodeInfo.ServiceID)
	return
}

// GetNodeInfo 获取节点信息
func (s *StatisticRearNode) GetNodeInfo() *types.NodeInfo {
	return s.NodeInfo
}
