package task_nodes

import (
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"time"
)

type ANodeProcessor struct {
}

func (a *ANodeProcessor) GetNodeName() string {
	return "A(空节点)"
}

func (a *ANodeProcessor) ProcessNode(nodeCtx *NodeContext) error {
	// 无逻辑

	// 无工具
	err, toolsNum := a.registerTools(nodeCtx.McpServer)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("%s 节点注册了 %d 个工具", a.GetNodeName(), toolsNum))
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
		"data":         a.GetNodeName(),
		"tools_num":    newToolsNum,
	}
	if serverToolsNum, ok := nodeCtx.Data["tools_num"]; ok {
		nodeCtx.Data["tools_num"] = serverToolsNum.(int) + 0
	} else {
		nodeCtx.Data["tools_num"] = 0
	}
	return nil
}

func (a *ANodeProcessor) registerTools(mcpServer *mcp.Server) (error, int) {
	return nil, 0
}
