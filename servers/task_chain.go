package servers

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/servers/task_nodes"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// TaskChain 任务链
type TaskChain struct {
	ChainID   int32                             `json:"chain_id"`
	ServiceID string                            `json:"service_id"`
	Nodes     []task_nodes.Node                 `json:"nodes"`
	Tools     []*mcp.Tool                       `json:"tools"`
	ToolMeta  map[string]map[string]interface{} `json:"tool_meta"` // 工具名 -> 元数据
}

// SimpleToolRegistry 简单工具注册器实现
type SimpleToolRegistry struct {
	tools    []*mcp.Tool
	toolMeta map[string]map[string]interface{}
}

func NewSimpleToolRegistry() *SimpleToolRegistry {
	return &SimpleToolRegistry{
		tools:    make([]*mcp.Tool, 0),
		toolMeta: make(map[string]map[string]interface{}),
	}
}

func (r *SimpleToolRegistry) RegisterTool(tool *mcp.Tool) error {
	// 检查工具名是否重复
	for _, existingTool := range r.tools {
		if existingTool.Name == tool.Name {
			return fmt.Errorf("工具名称重复: %s", tool.Name)
		}
	}

	r.tools = append(r.tools, tool)

	// 暂时简化元数据处理，避免类型断言问题
	// TODO: 根据实际的 Meta 字段类型调整
	//r.toolMeta[tool.Name] = make(map[string]interface{})
	r.toolMeta[tool.Name] = tool.Meta.GetMeta()
	// 如果需要存储元数据，可以在工具名称中编码，或使用其他方式
	logger.Debug("工具Meta字段类型", zap.String("type", fmt.Sprintf("%T", tool.Meta)))

	logger.Debug("工具注册成功", zap.String("tool_name", tool.Name))
	return nil
}

func (r *SimpleToolRegistry) GetAllTools() []*mcp.Tool {
	return r.tools
}

func (r *SimpleToolRegistry) GetToolMeta(toolName string) map[string]interface{} {
	return r.toolMeta[toolName]
}

// NodeFactory 节点工厂
type NodeFactory struct{}

func (f *NodeFactory) CreateNode(order int, nodeModel *models.AeMcpTaskNode) (task_nodes.Node, error) {
	baseNode := &task_nodes.BaseNode{
		Info: &task_nodes.NodeInfo{
			NodeID:      nodeModel.Id,
			NodeType:    nodeModel.NodeType,
			NodeName:    nodeModel.NodeName,
			Description: nodeModel.Description,
			Order:       order,
		},
		//Config:  nodeModel.Config,
		Enabled: nodeModel.Enabled,
	}

	switch nodeModel.NodeType {
	case "logger":
		return &task_nodes.LoggerNode{BaseNode: *baseNode}, nil
	case "echo_tool":
		return &task_nodes.EchoToolNode{BaseNode: *baseNode}, nil
	default:
		return nil, fmt.Errorf("未知节点类型: %s", nodeModel.NodeType)
	}
}

// InitializeTaskChain 初始化任务链
func InitializeTaskChain(chainID int32, serviceID string) (*TaskChain, error) {
	logger.Info("初始化任务链", zap.Int("chain_id", int(chainID)))

	// 获取任务链和节点信息
	chainModel := &models.AeMcpTaskChain{}
	err := chainModel.GetOne(chainID)
	if err != nil {
		return nil, fmt.Errorf("获取任务链失败: %w", err)
	}
	// 节点信息
	nodesModel := &models.AeMcpTaskNode{}
	err, nodeModels := nodesModel.GetChianNodes(chainModel.NodeIds)
	if err != nil {
		return nil, fmt.Errorf("获取节点失败: %w", err)
	}

	// 创建链
	chain := &TaskChain{
		ChainID:   chainID,
		ServiceID: serviceID,
		Nodes:     []task_nodes.Node{},
		ToolMeta:  make(map[string]map[string]interface{}),
	}

	// 创建节点
	factory := &NodeFactory{}
	for i, nodeModel := range nodeModels {
		if !nodeModel.Enabled {
			logger.Info("跳过未启用的节点", zap.Int("node_id", int(nodeModel.Id)))
			continue
		}

		node, err1 := factory.CreateNode(i, nodeModel)
		if err1 != nil {
			logger.Error("创建节点失败", zap.Error(err1))
			continue
		}
		chain.Nodes = append(chain.Nodes, node)
	}

	// 初始化链
	if err = chain.Initialize(); err != nil {
		return nil, err
	}

	return chain, nil
}

