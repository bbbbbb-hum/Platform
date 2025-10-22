package pools

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models/config"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

type (
	ConnectionPool struct {
		services map[string]*ExternalService // key是服务ID
		mutex    sync.RWMutex
		// 维护协程控制
		maintainStop chan struct{}
	}
	// 外部服务定义
	ExternalService struct {
		Id                int32                       // 外部MCP服务自增ID
		ExternalServiceId string                      // 外部MCP服务UUID
		Type              string                      // 外部服务类型
		ServiceName       string                      // 外部MCP服务名称
		MaxInstance       int32                       // 最大实例数
		Tools             []*mcp.Tool                 // 外部工具列表
		LaunchInfo        *LaunchInfo                 // 启动信息
		ConnectInfo       *ConnectInfo                // 连接配置信息
		Accounts          []*ExternalAccount          // 外部账户信息
		InstanceMap       map[string]*ServiceInstance // 运行实例列表
	}
	LaunchInfo struct {
		Command         string                 `json:"command"`          // 启动命令
		Args            []string               `json:"args"`             // 启动参数
		Workdir         string                 `json:"workdir"`          // 工作目录
		Env             map[string]interface{} `json:"env"`              // 环境变量
		MaxRestarts     int                    `json:"max_restarts"`     // 最大重启数
		LaunchTimeout   int                    `json:"launch_timeout"`   // 启动超时
		ShutdownTimeout int                    `json:"shutdown_timeout"` // 关闭超时
		IdleTtl         int                    `json:"idle_ttl"`         // 空闲超时
	}

	ConnectInfo struct {
		Url            string            `json:"url,omitempty"`             // sse服务地址
		Headers        map[string]string `json:"headers,omitempty"`         // 请求头
		ConnectTimeout int               `json:"connect_timeout,omitempty"` //连接超时时间（毫秒）
		MaxConnect     int               `json:"max_connect,omitempty"`     //实例最大连接数
		MaxRetry       int               `json:"max_retry,omitempty"`       //最大重试次数
		Interval       int               `json:"interval,omitempty"`        //重试间隔
		ClientVersion  string            `json:"client_version,omitempty"`  // 客户端版本（可选）
	}
	// ExternalAccount 外部账户
	ExternalAccount struct {
		AccountID int32             // 账户ID
		AuthInfo  map[string]string // 认证信息
	}
	// 外部服务运行实例
	ServiceInstance struct {
		InstanceId      string                // 实例ID
		AccountId       int32                 // 账户ID
		Connections     []*ExternalConnection // 连接列表
		RoundRobinIndex int                   // 轮询索引
		mutex           sync.Mutex            // 保护轮询索引

		// 实例级配置快照（用于快速重建连接）
		ResolvedConnectInfo *ConnectInfo // sse/httpStreamable 使用
		ResolvedLaunchInfo  *LaunchInfo  // stdio 使用
		TargetConnections   int          // 目标连接数（sse=MaxConnect，httpStreamable/stdio=1）
	}
	// ExternalConnection 单个连接信息
	ExternalConnection struct {
		ConnectionID string
		Session      *mcp.ClientSession
		LastPing     time.Time
		ActiveUsers  int // 当前活跃用户数（可选，用于后期优化）
	}
	// headerTransport 自定义传输层，用于添加请求头
	headerTransport struct {
		Transport http.RoundTripper
		Headers   map[string]string
	}
)

// RoundTrip 实现 http.RoundTripper 接口
func (ht *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 添加自定义请求头
	for key, value := range ht.Headers {
		req.Header.Set(key, value)
	}

	// 使用底层传输层发送请求
	return ht.Transport.RoundTrip(req)
}

var (
	GlobalConnectionPool *ConnectionPool
	oncePool             sync.Once
)

// GetConnectPool 获取连接池
func GetConnectPool() *ConnectionPool {
	oncePool.Do(func() {
		GlobalConnectionPool = &ConnectionPool{
			services: make(map[string]*ExternalService),
		}
	})
	return GlobalConnectionPool
}

