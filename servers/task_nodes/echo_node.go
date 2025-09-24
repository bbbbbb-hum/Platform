package task_nodes

import (
	"AgentEarth_AgentPlatform/servers/types"
	"fmt"
	"reflect"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// EchoNode (简单的业务节点)回声工具节点 - 提供工具 B
type EchoNode struct {
	NodeInfo *NodeInfo   //节点信息
	Tools    []*mcp.Tool // 节点支持的工具
}

func (e *EchoNode) Init(config InitConfig) error {
	logger.Info("初始化回声工具节点", zap.String("node_id", string(config.NodeModel.Id)))
	e.NodeInfo = &NodeInfo{
		NodeID:      config.NodeModel.Id,
		NodeType:    config.NodeModel.NodeType,
		NodeName:    config.NodeModel.NodeName,
		Description: config.NodeModel.Description,
	}
	// 在初始化时将工具挂到节点上
	echoParamsSchema, err := jsonschema.ForType(reflect.TypeOf(EchoParams{}), &jsonschema.ForOptions{
		IgnoreInvalidTypes: true,
	})
	if err != nil {
		return err
	}
	// 将节点ID编码到工具名称中
	toolName := fmt.Sprintf("echo__%d", config.NodeModel.Id) // 使用双下划线分隔

	echoTool := &mcp.Tool{
		Meta: mcp.Meta{
			"server_id": config.ServerID,
			"chain_id":  config.ChianID,
			"node_type": config.NodeModel.NodeType,
			"node_id":   config.NodeModel.Id,
		},
		Name:        toolName,
		Title:       "Echo Tool",
		Description: "回声工具",
		InputSchema: echoParamsSchema,
	}
	e.Tools = append(e.Tools, echoTool)
	logger.Info("Echo 新增了1个工具", zap.String("tool_name", echoTool.Name))
	return nil
}

func (e *EchoNode) GetTools(rc *types.RunningContext) (currentToolList []*mcp.Tool) {
	// 获取上下文中工具列表
	currentToolList = rc.Tools
	// todo 处理上下文中工具，可以增删改查

	// 将当前节点生成的工具列表加入到工具列表中
	currentToolList = append(currentToolList, e.Tools...)
	return
}
func (e *EchoNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp map[string]*types.CallToolResult) (currentResp map[string]*types.CallToolResult, err error) {
	currentResp = lastStepResp
	// 检查是否为当前节点的 echo 工具调用参数
	var text string
	if v, ok := userParamMap["text"]; ok {
		if s, ok := v.(string); ok {
			text = s
		}
	}
	if text == "" {
		err = fmt.Errorf("echo工具缺少参数")
		return
	}
	result := fmt.Sprintf("Echo: %s", text)

	currentResp[userCmd].Result = &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}
	currentResp[userCmd].StructuredResult = map[string]interface{}{
		"echoed_text": result,
		"timestamp":   time.Now().Unix(),
		"node_id":     e.NodeInfo.NodeID,
	}
	// 将结果保存到节点上下文
	rc.ResultMap[e.NodeInfo.NodeID] = currentResp
	return
}

// GetNodeInfo 获取节点信息
func (e *EchoNode) GetNodeInfo() *NodeInfo {
	return e.NodeInfo
}

type EchoParams struct {
	Text string `json:"text"`
}
