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
	// - 每个 node 固定单实例多连接（由 max_connect 控制目标连接数）
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
		NodeID            int32            // 节点ID（内部标识）
		NodeServiceKey    string           // 连接池节点服务键（node:{id}）
		ServiceName       string           // 节点服务名称
		Tools             []*mcp.Tool      // 工具列表
		ConnectInfo       *ConnectInfo     // 连接配置信息
		Instance          *ServiceInstance // 单实例（维护连接列表）
		ConfigFingerprint string           // 当前生效配置指纹（用于自动重建）
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
	// 节点服务运行实例（多连接）
	ServiceInstance struct {
		InstanceId        string                // 实例ID
		Connections       []*ExternalConnection // 连接列表
		RoundRobinIndex   int                   // 轮询索引
		TargetConnections int                   // 目标连接数（用于维护协程补齐）
		mutex             sync.Mutex            // 保护连接与轮询索引
	}
	// ExternalConnection 单个连接信息
	ExternalConnection struct {
		Session     *mcp.ClientSession
		Transport   *http.Transport // HTTP Transport 引用，用于关闭时释放空闲连接
		LastPing    time.Time
		ActiveUsers int // 当前活跃用户数（可选，用于后期优化）
	}
	// headerTransport 自定义传输层，用于添加请求头
	headerTransport struct {
		Transport http.RoundTripper
		Headers   map[string]string
	}
	NodePoolState struct {
		NodeID            int32
		NodeServiceKey    string
		ActiveConnections int
		TargetConnections int
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
func (c *ConnectionPool) InitializeNode(nodeID int32, protocol, nodeURL string, timeout int, maxConnect int) error {
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

	connectTimeout := timeout
	if connectTimeout <= 0 {
		connectTimeout = getConnectTimeout(nil)
	}
	callTimeout := timeout
	if callTimeout <= 0 {
		callTimeout = getCallTimeout(nil)
	}
	targetConnections, maxConnectDecision := normalizeNodeMaxConnect(maxConnect)
	logger.Debug("initialize node target connections",
		zap.Int32("node_id", nodeID),
		zap.String("node_service_key", nodeServiceKey),
		zap.Int("requested_max_connect", maxConnect),
		zap.Int("effective_max_connect", targetConnections),
		zap.String("max_connect_decision", maxConnectDecision))
	if maxConnect != targetConnections {
		logger.Info("max_connect normalized",
			zap.Int32("node_id", nodeID),
			zap.Int("requested_max_connect", maxConnect),
			zap.Int("effective_max_connect", targetConnections))
	}

	// 3) 单实例多连接：每个 node 维护 N 条上游连接（由 targetConnections 决定）；配置指纹用于判断是否重建。
	configFingerprint := buildNodeConfigFingerprint(strings.TrimSpace(nodeURL), connectTimeout, callTimeout, targetConnections)
	if existing, err := c.getService(nodeServiceKey); err == nil && existing != nil {
		if existing.ConfigFingerprint == configFingerprint {
			return nil
		}
		logger.Info("node config changed, rebuilding pool",
			zap.Int32("node_id", nodeID),
			zap.String("node_service_key", nodeServiceKey),
			zap.String("old_fingerprint", existing.ConfigFingerprint),
			zap.String("new_fingerprint", configFingerprint))
		_ = c.removeService(nodeServiceKey)
	}
	service := &ExternalService{
		NodeID:         nodeID,
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
		Instance:          nil,
		ConfigFingerprint: configFingerprint,
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
	tools, err := c.fetchTools(firstConn.Session, callTimeout)
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
		logger.Info("initialize node tools",
			zap.Int32("node_id", nodeID),
			zap.String("node_service_key", nodeServiceKey),
			zap.Int("tools_count", len(tools)))
	}

	// 5) 初始化成功后再写入池，避免半初始化对象进入 services。
	c.mutex.Lock()
	c.services[service.NodeServiceKey] = service
	c.mutex.Unlock()
	return nil
}

// closeNodeServiceSessions 关闭节点实例下已建立的连接资源。
// 用于初始化失败或重建前的清理，避免半初始化连接残留。
// 统一走 closeConnection，确保 Session + Transport 都释放。
func closeNodeServiceSessions(service *ExternalService) {
	closeServiceConnections(service)
}

func closeServiceConnections(service *ExternalService) {
	if service == nil || service.Instance == nil || len(service.Instance.Connections) == 0 {
		return
	}
	for _, conn := range service.Instance.Connections {
		closeConnection(conn, service.NodeServiceKey)
	}
}

// 获取工具（使用节点统一超时）
func (c *ConnectionPool) fetchTools(session *mcp.ClientSession, timeout int) (list []*mcp.Tool, err error) {
	// ListTools 是节点初始化可用性的关键步骤，失败即视为“连接不可用”。
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()
	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("listtools_timeout: timeout=%d err=%w", timeout, err)
		}
		logger.Warn("list_tools failed",
			zap.String("stage", "list_tools"),
			zap.String("error_type", "upstream_error"),
			zap.Int("timeout", timeout),
			zap.Int64("elapsed_ms", time.Since(start).Milliseconds()),
			zap.Error(err))
		return
	}
	return result.Tools, nil
}