// 初始化指定服务
func (c *ConnectionPool) InitializeService(externalServiceId string) error {
	logger.Info("初始化外部服务...", zap.String("external_service_id", externalServiceId))
	ctx := context.Background()

	// 加载服务配置
	service, err := c.loadServiceConfigs(externalServiceId)
	if err != nil {
		return err
	}

	// 加载账号配置
	//service, err = c.loadAccountConfigs(service)
	//if err != nil {
	//	return err
	//}

	// 创建实例信息
	switch service.Type {
	case "sse":
		service, err = c.createInstancesForSSE(ctx, service)
	case "stdio":
		service, err = c.createInstancesForStdio(ctx, service)
	case "httpStreamable":
		service, err = c.createInstancesForHttpStreamable(ctx, service)
	default:
		err = fmt.Errorf("该类型暂不支持！")
	}
	if err != nil {
		return err
	}

	// 获取工具列表（使用第一个实例的第一个连接）
	if len(service.InstanceMap) > 0 {
		var firstInstance *ServiceInstance
		for _, instance := range service.InstanceMap {
			firstInstance = instance
			break
		}
		if firstInstance == nil {
			return fmt.Errorf("没有可用账号连接")
		}
		if len(firstInstance.Connections) > 0 {
			tools, err1 := c.fetchTools(firstInstance.Connections[0].Session)
			if err1 != nil {
				logger.Warn("获取工具列表失败", zap.Error(err1))
			} else {
				service.Tools = tools
			}

		}
	}
	// 添加到连接池
	c.mutex.Lock()
	c.services[service.ExternalServiceId] = service
	c.mutex.Unlock()

	logger.Info("初始化外部服务完成...",
		zap.String("external_service_id", service.ExternalServiceId),
		zap.Int("账号数量", len(service.Accounts)),
		zap.Int("工具数量", len(service.Tools)))
	return nil
}

