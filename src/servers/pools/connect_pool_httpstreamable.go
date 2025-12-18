package pools

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// 按httpStreamable创建实例
func (c *ConnectionPool) createInstancesForHttpStreamable(ctx context.Context, service *ExternalService) (newService *ExternalService, err error) {
	newService = service

	// 逻辑与 SSE 一致：以“账号数据是否存在”为准
	if len(service.Accounts) > 0 {
		// 需要鉴权 / 需要按账号实例化
		for i, account := range service.Accounts {
			if account.AuthInfo != nil && len(newService.InstanceMap) < int(service.MaxInstance) {
				// 创建连接配置副本，避免修改原始配置
				connectInfoCopy := *service.ConnectInfo
				// URL query 参数占位符替换：$(keyX)=$(VALUE) -> keyX=auth[keyX]
				connectInfoCopy.Url = replaceURLAuthPlaceholders(connectInfoCopy.Url, account.AuthInfo)

				// headers 合并逻辑先保持原样；若原始 headers 为空，则保持 nil
				if service.ConnectInfo.Headers != nil {
					connectInfoCopy.Headers = make(map[string]string)
				} else {
					connectInfoCopy.Headers = nil
				}

				// 合并原始headers和认证信息
				if service.ConnectInfo.Headers != nil {
					for k, v := range service.ConnectInfo.Headers {
						connectInfoCopy.Headers[k] = v
					}
					// 如果 headers 里配置了占位符，则按“模板替换”模式处理
					if headersContainAuthPlaceholders(connectInfoCopy.Headers) {
						connectInfoCopy.Headers = replaceHeaderAuthPlaceholders(connectInfoCopy.Headers, account.AuthInfo)
					} else {
						for k, v := range account.AuthInfo {
							connectInfoCopy.Headers[k] = v // 认证信息覆盖默认headers
						}
					}
				}

				//按当前账号信息创建实例
				instance := &ServiceInstance{
					AccountId:           account.AccountID,
					InstanceId:          fmt.Sprintf("instance_%d_%d_%d", service.Id, account.AccountID, i),
					Connections:         make([]*ExternalConnection, 0),
					ResolvedConnectInfo: &connectInfoCopy,
					TargetConnections:   1,
				}
				//创建连接
				connection, err1 := c.createHttpStreamableConnections(ctx, &connectInfoCopy, service.Id, account.AccountID)
				if err1 != nil {
					logger.Error("创建http实例连接失败", zap.String("ServiceName", service.ServiceName), zap.Error(err1))
					continue
				}
				instance.Connections = append(instance.Connections, connection)
				newService.InstanceMap[instance.InstanceId] = instance
			}
		}
	} else {
		// 无需鉴权
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
				logger.Error("创建http实例连接失败", zap.String("ServiceName", service.ServiceName), zap.Error(err1))
				continue
			}
			instance.Connections = append(instance.Connections, connection)
			newService.InstanceMap[instance.InstanceId] = instance
		}
	}
	return
}

// 创建httpStreamable连接
func (c *ConnectionPool) createHttpStreamableConnections(ctx context.Context, connectInfo *ConnectInfo, sid, aid int32) (connection *ExternalConnection, err error) {
	logger.Info("创建HTTP连接...", zap.String("Url", connectInfo.Url))
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
		logger.Error("创建HTTP连接失败", zap.String("Url", connectInfo.Url), zap.Error(err))
		return
	}
	connection = &ExternalConnection{
		ConnectionID: fmt.Sprintf("connection_%d_%d", sid, aid),
		Session:      session,
		LastPing:     time.Now(),
		ActiveUsers:  0,
	}
	logger.Info("创建HTTP连接成功", zap.String("ConnectionID", connection.ConnectionID))
	return
}