// CallTool 调用工具（单实例多连接，选择可用连接；失败时记录并返回错误）。
func (c *ConnectionPool) CallTool(nodeServiceKey, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	// 获取服务
	service, err := c.getService(nodeServiceKey)
	if err != nil {
		return nil, err
	}

	_, connection, err := getAvailableConnection(service)
	if err != nil {
		return nil, err
	}
	// nodeServiceKey 形如 node:{id}，解析出真实 node_id 便于日志与排障。
	nodeID := parseNodeIDFromServiceKey(nodeServiceKey)
	// 统计工具调用入参键数量，仅用于日志，不影响业务逻辑。
	argsCount := 0
	if args != nil {
		argsCount = len(args)
	}
	logger.Info("call_tool start",
		zap.String("node_service_key", nodeServiceKey),
		zap.Int32("node_id", nodeID),
		zap.String("tool_name", toolName),
		zap.Int("args_count", argsCount))

	// callOnce 封装单次调用：
	// - 使用节点统一 timeout（毫秒）作为调用超时
	// - 输出统一的 errorType（call_timeout/upstream_error）
	// - 成功时更新连接健康度/活跃度
	callOnce := func(conn *ExternalConnection) (*mcp.CallToolResult, string, error, int64) {
		timeout := getCallTimeout(service.ConnectInfo)
		start := time.Now()
		callCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
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
		// 成功调用后刷新心跳；ActiveUsers 为粗粒度活跃度累计（非实时并发）。
		conn.LastPing = time.Now()
		conn.ActiveUsers++
		return result, "", nil, elapsedMS
	}

	result, errorType, callErr, elapsedMS := callOnce(connection)
	if callErr == nil {
		logger.Info("call_tool end",
			zap.String("node_service_key", nodeServiceKey),
			zap.Int32("node_id", nodeID),
			zap.String("tool_name", toolName),
			zap.Bool("success", true),
			zap.Int("attempt", 1),
			zap.Int64("latency_ms", elapsedMS))
		return result, nil
	}

	// 首次失败先记日志，直接返回错误。
	logger.Error("call_tool failed",
		zap.String("stage", "call_tool"),
		zap.String("error_type", errorType),
		zap.Int("timeout", getCallTimeout(service.ConnectInfo)),
		zap.Int64("elapsed_ms", elapsedMS),
		zap.Int("attempt", 1),
		zap.String("node_service_key", nodeServiceKey),
		zap.Int32("node_id", nodeID),
		zap.String("tool_name", toolName),
		zap.Error(callErr))
	return nil, fmt.Errorf("%s: %v", errorType, callErr)
}

// CallToolByNode 按 node_id 调用工具（对外使用 node_id 时的便捷入口）。
func (c *ConnectionPool) CallToolByNode(nodeID int32, toolName string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	return c.CallTool(buildNodeServiceKey(nodeID), toolName, args)
}

// getService 获取节点服务对象（只读路径使用 RLock）。
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

// GetServiceTools 获取单个节点服务的工具列表缓存。
func (c *ConnectionPool) GetServiceTools(nodeServiceKey string) []*mcp.Tool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	service, exists := c.services[nodeServiceKey]
	if exists {
		return service.Tools
	}
	return nil
}

// GetNodeTools 按 node_id 获取工具列表（内部转为 nodeServiceKey）。
func (c *ConnectionPool) GetNodeTools(nodeID int32) []*mcp.Tool {
	return c.GetServiceTools(buildNodeServiceKey(nodeID))
}

// RemoveNode 释放指定 node_id 对应的连接资源（显式释放入口）。
func (c *ConnectionPool) RemoveNode(nodeID int32) error {
	if nodeID <= 0 {
		return fmt.Errorf("invalid_node_id")
	}
	return c.removeService(buildNodeServiceKey(nodeID))
}