// 加载服务配置
func (c *ConnectionPool) loadServiceConfigs(externalServiceId string) (service *ExternalService, err error) {
	logger.Info("加载外部服务配置...", zap.String("external_service_id", externalServiceId))
	externalServiceConfigModel := config.AeMcpExternalServicesConfig{}
	err = externalServiceConfigModel.GetOneByExternalServiceId(externalServiceId)
	if err != nil {
		logger.Error("获取外部服务配置失败", zap.String("external_service_id", externalServiceId), zap.Error(err))
	}
	// 解析JSONB启动配置信息
	var launchInfo LaunchInfo
	launchInfoByte, err := json.Marshal(externalServiceConfigModel.LaunchInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	err = json.Unmarshal(launchInfoByte, &launchInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	// 解析JSONB连接配置信息
	var connectInfo ConnectInfo
	connectInfoByte, err := json.Marshal(externalServiceConfigModel.ConnectInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	err = json.Unmarshal(connectInfoByte, &connectInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	service = &ExternalService{
		Id:                externalServiceConfigModel.Id,
		ExternalServiceId: externalServiceConfigModel.ExternalServiceId,
		Type:              externalServiceConfigModel.Type,
		ServiceName:       externalServiceConfigModel.Name,
		MaxInstance:       externalServiceConfigModel.MaxInstance,
		Tools:             nil,
		LaunchInfo:        &launchInfo,
		ConnectInfo:       &connectInfo,
		Accounts:          nil,
		InstanceMap:       make(map[string]*ServiceInstance),
	}
	return
}

// 加载账号信息
func (c *ConnectionPool) loadAccountConfigs(service *ExternalService) (*ExternalService, error) {
	// 从数据库加载账号配置
	if service.ConnectInfo.Headers != nil {
		var accountModel = config.AeMcpExternalServicesAccount{}
		accounts, err := accountModel.GetListByConfigId(service.Id)
		if err != nil {
			return service, fmt.Errorf("加载账号配置失败: %v", err)
		}
		if len(accounts) == 0 {
			return service, fmt.Errorf("没有可用账号配置")
		}

		// 初始化 Accounts 切片
		service.Accounts = make([]*ExternalAccount, len(accounts))

		for i, account := range accounts {
			var authInfo map[string]string
			err = account.AuthInfo.Scan(&authInfo)
			if err != nil {
				return service, fmt.Errorf("解析 id=%d 账号配置失败: %v", account.Id, err)
			}
			var externalAccount = &ExternalAccount{
				AccountID: account.ConfigId,
				AuthInfo:  authInfo,
			}
			service.Accounts[i] = externalAccount
		}
	}
	return service, nil
}

// 按SSE配置创建实例信息
func (c *ConnectionPool) createInstancesForSSE(ctx context.Context, service *ExternalService) (newService *ExternalService, err error) {
	newService = service
	if service.ConnectInfo.Headers != nil {
		//需要鉴权
		// 准备认证头
		for i, account := range service.Accounts {
			if account.AuthInfo != nil && len(newService.InstanceMap) < int(service.MaxInstance) {
				// 创建连接配置副本，避免修改原始配置
				connectInfoCopy := *service.ConnectInfo
				connectInfoCopy.Headers = make(map[string]string)

				// 合并原始headers和认证信息
				for k, v := range service.ConnectInfo.Headers {
					connectInfoCopy.Headers[k] = v
				}
				for k, v := range account.AuthInfo {
					connectInfoCopy.Headers[k] = v // 认证信息覆盖默认headers
				}

				//按当前账号信息创建实例
				instance := &ServiceInstance{
					AccountId:           account.AccountID,
					InstanceId:          fmt.Sprintf("instance_%d_%d_%d", service.Id, account.AccountID, i),
					Connections:         make([]*ExternalConnection, 0),
					ResolvedConnectInfo: &connectInfoCopy,
					TargetConnections:   service.ConnectInfo.MaxConnect,
				}
				//创建连接
				instance.Connections, err = c.createSSEConnections(ctx, &connectInfoCopy, service.Id, account.AccountID, service.ConnectInfo.MaxConnect)
				if err != nil {
					logger.Error("创建实例连接失败", zap.Error(err))
					continue
				}
				newService.InstanceMap[instance.InstanceId] = instance
			}
		}
	} else {
		//无需鉴权
		for i := 0; i < int(service.MaxInstance); i++ {
			instance := &ServiceInstance{
				AccountId:           int32(i),
				InstanceId:          fmt.Sprintf("instance_%d_%d", service.Id, i),
				Connections:         make([]*ExternalConnection, 0),
				ResolvedConnectInfo: service.ConnectInfo,
				TargetConnections:   service.ConnectInfo.MaxConnect,
			}
			//创建连接
			instance.Connections, err = c.createSSEConnections(ctx, service.ConnectInfo, service.Id, int32(i), service.ConnectInfo.MaxConnect)
			if err != nil {
				logger.Error("创建实例连接失败", zap.Error(err))
				continue
			}
			newService.InstanceMap[instance.InstanceId] = instance
		}
	}

	return
}

// 创建SSE连接
func (c *ConnectionPool) createSSEConnections(ctx context.Context, connectInfo *ConnectInfo, sid, aid int32, connectsNum int) (connections []*ExternalConnection, err error) {
	logger.Info("创建SSE连接...")

	// 创建HTTP客户端
	httpClient := &http.Client{
		Transport: &headerTransport{
			Transport: http.DefaultTransport,
			Headers:   connectInfo.Headers,
		},
	}
	// 创建MCP传输
	transport := mcp.NewSSEClientTransport(connectInfo.Url, &mcp.SSEClientTransportOptions{
		HTTPClient: httpClient,
	})

	// 创建MCP客户端
	// 配置客户端选项，启用心跳
	clientOpts := &mcp.ClientOptions{
		KeepAlive: 30 * time.Second, // 每30秒发送一次心跳
		// 心跳超时时间，建议为心跳间隔的2倍
	}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "AgentEarth-Proxy-SSE",
		Version: "v1.0.0",
	}, clientOpts)

	// 创建多个连接
	successCount := 0
	for i := 0; i < connectsNum; i++ {
		session, err1 := client.Connect(ctx, transport, nil)

		if err1 != nil {
			logger.Error("创建连接失败", zap.Error(err1), zap.Int("index", i))
			continue
		}

		connection := &ExternalConnection{
			ConnectionID: fmt.Sprintf("connection_%d_%d_%d", sid, aid, i),
			Session:      session,
			LastPing:     time.Now(),
			ActiveUsers:  0,
		}

		connections = append(connections, connection)
		successCount++
	}

	if successCount == 0 {
		return nil, fmt.Errorf("所有连接都失败了")
	}

	logger.Info("账号连接创建完成",
		zap.Int("account_id", int(aid)),
		zap.Int("成功连接数", successCount),
		zap.Int("期望连接数", connectInfo.MaxConnect))

	return
}

// 按STDIO配置创建实例
// 因为stdio启动一次就会创建一个进程，而一个进程只能保持一个连接
func (c *ConnectionPool) createInstancesForStdio(ctx context.Context, service *ExternalService) (newService *ExternalService, err error) {
	// 按STDIO配置启动服务
	if len(service.Accounts) > 0 {
		// 需要鉴权
		for i, account := range service.Accounts {
			if account.AuthInfo != nil && len(service.InstanceMap) < int(service.MaxInstance) {
				// 创建连接配置副本，避免修改原始配置
				launchInfoCopy := *service.LaunchInfo
				launchInfoCopy.Env = make(map[string]interface{})

				// 添加认证信息
				for k, v := range account.AuthInfo {
					launchInfoCopy.Env[k] = v
				}
				instance := &ServiceInstance{
					AccountId:          account.AccountID,
					InstanceId:         fmt.Sprintf("instance_%d_%d_%d", service.Id, account.AccountID, i), // 实例ID 使用格式 instance_服务ID_账号ID_实例数量索引
					Connections:        []*ExternalConnection{},
					ResolvedLaunchInfo: &launchInfoCopy,
					TargetConnections:  1,
				}
				// 按当前账号信息创建实例
				connection, err1 := c.createStdioConnection(ctx, &launchInfoCopy, service.Id, account.AccountID)
				if err1 != nil {
					logger.Error("创建实例失败", zap.Int("index", i), zap.Error(err1))
					continue
				}
				instance.Connections = append(instance.Connections, connection)
				service.InstanceMap[instance.InstanceId] = instance
			}
		}
	} else {
		// 无需鉴权
		for i := 0; i < int(service.MaxInstance); i++ {
			instance := &ServiceInstance{
				AccountId:          int32(i),
				InstanceId:         fmt.Sprintf("instance_%d_%d", service.Id, i), // 实例ID 使用格式 instance_服务ID_实例数量索引
				Connections:        []*ExternalConnection{},
				ResolvedLaunchInfo: service.LaunchInfo,
				TargetConnections:  1,
			}
			connection, err1 := c.createStdioConnection(ctx, service.LaunchInfo, service.Id, int32(i))
			if err1 != nil {
				logger.Error("创建实例失败", zap.Int("index", i), zap.Error(err1))
				continue
			}
			instance.Connections = append(instance.Connections, connection)
			service.InstanceMap[instance.InstanceId] = instance
		}
	}
	newService = service

	return
}

// 创建stdio连接
func (c *ConnectionPool) createStdioConnection(ctx context.Context, launchInfo *LaunchInfo, sid, aid int32) (connection *ExternalConnection, err error) {
	logger.Debug("创建stdio连接...")
	cmd := exec.Command(launchInfo.Command, launchInfo.Args...)

	if len(launchInfo.Env) > 0 {
		// 首先继承父进程的所有环境变量
		cmd.Env = os.Environ()
		// 然后添加自定义环境变量
		for s, k := range launchInfo.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", s, k))
		}
		logger.Debug("环境变量设置完成",
			zap.Int("total_env_vars", len(cmd.Env)),
			zap.Int("custom_env_vars", len(launchInfo.Env)),
			zap.Strings("custom_vars", func() []string {
				var customs []string
				for s, k := range launchInfo.Env {
					customs = append(customs, fmt.Sprintf("%s=%s", s, k))
				}
				return customs
			}()))
	} else {
		// 即使没有自定义环境变量，也要继承父进程环境变量
		cmd.Env = os.Environ()
	}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "AgentEarth-Proxy-Stdio",
		Version: "v1.0.0",
	}, nil)

	// 创建带超时的context，100秒后自动取消
	connectCtx, cancel := context.WithTimeout(ctx, time.Duration(launchInfo.LaunchTimeout)*time.Millisecond)
	defer cancel()

	logger.Debug("连接前...", zap.Any("command", cmd), zap.Duration("timeout", time.Duration(launchInfo.LaunchTimeout)*time.Millisecond))
	startTime := time.Now()
	session, err1 := client.Connect(connectCtx, &mcp.CommandTransport{Command: cmd}, nil)
	duration := time.Since(startTime)
	logger.Debug("连接后...", zap.Any("session", session), zap.Duration("duration", duration))

	if err1 != nil {
		if connectCtx.Err() == context.DeadlineExceeded {
			logger.Error("连接超时，已中断cmd命令执行",
				zap.Error(err1),
				zap.Int32("aid", aid),
				zap.Duration("timeout", time.Duration(launchInfo.LaunchTimeout)*time.Millisecond),
				zap.Duration("elapsed", duration))
		} else {
			logger.Error("创建连接失败", zap.Error(err1), zap.Int32("aid", aid))
		}
		err = err1
		return
	}

	connection = &ExternalConnection{
		ConnectionID: fmt.Sprintf("connection_%d_%d", sid, aid),
		Session:      session,
		LastPing:     time.Now(),
		ActiveUsers:  0,
	}
	return
}

