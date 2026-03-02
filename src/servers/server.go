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
	// 语义：本次启动/重载以数据库当前配置为准，旧缓存全部丢弃重建。
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
			// key 使用 server_id，对应网关访问路径 /mcp-server/{server_id}
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
	// 注意：工具列表是链上各节点聚合后的结果（尤其含 proxy 节点透出的外部工具）。
	server.toolDescList = server.ChainInstance.GetTools(&types.RunningContext{})

	//
	//根据server.toolDescList 注册mcp工具
	//
	var toolModelList = []*models.AeMcpTools{}
	for _, tool := range server.toolDescList {
		// 在工具描述中添加价格信息
		tool.ToolDesc = fmt.Sprintf("%s (Price: %.8f XLCredit)", tool.ToolDesc, service.XlcreditPrice)

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
		// 这里写库是为了给管理侧展示“当前可用工具快照”。
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
		// 工具元数据使用“先删后插”方式保持和当前链配置一致。
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
		// 无可用工具时直接将服务标记为下线，避免网关暴露空服务。
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
	// 网关鉴权中间件会在 ctx 注入 user_id / key_id，这里读取用于审计日志。
	userId, ok := ctx.Value(helpers.ContextKeyUserID).(string)
	if !ok {
		return nil, nil, errors.New("user information not obtained")
	}
	keyId, ok := ctx.Value("key_id").(int64)
	if !ok {
		return nil, nil, errors.New("key_id not obtained")
	}
	// 创建节点上下文
	// 运行上下文会贯穿整条任务链，节点可通过 Stats 读取调用方信息。
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
	// 链路执行顺序由 task_chain.NodeInstances 决定。
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
	// 这是网关侧核心审计日志，字段顺序与下游日志消费约定保持一致。
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

// server.go 第 327-352 行
// ReloadAggregateNodes 热更新指定服务的聚合节点配置

// ===== 第1步：根据 serverID 找到服务实例 =====
func ReloadAggregateNodes(serverID string) error {
	// McpServicesMap 是一个 map，key 是 serverID，value 是服务实例
	// 这里通过 serverID 从 map 中找到对应的服务
	server, ok := McpServicesMap[serverID]
	
	// 如果找不到服务，或者服务是 nil，说明 serverID 无效
	if !ok || server == nil {
		// 返回错误，提示找不到服务
		return fmt.Errorf("server_not_found: %s", serverID)
	}

	// ===== 第2步：遍历链上的所有节点 =====
	// server.ChainInstance.NodeInstances 是这个服务链上所有节点的列表
	// 遍历每个节点，找到聚合节点
	for _, nodeInstance := range server.ChainInstance.NodeInstances {
		
		// ===== 第3步：判断是不是聚合节点 =====
		// nodeInstance.NodeInfo.NodeHandle 存储了节点的类型
		// 如果是 "aggregate_handle"，说明是聚合节点
		if nodeInstance.NodeInfo.NodeHandle == "aggregate_handle" {
			
			// ===== 第4步：从数据库重新读取配置 =====
			// 因为运营可能修改了数据库中的 node_config，需要重新读取
			nodeModel := &models.AeMcpTaskNode{}
			
			// 通过节点ID查询数据库，获取最新的节点信息
			// nodeInstance.NodeInfo.NodeID 是这个聚合节点在数据库中的 ID
			err, nodes := nodeModel.GetChianNodes([]int32{nodeInstance.NodeInfo.NodeID})
			
			// 如果查询失败，或者没找到节点，返回错误
			if err != nil || len(nodes) == 0 {
				return fmt.Errorf("get_node_config_failed: %w", err)
			}
			
			// ===== 第5步：调用 RefreshConfig 刷新配置 =====
			// nodes[0] 是查询到的节点对象
			// nodes[0].NodeConfig 是最新的配置内容，比如 '["天气节点", "搜索节点"]'
			
			// 这里使用类型断言，判断 nodeInstance.Node 是否实现了 RefreshConfig 方法
			// 如果实现了，说明它是聚合节点，可以刷新配置
			if agg, ok := nodeInstance.Node.(interface{ RefreshConfig(string) error }); ok {
				
				// 调用聚合节点的 RefreshConfig 方法，传入新配置
				// 这会重新解析配置、重新初始化子节点、更新工具列表
				if err := agg.RefreshConfig(nodes[0].NodeConfig); err != nil {
					// 如果刷新失败，返回错误
					return fmt.Errorf("refresh_config_failed: %w", err)
				}
			}
		}
	}

	// ===== 第6步：记录日志 =====
	// 热更新完成后，记录日志
	logger.Info("聚合节点热更新完成", zap.String("server_id", serverID))
	
	// 返回 nil，表示成功
	return nil
}//总结，就算运营改了数据库中的 node_config，加了或者删除了一个节点，也会在调用时重新读取并刷新配置。

// ReloadAggregateNodeByName 根据节点名称热更新聚合节点配置
func ReloadAggregateNodeByName(nodeName string) error {
	// 1. 遍历所有服务
	for _, server := range McpServicesMap {
		// 2. 遍历链上的节点
		for _, nodeInstance := range server.ChainInstance.NodeInstances {
			// 3. 找到聚合节点，且节点名称匹配
			if nodeInstance.NodeInfo.NodeHandle == "aggregate_handle" &&
				nodeInstance.NodeInfo.NodeName == nodeName {
				
				// 4. 从数据库重新读取配置
				nodeModel := &models.AeMcpTaskNode{}
				err, nodes := nodeModel.GetChianNodes([]int32{nodeInstance.NodeInfo.NodeID})
				if err != nil || len(nodes) == 0 {
					return fmt.Errorf("get_node_config_failed: %w", err)
				}
				
				newConfig := nodes[0].NodeConfig
				
				// 5. 检查配置是否变化
				if aggWithConfig, ok := nodeInstance.Node.(interface{ GetCurrentConfig() string }); ok {
					oldConfig := aggWithConfig.GetCurrentConfig()
					if newConfig == oldConfig {
						logger.Info("配置未变化，跳过刷新",
							zap.String("node_name", nodeName),
							zap.String("config", newConfig))
						return nil
					}
					logger.Info("检测到配置变化，开始刷新",
						zap.String("node_name", nodeName),
						zap.String("old_config", oldConfig),
						zap.String("new_config", newConfig))
				}
				
				// 6. 调用 RefreshConfig 刷新配置
				if agg, ok := nodeInstance.Node.(interface{ RefreshConfig(string) error }); ok {
					if err := agg.RefreshConfig(newConfig); err != nil {
						return fmt.Errorf("refresh_config_failed: %w", err)
					}
				}
				
				logger.Info("聚合节点热更新完成", zap.String("node_name", nodeName))
				return nil
			}
		}
	}
	
	return fmt.Errorf("node_not_found: %s", nodeName)
}