// Initialize 初始化链
func (chain *TaskChain) Initialize() error {
	// 创建工具注册器
	toolRegistry := NewSimpleToolRegistry()

	// 创建上下文信息
	ctx := &task_nodes.NodeContext{
		ChainID:      chain.ChainID,
		ServiceID:    chain.ServiceID,
		IsInitPhase:  true,
		ToolRegistry: toolRegistry,
		SharedData:   make(map[string]interface{}),
		Stats:        make(map[string]interface{}),
	}

	// 1. 初始化所有节点
	for _, node := range chain.Nodes {
		ctx.CurrentNode = node.GetNodeInfo()
		if err := node.Init(ctx); err != nil {
			return fmt.Errorf("初始化节点 %d 失败: %w", ctx.CurrentNode.NodeID, err)
		}
	}

	// 2. 获取所有注册的工具
	chain.Tools = toolRegistry.GetAllTools()
	chain.ToolMeta = toolRegistry.toolMeta

	logger.Info("任务链初始化完成",
		zap.Int("chain_id", int(chain.ChainID)),
		zap.Int("nodes_count", len(chain.Nodes)),
		zap.Int("tools_count", len(chain.Tools)))

	return nil
}

// ProcessToolCall 处理工具调用
func (chain *TaskChain) ProcessToolCall(toolName string, args map[string]interface{}, meta map[string]interface{}) (*mcp.CallToolResult, interface{}, error) {
	//创建处理节点的上下文
	//conetxt需要从外面传入
	ctx := &task_nodes.NodeContext{
		ChainID:     chain.ChainID,
		ServiceID:   chain.ServiceID,
		ToolName:    toolName,
		ToolArgs:    args,
		ToolMeta:    meta,
		IsInitPhase: false,
		SharedData:  make(map[string]interface{}),
		Stats:       make(map[string]interface{}),
	}

	// 执行所有节点
	for _, node := range chain.Nodes {
		ctx.CurrentNode = node.GetNodeInfo()
		//Node和chain需要一样实现i-b接口
		if err := node.Process(ctx); err != nil {
			return nil, nil, fmt.Errorf("节点 %d 处理失败: %w", ctx.CurrentNode.NodeID, err)
		}
	}

	return ctx.Result, ctx.StructuredResult, nil
}

// GetTools 获取链的所有工具
func (chain *TaskChain) GetTools() []*mcp.Tool {
	return chain.Tools
}

// GetToolMeta 获取工具元数据
func (chain *TaskChain) GetToolMeta(toolName string) map[string]interface{} {
	// 从工具名称中解析节点ID
	meta := make(map[string]interface{})

	// 解析编码在工具名称中的信息
	if parts := strings.Split(toolName, "__"); len(parts) == 2 {
		baseName := parts[0]
		nodeID := parts[1]

		meta["base_name"] = baseName
		meta["node_id"] = nodeID
		meta["tool_name"] = toolName

		logger.Debug("解析工具元数据",
			zap.String("tool_name", toolName),
			zap.String("base_name", baseName),
			zap.String("node_id", nodeID))
	}

	// 合并存储的元数据
	if storedMeta, exists := chain.ToolMeta[toolName]; exists {
		for k, v := range storedMeta {
			meta[k] = v
		}
	}

	return meta
}
