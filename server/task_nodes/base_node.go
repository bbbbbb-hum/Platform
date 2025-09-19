package task_nodes

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NodeProcessor 节点处理器接口
type NodeProcessor interface {
	// GetNodeName 获取节点名称
	GetNodeName() string
	// ProcessNode 处理节点逻辑（可选，用于节点间的数据传递）
	ProcessNode(nodeCtx *NodeContext) error
	// RegisterTools 注册节点相关的所有工具到MCP服务器
	registerTools(mcpServer *mcp.Server) (error, int)
}

// GetNodeProcessors 获取所有节点处理器映射
func GetNodeProcessors() map[string]NodeProcessor {
	return map[string]NodeProcessor{
		"a_processor":     &ANodeProcessor{},
		"b_processor":     &BNodeProcessor{},
		"c_processor":     &CNodeProcessor{},
		"gaode_processor": &GaoDeNodeProcessor{}, // 远程工具代理处理器
	}
}

type NodeContext struct {
	ChainID   int32
	Data      D
	McpServer *mcp.Server
	// 可以添加其他链级别的信息
}

type D map[string]interface{}
