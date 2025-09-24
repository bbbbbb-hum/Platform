package task_chain

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/servers/ctx"
	ctx2 "AgentEarth_AgentPlatform/servers/ctx"
	"AgentEarth_AgentPlatform/servers/task_nodes"
	"fmt"

	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type (
	ChainInstance struct {
		*Chain
	}
)

type InitConfig struct {
	ServiceId  string
	ChainModel *models.AeMcpTaskChain
}

func (i *ChainInstance) Init(config InitConfig) error {
	logger.Info("初始化任务链", zap.Int("chain_id", int(config.ChainModel.Id)))
	i.Chain = &Chain{
		ChainInfo: &ChainInfo{
			ChainID:   config.ChainModel.Id,
			ServiceID: config.ServiceId,
		},
	}
	// 链上所有节点信息
	nodesModel := &models.AeMcpTaskNode{}
	err, nodeModels := nodesModel.GetChianNodes(config.ChainModel.NodeIds)
	if err != nil {
		return fmt.Errorf("获取节点失败: %w", err)
	}
	// 获取NodeInstanceMap
	nodeInstanceMap := task_nodes.GetNodeInstanceMap()

	// 初始化所有节点
	for _, nodeModel := range nodeModels {
		// 根据node_type创建对应的节点实例
		if node, ok := task_nodes.CreateNodeByType(nodeModel.NodeType); ok {
			// 调用节点的Init方法
			err = node.Init(task_nodes.InitConfig{
				ServerID:  config.ServiceId,
				ChianID:   config.ChainModel.Id,
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
				Tools: node.GetTools(&ctx.RunningContext{
					ServiceID: config.ServiceId,
				}),
			}
			// 将instance放入NodeInstanceMap
			nodeInstanceMap.AddNode(config.ChainModel.Id, nodeInstance)

			// 也加到链的Nodes中
			i.Chain.Nodes = append(i.Chain.Nodes, node) //Node实例放在链实例中就够了，不需要额外的map

			logger.Info("成功初始化节点", zap.String("node_type", nodeModel.NodeType), zap.Int32("node_id", nodeModel.Id), zap.Int32("chain_id", config.ChainModel.Id))

		} else {
			logger.Error("不支持的节点类型", zap.String("node_type", nodeModel.NodeType), zap.Int32("node_id", nodeModel.Id))
		}
	}

	return nil
}

func (i *ChainInstance) Process(ctx *ctx.RunningContext, userCmd string, userParamMap map[string]interface{}) (currentResp map[string]*ctx.CallToolResult, err error) {
	// TODO: 节点处理逻辑
	var lastResp map[string]*ctx2.CallToolResult
	for _, node := range i.Nodes {
		lastResp, err = node.Process(ctx, userCmd, userParamMap, lastResp)
		if err != nil {
			logger.Error("节点处理失败", zap.Error(err), zap.String("node_name", node.GetNodeInfo().NodeName))
			continue
		}
	}
	// 可以在这里处理链的逻辑
	currentResp = lastResp
	return
}

func (i *ChainInstance) GetChain() *Chain {
	return i.Chain
}

func (i *ChainInstance) GetTools() []*mcp.Tool {
	var tools []*mcp.Tool
	for _, node := range i.Nodes {
		tools = append(tools, node.GetTools(&ctx.RunningContext{
			ServiceID: i.ChainInfo.ServiceID,
		})...)
	}
	return tools
}
