package task_nodes

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"time"
)

// ValidationNodeProcessor 验证节点处理器
type ValidationNodeProcessor struct{}

func (v *ValidationNodeProcessor) GetNodeName() string {
	return "验证处理器"
}

func (v *ValidationNodeProcessor) ProcessNode(nodeCtx *NodeContext) error {
	logger.Debug("上个节点数据", zap.Any("data", nodeCtx.Data))
	// 其他逻辑

	// 注册工具
	err := v.registerTools(nodeCtx.McpServer)
	if err != nil {
		return err
	}
	// 更新节点数据
	nodeCtx.Data = map[string]interface{}{
		"processed_at": time.Now().Unix(),
		"status":       "processed",
		"data":         v.GetNodeName(),
	}
	return nil
}
func (v *ValidationNodeProcessor) registerTools(mcpServer *mcp.Server) error {
	mcp.AddTool[map[string]interface{}](mcpServer, &mcp.Tool{
		Name:        "validate_input",
		Title:       "输入验证",
		Description: "验证输入参数",
	}, v.ValidateInputTool)

	logger.Info("验证节点处理器注册了 1 个工具")
	return nil
}

func (v *ValidationNodeProcessor) ValidateInputTool(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, interface{}, error) {
	if len(args) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "验证失败: 输入参数为空"},
			},
		}, nil, fmt.Errorf("输入参数为空")
	}

	result := map[string]interface{}{
		"valid":        true,
		"input":        args,
		"validated_at": time.Now().Unix(),
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "输入验证通过"},
		},
	}, result, nil
}
