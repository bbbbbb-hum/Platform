package pools

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// createInstancesForHttpStreamable 基于 httpStreamable 协议创建节点实例。
// 说明：
// - 目标连接数来自 ConnectInfo.MaxConnect（若未传则走默认值）。
// - 单实例多连接：一个节点仅有一个 ServiceInstance，但内部维护 N 条连接。
// - 这里不强制返回错误，交由上层 InitializeNode 统一决定失败与否。
func (c *ConnectionPool) createInstancesForHttpStreamable(ctx context.Context, service *ExternalService) (newService *ExternalService, err error) {
	newService = service
	targetConnections := 1
	if service != nil && service.ConnectInfo != nil {
		targetConnections = getNodeMaxConnect(service.ConnectInfo.MaxConnect)
	}
	// 多连接模型：每个节点仍是单实例，但实例内维护 N 条连接。
	instance := &ServiceInstance{
		InstanceId:        fmt.Sprintf("instance_node_%d", service.NodeID),
		Connections:       make([]*ExternalConnection, 0, targetConnections),
		TargetConnections: targetConnections,
	}
	for i := 0; i < targetConnections; i++ {
		connection, err1 := c.createHttpStreamableConnections(ctx, service.ConnectInfo, service.NodeID, i)
		if err1 != nil {
			// 保持历史行为：这里记录错误，但不抛出 err，交由上层统一判定失败。
			logger.Error("创建http实例连接失败",
				zap.String("ServiceName", service.ServiceName),
				zap.Int32("node_id", service.NodeID),
				zap.String("node_service_key", buildNodeServiceKey(service.NodeID)),
				zap.Int("connection_index", i),
				zap.Error(err1))
			continue
		}
		instance.Connections = append(instance.Connections, connection)
	}
	newService.Instance = instance
	// 返回带 Instance 的 service；后续由 InitializeNode 决定是否写入池。
	return
}

// createHttpStreamableConnections 创建单条 httpStreamable 连接。
// 说明：
// - 为每条连接生成独立 Transport，便于释放空闲连接与避免共享污染。
// - 使用节点统一超时，失败时带 error_type 与耗时便于排障。
func (c *ConnectionPool) createHttpStreamableConnections(ctx context.Context, connectInfo *ConnectInfo, nodeID int32, connectionIndex int) (connection *ExternalConnection, err error) {
	nodeServiceKey := buildNodeServiceKey(nodeID)
	logger.Info("创建HTTP连接...",
		zap.String("url", connectInfo.Url),
		zap.Int32("node_id", nodeID),
		zap.String("node_service_key", nodeServiceKey),
		zap.Int("connection_index", connectionIndex))

	// 确保 headers 可写，避免 nil map 写入 panic。
	if connectInfo.Headers == nil {
		connectInfo.Headers = make(map[string]string)
	}
	connectTimeout := time.Duration(getConnectTimeout(connectInfo)) * time.Millisecond
	// 配置 MCP Streamable HTTP 的常用请求头。
	// 说明：这是 MCP over HTTP 的流式能力请求头，不代表业务协议是 SSE。
	connectInfo.Headers["Accept"] = "text/event-stream, application/json"
	connectInfo.Headers["Connection"] = "keep-alive"
	connectInfo.Headers["Accept-Encoding"] = "gzip, deflate"
	// 创建自定义 Transport：优先 IPv4，IPv6 作为备用。
	// 这样可避免某些环境 IPv6 可达性不稳定导致初始化失败。
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout:   connectTimeout,
				KeepAlive: 30 * time.Second,
			}

			// 优先尝试 IPv4
			conn, err := dialer.DialContext(ctx, "tcp4", addr)
			if err == nil {
				logger.Debug("使用 IPv4 连接成功", zap.String("addr", addr))
				return conn, nil
			}

			// IPv4 失败，尝试 IPv6（如果环境支持）
			logger.Warn("IPv4 连接失败，尝试 IPv6", zap.String("addr", addr), zap.Error(err))
			conn, err = dialer.DialContext(ctx, "tcp6", addr)
			if err == nil {
				logger.Debug("使用 IPv6 连接成功", zap.String("addr", addr))
				return conn, nil
			}

			// 都失败，返回错误
			logger.Error("IPv4 和 IPv6 连接均失败", zap.String("addr", addr), zap.Error(err))
			return nil, err
		},
		// 保持 HTTP2 尝试，提升连接复用与长连接稳定性。
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 创建 HTTP 客户端（注入 headers）。
	httpClient := &http.Client{
		Transport: &headerTransport{
			Transport: transport,
			Headers:   connectInfo.Headers,
		},
	}
	logger.Info("客户端连接头:",
		zap.Int32("node_id", nodeID),
		zap.String("node_service_key", nodeServiceKey),
		zap.Int("connection_index", connectionIndex),
		zap.Any("headers", connectInfo.Headers))
	// 创建 MCP Client。该客户端与具体上游 endpoint 建立 streamable 会话。
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "time-client",
		Version: "1.0.0",
	}, nil)
	// 发起上游连接：超时由 connectCtx 控制；失败时记录 error_type 与耗时。
	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	start := time.Now()
	session, err := client.Connect(connectCtx, &mcp.StreamableClientTransport{
		Endpoint:   connectInfo.Url,
		HTTPClient: httpClient,
	}, nil)
	if err != nil {
		errorType := "upstream_error"
		if connectCtx.Err() == context.DeadlineExceeded {
			errorType = "connect_timeout"
		}
		logger.Error("创建HTTP连接失败",
			zap.String("stage", "initialize"),
			zap.String("error_type", errorType),
			zap.Int("timeout", int(connectTimeout/time.Millisecond)),
			zap.Int64("elapsed_ms", time.Since(start).Milliseconds()),
			zap.Int32("node_id", nodeID),
			zap.String("node_service_key", nodeServiceKey),
			zap.String("url", connectInfo.Url),
			zap.Int("connection_index", connectionIndex),
			zap.Error(err))
		if errorType == "connect_timeout" {
			return nil, fmt.Errorf("connect_timeout: timeout=%d err=%w", int(connectTimeout/time.Millisecond), err)
		}
		return
	}
	connection = &ExternalConnection{
		ConnectionID: fmt.Sprintf("connection_%d_%d", nodeID, connectionIndex),
		Session:      session,
		Transport:    transport, // 保存 Transport 引用，用于关闭时释放空闲连接
		LastPing:     time.Now(),
		ActiveUsers:  0,
	}
	// 连接对象由上层挂到 service.Instance 上，并纳入维护协程管理。
	logger.Info("创建HTTP连接成功",
		zap.String("connection_id", connection.ConnectionID),
		zap.Int32("node_id", nodeID),
		zap.String("node_service_key", nodeServiceKey),
		zap.Int("connection_index", connectionIndex))
	return
}
