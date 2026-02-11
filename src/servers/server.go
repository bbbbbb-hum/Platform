package servers

import (
	"AgentEarth_AgentPlatform/src/helpers"
	"AgentEarth_AgentPlatform/src/helpers/cache"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/task_chain"
	"AgentEarth_AgentPlatform/src/servers/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var McpServicesMap = map[string]*Server{}
var RequestLogs = map[string]*models.AeMcpServicesRequestLogs{}

type Server struct {
	ServerName    string
	mcpServer     *mcp.Server
	ChainInstance *task_chain.ChainInstance
	toolDescList  []*types.ToolDesc
	LimitCalls    int64 // 调用次数限制
	LimitType     int64 // 调用限制类型：1 天; 2 周；3 月；4 季度；5 年；
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
		//if !service.Enabled {
		//	logger.Info("跳过未启用的服务", zap.String("server_name", service.ServerName), zap.String("server_id", service.ServerId))
		//	continue
		//}

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

// InitializeByServiceId 根据服务ID初始化MCP服务
func InitializeByServiceId(id int32) error {
	service := &models.AeMcpServices{}
	err := service.GetOne(id)
	if err != nil {
		return fmt.Errorf("获取MCP服务失败: %w", err)
	}
	if service.Id == 0 {
		return fmt.Errorf("MCP服务不存在")
	}
	// 检查一下Map中是否已经存在该服务
	if _, ok := McpServicesMap[service.ServerId]; ok {
		logger.Info("MCP服务已存在", zap.String("service_id", service.ServerId))
		return nil
	}
	server := createMcpServer(service)
	if server != nil {
		McpServicesMap[service.ServerId] = server
	}
	logger.Info("MCP服务初始化完成", zap.String("service_id", service.ServerId))
	return nil
}

// 创建MCP服务实例
func createMcpServer(service *models.AeMcpServices) *Server {
	server := &Server{
		ServerName: service.ServerName,
	}

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
	var toolModelList = []*models.AeMcpTools{}
	for _, tool := range server.toolDescList {
		// 注册工具，捕获并跳过可能的 panic
		if !safeAddTool(server, tool) {
			logger.Warn("注册工具失败，已跳过", zap.String("tool_name", tool.ToolName))
			continue
		}
		var schemaMap map[string]interface{}
		if tool.ToolInputSchema != nil {
			b, err1 := json.Marshal(tool.ToolInputSchema)
			if err1 != nil {
				logger.Error("ToolInputSchema json.Marshal失败", zap.Error(err1))
				return nil
			}
			if err = json.Unmarshal(b, &schemaMap); err != nil {
				logger.Error("ToolInputSchema json.Unmarshal失败", zap.Error(err1))
				return nil
			}
		}
		// 新增工具
		toolModel := models.AeMcpTools{
			Id:            0,
			ServiceId:     service.Id,
			Name:          tool.ToolName,
			Description:   tool.ToolDesc,
			ArgsSchema:    schemaMap,
			CreateTime:    time.Now(),
			UpdateTime:    time.Now(),
			XlcreditPrice: service.XlcreditPrice,
		}
		// 缓存工具调用价格到Redis
		err = cache.SetToolsPrice(service.ServerId, tool.ToolName, service.XlcreditPrice)
		if err != nil {
			logger.Error("设置工具价格失败", zap.String("server_id", service.ServerId), zap.String("tool_name", tool.ToolName), zap.Error(err))
		}
		toolModelList = append(toolModelList, &toolModel)
	}
	if len(toolModelList) > 0 {
		db := models.GetDB()
		err = db.Transaction(func(tx *gorm.DB) error {
			if err = tx.Where("service_id=?", service.Id).Delete(&models.AeMcpTools{}).Error; err != nil {
				return err
			}
			if err = tx.Create(toolModelList).Error; err != nil {
				return err
			}
			// 上线服务
			err = service.UpdateEnabled(true)
			if err != nil {
				logger.Error("上线服务失败", zap.Error(err))
				return nil
			}
			return nil
		})
		if err != nil {
			logger.Error("更新工具列表失败~", zap.Error(err))
		}
	} else {
		//下线无工具服务
		err = service.UpdateEnabled(false)
		if err != nil {
			logger.Error("下线服务失败", zap.Error(err))
			return nil
		}
	}

	// 获取调用次数上限与tokens
	var serviceLimitModel = &models.AeMcpServicesLimit{}
	err = serviceLimitModel.GetOneByServerId(service.ServerId)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("获取服务调用限制失败", zap.Error(err))
	}
	server.LimitType = serviceLimitModel.LimitType
	server.LimitCalls = serviceLimitModel.LimitCalls
	logger.Info("MCP服务器创建成功", zap.String("service_id", service.ServerId), zap.Int("tools_count", len(server.toolDescList)))

	return server
}

// safeAddTool wraps mcp.AddTool with panic recovery. It returns false if a panic occurred.
func safeAddTool(server *Server, tool *types.ToolDesc) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("AddTool 发生 panic，已跳过该工具",
				zap.String("tool_name", tool.ToolName),
				zap.Any("tool_input_schema", tool.ToolInputSchema),
				zap.Any("recover", r))
			ok = false
		}
	}()
	mcp.AddTool[map[string]interface{}](server.mcpServer, &mcp.Tool{
		Name:        tool.ToolName,
		Description: tool.ToolDesc,
		Title:       fmt.Sprintf("%s Tool", tool.ToolName),
		InputSchema: tool.ToolInputSchema,
	}, server.OnCallTool)
	return true
}

