package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/servers/pools"
	"AgentEarth_AgentPlatform/src/servers/types"
	"encoding/json"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

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
			CleanDefaultNull(tool.InputSchema)
			b, _ := json.MarshalIndent(tool.InputSchema, "", "  ")
			logger.Debug("工具信息", zap.String("工具名称", tool.Name), zap.String("Input Schema 参数", string(b)))
			// 将该节点上贡献的工具名称保存到节点信息中
			p.NodeInfo.ToolNames = append(p.NodeInfo.ToolNames, tool.Name)
			// 修改工具输入参数的schema的版本
			if tool.InputSchema != nil && tool.InputSchema.Schema != "https://json-schema.org/draft/2020-12/schema" {
				tool.InputSchema.Schema = "https://json-schema.org/draft/2020-12/schema"
			}
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

// CleanDefaultNull 递归清理 schema 中的 default == nil 情形。
// 参数 s 可以是 *jsonschema.Schema（建议），函数会通过反射遍历常见子 schema 容器字段并递归调用。
// 如果某个字段名与库实现不同，但逻辑上是子 schema 容器（map[string]*Schema、[]*Schema、*Schema），
// 反射处理也能覆盖到。
func CleanDefaultNull(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	// 使用反射递归处理任意结构上的 Default 字段和子 schema 字段
	cleanValue(reflect.ValueOf(s))
}

// cleanValue 接受 reflect.Value，查找并清理 Default 字段，并递归进入常见子 schema 容器。
func cleanValue(v reflect.Value) {
	if !v.IsValid() {
		return
	}
	// 如果是指针，取元素
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	// 只处理 struct 类型（schema 期望为 struct）
	if v.Kind() != reflect.Struct {
		return
	}

	typ := v.Type()

	// 1) 查找 Default 字段并删除（设为 nil / zero）
	if f := v.FieldByName("Default"); f.IsValid() && f.CanSet() {
		// 如果 Default 是 interface{} 或指针类型且为 nil，则将其设置为零值（已经是零值）
		// 如果 Default 非 nil 且是 reflect.Zero 的值，不做修改。
		// 目标：确保当 Default 为 nil 时，不会被编码成 explicit null（视序列化实现而定）。
		z := reflect.Zero(f.Type())
		f.Set(z)
	}

	// 2) 常见的 child containers（属性、definitions、items、组合关键字等）
	// 列表里放常见字段名，优先尝试这些名字
	fieldNames := []string{
		"Properties", "PatternProperties", "Definitions",
		"AdditionalProperties", "Items", "Items2020",
		"AllOf", "AnyOf", "OneOf", "Not", "If", "Then", "Else",
		"Dependencies", "Schema", // 兜底项
	}

	for _, name := range fieldNames {
		if f := v.FieldByName(name); f.IsValid() {
			cleanContainerField(f)
		}
	}

	// 3) 兜底：检查所有 struct 字段，若字段为 map/string->*Schema 或 []*Schema 或 *Schema，则递归
	for i := 0; i < typ.NumField(); i++ {
		f := v.Field(i)
		if !f.IsValid() {
			continue
		}
		cleanContainerField(f)
	}
}

// cleanContainerField 处理可能的容器字段：map[string]*Schema、[]*Schema、*Schema、interface{}（可能为上述之一）
func cleanContainerField(f reflect.Value) {
	// 如果是指针或接口，取元素
	for f.Kind() == reflect.Ptr || f.Kind() == reflect.Interface {
		if f.IsNil() {
			return
		}
		f = f.Elem()
	}

	switch f.Kind() {
	case reflect.Map:
		// map: 遍历值，若值是 struct/ptr，递归
		for _, key := range f.MapKeys() {
			val := f.MapIndex(key)
			if !val.IsValid() || val.IsZero() {
				continue
			}
			// 递归处理 map 中的每个 value
			if val.CanInterface() {
				cleanValue(reflect.ValueOf(val.Interface()))
			} else {
				cleanValue(val)
			}
		}
	case reflect.Slice, reflect.Array:
		// slice/array: 遍历元素
		n := f.Len()
		for i := 0; i < n; i++ {
			elem := f.Index(i)
			if !elem.IsValid() || elem.IsZero() {
				continue
			}
			if elem.CanInterface() {
				cleanValue(reflect.ValueOf(elem.Interface()))
			} else {
				cleanValue(elem)
			}
		}
	case reflect.Struct:
		// struct: 递归进入
		cleanValue(f)
	default:
		// 其他类型无操作
		return
	}
}
