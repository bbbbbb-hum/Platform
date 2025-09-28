package pools

import (
	config2 "AgentEarth_AgentPlatform/src/models/config"
	"context"
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

type (
	ConnectionPool struct {
		services map[string]*ExternalService
		mutex    sync.RWMutex
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
	}
	// ExternalConnection 单个连接信息
	ExternalConnection struct {
		ConnectionID string
		Session      *mcp.ClientSession
		Status       string // connected, disconnected, error
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

// InitializeConnectPool 初始化连接池
func (c *ConnectionPool) InitializeConnectPool() error {
	logger.Info("初始化连接池...")
	externalServiceConfigModel := config2.AeMcpExternalServicesConfig{}
	list, err := externalServiceConfigModel.GetList()
	if err != nil {
		logger.Error("获取外部服务列表失败", zap.Error(err))
		return err
	}
	for _, v := range list {
		err = c.InitializeService(v.ExternalServiceId)
		if err != nil {
			logger.Error("创建连接失败", zap.String("service_name", v.Name), zap.Error(err))
			return err
		}
	}

	logger.Info("初始化连接池完成...")
	return nil
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
	service, err = c.loadAccountConfigs(service)
	if err != nil {
		return err
	}

	// 创建实例信息
	switch service.Type {
	case "sse":
		service, err = c.createInstancesForSSE(ctx, service)
	case "stdio":
		service, err = c.createInstancesForStdio(ctx, service)
	//case "httpStreamable":
	//service, err = c.createInstancesForHttpStreamable(ctx, service)
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
	externalServiceConfigModel := config2.AeMcpExternalServicesConfig{}
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
		var accountModel = config2.AeMcpExternalServicesAccount{}
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
					AccountId:   account.AccountID,
					InstanceId:  fmt.Sprintf("instance_%d_%d_%d", service.Id, account.AccountID, i),
					Connections: make([]*ExternalConnection, 0),
				}
				//创建连接
				instance.Connections, err = c.createSSEConnections(ctx, &connectInfoCopy, service.Id, account.AccountID)
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
				AccountId:   int32(i),
				InstanceId:  fmt.Sprintf("instance_%d_%d", service.Id, i),
				Connections: make([]*ExternalConnection, 0),
			}
			//创建连接
			instance.Connections, err = c.createSSEConnections(ctx, service.ConnectInfo, service.Id, int32(i))
			if err != nil {
				logger.Error("创建实例连接失败", zap.Error(err))
				continue
			}
			newService.InstanceMap[instance.InstanceId] = instance
		}
	}

	return service, nil
}

// 创建SSE连接
func (c *ConnectionPool) createSSEConnections(ctx context.Context, connectInfo *ConnectInfo, sid, aid int32) (connections []*ExternalConnection, err error) {
	logger.Info("创建连接...")

	// 创建HTTP客户端
	httpClient := &http.Client{
		//Timeout: time.Duration(connectInfo.ConnectTimeout) * time.Millisecond,
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
		Name:    "AgentEarth-Proxy",
		Version: "v1.0.0",
	}, clientOpts)

	// 创建多个连接
	successCount := 0
	for i := 0; i < connectInfo.MaxConnect; i++ {
		session, err1 := client.Connect(ctx, transport, nil)

		if err1 != nil {
			logger.Error("创建连接失败", zap.Error(err1), zap.Int("index", i))
			continue
		}

		connection := &ExternalConnection{
			ConnectionID: fmt.Sprintf("%d-%d-%d", sid, aid, i),
			Session:      session,
			Status:       "connected",
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
	if service.Accounts != nil && len(service.Accounts) > 0 {
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
				// 按当前账号信息创建实例
				instance, err1 := c.createStdioInstance(ctx, service, int(account.AccountID), i)
				if err1 != nil {
					logger.Error("创建实例失败", zap.Int("index", i), zap.Error(err1))
					continue
				}
				service.InstanceMap[instance.InstanceId] = instance
			}
		}
	} else {
		// 无需鉴权
		for i := 0; i < int(service.MaxInstance); i++ {
			instance, err1 := c.createStdioInstance(ctx, service, 0, i)
			if err1 != nil {
				logger.Error("创建实例失败", zap.Int("index", i), zap.Error(err1))
				continue
			}
			service.InstanceMap[instance.InstanceId] = instance
		}
	}
	newService = service

	return
}

func (c *ConnectionPool) createStdioInstance(ctx context.Context, service *ExternalService, aid, i int) (instance *ServiceInstance, err error) {
	cmd := exec.Command(service.LaunchInfo.Command, service.LaunchInfo.Args...)
	if len(service.LaunchInfo.Env) > 0 {
		for s, k := range service.LaunchInfo.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", s, k))
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
	session, err1 := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err1 != nil {
		logger.Error("创建连接失败", zap.Error(err1), zap.Int("index", i))
		err = err1
		return
	}

	connection := &ExternalConnection{
		ConnectionID: fmt.Sprintf("%d-%d-%d", service.Id, aid, i),
		Session:      session,
		Status:       "connected",
		LastPing:     time.Now(),
		ActiveUsers:  0,
	}
	instance = &ServiceInstance{
		AccountId:   int32(aid),
		InstanceId:  fmt.Sprintf("instance_%d_%d", service.Id, i), // 实例ID 使用格式 instance_服务ID_实例数量索引
		Connections: []*ExternalConnection{connection},
	}
	return
}

func (c *ConnectionPool) fetchTools(session *mcp.ClientSession) (list []*mcp.Tool, err error) {
	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		return
	}
	return result.Tools, nil
}

// CallTool 调用工具（支持指定实例或自动选择）
func (c *ConnectionPool) CallTool(serviceID, toolName string, args map[string]interface{}, instanceID ...string) (*mcp.CallToolResult, interface{}, error) {
	// 获取服务
	service, err := c.getService(serviceID)
	if err != nil {
		return nil, nil, err
	}

	// 选择实例
	var instance *ServiceInstance
	if len(instanceID) > 0 && instanceID[0] != "" {
		// 使用指定账号
		instance = service.InstanceMap[instanceID[0]]
		if instance == nil {
			return nil, nil, fmt.Errorf("账号不存在: %s", instanceID[0])
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
			return nil, nil, fmt.Errorf("没有可用的账号")
		}
	}

	// 选择连接
	connection := c.selectConnection(instance)
	if connection == nil {
		return nil, nil, fmt.Errorf("没有可用的连接")
	}

	// 调用工具
	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	result, err := connection.Session.CallTool(context.Background(), params)
	if err != nil {
		return nil, nil, fmt.Errorf("工具调用失败: %v", err)
	}

	// 更新连接状态（这里简化处理，实际应该在请求完成后减少ActiveUsers）
	connection.LastPing = time.Now()
	connection.ActiveUsers++

	return result, result.StructuredContent, nil
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
		if conn.Status == "connected" {
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
