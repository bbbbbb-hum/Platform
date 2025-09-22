package servers

import (
	"AgentEarth_AgentPlatform/models"
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

var McpServicesMap = map[string]*Server{}

type Server struct {
	mcpServer *mcp.Server
	taskChain *TaskChain
}

func (s *Server) GetServer() *mcp.Server {
	return s.mcpServer
}

func Initialize() error {
	logger.Info("开始初始化MCP服务...")
	// 获取所有MCP服务配置
	serviceModel := &models.AeMcpServices{}
	err, serviceList := serviceModel.GetList()
	if err != nil {
		return fmt.Errorf("获取MCP服务列表失败: %w", err)
	}

	logger.Info("从数据库获取到MCP服务记录", zap.Int("count", len(serviceList)))

	// 清空现有的服务映射表
	McpServicesMap = make(map[string]*Server)
	// 遍历服务列表，为每个启用的服务创建实例
	enabledCount := 0
	for _, service := range serviceList {
		if !service.Enabled {
			logger.Info("跳过未启用的服务", zap.String("server_name", service.ServerName), zap.String("server_id", service.ServerId))
			continue
		}

		// 创建MCP服务实例
		server := createMcpServer(service)
		if server != nil {
			McpServicesMap[service.ServerId] = server
			enabledCount++
			logger.Info("成功加载MCP服务", zap.String("server_name", service.ServerName), zap.String("server_id", service.ServerId))
		} else {
			logger.Error("创建MCP服务失败", zap.String("server_name", service.ServerName), zap.String("server_id", service.ServerId))
		}
	}
	logger.Info("MCP服务初始化完成", zap.Int("enabled_count", enabledCount))
	return nil
}

func createMcpServer(service *models.AeMcpServices) *Server {
	server := &Server{}

	// 根据配置创建MCP服务器实例
	implementation := &mcp.Implementation{
		Name:    service.ServerId,
		Title:   service.ServerName,
		Version: service.ProtocolVersion,
	}
	// 创建MCPServer
	server.mcpServer = mcp.NewServer(implementation, nil)

	// 初始化任务链
	taskChain, err := InitializeTaskChain(service.TaskChainId, service.ServerId)
	if err != nil {
		logger.Error("初始化任务链失败", zap.Error(err))
		return nil
	}
	server.taskChain = taskChain

	// 获取工具列表并注册
	tools := taskChain.GetTools()
	for _, tool := range tools {
		// 使用新的工具调用处理器
		mcp.AddTool[map[string]interface{}](server.mcpServer, tool, server.OnCallTool)
	}

	logger.Info("MCP服务器创建成功",
		zap.String("service_id", service.ServerId),
		zap.Int("tools_count", len(tools)))

	return server
}

// OnCallTool 新的工具调用处理器
func (s *Server) OnCallTool(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, interface{}, error) {
	toolName := req.Params.Name

	logger.Info("处理工具调用",
		zap.String("tool_name", toolName),
		zap.Any("args", args))

	// 获取工具元数据
	toolMeta := s.taskChain.GetToolMeta(toolName)

	// 通过任务链处理工具调用
	return s.taskChain.ProcessToolCall(toolName, args, toolMeta)
}
