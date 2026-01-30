package task_chain

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/task_nodes"
	"AgentEarth_AgentPlatform/src/servers/types"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"go.uber.org/zap"
)

type (
	// ChainInstance 任务链实例
	ChainInstance struct {
		ServerID      string
		ChainID       int32
		NodeInstances []*types.NodeInstance
	}
)

func (i *ChainInstance) Init(config types.InitConfig) error {
	logger.Info("初始化任务链", zap.Int("chain_id", int(config.ChainModel.Id)))
	i.ServerID = config.ServiceID
	i.ChainID = config.ChainModel.Id
	// 获取链上所有节点数据
	nodesModel := &models.AeMcpTaskNode{}
	err, nodeModels := nodesModel.GetChianNodes(config.ChainModel.NodeIds)
	if err != nil {
		return fmt.Errorf("获取节点失败: %w", err)
	}
	// 初始化所有节点
	for _, nodeModel := range nodeModels {
		// 根据node_type获取对应的节点工厂函数来创建节点实例
		if node, ok := task_nodes.CreateNodeByType(nodeModel.NodeHandle); ok {
			// 调用节点的Init方法，初始化节点
			err = node.Init(types.InitConfig{
				ServiceID:  config.ServiceID,
				ChainModel: config.ChainModel,
				NodeModel:  nodeModel,
			})
			if err != nil {
				logger.Error("初始化节点失败", zap.Error(err), zap.String("node_handle", nodeModel.NodeHandle), zap.Int32("node_id", nodeModel.Id))
				continue
			}
			// 创建NodeInstance包装器
			nodeInstance := &types.NodeInstance{
				Node:     node,
				NodeInfo: node.GetNodeInfo(),
			}
			// 将node实例加到链的NodeInstances中
			i.NodeInstances = append(i.NodeInstances, nodeInstance) //Node实例放在链实例中就够了，不需要额外的map

			logger.Info("成功初始化节点", zap.String("node_handle", nodeModel.NodeHandle), zap.Int32("node_id", nodeModel.Id), zap.Int32("chain_id", config.ChainModel.Id))

		} else {
			logger.Error("不支持的节点类型", zap.String("node_handle", nodeModel.NodeHandle), zap.Int32("node_id", nodeModel.Id))
		}
	}

	return nil
}

func (i *ChainInstance) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}) (currentResp *mcp.CallToolResult, err error) {
	// TODO: 节点处理逻辑
	var lastResp *mcp.CallToolResult
	var err1 error
	for k, instance := range i.NodeInstances {
		logger.Debug("节点流转开始...", zap.String("node_name", instance.Node.GetNodeInfo().NodeName), zap.Int("node_index", k))
		lastResp, err1 = instance.Node.Process(rc, userCmd, userParamMap, lastResp)
		if err1 != nil {
			logger.Error("节点处理失败", zap.Error(err1), zap.String("node_name", instance.Node.GetNodeInfo().NodeName))
			continue
		}
		logger.Debug("节点流转成功", zap.String("node_name", instance.Node.GetNodeInfo().NodeName), zap.Int("node_index", k))
	}
	// 可以在这里处理链的逻辑
	currentResp = lastResp
	if currentResp == nil {
		currentResp = &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: "No results obtained.",
				},
			},
			StructuredContent: nil,
		}
	}
	return
}

func (i *ChainInstance) GetTools(rc *types.RunningContext) []*types.ToolDesc {
	var lastStepToolList []*types.ToolDesc
	for _, nodeInstance := range i.NodeInstances {
		// 将处理后的工具添加到上下文中的tools中
		lastStepToolList = append(lastStepToolList, nodeInstance.Node.GetTools(rc)...)
	}
	return lastStepToolList
}

// GetNodeInfo 获取节点信息
func (i *ChainInstance) GetNodeInfo() *types.NodeInfo {
	return &types.NodeInfo{}
}
