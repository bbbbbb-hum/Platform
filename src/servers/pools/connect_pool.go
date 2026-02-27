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
	// ConnectionPool 是 node 维度的连接资源管理器。
	// 设计目标：
	// - key 固定为 node:{id}
	// - 每个 node 固定单实例单连接
	// - 调用时可快速重连，后台可周期自愈
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
		InstanceId        string                // 实例ID
		Connections       []*ExternalConnection // 连接列表
		RoundRobinIndex   int                   // 轮询索引
		TargetConnections int                   // 目标连接数（用于维护协程补齐）
		mutex             sync.Mutex            // 保护连接与轮询索引
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
		// 全局单例：整个进程只维护一个连接池，避免多池导致连接重复。
		GlobalConnectionPool = &ConnectionPool{
			services: make(map[string]*ExternalService),
		}
	})
	return GlobalConnectionPool
}

// InitializeNode 按节点配置初始化连接（支持单节点多连接，默认1条）。
func (c *ConnectionPool) InitializeNode(nodeID int32, protocol, nodeURL string, timeoutMS int, maxConnect int) error {
	// 1) 入参校验：node_id 与 node_config.url 必须存在
	if nodeID <= 0 {
		return fmt.Errorf("invalid_node_config: node_id is required")
	}
	if strings.TrimSpace(nodeURL) == "" {
		return fmt.Errorf("invalid_node_config: node_config.url is required")
	}
	// 2) 协议收敛：只允许 http（空值按 http 处理）
	if strings.TrimSpace(protocol) == "" {
		protocol = "http"
	}
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol != "http" {
		return fmt.Errorf("protocol_not_supported: only http is allowed node_id=%d protocol=%s", nodeID, protocol)
	}

	nodeServiceKey := buildNodeServiceKey(nodeID)
	// 已存在连接则直接复用，避免重复初始化。
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
	targetConnections := getNodeMaxConnect(maxConnect)

	// 3) 固定单实例单连接配置：每个 node 只维护一条上游连接。
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
			MaxConnect:     targetConnections,
			MaxRetry:       1,
			Interval:       1000,
		},
		Instance: nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(connectTimeout)*time.Millisecond)
	defer cancel()

	var err error
	// 4) 先建立连接，再拉取工具列表；任何一步失败都返回可诊断错误。
	service, err = c.createInstancesForHttpStreamable(ctx, service)
	if err != nil {
		return fmt.Errorf("http_connect_failed: node_id=%d node_service_key=%s url=%s err=%w", nodeID, nodeServiceKey, strings.TrimSpace(nodeURL), err)
	}

	if service.Instance == nil || len(service.Instance.Connections) == 0 {
		closeNodeServiceSessions(service)
		return fmt.Errorf("list_tools_failed: no_instance_created node_id=%d node_service_key=%s protocol=%s", nodeID, nodeServiceKey, protocol)
	}
	// 使用第一条可用连接拉取工具清单。
	var firstConn *ExternalConnection
	for _, conn := range service.Instance.Connections {
		if conn != nil && conn.Session != nil {
			firstConn = conn
			break
		}
	}
	if firstConn == nil {
		closeNodeServiceSessions(service)
		return fmt.Errorf("list_tools_failed: no_connection_available node_id=%d node_service_key=%s protocol=%s", nodeID, nodeServiceKey, protocol)
	}
	tools, err := c.fetchTools(firstConn.Session)
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

	// 5) 初始化成功后再写入池，避免半初始化对象进入 services。
	c.mutex.Lock()
	c.services[service.NodeServiceKey] = service
	c.mutex.Unlock()
	return nil
}

func closeNodeServiceSessions(service *ExternalService) {
	if service == nil {
		return
	}
	if service.Instance == nil || len(service.Instance.Connections) == 0 {
		return
	}
	for _, conn := range service.Instance.Connections {
		if conn == nil || conn.Session == nil {
			continue
		}
		// 这里只关 Session，Transport 的空闲连接释放由 closeConnection 处理。
		_ = conn.Session.Close()
	}
}

