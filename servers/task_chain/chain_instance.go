package task_chain

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/servers/task_nodes"
	"AgentEarth_AgentPlatform/servers/types"
	"fmt"

	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

type (
	// ChainInstance 任务链实例
	ChainInstance struct {
		ChainInfo     *ChainInfo
		NodeInstances []*task_nodes.NodeInstance
	}
)

func (i *ChainInstance) Init(config InitConfig) error {
	logger.Info("初始化任务链", zap.Int("chain_id", int(config.ChainModel.Id)))
	// 初始化链信息
	i.ChainInfo = &ChainInfo{
		ChainID:   config.ChainModel.Id,
		ServiceID: config.ServiceId,
	}
	// 获取链上所有节点数据
	nodesModel := &models.AeMcpTaskNode{}
	err, nodeModels := nodesModel.GetChianNodes(config.ChainModel.NodeIds)
	if err != nil {
		return fmt.Errorf("获取节点失败: %w", err)
	}
	// 初始化所有节点
	for _, nodeModel := range nodeModels {
		// 根据node_type获取对应的节点工厂函数来创建节点实例
		if node, ok := task_nodes.CreateNodeByType(nodeModel.NodeType); ok {
			// 调用节点的Init方法，初始化节点
			err = node.Init(task_nodes.InitConfig{
				NodeModel: nodeModel,
			})
			if err != nil {
				logger.Error("初始化节点失败", zap.Error(err), zap.String("node_type", nodeModel.NodeType), zap.Int32("node_id", nodeModel.Id))
				continue
			}
			// 创建NodeInstance包装器
			nodeInstance := &task_nodes.NodeInstance{
				Node:     node,
				NodeInfo: node.GetNodeInfo(),
			}
			// 将node实例加到链的NodeInstances中
			i.NodeInstances = append(i.NodeInstances, nodeInstance) //Node实例放在链实例中就够了，不需要额外的map

			logger.Info("成功初始化节点", zap.String("node_type", nodeModel.NodeType), zap.Int32("node_id", nodeModel.Id), zap.Int32("chain_id", config.ChainModel.Id))

		} else {
			logger.Error("不支持的节点类型", zap.String("node_type", nodeModel.NodeType), zap.Int32("node_id", nodeModel.Id))
		}
	}

	return nil
}

func (i *ChainInstance) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}) (currentResp map[string]*types.CallToolResult, err error) {
	// TODO: 节点处理逻辑
	var lastResp map[string]*types.CallToolResult

	for _, instance := range i.NodeInstances {
		lastResp, err = instance.Node.Process(rc, userCmd, userParamMap, lastResp)
		if err != nil {
			logger.Error("节点处理失败", zap.Error(err), zap.String("node_name", instance.Node.GetNodeInfo().NodeName))
			continue
		}
	}
	// 可以在这里处理链的逻辑
	currentResp = lastResp
	return
}

func (i *ChainInstance) GetChainInfo() *ChainInfo {
	return i.ChainInfo
}

func (i *ChainInstance) GetTools(rc *types.RunningContext) []*task_nodes.ToolDesc {
	var lastStepToolList []*task_nodes.ToolDesc
	for _, nodeInstance := range i.NodeInstances {
		// 将处理后的工具添加到上下文中的tools中
		lastStepToolList = nodeInstance.Node.GetTools(rc, lastStepToolList)
	}
	return lastStepToolList
}