// 按httpStreamable创建实例
func (c *ConnectionPool) createInstancesForHttpStreamable(ctx context.Context, service *ExternalService) (newService *ExternalService, err error) {
	newService = service
	//if service.ConnectInfo.Headers != nil {
	//	//需要鉴权
	//	// 准备认证头
	//	for i, account := range service.Accounts {
	//		if account.AuthInfo != nil && len(newService.InstanceMap) < int(service.MaxInstance) {
	//			// 创建连接配置副本，避免修改原始配置
	//			connectInfoCopy := *service.ConnectInfo
	//			connectInfoCopy.Headers = make(map[string]string)
	//
	//			// 合并原始headers和认证信息
	//			for k, v := range service.ConnectInfo.Headers {
	//				connectInfoCopy.Headers[k] = v
	//			}
	//			for k, v := range account.AuthInfo {
	//				connectInfoCopy.Headers[k] = v // 认证信息覆盖默认headers
	//			}
	//
	//			//按当前账号信息创建实例
	//			instance := &ServiceInstance{
	//				AccountId:           account.AccountID,
	//				InstanceId:          fmt.Sprintf("instance_%d_%d_%d", service.Id, account.AccountID, i),
	//				Connections:         make([]*ExternalConnection, 0),
	//				ResolvedConnectInfo: &connectInfoCopy,
	//				TargetConnections:   1,
	//			}
	//			//创建连接
	//			connection, err1 := c.createHttpStreamableConnections(ctx, &connectInfoCopy, service.Id, account.AccountID)
	//			if err1 != nil {
	//				logger.Error("创建http实例连接失败", zap.Error(err1))
	//				continue
	//			}
	//			instance.Connections = append(instance.Connections, connection)
	//			newService.InstanceMap[instance.InstanceId] = instance
	//		}
	//	}
	//} else {
	//无需鉴权
	for i := 0; i < int(service.MaxInstance); i++ {
		instance := &ServiceInstance{
			AccountId:           int32(i),
			InstanceId:          fmt.Sprintf("instance_%d_%d", service.Id, i),
			Connections:         make([]*ExternalConnection, 0),
			ResolvedConnectInfo: service.ConnectInfo,
			TargetConnections:   1,
		}
		//创建连接
		connection, err1 := c.createHttpStreamableConnections(ctx, service.ConnectInfo, service.Id, int32(i))
		if err1 != nil {
			logger.Error("创建http实例连接失败", zap.Error(err1))
			continue
		}
		instance.Connections = append(instance.Connections, connection)
		newService.InstanceMap[instance.InstanceId] = instance
	}
	//}
	return
}