// 获取工具
func (c *ConnectionPool) fetchTools(session *mcp.ClientSession) (list []*mcp.Tool, err error) {
	// ListTools 是节点初始化可用性的关键步骤，失败即视为“连接不可用”。
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

	instance, connection, connectionIndex, err := getAvailableConnection(service)
	if err != nil {
		return nil, err
	}

	// callOnce 封装单次调用，便于首调与重试复用完全一致的超时/日志语义。
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

	// 首次失败先记日志，再进入快速重连。
	logger.Error("call_tool failed",
		zap.String("stage", "call_tool"),
		zap.String("error_type", errorType),
		zap.Int("timeout_ms", getCallTimeoutMS(service.ConnectInfo)),
		zap.Int64("elapsed_ms", elapsedMS),
		zap.Int("attempt", 1),
		zap.String("node_service_key", nodeServiceKey),
		zap.Int32("node_id", parseNodeIDFromServiceKey(nodeServiceKey)),
		zap.Int("connection_index", connectionIndex),
		zap.String("tool_name", toolName),
		zap.Error(callErr))

	// 快速重连策略：仅重建当前槽位连接，避免影响其它健康连接。
	retryConnection, reconnectErr := c.fastReconnectNodeConnection(service, instance, connectionIndex)
	if reconnectErr != nil {
		logger.Error("call_tool failed",
			zap.String("stage", "call_tool_reconnect"),
			zap.String("error_type", "reconnect_failed"),
			zap.Int("attempt", 1),
			zap.String("node_service_key", nodeServiceKey),
			zap.Int32("node_id", parseNodeIDFromServiceKey(nodeServiceKey)),
			zap.Int("connection_index", connectionIndex),
			zap.String("tool_name", toolName),
			zap.Error(reconnectErr))
		return nil, fmt.Errorf("%s: %v", errorType, callErr)
	}

	// 重试一次仍失败则透传最终错误，避免无限重试拖垮上游。
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
			zap.Int("connection_index", connectionIndex),
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
	// 只读路径用 RLock，允许并发读取多个 node 服务状态。
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
	// 先从 map 移除再释放资源，避免并发读到“正在关闭”的对象。
	c.mutex.Lock()
	service, exists := c.services[nodeServiceKey]
	if exists {
		delete(c.services, nodeServiceKey)
	}
	c.mutex.Unlock()

	if !exists || service == nil {
		return nil
	}

	// remove 是显式释放场景，关闭该节点全部连接。
	if service.Instance != nil {
		for _, conn := range service.Instance.Connections {
			if conn == nil || conn.Session == nil {
				continue
			}
			_ = conn.Session.Close()
		}
	}
	return nil
}

