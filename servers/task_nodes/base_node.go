package task_nodes

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Node 节点接口 - 定义节点的基本行为
type Node interface {
	// 初始化节点（服务启动时调用一次）
	Init(ctx *NodeContext) error

	// 处理节点逻辑（每次工具调用都会执行）
	Process(ctx *NodeContext) error

	// 获取节点信息
	GetNodeInfo() *NodeInfo
}

// NodeInfo 节点基本信息
type NodeInfo struct {
	NodeID      int32  `json:"node_id"`     // 数据库中的节点ID
	NodeType    string `json:"node_type"`   // 节点类型
	NodeName    string `json:"node_name"`   // 节点名称
	Description string `json:"description"` // 节点描述
	Order       int    `json:"order"`       // 执行顺序
}

// ToolRegistry 工具注册器接口
type ToolRegistry interface {
	RegisterTool(tool *mcp.Tool) error
	GetAllTools() []*mcp.Tool
}

// NodeContext 节点上下文 - 在节点间传递数据
type NodeContext struct {
	// 链级别信息
	ChainID   int32  `json:"chain_id"`
	ServiceID string `json:"service_id"`

	// 工具注册器（仅在初始化阶段可用）
	ToolRegistry ToolRegistry `json:"-"`

	// 工具调用信息
	ToolName string                 `json:"tool_name,omitempty"` // 当前调用的工具名
	ToolArgs map[string]interface{} `json:"tool_args,omitempty"` // 工具参数
	ToolMeta map[string]interface{} `json:"tool_meta,omitempty"` // 工具元数据

	// 节点间共享数据
	SharedData map[string]interface{} `json:"shared_data"` // 节点间传递的数据

	// 执行状态
	IsInitPhase bool      `json:"is_init_phase"` // 是否为初始化阶段
	CurrentNode *NodeInfo `json:"current_node"`  // 当前执行的节点

	// 结果数据
	Result           *mcp.CallToolResult `json:"-"` // 最终返回结果
	StructuredResult interface{}         `json:"-"` // 结构化结果

	// 统计信息
	Stats map[string]interface{} `json:"stats"` // 统计数据
}

// BaseNode 基础节点结构体 - 提供通用功能
type BaseNode struct {
	Info    *NodeInfo              `json:"info"`
	Config  map[string]interface{} `json:"config"`  // 节点配置
	Enabled bool                   `json:"enabled"` // 是否启用
}

func (b *BaseNode) GetNodeInfo() *NodeInfo {
	return b.Info
}

// 可选：提供默认的空实现，子类可以选择性重写
func (b *BaseNode) Init(ctx *NodeContext) error {
	// 默认初始化逻辑
	return nil
}

func (b *BaseNode) Process(ctx *NodeContext) error {
	// 默认处理逻辑
	return nil
}
