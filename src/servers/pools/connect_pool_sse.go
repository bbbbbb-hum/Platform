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

// 按SSE配置创建实例信息
func (c *ConnectionPool) createInstancesForSSE(ctx context.Context, service *ExternalService) (newService *ExternalService, err error) {
	newService = service
	// 是否需要按账号生成实例：以“账号数据是否存在”为准
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
					// 如果 headers 里配置了占位符，则按“模板替换”模式处理，
					// 避免 Authorization: Bearer $(VALUE) 被直接覆盖成裸 token
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
					TargetConnections:   service.ConnectInfo.MaxConnect,
				}
				//创建连接
				instance.Connections, err = c.createSSEConnections(ctx, &connectInfoCopy, service.Id, account.AccountID, service.ConnectInfo.MaxConnect)
				if err != nil {
					logger.Error("创建实例连接失败", zap.String("service_name", service.ServiceName), zap.Error(err))
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
				logger.Error("创建实例连接失败", zap.String("service_name", service.ServiceName), zap.Error(err))
				continue
			}
			newService.InstanceMap[instance.InstanceId] = instance
		}
	}

	return
}

// 创建SSE连接
func (c *ConnectionPool) createSSEConnections(ctx context.Context, connectInfo *ConnectInfo, sid, aid int32, connectsNum int) (connections []*ExternalConnection, err error) {
	logger.Info("创建SSE连接...", zap.String("connect_url", connectInfo.Url))

	// 创建HTTP客户端
	httpClient := &http.Client{
		Transport: &headerTransport{
			Transport: http.DefaultTransport,
			Headers:   connectInfo.Headers,
		},
	}
	// 创建MCP传输
	transport := &mcp.SSEClientTransport{
		Endpoint:   connectInfo.Url,
		HTTPClient: httpClient,
	}

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
			logger.Error("创建连接失败", zap.Int("index", i), zap.String("connect_url", connectInfo.Url), zap.Error(err1))
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
