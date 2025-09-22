package task_nodes

import (
	"fmt"
	"reflect"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

// EchoToolNode 回声工具节点 - 提供工具
type EchoToolNode struct {
	BaseNode
}

func (e *EchoToolNode) Init(ctx *NodeContext) error {
	logger.Info("初始化回声工具节点", zap.String("node_id", fmt.Sprint(e.Info.NodeID)))

	// 在初始化时注册工具到工具注册器
	if ctx.IsInitPhase && ctx.ToolRegistry != nil {
		echoParamsSchema, err := jsonschema.ForType(reflect.TypeOf(EchoParams{}), &jsonschema.ForOptions{
			IgnoreInvalidTypes: true,
		})
		if err != nil {
			return err
		}
		// 将节点ID编码到工具名称中
		toolName := fmt.Sprintf("echo__%d", e.Info.NodeID) // 使用双下划线分隔

		echoTool := &mcp.Tool{
			Name:        toolName,
			Title:       "Echo Tool",
			Description: "回声工具",
			// Meta: 暂时不使用，避免类型问题
			InputSchema: echoParamsSchema,
		}

		if err = ctx.ToolRegistry.RegisterTool(echoTool); err != nil { //通过context传递，实现i-b接口
			logger.Error("注册Echo工具失败", zap.Error(err))
			return err
		}

		logger.Info("成功注册Echo工具", zap.String("tool_name", echoTool.Name))
	}

	return nil
}

func (e *EchoToolNode) Process(ctx *NodeContext) error {
	// 检查是否为当前节点的 echo 工具调用
	if e.isMyTool(ctx.ToolName) { //节点不能对应链上的工具
		message, ok := ctx.ToolArgs["text"].(string) // 注意：参数名改为 text
		if !ok {
			return fmt.Errorf("echo工具缺少text参数")
		}

		result := fmt.Sprintf("Echo: %s", message)

		ctx.Result = &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: result},
			},
		}
		ctx.StructuredResult = map[string]interface{}{
			"echoed_text": result,
			"timestamp":   time.Now().Unix(),
			"node_id":     e.Info.NodeID,
		}
	}
	return nil
}

// isMyTool 检查工具是否属于当前节点
func (e *EchoToolNode) isMyTool(toolName string) bool {
	expectedToolName := fmt.Sprintf("echo__%d", e.Info.NodeID)
	return toolName == expectedToolName
}

type EchoParams struct {
	Text string `json:"text"`
}
