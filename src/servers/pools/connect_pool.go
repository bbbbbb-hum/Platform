package pools

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

type (
	ConnectionPool struct {
		services map[string]*ExternalService // key是节点服务键（node:{id}）
		mutex    sync.RWMutex
		// 维护协程控制
		maintainStop chan struct{}
		maintainWG   sync.WaitGroup
	}
	// 节点服务定义（node_config 语义）
	ExternalService struct {
		Id             int32            // 节点ID的负值（仅用于构造稳定连接ID）
		NodeServiceKey string           // 连接池节点服务键（node:{id}）
		ServiceName    string           // 节点服务名称
		Tools          []*mcp.Tool      // 工具列表
		ConnectInfo    *ConnectInfo     // 连接配置信息
		Instance       *ServiceInstance // 单实例（固定一条连接）
	}

	ConnectInfo struct {
		Url            string            `json:"url,omitempty"`             // httpStreamable 服务地址
		Headers        map[string]string `json:"headers,omitempty"`         // 请求头
		ConnectTimeout int               `json:"connect_timeout,omitempty"` //连接超时时间（毫秒）
		CallTimeout    int               `json:"call_timeout,omitempty"`    //调用超时时间（毫秒）
		MaxConnect     int               `json:"max_connect,omitempty"`     //实例最大连接数
		MaxRetry       int               `json:"max_retry,omitempty"`       //最大重试次数
		Interval       int               `json:"interval,omitempty"`        //重试间隔
		ClientVersion  string            `json:"client_version,omitempty"`  // 客户端版本（可选）
	}
	// 节点服务运行实例（单连接）
	ServiceInstance struct {
		InstanceId string              // 实例ID
		Connection *ExternalConnection // 单连接
		mutex      sync.Mutex          // 保护连接
	}
	// ExternalConnection 单个连接信息
	ExternalConnection struct {
		ConnectionID string
		Session      *mcp.ClientSession
		Transport    *http.Transport // HTTP Transport 引用，用于关闭时释放空闲连接
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

// InitializeNode 按节点配置初始化连接（最小改动：复用现有连接池结构）。
func (c *ConnectionPool) InitializeNode(nodeID int32, protocol, nodeURL string, timeoutMS int) error {
	if nodeID <= 0 {
		return fmt.Errorf("invalid_node_config: node_id is required")
	}
	if strings.TrimSpace(nodeURL) == "" {
		return fmt.Errorf("invalid_node_config: node_config.url is required")
	}
	if strings.TrimSpace(protocol) == "" {
		protocol = "http"
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol != "http" {
		return fmt.Errorf("protocol_not_supported: only http is allowed node_id=%d protocol=%s", nodeID, protocol)
	}

	nodeServiceKey := buildNodeServiceKey(nodeID)
	if _, err := c.getService(nodeServiceKey); err == nil {
		return nil
	}

	connectTimeout := timeoutMS
	if connectTimeout <= 0 {
		connectTimeout = getConnectTimeoutMS(nil)
	}
	callTimeout := timeoutMS
	if callTimeout <= 0 {
		callTimeout = getCallTimeoutMS(nil)
	}

	service := &ExternalService{
		Id:             -nodeID,
		NodeServiceKey: nodeServiceKey,
		ServiceName:    fmt.Sprintf("node_%d", nodeID),
		Tools:          nil,
		ConnectInfo: &ConnectInfo{
			Url:            strings.TrimSpace(nodeURL),
			Headers:        map[string]string{},
			ConnectTimeout: connectTimeout,
			CallTimeout:    callTimeout,
			MaxConnect:     1,
			MaxRetry:       1,
			Interval:       1000,
		},
		Instance: nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(connectTimeout)*time.Millisecond)
	defer cancel()

	var err error
	service, err = c.createInstancesForHttpStreamable(ctx, service)
	if err != nil {
		return fmt.Errorf("http_connect_failed: node_id=%d node_service_key=%s url=%s err=%w", nodeID, nodeServiceKey, strings.TrimSpace(nodeURL), err)
	}

	if service.Instance == nil || service.Instance.Connection == nil || service.Instance.Connection.Session == nil {
		closeNodeServiceSessions(service)
		return fmt.Errorf("list_tools_failed: no_instance_created node_id=%d node_service_key=%s protocol=%s", nodeID, nodeServiceKey, protocol)
	}
	tools, err := c.fetchTools(service.Instance.Connection.Session)
	if err != nil {
		logger.Warn("list_tools failed",
			zap.String("stage", "list_tools"),
			zap.String("error_type", "upstream_error"),
			zap.Int32("node_id", nodeID),
			zap.String("node_service_key", nodeServiceKey),
			zap.String("protocol", protocol),
			zap.Error(err))
		closeNodeServiceSessions(service)
		return fmt.Errorf("list_tools_failed: protocol=%s node_id=%d node_service_key=%s url=%s err=%w", protocol, nodeID, nodeServiceKey, strings.TrimSpace(nodeURL), err)
	} else {
		service.Tools = tools
	}

	c.mutex.Lock()
	c.services[service.NodeServiceKey] = service
	c.mutex.Unlock()
	return nil
}

func closeNodeServiceSessions(service *ExternalService) {
	if service == nil {
		return
	}
	if service.Instance == nil || service.Instance.Connection == nil || service.Instance.Connection.Session == nil {
		return
	}
	_ = service.Instance.Connection.Session.Close()
}

// 获取工具
func (c *ConnectionPool) fetchTools(session *mcp.ClientSession) (list []*mcp.Tool, err error) {
	timeoutMS := getListToolsTimeoutMS()
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()
	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("listtools_timeout: timeout_ms=%d err=%w", timeoutMS, err)
		}
		logger.Warn("list_tools failed",
			zap.String("stage", "list_tools"),
			zap.String("error_type", "upstream_error"),
			zap.Int("timeout_ms", timeoutMS),
			zap.Int64("elapsed_ms", time.Since(start).Milliseconds()),
			zap.Error(err))
		return
	}
	return result.Tools, nil
}

// CallTool 调用工具（单实例单连接，失败时快速重连并重试一次）。
func (c *ConnectionPool) CallTool(nodeServiceKey, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// 获取服务
	service, err := c.getService(nodeServiceKey)
	if err != nil {
		return nil, err
	}

	instance, connection, err := getAvailableConnection(service)
	if err != nil {
		return nil, err
	}

	callOnce := func(conn *ExternalConnection) (*mcp.CallToolResult, string, error, int64) {
		timeoutMS := getCallTimeoutMS(service.ConnectInfo)
		start := time.Now()
		callCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMS)*time.Millisecond)
		defer cancel()

		params := &mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		}
		result, callErr := conn.Session.CallTool(callCtx, params)
		elapsedMS := time.Since(start).Milliseconds()
		if callErr != nil {
			errorType := "upstream_error"
			if callCtx.Err() == context.DeadlineExceeded {
				errorType = "call_timeout"
			}
			return nil, errorType, callErr, elapsedMS
		}
		conn.LastPing = time.Now()
		conn.ActiveUsers++
		return result, "", nil, elapsedMS
	}

	result, errorType, callErr, elapsedMS := callOnce(connection)
	if callErr == nil {
		return result, nil
	}

	logger.Error("call_tool failed",
		zap.String("stage", "call_tool"),
		zap.String("error_type", errorType),
		zap.Int("timeout_ms", getCallTimeoutMS(service.ConnectInfo)),
		zap.Int64("elapsed_ms", elapsedMS),
		zap.Int("attempt", 1),
		zap.String("node_service_key", nodeServiceKey),
		zap.Int32("node_id", parseNodeIDFromServiceKey(nodeServiceKey)),
		zap.String("tool_name", toolName),
		zap.Error(callErr))

	retryConnection, reconnectErr := c.fastReconnectNodeService(service, instance)
	if reconnectErr != nil {
		logger.Error("call_tool failed",
			zap.String("stage", "call_tool_reconnect"),
			zap.String("error_type", "reconnect_failed"),
			zap.Int("attempt", 1),
			zap.String("node_service_key", nodeServiceKey),
			zap.Int32("node_id", parseNodeIDFromServiceKey(nodeServiceKey)),
			zap.String("tool_name", toolName),
			zap.Error(reconnectErr))
		return nil, fmt.Errorf("%s: %v", errorType, callErr)
	}

	result, retryErrorType, retryErr, retryElapsedMS := callOnce(retryConnection)
	if retryErr != nil {
		logger.Error("call_tool failed",
			zap.String("stage", "call_tool"),
			zap.String("error_type", retryErrorType),
			zap.Int("timeout_ms", getCallTimeoutMS(service.ConnectInfo)),
			zap.Int64("elapsed_ms", retryElapsedMS),
			zap.Int("attempt", 2),
			zap.String("node_service_key", nodeServiceKey),
			zap.Int32("node_id", parseNodeIDFromServiceKey(nodeServiceKey)),
			zap.String("tool_name", toolName),
			zap.Error(retryErr))
		return nil, fmt.Errorf("%s: %v", retryErrorType, retryErr)
	}

	logger.Warn("call_tool recovered after reconnect",
		zap.String("node_service_key", nodeServiceKey),
		zap.Int32("node_id", parseNodeIDFromServiceKey(nodeServiceKey)),
		zap.String("tool_name", toolName))
	return result, nil
}

