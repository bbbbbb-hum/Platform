package types

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunningContext 节点上下文 - 在节点间传递数据
type RunningContext struct {
	// 链级别信息
	ServiceID string                               `json:"service_id"`
	ChainID   int32                                `json:"chain_id"`
	Tools     []*mcp.Tool                          `json:"tools"`
	ResultMap map[int32]map[string]*CallToolResult `json:"result_map"` // 结果数据 map[nodeId]map[tool_name]mcp结果
	Stats     map[string]interface{}               `json:"stats"`      // 其他信息
}

// CallToolResult 工具调用结果
type CallToolResult struct {
	Result           *mcp.CallToolResult
	StructuredResult interface{}
}
