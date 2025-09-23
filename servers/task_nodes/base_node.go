package task_nodes

import (
	"AgentEarth_AgentPlatform/models"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Node 节点接口 - 定义节点的基本行为
type Node interface {
	Init(ctx *NodeContext, node *models.AeMcpTaskNode) error
	GetTools(ctx *NodeContext, lastStepToolList []*mcp.Tool) (currentToolList []*mcp.Tool, err error)
	Process(ctx *NodeContext, userCmd string, userParamMap any, lastStepResp map[string]*CallToolResult) (currentResp map[string]*CallToolResult, err error)
	GetNodeInfo() *NodeInfo
}

var NodeMap = map[string]Node{
	"logger": &LoggerNode{},
	"echo":   &EchoNode{},
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
	ChainID   int32                                `json:"chain_id"`
	ServiceID string                               `json:"service_id"`
	ResultMap map[int32]map[string]*CallToolResult `json:"result_map"` // 结果数据 map[nodeId]map[tool_name]mcp结果
	Stats     map[string]interface{}               `json:"stats"`      // 其他信息
}

type CallToolResult struct {
	Result           *mcp.CallToolResult
	StructuredResult interface{}
}
