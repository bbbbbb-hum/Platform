package task_nodes

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"time"
)

// EchoNodeProcessor 回声节点处理器 - 提供多个回声相关的工具
type EchoNodeProcessor struct{}

func (e *EchoNodeProcessor) GetNodeName() string {
	return "Echo处理器"
}

func (e *EchoNodeProcessor) ProcessNode(nodeCtx *NodeContext) error {
	logger.Debug("上个节点数据", zap.Any("data", nodeCtx.Data))
	// 其他逻辑

	// 注册工具
	err := e.registerTools(nodeCtx.McpServer)
	if err != nil {
		return err
	}
	// 更新节点数据
	nodeCtx.Data = map[string]interface{}{
		"processed_at": time.Now().Unix(),
		"status":       "processed",
		"data":         e.GetNodeName(),
	}
	return nil
}

func (e *EchoNodeProcessor) registerTools(mcpServer *mcp.Server) error {
	// 注册基本回声工具
	mcp.AddTool[struct {
		Text string `json:"text"`
	}](mcpServer, &mcp.Tool{
		Name:        "echo",
		Title:       "基本回声",
		Description: "返回输入的文本内容",
	}, e.EchoTool)

	// 注册格式化回声工具
	mcp.AddTool[struct {
		Text   string `json:"text"`
		Prefix string `json:"prefix"`
		Suffix string `json:"suffix"`
	}](mcpServer, &mcp.Tool{
		Name:        "echo_formatted",
		Title:       "格式化回声",
		Description: "返回格式化后的文本内容",
	}, e.EchoFormattedTool)

	logger.Info("Echo节点处理器注册了 2 个工具")

	return nil
}

// EchoTool 基本回声工具
func (e *EchoNodeProcessor) EchoTool(ctx context.Context, req *mcp.CallToolRequest, args struct {
	Text string `json:"text"`
}) (*mcp.CallToolResult, interface{}, error) {
	result := fmt.Sprintf("Echo: %s", args.Text)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, result, nil
}

// EchoFormattedTool 格式化回声工具
func (e *EchoNodeProcessor) EchoFormattedTool(ctx context.Context, req *mcp.CallToolRequest, args struct {
	Text   string `json:"text"`
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}) (*mcp.CallToolResult, interface{}, error) {
	result := fmt.Sprintf("%s%s%s", args.Prefix, args.Text, args.Suffix)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, result, nil
}