// 创建httpStreamable连接
func (c *ConnectionPool) createHttpStreamableConnections(ctx context.Context, connectInfo *ConnectInfo, sid, aid int32) (connection *ExternalConnection, err error) {
	logger.Info("创建HTTP连接...")
	// 创建HTTP客户端
	httpClient := &http.Client{
		Transport: &headerTransport{
			Transport: http.DefaultTransport,
			Headers:   connectInfo.Headers,
		},
	}
	// 创建MCP传输
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "time-client",
		Version: "1.0.0",
	}, nil)
	// Connect to the server.
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   connectInfo.Url,
		HTTPClient: httpClient,
	}, nil)
	if err != nil {
		logger.Error("Failed to connect", zap.Error(err))
	}
	connection = &ExternalConnection{
		ConnectionID: fmt.Sprintf("connection_%d_%d", sid, aid),
		Session:      session,
		LastPing:     time.Now(),
		ActiveUsers:  0,
	}
	return
}

// 获取工具
func (c *ConnectionPool) fetchTools(session *mcp.ClientSession) (list []*mcp.Tool, err error) {
	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		return
	}
	return result.Tools, nil
}

// CallTool 调用工具（支持指定实例或自动选择）
func (c *ConnectionPool) CallTool(serviceID, toolName string, args map[string]interface{}, instanceID ...string) (*mcp.CallToolResult, error) {
	// 获取服务
	service, err := c.getService(serviceID)
	if err != nil {
		return nil, err
	}

	// 选择实例
	var instance *ServiceInstance
	if len(instanceID) > 0 && instanceID[0] != "" {
		// 使用指定账号
		instance = service.InstanceMap[instanceID[0]]
		if instance == nil {
			return nil, fmt.Errorf("账号不存在: %s", instanceID[0])
		}
	} else {
		// 自动选择账号（选择第一个可用的）
		for _, v := range service.InstanceMap {
			if len(v.Connections) > 0 {
				instance = v
				break
			}
		}
		if instance == nil {
			return nil, fmt.Errorf("没有可用的账号")
		}
	}

	// 选择连接
	connection := c.selectConnection(instance)
	if connection == nil {
		return nil, fmt.Errorf("没有可用的连接")
	}

	// 调用工具
	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	result, err := connection.Session.CallTool(context.Background(), params)
	if err != nil {
		return nil, fmt.Errorf("工具调用失败: %v", err)
	}

	// 更新连接状态（这里简化处理，实际应该在请求完成后减少ActiveUsers）
	connection.LastPing = time.Now()
	connection.ActiveUsers++

	return result, nil
}

