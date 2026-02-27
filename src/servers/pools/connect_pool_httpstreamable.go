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

// 按httpStreamable创建实例
func (c *ConnectionPool) createInstancesForHttpStreamable(ctx context.Context, service *ExternalService) (newService *ExternalService, err error) {
	newService = service
	// 单连接模型：每个节点只创建一个实例和一个连接。
	instance := &ServiceInstance{
		InstanceId: fmt.Sprintf("instance_%d", service.Id),
	}
	connection, err1 := c.createHttpStreamableConnections(ctx, service.ConnectInfo, service.Id)
	if err1 != nil {
		logger.Error("创建http实例连接失败", zap.String("ServiceName", service.ServiceName), zap.Error(err1))
		return newService, nil
	}
	instance.Connection = connection
	newService.Instance = instance
	return
}

// 创建httpStreamable连接
func (c *ConnectionPool) createHttpStreamableConnections(ctx context.Context, connectInfo *ConnectInfo, nodeServiceID int32) (connection *ExternalConnection, err error) {
	logger.Info("创建HTTP连接...", zap.String("Url", connectInfo.Url))

	if connectInfo.Headers == nil { // 增加判空，兼容未初始化的场景
		connectInfo.Headers = make(map[string]string)
	}
	connectTimeoutMS := getConnectTimeoutMS(connectInfo)
	connectTimeout := time.Duration(connectTimeoutMS) * time.Millisecond
	// 配置核心流式请求头
	connectInfo.Headers["Accept"] = "text/event-stream, application/json"
	connectInfo.Headers["Connection"] = "keep-alive"
	connectInfo.Headers["Accept-Encoding"] = "gzip, deflate"
	// 创建自定义 Transport，优先使用 IPv4，IPv6 作为备用
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
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 创建HTTP客户端
	httpClient := &http.Client{
		Transport: &headerTransport{
			Transport: transport,
			Headers:   connectInfo.Headers,
		},
	}
	logger.Info("客户端连接头:", zap.Int32("node_service_id", nodeServiceID), zap.Any("headers", connectInfo.Headers))
	// 创建MCP传输
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "time-client",
		Version: "1.0.0",
	}, nil)
	// Connect to the server.
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
		nodeID := int32(0)
		nodeServiceKey := ""
		if nodeServiceID < 0 {
			nodeID = -nodeServiceID
			nodeServiceKey = buildNodeServiceKey(nodeID)
		}
		logger.Error("创建HTTP连接失败",
			zap.String("stage", "initialize"),
			zap.String("error_type", errorType),
			zap.Int("timeout_ms", connectTimeoutMS),
			zap.Int64("elapsed_ms", time.Since(start).Milliseconds()),
			zap.Int32("node_id", nodeID),
			zap.String("node_service_key", nodeServiceKey),
			zap.String("Url", connectInfo.Url),
			zap.Error(err))
		if errorType == "connect_timeout" {
			return nil, fmt.Errorf("connect_timeout: timeout_ms=%d err=%w", connectTimeoutMS, err)
		}
		return
	}
	connection = &ExternalConnection{
		ConnectionID: fmt.Sprintf("connection_%d", nodeServiceID),
		Session:      session,
		Transport:    transport, // 保存 Transport 引用，用于关闭时释放空闲连接
		LastPing:     time.Now(),
		ActiveUsers:  0,
	}
	logger.Info("创建HTTP连接成功", zap.String("ConnectionID", connection.ConnectionID))
	return
}
