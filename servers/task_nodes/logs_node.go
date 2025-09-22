package task_nodes

import (
	"fmt"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"time"
)

// LoggerNode 日志记录节点 - 只记录，不提供工具
type LoggerNode struct {
	BaseNode
	logFile string
}

func (l *LoggerNode) Init(ctx *NodeContext) error {
	logger.Info("初始化日志节点", zap.String("node_id", fmt.Sprint(l.Info.NodeID)))
	// 初始化日志文件等
	return nil
}

func (l *LoggerNode) Process(ctx *NodeContext) error {
	if !ctx.IsInitPhase {
		// 记录工具调用
		logger.Info("工具调用记录",
			zap.String("tool_name", ctx.ToolName),
			zap.Any("args", ctx.ToolArgs))

		// 更新统计信息
		if ctx.Stats == nil {
			ctx.Stats = make(map[string]interface{})
		}

		callCount, _ := ctx.Stats["call_count"].(int)
		ctx.Stats["call_count"] = callCount + 1
		ctx.Stats["last_call_time"] = time.Now().Unix()
	}
	return nil
}