// OnCallTool 新的工具调用处理器
func (s *Server) OnCallTool(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, interface{}, error) {
	// 记录开始时间
	startTime := time.Now()

	toolName := req.Params.Name
	userId, ok := ctx.Value(helpers.ContextKeyUserID).(string)
	if !ok {
		return nil, nil, errors.New("user information not obtained")
	}
	keyId, ok := ctx.Value("key_id").(int64)
	if !ok {
		return nil, nil, errors.New("key_id not obtained")
	}
	// 创建节点上下文
	ctxNode := &types.RunningContext{
		ChainID:   s.ChainInstance.ChainID,
		ServiceID: s.ChainInstance.ServerID,
		Stats: map[string]interface{}{
			"user_id": userId,
			"key_id":  keyId,
		},
	}
	logger.Debug("开始处理", zap.String("tool_name", toolName))

	// 链处理
	result, err := s.ChainInstance.Process(ctxNode, toolName, args)

	// 计算耗时
	duration := time.Since(startTime)

	// 确定isError和消息
	var isError bool
	var message string
	if err != nil {
		// 1. 工具链调用错误
		isError = true
		message = err.Error()
	} else {
		// 2. 工具链调用正确，直接从结构体中获取isError和message
		isError = result.IsError
		// 只有当isError为true时才设置message
		if isError && len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				message = textContent.Text
			}
		}
	}

	// 从context获取apiKeyName和userID
	apiKeyName := ""
	if val, ok := ctx.Value(helpers.ContextKeyApiKeyName).(string); ok {
		apiKeyName = val
	}

	// 直接输出日志
	logger.Logger.Info("MCP服务日志",
		zap.String("log_type", "AgentGWCall"),                                     // 0. 必须存在的字段 -- "AgentGWCall"
		zap.String("user_id", userId),                                             // 1. 用户ID
		zap.String("apikey_name", apiKeyName),                                     // 2. apikey的名称
		zap.String("service_name", s.ServerName),                                  // 3. 访问的服务名称
		zap.String("method", toolName),                                            // 4. 访问的服务中的工具名称
		zap.Any("param", args),                                                    // 5. 访问服务需要的参数列表
		zap.Bool("is_error", isError),                                             // 6. 是否为错误
		zap.String("msg", message),                                                // 7. 当MCP失败时，错误信息
		zap.Any("response_data", result),                                          // 8. 返回给用户的内容
		zap.String("duration_ms", strconv.FormatInt(duration.Milliseconds(), 10)), // 9. 调用时间 -- ms
	)

	if err != nil {
		return nil, nil, err
	}

	return result, result.StructuredContent, nil
}
