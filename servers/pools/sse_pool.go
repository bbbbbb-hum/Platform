package pools

import (
	"AgentEarth_AgentPlatform/models/config"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
)

type (
	// SSEConnectionPool SSE连接池
	SSEConnectionPool struct {
		services map[string]*SSEExternalService
		mutex    sync.RWMutex
	}
	// SSEExternalService SSE外部服务
	SSEExternalService struct {
		Id                int32                                  // 远程MCP服务自增ID
		ExternalServiceId string                                 // 远程MCP服务UUID
		ServiceName       string                                 // 远程MCP服务名称
		MaxInstance       int32                                  // 最大实例数
		Tools             []*mcp.Tool                            // 远程工具列表
		ConnectInfo       *ConnectInfo                           // 连接配置信息
		Accounts          []*ExternalAccount                     // 外部账户信息
		InstanceMap       map[string]*SSEExternalServiceInstance // 运行实例列表
	}
	// SSEExternalServiceInstance SSE外部服务实例
	SSEExternalServiceInstance struct {
		InstanceId      string                // 实例ID
		AccountId       int32                 // 账户ID
		Connections     []*ExternalConnection // 连接列表
		RoundRobinIndex int                   // 轮询索引
		mutex           sync.Mutex            // 保护轮询索引
	}

	// ExternalAccount 外部账户
	ExternalAccount struct {
		AccountID int32             // 账户ID
		AuthInfo  map[string]string // 认证信息
	}
	// ExternalConnection 单个连接信息
	ExternalConnection struct {
		ConnectionID string
		Session      *mcp.ClientSession
		Status       string // connected, disconnected, error
		LastPing     time.Time
		ActiveUsers  int // 当前活跃用户数（可选，用于后期优化）
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

	// headerTransport 自定义传输层，用于添加请求头
	headerTransport struct {
		Transport http.RoundTripper
		Headers   map[string]string
	}
)

var (
	GlobalSSEConnectionPool *SSEConnectionPool
	onceSSE                 sync.Once
)

// GetSSEPool 获取SSE连接池
func GetSSEPool() *SSEConnectionPool {
	onceSSE.Do(func() {
		GlobalSSEConnectionPool = &SSEConnectionPool{
			services: make(map[string]*SSEExternalService),
		}
	})
	return GlobalSSEConnectionPool
}

// RoundTrip 实现 http.RoundTripper 接口
func (ht *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 添加自定义请求头
	for key, value := range ht.Headers {
		req.Header.Set(key, value)
	}

	// 使用底层传输层发送请求
	return ht.Transport.RoundTrip(req)
}

// InitializeSSEPool 初始化SSE连接池
func (p *SSEConnectionPool) InitializeSSEPool(externalServiceId string) error {
	logger.Info("初始化SSE连接池...", zap.String("external_service_id", externalServiceId))
	ctx := context.Background()
	err := p.initializeService(ctx, externalServiceId)
	if err != nil {
		logger.Error("创建远程连接失败",
			zap.String("external_service_id", externalServiceId),
			zap.Error(err))
		return err
	}
	return nil
}

// 初始化单个SSE服务
func (p *SSEConnectionPool) initializeService(ctx context.Context, externalServiceId string) error {
	// 从数据库加载sse服务配置
	service, err := p.loadSSEServiceConfigs(externalServiceId)
	if err != nil {
		return err
	}
	// 加载账号配置
	service, err = p.loadAccountConfigs(service)
	if err != nil {
		return err
	}
	// 创建实例信息
	service, err = p.createInstances(ctx, service)
	if err != nil {
		return err
	}

	// 获取工具列表（使用第一个可用账号的第一个连接）
	if len(service.InstanceMap) > 0 {
		var firstInstance *SSEExternalServiceInstance
		for _, instance := range service.InstanceMap {
			firstInstance = instance
			break
		}
		if firstInstance == nil {
			return errors.New("没有可用账号连接")
		}
		if len(firstInstance.Connections) > 0 {
			tools, err1 := p.fetchTools(firstInstance.Connections[0].Session)
			if err1 != nil {
				logger.Warn("获取工具列表失败", zap.Error(err1))
			} else {
				service.Tools = tools
			}

		}
	}
	// 添加到连接池
	p.mutex.Lock()
	p.services[service.ExternalServiceId] = service
	p.mutex.Unlock()

	logger.Info("服务初始化完成",
		zap.String("external_service_id", service.ExternalServiceId),
		zap.Int("账号数量", len(service.Accounts)),
		zap.Int("工具数量", len(service.Tools)))

	return nil
}

// loadSSEServiceConfigs 加载SSE服务配置
func (p *SSEConnectionPool) loadSSEServiceConfigs(externalServiceId string) (service *SSEExternalService, err error) {
	dbEsc := &config.AeMcpExternalServicesConfig{}
	err = dbEsc.GetOneByExternalServiceId(externalServiceId)
	if err != nil {
		err = fmt.Errorf("加载远程服务配置失败: %v", err)
		return
	}

	// 解析JSONB连接配置信息
	var connectInfo ConnectInfo
	connectInfoByte, err := json.Marshal(dbEsc.ConnectInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	err = json.Unmarshal(connectInfoByte, &connectInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	service = &SSEExternalService{
		Id:                dbEsc.Id,
		ExternalServiceId: dbEsc.ExternalServiceId,
		ServiceName:       dbEsc.Name,
		ConnectInfo:       &connectInfo,
		MaxInstance:       dbEsc.MaxInstance,
		InstanceMap:       make(map[string]*SSEExternalServiceInstance), // 初始化 InstanceMap
	}
	return
}

// loadAccountConfigs 加载账号配置
func (p *SSEConnectionPool) loadAccountConfigs(service *SSEExternalService) (*SSEExternalService, error) {
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

// createInstance 根据账号创建实例（如果没有账号信息，按照实例最大数配置）
func (p *SSEConnectionPool) createInstances(ctx context.Context, service *SSEExternalService) (newService *SSEExternalService, err error) {
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
				instance := &SSEExternalServiceInstance{
					AccountId:   account.AccountID,
					InstanceId:  fmt.Sprintf("instance_%d_%d_%d", service.Id, account.AccountID, i),
					Connections: make([]*ExternalConnection, 0),
				}
				//创建连接
				instance.Connections, err = p.createConnections(ctx, &connectInfoCopy, service.Id, account.AccountID)
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
			instance := &SSEExternalServiceInstance{
				AccountId:   int32(i),
				InstanceId:  fmt.Sprintf("instance_%d_%d", service.Id, i),
				Connections: make([]*ExternalConnection, 0),
			}
			//创建连接
			instance.Connections, err = p.createConnections(ctx, service.ConnectInfo, service.Id, int32(i))
			if err != nil {
				logger.Error("创建实例连接失败", zap.Error(err))
				continue
			}
			newService.InstanceMap[instance.InstanceId] = instance
		}
	}
	return
}

// createConnections 创建连接
func (p *SSEConnectionPool) createConnections(ctx context.Context, connectInfo *ConnectInfo, sid, aid int32) (connections []*ExternalConnection, err error) {
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

// fetchTools 获取工具列表
func (p *SSEConnectionPool) fetchTools(session *mcp.ClientSession) (list []*mcp.Tool, err error) {
	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		return
	}
	return result.Tools, nil
}

// CallTool 调用工具（支持指定实例或自动选择）
func (p *SSEConnectionPool) CallTool(serviceID, toolName string, args map[string]interface{}, instanceID ...string) (*mcp.CallToolResult, interface{}, error) {
	// 获取服务
	service, err := p.getService(serviceID)
	if err != nil {
		return nil, nil, err
	}

	// 选择实例
	var instance *SSEExternalServiceInstance
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
	connection := p.selectConnection(instance)
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
	// TODO: 在实际场景中，应该在请求开始时+1，请求结束时-1

	return result, result.StructuredContent, nil
}

// getService 获取服务
func (p *SSEConnectionPool) getService(externalServiceId string) (*SSEExternalService, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	service, exists := p.services[externalServiceId]
	if !exists {
		return nil, fmt.Errorf("服务不存在: %s", externalServiceId)
	}
	return service, nil
}

// selectConnection 为账号选择一个连接（轮询）
func (p *SSEConnectionPool) selectConnection(instance *SSEExternalServiceInstance) *ExternalConnection {
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
func (p *SSEConnectionPool) GetServiceTools(externalServiceId string) []*mcp.Tool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	service, exists := p.services[externalServiceId]
	if exists {
		return service.Tools
	}
	return nil
}

// Close 关闭连接池
func (p *SSEConnectionPool) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, service := range p.services {
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
	p.services = make(map[string]*SSEExternalService)
}
