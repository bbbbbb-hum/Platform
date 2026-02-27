package types

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// WithPre 返回一个装饰后的 Processor，在调用 Process 前执行统一的前置逻辑
func WithPre(inner Processor) Processor {
	return &processorWithPre{inner: inner}
}

type processorWithPre struct {
	inner Processor
}

func (w *processorWithPre) Init(config InitConfig) error {
	return w.inner.Init(config)
}

func (w *processorWithPre) GetTools(rc *RunningContext) []*ToolDesc {
	return w.inner.GetTools(rc)
}

func (w *processorWithPre) GetNodeInfo() *NodeInfo {
	return w.inner.GetNodeInfo()
}

func (w *processorWithPre) Process(rc *RunningContext, userCmd string, userParamMap map[string]interface{}, lastResp *mcp.CallToolResult) (currentResp *mcp.CallToolResult, err error) {
	if err = w.pre(rc, userCmd, userParamMap, lastResp); err != nil {
		if errors.Is(err, errSkipNode) {
			logger.Debug("跳过该节点...", zap.String("userCmd", userCmd))
			// 跳过执行，不报错，直接透传上一步结果
			return lastResp, nil
		}
		return nil, err
	}
	return w.inner.Process(rc, userCmd, userParamMap, lastResp)
}

// pre 统一的前置处理（默认无操作，可按需扩展）
func (w *processorWithPre) pre(_ *RunningContext, userCmd string, _ map[string]interface{}, _ *mcp.CallToolResult) error {
	// 工具白名单：ToolNames 为空表示不限制；非空时必须包含 userCmd 才允许执行
	nodeInfo := w.GetNodeInfo()
	if nodeInfo == nil {
		return nil
	}
	if len(nodeInfo.ToolNames) == 0 {
		return nil
	}
	for _, toolName := range nodeInfo.ToolNames {
		if toolName == userCmd {
			return nil
		}
	}
	return errSkipNode
}

var errSkipNode = errors.New("skip node by pre")