// CallToolByNode 按 node_id 调用工具。
func (c *ConnectionPool) CallToolByNode(nodeID int32, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	return c.CallTool(buildNodeServiceKey(nodeID), toolName, args)
}

// getService 获取服务
func (c *ConnectionPool) getService(nodeServiceKey string) (*ExternalService, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	service, exists := c.services[nodeServiceKey]
	if !exists {
		return nil, fmt.Errorf("服务不存在: %s", nodeServiceKey)
	}
	return service, nil
}

// GetServiceTools 获取单个服务工具列表
func (c *ConnectionPool) GetServiceTools(nodeServiceKey string) []*mcp.Tool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	service, exists := c.services[nodeServiceKey]
	if exists {
		return service.Tools
	}
	return nil
}

// GetNodeTools 按 node_id 获取工具列表。
func (c *ConnectionPool) GetNodeTools(nodeID int32) []*mcp.Tool {
	return c.GetServiceTools(buildNodeServiceKey(nodeID))
}

// RemoveNode 释放指定 node_id 对应的连接资源。
func (c *ConnectionPool) RemoveNode(nodeID int32) error {
	if nodeID <= 0 {
		return fmt.Errorf("invalid_node_id")
	}
	return c.removeService(buildNodeServiceKey(nodeID))
}

