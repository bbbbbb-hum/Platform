package task_nodes

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/servers/tools"
	"fmt"
	"reflect"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// EchoToolNode 回声工具节点 - 提供工具
type EchoNode struct {
	NodeInfo *NodeInfo `json:"node_info"`
}

func (e *EchoNode) Init(ctx *RunningContext, node *models.AeMcpTaskNode) error {
	logger.Info("初始化回声工具节点", zap.String("node_id", string(node.Id)))
	e.NodeInfo = &NodeInfo{
		NodeID:      node.Id,
		NodeType:    node.NodeType,
		NodeName:    node.NodeName,
		Description: node.Description,
	}
	// 获取节点顺序
	if nodeStats, ok := ctx.Stats[node.NodeType].(map[string]interface{}); ok {
		if order := nodeStats["node_order"]; order != nil {
			e.NodeInfo.Order = order.(int)
		}
	}
	// 在初始化时注册工具到工具注册器
	echoParamsSchema, err := jsonschema.ForType(reflect.TypeOf(EchoParams{}), &jsonschema.ForOptions{
		IgnoreInvalidTypes: true,
	})
	if err != nil {
		return err
	}
	// 将节点ID编码到工具名称中
	toolName := fmt.Sprintf("echo__%d", node.Id) // 使用双下划线分隔

	echoTool := &mcp.Tool{
		Meta: mcp.Meta{
			"server_id": ctx.ServiceID,
			"chain_id":  ctx.ChainID,
			"node_type": node.NodeType,
			"node_id":   node.Id,
		},
		Name:        toolName,
		Title:       "Echo Tool",
		Description: "回声工具",
		InputSchema: echoParamsSchema,
	}
	toolsMap := tools.GetToolsMap()
	//可以添加，修改或者删除当然链上的所有工具
	//这里只做添加
	toolsMap.AddTool(ctx.ServiceID, toolName, echoTool)
	logger.Info("将Echo工具放入工具Map中", zap.String("tool_name", echoTool.Name))
	return nil
}

func (e *EchoNode) GetTools(ctx *RunningContext, lastStepToolList []*mcp.Tool) (currentToolList []*mcp.Tool, err error) {
	// 获取工具列表
	toolsMap := tools.GetToolsMap()
	currentToolList = lastStepToolList
	if toolsList, ok := toolsMap.GetServerTools(ctx.ServiceID); ok {
		for _, tool := range toolsList {
			currentToolList = append(currentToolList, tool)
		}
	}
	return
}
func (e *EchoNode) Process(ctx *RunningContext, userCmd string, userParamMap any, lastStepResp map[string]*CallToolResult) (currentResp map[string]*CallToolResult, err error) {
	currentResp = lastStepResp
	// 检查是否为当前节点的 echo 工具调用参数
	params, ok := userParamMap.(EchoParams)
	if !ok || params.Text == "" {
		err = fmt.Errorf("echo工具缺少参数")
		return
	}
	result := fmt.Sprintf("Echo: %s", params.Text)

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
	ctx.ResultMap[e.NodeInfo.NodeID] = currentResp
	return
}

func (e *EchoNode) GetNodeInfo() *NodeInfo {
	return e.NodeInfo
}

type EchoParams struct {
	Text string `json:"text"`
}