// getService 获取服务
func (c *ConnectionPool) getService(externalServiceId string) (*ExternalService, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	service, exists := c.services[externalServiceId]
	if !exists {
		return nil, fmt.Errorf("服务不存在: %s", externalServiceId)
	}
	return service, nil
}

// selectConnection 为账号选择一个连接（轮询）
func (c *ConnectionPool) selectConnection(instance *ServiceInstance) *ExternalConnection {
	instance.mutex.Lock()
	defer instance.mutex.Unlock()

	// 获取可用连接
	var availableConnections []*ExternalConnection
	for _, conn := range instance.Connections {
		if conn != nil && conn.Session != nil {
			availableConnections = append(availableConnections, conn)
		}
	}

	if len(availableConnections) == 0 {
		return nil
	}

	// 轮询选择
	selected := availableConnections[instance.RoundRobinIndex%len(availableConnections)]
	instance.RoundRobinIndex = (instance.RoundRobinIndex + 1) % len(availableConnections)

	return selected
}

// GetServiceTools 获取单个服务工具列表
func (c *ConnectionPool) GetServiceTools(externalServiceId string) []*mcp.Tool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	service, exists := c.services[externalServiceId]
	if exists {
		return service.Tools
	}
	return nil
}

// Close 关闭所有连接
func (c *ConnectionPool) Close() {
	// 停止维护协程
	c.StopMaintainer()
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, service := range c.services {
		for instanceID, instance := range service.InstanceMap {
			for _, conn := range instance.Connections {
				if conn.Session != nil {
					if err := conn.Session.Close(); err != nil {
						logger.Error("关闭连接失败",
							zap.String("instanceID", instanceID),
							zap.Error(err))
					}
				}
			}
		}
	}
	c.services = make(map[string]*ExternalService)
}

// StartMaintainer 启动维护协程
func (c *ConnectionPool) StartMaintainer(interval time.Duration) {
	c.mutex.Lock()
	// 防重复启动
	if c.maintainStop != nil {
		c.mutex.Unlock()
		return
	}
	stop := make(chan struct{})
	c.maintainStop = stop
	c.mutex.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				c.maintainOnce()
			}
		}
	}()
}

// StopMaintainer 停止维护协程
func (c *ConnectionPool) StopMaintainer() {
	c.mutex.Lock()
	if c.maintainStop != nil {
		close(c.maintainStop)
		c.maintainStop = nil
	}
	c.mutex.Unlock()
}

