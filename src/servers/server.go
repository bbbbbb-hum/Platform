package servers

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/task_chain"
	"AgentEarth_AgentPlatform/src/servers/types"
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

var McpServicesMap = map[string]*Server{}

type Server struct {
	mcpServer     *mcp.Server
	ChainInstance *task_chain.ChainInstance
	toolDescList  []*types.ToolDesc
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
	chainModel := &models.AeMcpTaskChain{}
	err := chainModel.GetOne(service.TaskChainId)
	if err != nil {
		logger.Error("获取任务链失败", zap.Error(err))
		return nil
	}

	// 创建ChainInstance
	chainInstance := &task_chain.ChainInstance{}
	// 初始化链（这里会初始化所有节点实例）
	err = chainInstance.Init(types.InitConfig{
		ServiceID:  service.ServerId,
		ChainModel: chainModel,
	})
	if err != nil {
		logger.Error("初始化任务链失败", zap.Error(err))
		return nil
	}
	// 将链挂到server下中
	server.ChainInstance = chainInstance
	// 获取工具列表并注册
	server.toolDescList = server.ChainInstance.GetTools(&types.RunningContext{})

	//
	//根据server.toolDescList 注册mcp工具
	//
	for _, tool := range server.toolDescList {
		// 使用新的工具调用处理器
		mcp.AddTool[map[string]interface{}](server.mcpServer, &mcp.Tool{
			Name:        tool.ToolName,
			Description: tool.ToolDesc,
			Title:       fmt.Sprintf("%s Tool", tool.ToolName),
			InputSchema: tool.ToolInputSchema,
		}, server.OnCallTool)
	}
	logger.Info("MCP服务器创建成功", zap.String("service_id", service.ServerId), zap.Int("tools_count", len(server.toolDescList)))

	return server
}

// OnCallTool 新的工具调用处理器
func (s *Server) OnCallTool(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, interface{}, error) {
	toolName := req.Params.Name
	// 创建节点上下文
	ctxNode := &types.RunningContext{
		ChainID:   s.ChainInstance.ChainID,
		ServiceID: s.ChainInstance.ServerID,
		Stats:     make(map[string]interface{}),
	}
	logger.Debug("开始处理", zap.String("tool_name", toolName))
	// 链处理
	result, err := s.ChainInstance.Process(ctxNode, toolName, args)
	if err != nil {
		return nil, nil, err
	}
	return result, result.StructuredContent, nil
}
