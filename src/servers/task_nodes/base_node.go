package task_nodes

import (
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/types"

	"github.com/google/jsonschema-go/jsonschema"
)

type ToolDesc struct {
	ToolName        string
	ToolDesc        string
	ToolInputSchema *jsonschema.Schema
	// ToolOutputSchema string
	// ToolMeta map[string]interface{}
}

// Node 节点接口 - 定义节点的基本行为
type Node interface {
	// 初始化单个task_nodes数据库记录的实例
	Init(config InitConfig) error
	// 获取该节点支持的工具列表
	GetTools(ctx *types.RunningContext, lastStepToolList []*ToolDesc) (currentToolList []*ToolDesc)
	// 处理工具调用
	Process(ctx *types.RunningContext,
		userCmd string,
		userParamMap map[string]interface{},
		lastStepResp map[string]*types.CallToolResult) (currentResp map[string]*types.CallToolResult, err error)
	// 获取节点信息
	GetNodeInfo() *NodeInfo
}

type (
	// InitConfig 节点初始化参数
	InitConfig struct {
		NodeModel *models.AeMcpTaskNode
	}
	// NodeInfo 节点基本信息
	NodeInfo struct {
		NodeID                  int32    `json:"node_id"`                    // 数据库中的节点ID
		NodeHandle              string   `json:"node_handle"`                // 节点执行函数
		NodeName                string   `json:"node_name"`                  // 节点名称
		Description             string   `json:"description"`                // 节点描述
		Enabled                 bool     `json:"enabled"`                    // 节点是否启用
		ExternalServiceConfigID string   `json:"external_service_config_id"` // 外部服务配置ID
		ToolNames               []string `json:"tool_names"`                 // 贡献过的工具名称
	}
	// NodeInstance 节点实例
	NodeInstance struct {
		Node     Node      //节点实现的接口
		NodeInfo *NodeInfo //节点信息
	}
)

// NodeRegistry 节点注册表 - 根据node_type创建对应的节点实例
var NodeRegistry = map[string]func() Node{
	"echo_handle": func() Node {
		return &EchoNode{}
	},
	"logs_handle": func() Node {
		return &LogsNode{}
	},
	"empty_handle": func() Node {
		return &EmptyNode{}
	},
	"search_handle": func() Node {
		return &SearchNode{}
	},
	"proxy_handle": func() Node {
		return &ProxyNode{}
	},
}

// CreateNodeByType 根据node_type创建节点实例
func CreateNodeByType(nodeType string) (Node, bool) {
	if factory, exists := NodeRegistry[nodeType]; exists {
		return factory(), true
	}
	return nil, false
}
