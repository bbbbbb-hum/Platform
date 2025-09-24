package server

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/servers"
	"context"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EchoParams struct {
	Text string `json:"text" `
	//Text string `json:"text"`
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
	servers.McpServicesMap = make(map[string]*servers.Server)

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
			servers.McpServicesMap[service.ServerId] = server
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
func createMcpServerFromConfig(config *models.AeMcpServices) *servers.Server {
	server := &servers.Server{}

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

func InitializeMcpServicesV2() error {
	server := &servers.Server{}

	// 根据配置创建MCP服务器实例
	implementation := &mcp.Implementation{
		Name:    "echo",
		Title:   "回声输出",
		Version: "2024-11-05",
	}

	server.mcpServer = mcp.NewServer(implementation, nil)
	// 生成 schema
	//reflector := jsonschema.Reflector{
	//	AllowAdditionalProperties:  false,
	//	RequiredFromJSONSchemaTags: true,
	//}
	//echoInput := createEchoSchema()
	echoInput, err := jsonschema.ForType(reflect.TypeOf(EchoParams{}), &jsonschema.ForOptions{})
	mcp.AddTool(server.mcpServer, &mcp.Tool{
		Meta: mcp.Meta{
			"id": "1",
		},
		Annotations:  nil,
		Description:  "输出输入参数",
		InputSchema:  echoInput,
		Name:         "echo",
		OutputSchema: nil,
		Title:        "Echo",
	}, OnCallTool)
	//logsInput := createLogsSchema()
	logsInput, err := jsonschema.ForType(reflect.TypeOf(LogsParam{}), &jsonschema.ForOptions{})
	if err != nil {
		return err
	}
	mcp.AddTool(server.mcpServer, &mcp.Tool{
		Meta: mcp.Meta{
			"id": "2",
		},
		Annotations:  nil,
		Description:  "日志",
		InputSchema:  logsInput,
		Name:         "logs",
		OutputSchema: nil,
		Title:        "Logs",
	}, OnCallTool)
	servers.McpServicesMap["echo"] = server
	return nil
}

type LogsParam struct {
	Log string `json:"log"`
}

func OnCallTool(ctx context.Context, req *mcp.CallToolRequest, args any) (*mcp.CallToolResult, any, error) {
	logger.Info(fmt.Sprintf("工具名称: %s", req.Params.Name))
	logger.Info(fmt.Sprintf("工具参数: %s", req.Params.Arguments))
	logger.Info(fmt.Sprintf("工具参数类型: %v", req.Params.Meta))
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("您输入的为 %v", args)},
		},
	}, nil, nil
}

//func createMcpServerFromConfig(config *models.AeMcpExternalServicesConfig) *Server {
//	server := &Server{}
//	server.Init()
//	server.mcpServer = mcp.NewServer(config.Name, nil)
//
//	return server
//}

//server.Init()
//{
//	//初始化节点
//	for node in chain.nodes:{
//		node.init()
//
//	}
//	//获取工具列表并注册
//	tools = tools{}
//	for node in chanin.nodes:{
//		tools.append(node.getTools())
//	}
//	server.AddTools(tool1,server.OnCallTool)
//	server.AddTools(tool2,server.OnCallTool)
//	server.AddTools(tool3,server.OnCallTool)
//	server.AddTools(tool4,server.OnCallTool)
//	server.AddTools(tool5,server.OnCallTool)
//	server.AddTools(tool6,server.OnCallTool)
//}
//
//server.OnCallTool(toolName ="tool_1",toolParams)
//{
//
//	for node in chanin.nodes:{
//		node.Process(types,toolName="tool_1",toolParams)
//	}
//
//}
//
//gaodeNode.Init(){
//	gaodeMcp = initMCP(gaodeConnectionInfo,gaodeConnectionType)
//
//	tools = gaodeMcp.getTools()
//	for tool in tools:{
//		mapToolNameToMcp[tool.name] = tool
//	}
//}
//gaodeNode.GetTools(){
//	return tools
//}
//gaodeNode.Process(types,toolName="tool_1",toolParams)
//{
//	mapToolNameToMcp[toolName].callTool(types,toolName="tool_1",toolParams)
//}

//mcpServer
//|--tools:[]*mcp.Tool
//|  |--name
//|  |--description
//|  |--CallTool(types,req,args)
