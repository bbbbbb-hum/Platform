package server

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/server/task_nodes"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// TaskChainProcessor 任务链处理器
type TaskChainProcessor struct {
	Chain *models.AeMcpTaskChain
	Nodes []*models.AeMcpTaskNode
}

// ProcessTaskChain 处理任务链
func ProcessTaskChain(chainId int32, mcpServer *mcp.Server) error {
	logger.Info("开始处理任务链", zap.Int("chainId", int(chainId)))

	// 获取任务链和节点信息
	chainModel := &models.AeMcpTaskChain{}
	err := chainModel.GetOneById(chainId)
	if err != nil {
		return fmt.Errorf("获取任务链失败: %w", err)
	}
	if chainModel == nil {
		return fmt.Errorf("任务链不存在")
	}
	nodesModel := &models.AeMcpTaskNode{}
	err, nodes := nodesModel.GetChianNodes(chainModel.NodeIds)
	if err != nil {
		return fmt.Errorf("获取任务链失败: %w", err)
	}

	if len(nodes) == 0 {
		logger.Info("任务链没有可用的节点", zap.Int("chainId", int(chainId)))
		return nil
	}

	// 创建任务链处理器
	processor := &TaskChainProcessor{
		Chain: chainModel,
		Nodes: nodes,
	}

	// 为每个节点添加对应的MCP工具
	return processor.RegisterNodesToMcpServer(mcpServer)
}

// RegisterNodesToMcpServer 将节点处理器注册到MCP服务器
func (p *TaskChainProcessor) RegisterNodesToMcpServer(mcpServer *mcp.Server) error {
	// 初始化空的节点数据
	var nodeCtx = &task_nodes.NodeContext{
		ChainID:   p.Chain.Id,
		Data:      make(map[string]interface{}),
		McpServer: mcpServer,
	}
	for _, node := range p.Nodes {
		if !node.Enabled {
			continue
		}

		// 根据节点配置获取对应的处理器
		processor, err := p.GetNodeProcessor(node)
		if err != nil {
			logger.Error("获取节点处理器失败", zap.String("nodeName", node.NodeName), zap.Error(err))
			continue
		}
		// 处理节点
		err = processor.ProcessNode(nodeCtx)
		if err != nil {
			logger.Error("处理节点失败", zap.String("nodeName", node.NodeName), zap.Error(err))
		}
		logger.Info("成功注册节点处理器", zap.String("nodeName", node.NodeName), zap.String("nodeHandle", node.NodeHandle))
	}

	return nil
}

// GetNodeProcessor 根据节点配置获取对应的处理器
func (p *TaskChainProcessor) GetNodeProcessor(node *models.AeMcpTaskNode) (task_nodes.NodeProcessor, error) {
	// 根据节点的NodeHandle获取对应的处理器
	processors := task_nodes.GetNodeProcessors()

	if processor, exists := processors[node.NodeHandle]; exists {
		return processor, nil
	}
	return nil, fmt.Errorf("未找到节点处理器: %s", node.NodeHandle)
}