// maintainOnce 执行一次维护：检测连接、剔除无效、补齐缺口
func (c *ConnectionPool) maintainOnce() {
	// 快照服务列表，避免长时间持有全局锁
	c.mutex.RLock()
	services := make([]*ExternalService, 0, len(c.services))
	for _, s := range c.services {
		services = append(services, s)
	}
	c.mutex.RUnlock()
	logger.Debug("检查连接状态...")
	for _, svc := range services {
		// 遍历所有实例
		for _, inst := range svc.InstanceMap {
			// 1) 拿连接快照
			inst.mutex.Lock()
			connsSnapshot := make([]*ExternalConnection, len(inst.Connections))
			copy(connsSnapshot, inst.Connections)
			inst.mutex.Unlock()

			// 2) 健康检查（在锁外做 IO）
			healthy := make([]*ExternalConnection, 0, len(connsSnapshot))
			for _, conn := range connsSnapshot {
				if conn == nil || conn.Session == nil {
					continue
				}
				if c.isSessionHealthy(conn.Session) {
					healthy = append(healthy, conn)
				} else {
					// 关闭异常会话
					_ = conn.Session.Close()
					logger.Debug("关闭异常会话", zap.String("InstanceId", inst.InstanceId))
				}
			}

			// 3) 写回健康列表
			inst.mutex.Lock()
			inst.Connections = healthy
			inst.mutex.Unlock()
			// 4) 若不足则补齐（使用实例内目标连接数）
			target := inst.TargetConnections
			// 补齐数量
			var i int
			for {
				inst.mutex.Lock()
				current := len(inst.Connections)
				inst.mutex.Unlock()
				if current >= target {
					break
				}

				// 创建新连接（锁外创建）
				newConn, err := c.createReplacementConnection(svc, inst, current)
				if err != nil {
					logger.Warn("维护补齐连接失败", zap.String("service", svc.ServiceName), zap.Error(err))
					break // 避免紧急重试风暴，留给下次周期
				}

				// 追加（加锁）
				inst.mutex.Lock()
				inst.Connections = append(inst.Connections, newConn)
				i++
				inst.mutex.Unlock()
			}
			logger.Debug("恢复连接", zap.Int("数量", i))
		}
	}
	logger.Debug("检查连接状态完成")
}

// isSessionHealthy 使用轻量操作检测会话健康
func (c *ConnectionPool) isSessionHealthy(session *mcp.ClientSession) bool {
	// 使用短超时的 Ping 作为心跳
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := session.Ping(ctx, nil)
	return err == nil
}

// createReplacementConnection 针对实例按服务类型创建一个新连接
func (c *ConnectionPool) createReplacementConnection(svc *ExternalService, inst *ServiceInstance, index int) (*ExternalConnection, error) {
	ctx := context.Background()
	switch svc.Type {
	case "sse":
		// 优先使用实例快照
		ci := inst.ResolvedConnectInfo
		if ci == nil {
			ci = c.buildConnectInfoForInstance(svc, inst)
		}
		connects, err := c.createSSEConnections(ctx, ci, svc.Id, inst.AccountId, 1)
		if err != nil {
			return nil, err
		}
		if len(connects) > 0 {
			return connects[0], nil
		} else {
			return nil, nil
		}
	case "httpStreamable":

		ci := inst.ResolvedConnectInfo
		if ci == nil {
			ci = c.buildConnectInfoForInstance(svc, inst)
		}
		return c.createHttpStreamableConnections(ctx, ci, svc.Id, inst.AccountId)
	case "stdio":
		li := inst.ResolvedLaunchInfo
		if li == nil {
			li = c.buildLaunchInfoForInstance(svc, inst)
		}
		return c.createStdioConnection(ctx, li, svc.Id, inst.AccountId)
	default:
		return nil, fmt.Errorf("不支持的服务类型: %s", svc.Type)
	}
}

// buildConnectInfoForInstance 合并服务默认 Header 与账号认证信息
func (c *ConnectionPool) buildConnectInfoForInstance(svc *ExternalService, inst *ServiceInstance) *ConnectInfo {
	ciCopy := *svc.ConnectInfo
	ciCopy.Headers = make(map[string]string)
	for k, v := range svc.ConnectInfo.Headers {
		ciCopy.Headers[k] = v
	}
	if acct := findAccountById(svc.Accounts, inst.AccountId); acct != nil {
		for k, v := range acct.AuthInfo {
			ciCopy.Headers[k] = v
		}
	}
	return &ciCopy
}

// buildLaunchInfoForInstance 合并进程环境变量与账号认证信息
func (c *ConnectionPool) buildLaunchInfoForInstance(svc *ExternalService, inst *ServiceInstance) *LaunchInfo {
	liCopy := *svc.LaunchInfo
	liCopy.Env = make(map[string]interface{})
	for k, v := range svc.LaunchInfo.Env {
		liCopy.Env[k] = v
	}
	if acct := findAccountById(svc.Accounts, inst.AccountId); acct != nil {
		for k, v := range acct.AuthInfo {
			liCopy.Env[k] = v
		}
	}
	return &liCopy
}

func findAccountById(accounts []*ExternalAccount, id int32) *ExternalAccount {
	for _, a := range accounts {
		if a != nil && a.AccountID == id {
			return a
		}
	}
	return nil
}