// GetNodePoolState 获取节点连接池状态（活跃连接数/目标连接数）。
func (c *ConnectionPool) GetNodePoolState(nodeID int32) *NodePoolState {
	nodeServiceKey := buildNodeServiceKey(nodeID)
	c.mutex.RLock()
	service, exists := c.services[nodeServiceKey]
	c.mutex.RUnlock()
	if !exists || service == nil || service.Instance == nil {
		return nil
	}
	inst := service.Instance
	inst.mutex.Lock()
	defer inst.mutex.Unlock()
	active := 0
	for _, conn := range inst.Connections {
		if conn != nil && conn.Session != nil {
			active++
		}
	}
	target := inst.TargetConnections
	if target <= 0 {
		target = 1
	}
	return &NodePoolState{
		NodeID:            nodeID,
		NodeServiceKey:    nodeServiceKey,
		ActiveConnections: active,
		TargetConnections: target,
	}
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
	closeServiceConnections(service)
	return nil
}

// Close 关闭所有连接并停止维护协程（进程退出时使用）。
func (c *ConnectionPool) Close() {
	// 停止维护协程
	c.StopMaintainer()
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 全量关闭时走 closeConnection，确保 Session + Transport 都释放。
	for _, service := range c.services {
		closeServiceConnections(service)
	}
	c.services = make(map[string]*ExternalService)
}

// closeConnection 关闭单个连接并释放其资源（Session + Transport）。
func closeConnection(conn *ExternalConnection, instanceID string) {
	if conn == nil {
		return
	}
	// 先关闭 Session
	if conn.Session != nil {
		if err := conn.Session.Close(); err != nil {
			logger.Error("关闭连接失败",
				zap.String("instance_id", instanceID),
				zap.Error(err))
		} else {
			logger.Info("关闭连接成功", zap.String("instance_id", instanceID))
		}
	}
	// 再关闭 Transport 的空闲连接，释放文件描述符
	if conn.Transport != nil {
		conn.Transport.CloseIdleConnections()
	}
}

// StartMaintainer 启动维护协程（周期性健康检查与补齐连接）。
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

// StopMaintainer 停止维护协程并等待退出，避免与 Close 并发冲突。
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

// maintainOnce 执行一次维护：检测连接、剔除无效、补齐缺口。
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

		// 2) 健康检查（在锁外做 IO），不健康的连接会被关闭并移除。
		healthy := make([]*ExternalConnection, 0, len(connsSnapshot))
		for _, conn := range connsSnapshot {
			if conn == nil || conn.Session == nil {
				continue
			}
			if !c.isSessionHealthy(conn.Session) {
				closeConnection(conn, inst.InstanceId)
				logger.Warn("关闭异常会话",
					zap.String("instance_id", inst.InstanceId),
					zap.String("node_service_key", nodeServiceKey),
					zap.Int32("node_id", nodeID))
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
			newConn, err := c.createReplacementConnection(svc)
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

// isSessionHealthy 使用轻量 Ping 检测会话健康（短超时）。
func (c *ConnectionPool) isSessionHealthy(session *mcp.ClientSession) bool {
	// 使用短超时的 Ping 作为心跳
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := session.Ping(ctx, nil)
	return err == nil
}

// createReplacementConnection 针对实例创建一条新的 HTTP 连接（用于补齐/重连）。
func (c *ConnectionPool) createReplacementConnection(svc *ExternalService) (*ExternalConnection, error) {
	ctx := context.Background()
	if svc == nil || svc.ConnectInfo == nil {
		return nil, fmt.Errorf("连接配置缺失")
	}
	return c.createHttpStreamableConnections(ctx, svc.ConnectInfo, svc.NodeID)
}

// getAvailableConnection 轮询选择一条可用连接。
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
	if len(instance.Connections) == 0 {
		return nil, nil, fmt.Errorf("没有可用的连接")
	}
	startIndex := instance.RoundRobinIndex
	for i := 0; i < len(instance.Connections); i++ {
		idx := (startIndex + i) % len(instance.Connections)
		conn := instance.Connections[idx]
		if conn == nil || conn.Session == nil {
			continue
		}
		instance.RoundRobinIndex = (idx + 1) % len(instance.Connections)
		return instance, conn, nil
	}
	return nil, nil, fmt.Errorf("没有可用的连接")
}

// buildNodeConfigFingerprint 生成配置指纹，用于判断是否需要重建连接池。
func buildNodeConfigFingerprint(url string, connectTimeout, callTimeout, maxConnect int) string {
	return fmt.Sprintf("url=%s|connect=%d|call=%d|max_connect=%d", strings.TrimSpace(url), connectTimeout, callTimeout, maxConnect)
}