// Close 关闭所有连接
func (c *ConnectionPool) Close() {
	// 停止维护协程
	c.StopMaintainer()
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 全量关闭时走 closeConnection，确保 Session + Transport 都释放。
	for _, service := range c.services {
		if service == nil || service.Instance == nil {
			continue
		}
		for _, conn := range service.Instance.Connections {
			closeConnection(conn, service.NodeServiceKey)
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
	// 使用 stop channel 控制协程退出，避免 goroutine 泄漏。
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
		nodeServiceKey := svc.NodeServiceKey
		nodeID := parseNodeIDFromServiceKey(nodeServiceKey)

		// 1) 拿连接快照：只短暂持有实例锁，不在锁内做网络 IO。
		inst.mutex.Lock()
		connsSnapshot := make([]*ExternalConnection, len(inst.Connections))
		copy(connsSnapshot, inst.Connections)
		targetConnections := inst.TargetConnections
		if targetConnections <= 0 {
			targetConnections = 1
		}
		inst.mutex.Unlock()

		// 2) 健康检查（在锁外做 IO）
		healthy := make([]*ExternalConnection, 0, len(connsSnapshot))
		for idx, conn := range connsSnapshot {
			if conn == nil || conn.Session == nil {
				continue
			}
			if !c.isSessionHealthy(conn.Session) {
				closeConnection(conn, inst.InstanceId)
				logger.Warn("关闭异常会话",
					zap.String("instance_id", inst.InstanceId),
					zap.String("node_service_key", nodeServiceKey),
					zap.Int32("node_id", nodeID),
					zap.Int("connection_index", idx))
				continue
			}
			healthy = append(healthy, conn)
		}

		// 3) 写回健康连接并补齐到目标连接数（自愈）。
		inst.mutex.Lock()
		inst.Connections = healthy
		currentConnections := len(inst.Connections)
		inst.mutex.Unlock()
		createdCount := 0
		for currentConnections < targetConnections {
			newConn, err := c.createReplacementConnection(svc, inst, currentConnections)
			if err != nil {
				logger.Warn("维护补齐连接失败",
					zap.String("service_name", svc.ServiceName),
					zap.String("instance_id", inst.InstanceId),
					zap.String("node_service_key", nodeServiceKey),
					zap.Int32("node_id", nodeID),
					zap.Int("target_connections", targetConnections),
					zap.Int("current_connections", currentConnections),
					zap.Error(err))
				break
			}
			inst.mutex.Lock()
			inst.Connections = append(inst.Connections, newConn)
			currentConnections = len(inst.Connections)
			inst.mutex.Unlock()
			createdCount++
		}
		if createdCount > 0 {
			logger.Info("恢复连接",
				zap.String("instance_id", inst.InstanceId),
				zap.String("node_service_key", nodeServiceKey),
				zap.Int32("node_id", nodeID),
				zap.Int("created_connections", createdCount),
				zap.Int("target_connections", targetConnections),
				zap.Int("current_connections", currentConnections))
		} else {
			logger.Debug("恢复连接",
				zap.String("instance_id", inst.InstanceId),
				zap.String("node_service_key", nodeServiceKey),
				zap.Int32("node_id", nodeID),
				zap.Int("created_connections", createdCount),
				zap.Int("target_connections", targetConnections),
				zap.Int("current_connections", currentConnections))
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

// createReplacementConnection 针对实例创建一条新的 HTTP 连接。
func (c *ConnectionPool) createReplacementConnection(svc *ExternalService, inst *ServiceInstance, connectionIndex int) (*ExternalConnection, error) {
	ctx := context.Background()
	if svc == nil || svc.ConnectInfo == nil {
		return nil, fmt.Errorf("连接配置缺失")
	}
	return c.createHttpStreamableConnections(ctx, svc.ConnectInfo, svc.Id, connectionIndex)
}

func getAvailableConnection(service *ExternalService) (*ServiceInstance, *ExternalConnection, int, error) {
	if service == nil {
		return nil, nil, -1, fmt.Errorf("服务未初始化")
	}
	instance := service.Instance
	if instance == nil {
		return nil, nil, -1, fmt.Errorf("没有可用实例")
	}
	instance.mutex.Lock()
	defer instance.mutex.Unlock()
	if len(instance.Connections) == 0 {
		return nil, nil, -1, fmt.Errorf("没有可用的连接")
	}
	startIndex := instance.RoundRobinIndex
	for i := 0; i < len(instance.Connections); i++ {
		idx := (startIndex + i) % len(instance.Connections)
		conn := instance.Connections[idx]
		if conn == nil || conn.Session == nil {
			continue
		}
		instance.RoundRobinIndex = (idx + 1) % len(instance.Connections)
		return instance, conn, idx, nil
	}
	return nil, nil, -1, fmt.Errorf("没有可用的连接")
}

func (c *ConnectionPool) fastReconnectNodeConnection(service *ExternalService, instance *ServiceInstance, connectionIndex int) (*ExternalConnection, error) {
	if service == nil || instance == nil {
		return nil, fmt.Errorf("没有可用实例")
	}
	if connectionIndex < 0 {
		return nil, fmt.Errorf("连接索引无效")
	}

	// Step A: 先把实例上的连接指针置空，阻断并发请求继续使用旧连接。
	instance.mutex.Lock()
	if connectionIndex >= len(instance.Connections) {
		instance.mutex.Unlock()
		return nil, fmt.Errorf("连接索引越界")
	}
	oldConnection := instance.Connections[connectionIndex]
	instance.Connections[connectionIndex] = nil
	instance.mutex.Unlock()

	// Step B: 销毁旧连接（若存在），释放 Session/Transport 资源。
	if oldConnection != nil {
		closeConnection(oldConnection, instance.InstanceId)
	}

	// Step C: 基于同一 service 配置创建新连接。
	newConn, err := c.createReplacementConnection(service, instance, connectionIndex)
	if err != nil {
		return nil, err
	}

	// Step D: 安装新连接，后续请求即可直接复用。
	instance.mutex.Lock()
	if connectionIndex < len(instance.Connections) {
		instance.Connections[connectionIndex] = newConn
	} else {
		instance.Connections = append(instance.Connections, newConn)
	}
	instance.mutex.Unlock()
	return newConn, nil
}
