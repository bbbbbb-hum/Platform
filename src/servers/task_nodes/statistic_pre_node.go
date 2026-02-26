package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/cache"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/types"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shopspring/decimal"
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

// Process 处理工具调用 - 处理调用工具前置信息
func (s *StatisticPreNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (currentResp *mcp.CallToolResult, err error) {
	// 获取上一步的结果
	if lastStepResp != nil {
		currentResp = lastStepResp
	}
	// 获取工具价格信息
	toolsPrice := cache.GetToolsPrice(rc.ServiceID, userCmd)
	rc.Stats["tools_price"] = toolsPrice
	// 获取账户信息
	userId := rc.Stats["user_id"].(string)
	keyId := rc.Stats["key_id"].(int64)
	userBalanceKey := cache.GetUserBalanceKey(userId)
	userBalance := cache.GetUserBalance(userBalanceKey, userId)

	// 如果工具价格为0，则不进行价格对比
	if toolsPrice > 0 {
		// 对比价格（使用 decimal 避免浮点精度问题）
		userBalanceDec := decimal.NewFromFloat(userBalance)
		toolsPriceDec := decimal.NewFromFloat(toolsPrice)
		if userBalanceDec.LessThan(toolsPriceDec) {
			currentResp = &mcp.CallToolResult{
				Meta: mcp.Meta{
					"error": "Insufficient balance.",
				},
				StructuredContent: nil,
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: "Insufficient balance.",
					},
				},
			}
			err = fmt.Errorf("%s 账户余额不足~", userId)
			return
		}
		// 设置用户使用量增量 （更改到后置统计节点）
		//err = cache.SetUserUsageIncrement(userId, toolsPrice)
		//if err != nil {
		//	logger.Error("设置用户使用量增量失败", zap.String("user_id", userId), zap.Error(err))
		//	return
		//}
	}
	// 统计调用次数 (改用定时任务)
	// var serviceModel = &models.AeMcpServices{}
	// err = serviceModel.UpdateCallNum(s.NodeInfo.ServiceID)
	// if err != nil {
	// 	return
	// }
	// 创建统计日志 放入上下文
	rc.Stats["log"] = &models.AeMcpServicesRequestLogs{
		ServerId:       s.NodeInfo.ServiceID,
		ToolName:       userCmd,
		RequestTime:    time.Now(),
		ReturnTime:     time.Now(),
		ResponseTime:   0,
		Status:         0,
		CreateTime:     time.Now(),
		UpdateTime:     time.Now(),
		UserId:         userId,
		KeyId:          keyId,
		XlcreditAmount: toolsPrice,
	}
	return
}

// GetNodeInfo 获取节点信息
func (s *StatisticPreNode) GetNodeInfo() *types.NodeInfo {
	return s.NodeInfo
}
