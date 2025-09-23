package servers

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/servers/task_nodes"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// TaskChain 任务链
type TaskChain struct {
	ChainID   int32                             `json:"chain_id"`
	ServiceID string                            `json:"service_id"`
	Tools     []*mcp.Tool                       `json:"tools"`
	ToolMeta  map[string]map[string]interface{} `json:"tool_meta"` // 工具名 -> 元数据
}

// InitializeTaskChain 初始化任务链
func InitializeTaskChain(ctx *task_nodes.NodeContext) error {
	logger.Info("初始化任务链", zap.Int("chain_id", int(ctx.ChainID)))

	// 获取任务链和节点信息
	chainModel := &models.AeMcpTaskChain{}
	err := chainModel.GetOne(ctx.ChainID)
	if err != nil {
		return fmt.Errorf("获取任务链失败: %w", err)
	}
	// 链上所有节点信息
	nodesModel := &models.AeMcpTaskNode{}
	err, nodeModels := nodesModel.GetChianNodes(chainModel.NodeIds)
	if err != nil {
		return fmt.Errorf("获取节点失败: %w", err)
	}
	for i, nodeModel := range nodeModels {
		// 确保Stats中存在该节点类型的map
		if ctx.Stats[nodeModel.NodeType] == nil {
			ctx.Stats[nodeModel.NodeType] = make(map[string]interface{})
		}
		// 类型断言并添加node_order字段
		if nodeStats, ok := ctx.Stats[nodeModel.NodeType].(map[string]interface{}); ok {
			nodeStats["node_order"] = i
		}
		if node, ok := task_nodes.NodeMap[nodeModel.NodeType]; ok {
			// 初始化节点
			err = node.Init(ctx, nodeModel)
			if err != nil {
				logger.Error("初始化节点失败", zap.Error(err))
				continue
			}
		}
	}
	return nil
}

// ProcessToolCall 处理链上工具调用
func (chain *TaskChain) ProcessToolCall(ctx *task_nodes.NodeContext, toolName string, args map[string]interface{}) (map[string]*task_nodes.CallToolResult, error) {
	//conetxt需要从外面传入
	// 获取任务链和节点信息
	chainModel := &models.AeMcpTaskChain{}
	err := chainModel.GetOne(ctx.ChainID)
	if err != nil {
		return nil, fmt.Errorf("获取任务链失败: %w", err)
	}
	// 链上所有节点信息
	nodesModel := &models.AeMcpTaskNode{}
	err, nodeModels := nodesModel.GetChianNodes(chainModel.NodeIds)
	if err != nil {
		return nil, fmt.Errorf("获取节点失败: %w", err)
	}
	// 创建节点执行列表，包含节点和其order信息
	type NodeWithOrder struct {
		Node      task_nodes.Node
		NodeModel *models.AeMcpTaskNode
		Order     int
	}

	var nodesToExecute []NodeWithOrder

	// 从NodeMap中获取所有需要执行的节点，并获取其order
	for _, nodeModel := range nodeModels {
		if node, ok := task_nodes.NodeMap[nodeModel.NodeType]; ok {
			order := 0 // 默认order
			// 先尝试从节点的NodeInfo中获取Order
			if nodeInfo := node.GetNodeInfo(); nodeInfo != nil {
				order = nodeInfo.Order
			}
			nodesToExecute = append(nodesToExecute, NodeWithOrder{
				Node:      node,
				NodeModel: nodeModel,
				Order:     order,
			})
		}
	}

	// 按order排序
	sort.Slice(nodesToExecute, func(i, j int) bool {
		return nodesToExecute[i].Order < nodesToExecute[j].Order
	})

	// 上个节点处理结果
	lastStepResp := make(map[string]*task_nodes.CallToolResult)
	// 按顺序执行所有节点
	for _, nodeWithOrder := range nodesToExecute {
		// 执行节点
		lastStepResp, err = nodeWithOrder.Node.Process(ctx, toolName, args, lastStepResp)
		if err != nil {
			logger.Error("执行节点失败", zap.Error(err), zap.String("node_type", nodeWithOrder.NodeModel.NodeType))
			continue
		}
	}

	return lastStepResp, nil
}
