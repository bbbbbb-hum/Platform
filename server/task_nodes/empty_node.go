package task_nodes

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"time"
)

type EmptyNodeProcessor struct {
}

func (e *EmptyNodeProcessor) GetNodeName() string {
	return "空节点"
}

func (e *EmptyNodeProcessor) ProcessNode(nodeCtx *NodeContext) error {
	logger.Debug("上个节点数据", zap.Any("data", nodeCtx.Data))
	// 无逻辑

	// 无工具

	// 更新节点数据
	nodeCtx.Data = map[string]interface{}{
		"processed_at": time.Now().Unix(),
		"status":       "processed",
		"data":         e.GetNodeName(),
	}
	return nil
}

func (e *EmptyNodeProcessor) registerTools(mcpServer *mcp.Server) error {
	return nil
}
