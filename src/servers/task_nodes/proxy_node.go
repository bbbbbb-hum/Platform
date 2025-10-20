package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/servers/pools"
	"AgentEarth_AgentPlatform/src/servers/types"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// sanitizeMcpJSONSchema removes or normalizes unsupported nullable type usages
// in JSON Schema to avoid AddTool validation panics in the MCP SDK.
func sanitizeMcpJSONSchema(schema *jsonschema.Schema) *jsonschema.Schema {
	if schema == nil {
		return nil
	}
	// Convert to a generic map for flexible traversal and mutation
	var m map[string]interface{}
	b, err := json.Marshal(schema)
	if err != nil {
		return schema
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return schema
	}
	sanitizeSchemaMap(m)
	// Convert back to mcp.JSONSchema
	var out jsonschema.Schema
	b2, err := json.Marshal(m)
	if err != nil {
		return schema
	}
	if err = json.Unmarshal(b2, &out); err != nil {
		return schema
	}
	return &out
}

// makeDefaultSchema builds a minimal, valid JSON Schema object schema.
func makeDefaultSchema() *jsonschema.Schema {
	m := map[string]interface{}{
		"$schema":    "https://json-schema.org/draft/2020-12/schema",
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	var out jsonschema.Schema
	b, err := json.Marshal(m)
	if err != nil {
		return &out
	}
	if err = json.Unmarshal(b, &out); err != nil {
		return &out
	}
	return &out
}

func sanitizeSchemaMap(node map[string]interface{}) {
	if node == nil {
		return
	}
	// Normalize the "type" field: remove null-only types, strip null from unions
	if rawType, ok := node["type"]; ok {
		switch tv := rawType.(type) {
		case nil:
			delete(node, "type")
		case string:
			if tv == "null" {
				delete(node, "type")
			}
		case []interface{}:
			var kept []interface{}
			for _, item := range tv {
				// keep only non-null, valid string type names
				if s, ok := item.(string); ok && s != "null" && s != "" {
					kept = append(kept, s)
				}
			}
			switch len(kept) {
			case 0:
				delete(node, "type")
			case 1:
				node["type"] = kept[0]
			default:
				node["type"] = kept
			}
		}
	}

	// Recurse into known schema-holding keywords
	// properties, $defs/definitions
	if props, ok := node["properties"].(map[string]interface{}); ok {
		for k, v := range props {
			if child, ok := v.(map[string]interface{}); ok {
				sanitizeSchemaMap(child)
				props[k] = child
			}
		}
	}
	if defs, ok := node["$defs"].(map[string]interface{}); ok {
		for k, v := range defs {
			if child, ok := v.(map[string]interface{}); ok {
				sanitizeSchemaMap(child)
				defs[k] = child
			}
		}
	}
	if defs, ok := node["definitions"].(map[string]interface{}); ok {
		for k, v := range defs {
			if child, ok := v.(map[string]interface{}); ok {
				sanitizeSchemaMap(child)
				defs[k] = child
			}
		}
	}

	// items can be schema or array of schemas
	if items, ok := node["items"]; ok {
		switch it := items.(type) {
		case map[string]interface{}:
			sanitizeSchemaMap(it)
			node["items"] = it
		case []interface{}:
			for i, v := range it {
				if child, ok := v.(map[string]interface{}); ok {
					sanitizeSchemaMap(child)
					it[i] = child
				}
			}
			node["items"] = it
		}
	}

	// anyOf, oneOf, allOf arrays
	for _, key := range []string{"anyOf", "oneOf", "allOf"} {
		if arr, ok := node[key].([]interface{}); ok {
			for i, v := range arr {
				if child, ok := v.(map[string]interface{}); ok {
					sanitizeSchemaMap(child)
					arr[i] = child
				}
			}
			node[key] = arr
		}
	}

	// not sub-schema
	if n, ok := node["not"].(map[string]interface{}); ok {
		sanitizeSchemaMap(n)
		node["not"] = n
	}
}

// ProxyNode SSE代理节点，用于代理SSE类型的MCP服务
type ProxyNode struct {
	NodeInfo *types.NodeInfo //节点信息
}

// Init 初始化SSE代理节点
func (p *ProxyNode) Init(config types.InitConfig) error {
	logger.Info("初始化代理节点", zap.String("node_id", string(config.NodeModel.Id)))
	p.NodeInfo = &types.NodeInfo{
		ServiceID:               config.ServiceID,
		ChainID:                 config.ChainModel.Id,
		NodeID:                  config.NodeModel.Id,
		NodeHandle:              config.NodeModel.NodeHandle,
		NodeName:                config.NodeModel.NodeName,
		Description:             config.NodeModel.Description,
		Enabled:                 true,
		ExternalServiceConfigID: config.NodeModel.ExternalServiceId,
	}

	// 初始化外部MCP服务
	if config.NodeModel.ExternalServiceId != "" {
		err := pools.GetConnectPool().InitializeService(config.NodeModel.ExternalServiceId)
		if err != nil {
			logger.Error("初始化外部 MCP服务失败", zap.Error(err))
			return err
		}
	}

	logger.Info("代理节点初始化完成")
	return nil
}

// GetTools 获取工具列表 - 从外部MCP服务获取工具
func (p *ProxyNode) GetTools(rc *types.RunningContext) (currentToolList []*types.ToolDesc) {

	// 从连接池获取外部服务的工具
	if p.NodeInfo.ExternalServiceConfigID != "" {
		externalTools := pools.GetConnectPool().GetServiceTools(p.NodeInfo.ExternalServiceConfigID)
		logger.Debug("工具列表", zap.Int("工具数量", len(externalTools)))
		for _, tool := range externalTools {
			// 将该节点上贡献的工具名称保存到节点信息中
			p.NodeInfo.ToolNames = append(p.NodeInfo.ToolNames, tool.Name)
			// 先清洗不兼容的可空类型，避免 AddTool 校验时 panic
			if tool.InputSchema == nil {
				tool.InputSchema = makeDefaultSchema()
			}
			tool.InputSchema = sanitizeMcpJSONSchema(tool.InputSchema)
			// 统一强制使用 2020-12，以满足本地 SDK 校验要求
			tool.InputSchema.Schema = "https://json-schema.org/draft/2020-12/schema"
			currentToolList = append(currentToolList, &types.ToolDesc{
				ToolDesc:        tool.Description,
				ToolInputSchema: tool.InputSchema,
				ToolName:        tool.Name,
			})
		}
	}

	logger.Info("代理节点获取工具列表", zap.Int("工具数量", len(currentToolList)))
	return
}

// Process 处理工具调用 - 调用外部MCP服务
func (p *ProxyNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (currentResp *mcp.CallToolResult, err error) {
	// 获取上一步的结果
	if lastStepResp != nil {
		currentResp = lastStepResp
	}

	// 调用必应的MCP服务的工具
	currentResp, err = pools.GetConnectPool().CallTool(p.NodeInfo.ExternalServiceConfigID, userCmd, userParamMap)
	if err != nil {
		return
	}
	logger.Debug("代理工具结果", zap.Any("result", currentResp))
	return
}

// GetNodeInfo 获取节点信息
func (p *ProxyNode) GetNodeInfo() *types.NodeInfo {
	return p.NodeInfo
}