func (c *ConnectionPool) removeService(nodeServiceKey string) error {
	c.mutex.Lock()
	service, exists := c.services[nodeServiceKey]
	if exists {
		delete(c.services, nodeServiceKey)
	}
	c.mutex.Unlock()

	if !exists || service == nil {
		return nil
	}

	if service.Instance != nil && service.Instance.Connection != nil && service.Instance.Connection.Session != nil {
		_ = service.Instance.Connection.Session.Close()
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
		if service != nil && service.Instance != nil {
			closeConnection(service.Instance.Connection, service.NodeServiceKey)
		}
	}
	c.services = make(map[string]*ExternalService)
}

// closeConnection 关闭单个连接并释放其资源（Session + Transport）
func closeConnection(conn *ExternalConnection, instanceID string) {
	if conn == nil {
		return
	}
	// 先关闭 Session
	if conn.Session != nil {
		if err := conn.Session.Close(); err != nil {
			logger.Error("关闭连接失败",
				zap.String("instanceID", instanceID),
				zap.Error(err))
		} else {
			logger.Info("关闭连接成功", zap.String("instanceID", instanceID))
		}
	}
	// 再关闭 Transport 的空闲连接，释放文件描述符
	if conn.Transport != nil {
		conn.Transport.CloseIdleConnections()
	}
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
	// 在启动 goroutine 前登记
	c.maintainWG.Add(1)
	c.mutex.Unlock()

	go func() {
		defer c.maintainWG.Done()
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
	// 等待维护协程退出，避免与 Close 中的资源释放并发冲突
	c.maintainWG.Wait()
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
	logger.Info("检查连接状态...")
	for _, svc := range services {
		if svc == nil || svc.Instance == nil {
			continue
		}
		inst := svc.Instance

		// 1) 拿连接快照
		inst.mutex.Lock()
		conn := inst.Connection
		inst.mutex.Unlock()

		// 2) 健康检查（在锁外做 IO）
		if conn != nil && conn.Session != nil && !c.isSessionHealthy(conn.Session) {
			closeConnection(conn, inst.InstanceId)
			logger.Debug("关闭异常会话", zap.String("InstanceId", inst.InstanceId))
			inst.mutex.Lock()
			if inst.Connection == conn {
				inst.Connection = nil
			}
			inst.mutex.Unlock()
		}

		// 3) 固定单连接：为空则补齐一条
		inst.mutex.Lock()
		needsCreate := inst.Connection == nil || inst.Connection.Session == nil
		inst.mutex.Unlock()
		if !needsCreate {
			continue
		}
		newConn, err := c.createReplacementConnection(svc, inst)
		if err != nil {
			logger.Warn("维护补齐连接失败", zap.String("service_name", svc.ServiceName), zap.Error(err))
			continue
		}
		inst.mutex.Lock()
		inst.Connection = newConn
		inst.mutex.Unlock()
		logger.Debug("恢复连接", zap.Int("数量", 1))
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

// createReplacementConnection 针对实例创建一条新的 HTTP 连接。
func (c *ConnectionPool) createReplacementConnection(svc *ExternalService, inst *ServiceInstance) (*ExternalConnection, error) {
	ctx := context.Background()
	if svc == nil || svc.ConnectInfo == nil {
		return nil, fmt.Errorf("连接配置缺失")
	}
	return c.createHttpStreamableConnections(ctx, svc.ConnectInfo, svc.Id)
}

func getAvailableConnection(service *ExternalService) (*ServiceInstance, *ExternalConnection, error) {
	if service == nil {
		return nil, nil, fmt.Errorf("服务未初始化")
	}
	instance := service.Instance
	if instance == nil {
		return nil, nil, fmt.Errorf("没有可用实例")
	}
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if instance.Connection == nil || instance.Connection.Session == nil {
		return nil, nil, fmt.Errorf("没有可用的连接")
	}
	return instance, instance.Connection, nil
}

func (c *ConnectionPool) fastReconnectNodeService(service *ExternalService, instance *ServiceInstance) (*ExternalConnection, error) {
	if service == nil || instance == nil {
		return nil, fmt.Errorf("没有可用实例")
	}

	instance.mutex.Lock()
	oldConnection := instance.Connection
	instance.Connection = nil
	instance.mutex.Unlock()

	if oldConnection != nil {
		closeConnection(oldConnection, instance.InstanceId)
	}

	newConn, err := c.createReplacementConnection(service, instance)
	if err != nil {
		return nil, err
	}

	instance.mutex.Lock()
	instance.Connection = newConn
	instance.mutex.Unlock()
	return newConn, nil
}
