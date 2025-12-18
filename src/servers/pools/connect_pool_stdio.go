package pools

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

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
				// 复制原始 env 模板（可能包含 $(KEY)/$(VALUE)）
				if service.LaunchInfo.Env != nil {
					launchInfoCopy.Env = make(map[string]interface{}, len(service.LaunchInfo.Env))
					for k, v := range service.LaunchInfo.Env {
						launchInfoCopy.Env[k] = v
					}
				} else {
					launchInfoCopy.Env = nil
				}

				// Env 替换逻辑与 headers 一致：
				// - Env 模板含占位符：按模板替换
				// - 否则：保持原有逻辑，把 AuthInfo 注入 Env
				if envContainAuthPlaceholders(launchInfoCopy.Env) {
					launchInfoCopy.Env = replaceEnvAuthPlaceholders(launchInfoCopy.Env, account.AuthInfo)
				} else {
					if launchInfoCopy.Env == nil {
						launchInfoCopy.Env = make(map[string]interface{})
					}
					for k, v := range account.AuthInfo {
						launchInfoCopy.Env[k] = v
					}
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
					logger.Error("创建stdio实例失败", zap.Any("ServiceName", service.ServiceName), zap.Int("index", i), zap.Error(err1))
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
				logger.Error("创建stdio实例失败", zap.Any("ServiceName", service.ServiceName), zap.Int("index", i), zap.Error(err1))
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
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", s, fmt.Sprint(k)))
		}
		logger.Debug("环境变量设置完成",
			zap.Int("total_env_vars", len(cmd.Env)),
			zap.Int("custom_env_vars", len(launchInfo.Env)),
			zap.Strings("custom_vars", func() []string {
				var customs []string
				for s, k := range launchInfo.Env {
					customs = append(customs, fmt.Sprintf("%s=%s", s, fmt.Sprint(k)))
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

	logger.Info("连接前...", zap.Any("command", cmd), zap.Duration("timeout", time.Duration(launchInfo.LaunchTimeout)*time.Millisecond))
	startTime := time.Now()
	session, err1 := client.Connect(connectCtx, &mcp.CommandTransport{Command: cmd}, nil)
	duration := time.Since(startTime)
	logger.Info("连接后...", zap.Any("session", session), zap.Duration("duration", duration))

	if err1 != nil {
		if errors.Is(connectCtx.Err(), context.DeadlineExceeded) {
			logger.Error("连接超时，已中断cmd命令执行",
				zap.Int32("aid", aid),
				zap.Any("command", cmd),
				zap.Duration("timeout", time.Duration(launchInfo.LaunchTimeout)*time.Millisecond),
				zap.Duration("elapsed", duration),
				zap.Error(err1))
		} else {
			logger.Error("创建stdio连接失败", zap.Int32("aid", aid), zap.Any("command", cmd), zap.Error(err1))
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
