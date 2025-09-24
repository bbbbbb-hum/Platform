package task_nodes

import (
	"AgentEarth_AgentPlatform/servers/pools"
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"time"
)

type GaoDeNodeProcessor struct {
}

func (g *GaoDeNodeProcessor) GetNodeName() string {
	return "高德地图节点"
}

func (g *GaoDeNodeProcessor) ProcessNode(nodeCtx *NodeContext) error {
	// 打印当前节点数据

	// 注册工具
	err, toolsNum := g.registerTools(nodeCtx.McpServer)
	if err != nil {
		return err
	}
	logger.Debug(fmt.Sprintf("%s 节点注册了 %d 个工具", g.GetNodeName(), toolsNum))
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
		"data":         g.GetNodeName(),
		"tools_num":    newToolsNum,
	}
	if serverToolsNum, ok := nodeCtx.Data["tools_num"]; ok {
		nodeCtx.Data["tools_num"] = serverToolsNum.(int) + 0
	} else {
		nodeCtx.Data["tools_num"] = 0
	}
	return nil
}

func (g *GaoDeNodeProcessor) registerTools(mcpServer *mcp.Server) (error, int) {
	serviceID := "gaode_map" //晚点改成动态的
	// 获取远程连接池
	pool := pools.GetSSEPool()
	// 初始化高德地图服务
	err := pool.InitializeSSEPool(1)
	if err != nil {
		return err, 0
	}
	totalTools := 0
	tools := pool.GetServiceTools(serviceID)
	// 为每个远程服务的工具注册代理
	for _, tool := range tools {
		if err = g.registerProxyTool(mcpServer, serviceID, tool); err != nil {
			logger.Error("注册远程工具失败",
				zap.String("service_id", serviceID),
				zap.String("tool_name", tool.Name),
				zap.Error(err))
			continue
		}
		totalTools++
	}

	return nil, totalTools
}

// registerProxyTool 注册代理工具
func (g *GaoDeNodeProcessor) registerProxyTool(mcpServer *mcp.Server, serviceID string, remoteTool *mcp.Tool) error {
	// 创建代理工具，工具名加上服务前缀避免冲突
	proxyToolName := fmt.Sprintf("%s_%s", serviceID, remoteTool.Name)

	proxyTool := &mcp.Tool{
		Name:        proxyToolName,
		Title:       fmt.Sprintf("[%s] %s", serviceID, remoteTool.Title),
		Description: fmt.Sprintf("远程工具: %s\n%s", remoteTool.Title, remoteTool.Description),
		InputSchema: remoteTool.InputSchema, // 保持原有的参数结构
	}

	// 注册代理工具
	mcp.AddTool[D](mcpServer, proxyTool, g.createProxyHandler(serviceID, remoteTool.Name))

	logger.Debug("注册远程代理工具",
		zap.String("proxy_name", proxyToolName),
		zap.String("service_id", serviceID),
		zap.String("original_name", remoteTool.Name))

	return nil
}

// createProxyHandler 创建代理处理器
func (g *GaoDeNodeProcessor) createProxyHandler(serviceID, originalToolName string) func(context.Context, *mcp.CallToolRequest, D) (*mcp.CallToolResult, interface{}, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, args D) (*mcp.CallToolResult, interface{}, error) {
		logger.Info("代理远程工具调用",
			zap.String("service_id", serviceID),
			zap.String("tool_name", originalToolName),
			zap.Any("args", args))

		// 从连接池获取客户端并调用远程工具
		pool := pools.GetSSEPool()
		result, structuredResult, err := pool.CallTool(serviceID, originalToolName, args)

		if err != nil {
			logger.Error("远程工具调用失败",
				zap.String("service_id", serviceID),
				zap.String("tool_name", originalToolName),
				zap.Error(err))

			// 返回错误信息
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("远程工具调用失败: %v", err)},
				},
			}, D{"error": err.Error(), "service_id": serviceID, "tool_name": originalToolName}, nil
		}

		logger.Info("远程工具调用成功",
			zap.String("service_id", serviceID),
			zap.String("tool_name", originalToolName))

		// 在返回内容中添加来源信息
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				textContent.Text = fmt.Sprintf("[来自 %s] %s", serviceID, textContent.Text)
			}
		}

		return result, structuredResult, nil
	}
}
