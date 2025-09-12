package server

import (
	"AgentEarth_AgentPlatform/models"
	"context"
	"fmt"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var McpServicesMap = map[string]*Server{}

type Server struct {
	mcpServer *mcp.Server
}

func (s *Server) GetServer() *mcp.Server {
	return s.mcpServer
}

type EchoParams struct {
	Text string `json:"text"`
}

func Echo(ctx context.Context, req *mcp.CallToolRequest, args EchoParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "您输入的为 " + args.Text},
		},
	}, nil, nil
}

// InitializeMcpServices 初始化MCP服务映射表
// 从数据库加载所有启用的MCP服务并创建对应的服务实例
func InitializeMcpServices() error {
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
		server := createMcpServerFromConfig(service)
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

// createMcpServerFromConfig 根据配置创建MCP服务实例
func createMcpServerFromConfig(config *models.AeMcpServices) *Server {
	server := &Server{}

	// 根据配置创建MCP服务器实例
	implementation := &mcp.Implementation{
		Name:    config.ServerId,
		Title:   config.ServerName,
		Version: config.ProtocolVersion,
	}

	server.mcpServer = mcp.NewServer(implementation, nil)

	// 处理任务链
	if config.TaskChainId > 0 {
		if err := ProcessTaskChain(config.TaskChainId, server.mcpServer); err != nil {
			logger.Error("处理任务链失败", zap.Int("chain_id", int(config.TaskChainId)), zap.Error(err))
			// 不返回错误，继续创建基本服务
		} else {
			logger.Info("成功处理任务链", zap.Int("chain_id", int(config.TaskChainId)))
		}
	}
	// 这里可以根据config.Tags或其他配置来添加不同的工具
	// 目前先添加默认的Echo工具，后续可以根据服务配置动态添加
	//mcp.AddTool(server.mcpServer, &mcp.Tool{
	//	Meta:         nil,
	//	Annotations:  nil,
	//	Description:  fmt.Sprintf("%s - %s", config.Description, "输出输入参数"),
	//	InputSchema:  nil,
	//	Name:         "echo",
	//	OutputSchema: nil,
	//	Title:        "Echo",
	//}, Echo)

	return server
}
