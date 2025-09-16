package task_nodes

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"time"
)

type BNodeProcessor struct {
}

func (b *BNodeProcessor) GetNodeName() string {
	return "B(回声节点)"
}

func (b *BNodeProcessor) ProcessNode(nodeCtx *NodeContext) error {

	// 注册工具
	err, toolsNum := b.registerTools(nodeCtx.McpServer)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("%s 节点注册了 %d 个工具", b.GetNodeName(), toolsNum))
	var newToolsNum int
	if serverToolsNum, ok := nodeCtx.Data["tools_num"]; ok {
		newToolsNum = serverToolsNum.(int) + toolsNum
	} else {
		newToolsNum = toolsNum
	}
	// 更新节点数据
	nodeCtx.Data = D{
		"processed_at": time.Now().Unix(),
		"status":       "processed",
		"data":         b.GetNodeName(),
		"tools_num":    newToolsNum,
	}
	if serverToolsNum, ok := nodeCtx.Data["tools_num"]; ok {
		nodeCtx.Data["tools_num"] = serverToolsNum.(int) + 0
	} else {
		nodeCtx.Data["tools_num"] = 0
	}
	return nil
}

func (b *BNodeProcessor) registerTools(mcpServer *mcp.Server) (error, int) {
	mcp.AddTool[BNodeEchoToolArgs](mcpServer, &mcp.Tool{
		Name:        "echo",
		Title:       "回声",
		Description: "返回输入的文本内容",
	}, b.EchoTool)
	return nil, 1
}

//===============================================================================
// 工具
//===============================================================================

type (
	BNodeEchoToolArgs struct {
		Text string `json:"text"`
	}
)

func (b *BNodeProcessor) EchoTool(ctx context.Context, req *mcp.CallToolRequest, args BNodeEchoToolArgs) (*mcp.CallToolResult, interface{}, error) {

	result := fmt.Sprintf("Echo: %s", args.Text)

	// 构造结构化的返回对象
	structuredResult := D{
		"in":        args.Text,
		"out":       result,
		"timestamp": time.Now().Unix(),
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, structuredResult, nil
}
