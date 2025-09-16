package task_nodes

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"time"
)

type CNodeProcessor struct {
}

func (c *CNodeProcessor) GetNodeName() string {
	return "C(日志节点)"
}

func (c *CNodeProcessor) ProcessNode(nodeCtx *NodeContext) error {
	// 打印当前节点数据

	// 注册工具
	err, toolsNum := c.registerTools(nodeCtx.McpServer)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("%s 节点注册了 %d 个工具", c.GetNodeName(), toolsNum))
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
		"data":         c.GetNodeName(),
		"tools_num":    newToolsNum,
	}
	if serverToolsNum, ok := nodeCtx.Data["tools_num"]; ok {
		nodeCtx.Data["tools_num"] = serverToolsNum.(int) + 0
	} else {
		nodeCtx.Data["tools_num"] = 0
	}
	return nil
}

func (c *CNodeProcessor) registerTools(mcpServer *mcp.Server) (error, int) {
	mcp.AddTool[CNodeLogToolArgs](mcpServer, &mcp.Tool{
		Name:        "logs",
		Title:       "日志",
		Description: "输出日志内容",
	}, c.LogTool)
	return nil, 1
}

//===============================================================================
// 工具
//===============================================================================

type (
	CNodeLogToolArgs struct {
		Logs []string `json:"logs"`
	}
)

func (c *CNodeProcessor) LogTool(ctx context.Context, req *mcp.CallToolRequest, args CNodeLogToolArgs) (*mcp.CallToolResult, interface{}, error) {
	var result string
	for i, log := range args.Logs {
		result += fmt.Sprintf("第 %d 条日志：%s \n", i+1, log)
	}

	// 构造结构化的返回对象
	structuredResult := D{
		"in":        args.Logs,
		"out":       result,
		"timestamp": time.Now().Unix(),
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: result},
		},
	}, structuredResult, nil
}
