package server

import (
	"AgentEarth_AgentPlatform/models"
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"log"
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
	log.Println("开始初始化MCP服务...")

	// 获取所有MCP服务配置
	var serviceList []models.AeMcpServices
	serviceModel := &models.AeMcpServices{}

	if err := serviceModel.GetList(&serviceList); err != nil {
		return fmt.Errorf("获取MCP服务列表失败: %w", err)
	}

	// 清空现有的服务映射表
	McpServicesMap = make(map[string]*Server)

	// 遍历服务列表，为每个启用的服务创建实例
	enabledCount := 0
	for _, service := range serviceList {
		if !service.Enabled {
			log.Printf("跳过未启用的服务: %s (ID: %s)", service.ServerName, service.ServerId)
			continue
		}

		// 创建MCP服务实例
		server := createMcpServerFromConfig(service)
		if server != nil {
			McpServicesMap[service.ServerId] = server
			enabledCount++
			log.Printf("成功加载MCP服务: %s (ID: %s)", service.ServerName, service.ServerId)
		} else {
			log.Printf("创建MCP服务失败: %s (ID: %s)", service.ServerName, service.ServerId)
		}
	}

	log.Printf("MCP服务初始化完成，共加载 %d 个服务", enabledCount)
	return nil
}

// createMcpServerFromConfig 根据配置创建MCP服务实例
func createMcpServerFromConfig(config models.AeMcpServices) *Server {
	server := &Server{}

	// 根据配置创建MCP服务器实例
	implementation := &mcp.Implementation{
		Name:    config.ServerId,
		Title:   config.ServerName,
		Version: config.ProtocolVersion,
	}

	server.mcpServer = mcp.NewServer(implementation, nil)

	// 这里可以根据config.Tags或其他配置来添加不同的工具
	// 目前先添加默认的Echo工具，后续可以根据服务配置动态添加
	mcp.AddTool(server.mcpServer, &mcp.Tool{
		Meta:         nil,
		Annotations:  nil,
		Description:  fmt.Sprintf("%s - %s", config.Description, "输出输入参数"),
		InputSchema:  nil,
		Name:         "echo",
		OutputSchema: nil,
		Title:        "Echo",
	}, Echo)

	return server
}
