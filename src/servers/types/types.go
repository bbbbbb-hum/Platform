package types

import (
	"AgentEarth_AgentPlatform/src/models"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type (
	// RunningContext 节点上下文 - 在节点间传递数据
	RunningContext struct {
		// 链级别信息
		ServiceID string                 `json:"service_id"`
		ChainID   int32                  `json:"chain_id"`
		Stats     map[string]interface{} `json:"stats"` // 其他信息
	}
	ToolDesc struct {
		ToolName        string
		ToolDesc        string
		ToolInputSchema *jsonschema.Schema
	}
	InitConfig struct {
		ServiceID  string `json:"service_id"`
		ChainModel *models.AeMcpTaskChain
		NodeModel  *models.AeMcpTaskNode
	}

	// NodeInstance 节点实例
	NodeInstance struct {
		NodeInfo *NodeInfo
		Node     Processor //节点实现的接口
	}
	NodeInfo struct {
		ServiceID               string   `json:"service_id"`
		ChainID                 int32    `json:"chain_id"`
		NodeID                  int32    `json:"node_id"`                    // 数据库中的节点ID
		NodeHandle              string   `json:"node_handle"`                // 节点执行函数
		NodeName                string   `json:"node_name"`                  // 节点名称
		Description             string   `json:"description"`                // 节点描述
		Enabled                 bool     `json:"enabled"`                    // 节点是否启用
		Protocol                string   `json:"protocol"`                   // 节点协议（默认 http）
		NodeURL                 string   `json:"node_url"`                   // 节点内网地址
		TimeoutMS               int      `json:"timeout_ms"`                 // 节点调用超时时间
		ToolNames               []string `json:"tool_names"`                 // 贡献过的工具名称
	}
)

type Processor interface {
	Init(config InitConfig) error
	Process(rc *RunningContext, userCmd string, userParamMap map[string]interface{}, lastResp *mcp.CallToolResult) (currentResp *mcp.CallToolResult, err error)
	GetTools(rc *RunningContext) []*ToolDesc
	GetNodeInfo() *NodeInfo
}
