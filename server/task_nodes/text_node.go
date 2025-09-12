package task_nodes

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"strings"
	"time"
)

// TextNodeProcessor 文本处理节点处理器 - 演示一个节点可以注册多个工具
type TextNodeProcessor struct{}

func (t *TextNodeProcessor) GetNodeName() string {
	return "文本处理器"
}

func (t *TextNodeProcessor) ProcessNode(nodeCtx *NodeContext) error {
	logger.Debug("上个节点数据", zap.Any("data", nodeCtx.Data))
	// 其他逻辑

	// 注册工具
	err := t.registerTools(nodeCtx.McpServer)
	if err != nil {
		return err
	}
	// 更新节点数据
	nodeCtx.Data = map[string]interface{}{
		"processed_at": time.Now().Unix(),
		"status":       "processed",
		"data":         t.GetNodeName(),
	}
	return nil
}

func (t *TextNodeProcessor) registerTools(mcpServer *mcp.Server) error {
	// 注册文本格式化工具
	mcp.AddTool[struct {
		Text   string `json:"text"`
		Format string `json:"format"`
	}](mcpServer, &mcp.Tool{
		Name:        "text_format",
		Title:       "文本格式化",
		Description: "格式化文本内容",
	}, t.TextFormatTool)

	// 注册文本分析工具
	mcp.AddTool[struct {
		Text string `json:"text"`
	}](mcpServer, &mcp.Tool{
		Name:        "text_analyze",
		Title:       "文本分析",
		Description: "分析文本内容",
	}, t.TextAnalyzeTool)

	// 注册文本转换工具
	mcp.AddTool[struct {
		Text   string `json:"text"`
		Target string `json:"target"`
	}](mcpServer, &mcp.Tool{
		Name:        "text_convert",
		Title:       "文本转换",
		Description: "转换文本格式",
	}, t.TextConvertTool)

	logger.Info("文本处理节点处理器注册了 3 个工具")
	return nil
}
func (t *TextNodeProcessor) TextFormatTool(ctx context.Context, req *mcp.CallToolRequest, args struct {
	Text   string `json:"text"`
	Format string `json:"format"`
}) (*mcp.CallToolResult, interface{}, error) {
	var result string
	switch args.Format {
	case "upper":
		result = fmt.Sprintf("格式化(大写): %s", args.Text)
	case "lower":
		result = fmt.Sprintf("格式化(小写): %s", args.Text)
	default:
		result = fmt.Sprintf("格式化(默认): %s", args.Text)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, result, nil
}

func (t *TextNodeProcessor) TextAnalyzeTool(ctx context.Context, req *mcp.CallToolRequest, args struct {
	Text string `json:"text"`
}) (*mcp.CallToolResult, interface{}, error) {
	analysis := map[string]interface{}{
		"length":      len(args.Text),
		"word_count":  len(strings.Split(args.Text, " ")),
		"analyzed_at": time.Now().Unix(),
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("文本分析结果: %+v", analysis)},
		},
	}, analysis, nil
}

func (t *TextNodeProcessor) TextConvertTool(ctx context.Context, req *mcp.CallToolRequest, args struct {
	Text   string `json:"text"`
	Target string `json:"target"`
}) (*mcp.CallToolResult, interface{}, error) {
	result := fmt.Sprintf("转换为%s格式: %s", args.Target, args.Text)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, result, nil
}
