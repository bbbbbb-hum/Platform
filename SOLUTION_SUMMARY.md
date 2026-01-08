# 🎯 aim-mcp Docker Stdio 集成方案总结

## 问题描述

**需求**：在 ae-platform 服务中，通过 Docker 启动 aim-mcp 容器，并使用 stdio 方式进行交互。

**环境**：
- K3s/k3d 集群
- ae-platform 作为管理服务运行在 Pod 中
- aim-mcp 作为外部 MCP 服务通过镜像提供

## ✅ 解决方案

### 核心思路

通过**挂载 Docker Socket** 到 ae-platform 容器，使其能够在容器内执行 `docker run` 命令启动 aim-mcp 容器，并通过现有的 stdio 连接机制进行 MCP 协议通信。

### 架构图

```
┌────────────────────────────────────────────────┐
│  K3s Cluster                                   │
│  ┌──────────────────────────────────────────┐  │
│  │  Pod: ae-platform                        │  │
│  │  ┌────────────────────────────────────┐  │  │
│  │  │  Container: ae-platform            │  │  │
│  │  │                                     │  │  │
│  │  │  Go App (ae-platform-api)          │  │  │
│  │  │  ├─ ConnectionPool                 │  │  │
│  │  │  ├─ createStdioConnection()        │  │  │
│  │  │  └─ exec: docker run -i aim-mcp    │  │  │
│  │  │            │                        │  │  │
│  │  │            │ stdio (stdin/stdout)   │  │  │
│  │  │            ↓                        │  │  │
│  │  │     mcp.CommandTransport            │  │  │
│  │  └────────────┬───────────────────────┘  │  │
│  │               │                           │  │
│  │               │ /var/run/docker.sock      │  │
│  │               │ (hostPath volume)         │  │
│  └───────────────┼───────────────────────────┘  │
│                  │                              │
│                  ↓                              │
│  ┌───────────────────────────────────────────┐  │
│  │  Docker Daemon (Node)                     │  │
│  │  ┌─────────────────────────────────────┐  │  │
│  │  │  Container: aim-mcp-instance        │  │  │
│  │  │  (不受 K8s 管理)                     │  │  │
│  │  └─────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────┘  │
└────────────────────────────────────────────────┘
```

## 📦 已完成的修改

### 1. Kubernetes 配置

#### `k8s/deployment.yaml`
- ✅ 添加 Docker Socket 卷挂载
- ✅ 配置 hostPath 指向 `/var/run/docker.sock`

```yaml
volumeMounts:
  - name: docker-sock
    mountPath: /var/run/docker.sock

volumes:
  - name: docker-sock
    hostPath:
      path: /var/run/docker.sock
      type: Socket
```

### 2. Docker 镜像构建

#### `Dockerfile`（新建）
- ✅ 多阶段构建
- ✅ 安装 docker-cli
- ✅ 基于 Alpine Linux（轻量级）

### 3. 构建工具

#### `Makefile`
- ✅ 添加 `docker-build` 目标
- ✅ 添加 `docker-tag-latest` 目标
- ✅ 添加 `dockerimg` 目标（完整构建流程）

### 4. 文档和配置

#### 📄 新增文件

| 文件 | 说明 |
|------|------|
| `Dockerfile` | 包含 docker-cli 的镜像构建文件 |
| `docs/README.md` | AIM-MCP Docker Stdio 集成指南（完整配置文档） |
| `SOLUTION_SUMMARY.md` | 本文档 |

#### 📝 更新文件

| 文件 | 修改内容 |
|------|----------|
| `k8s/deployment.yaml` | 添加 Docker Socket 挂载 |
| `Makefile` | 添加 Docker 构建命令 |
| `k8s/README.md` | 添加 aim-mcp 相关说明 |

## 🚀 使用步骤

### 快速开始（5 分钟）

```bash
# 1. 构建镜像
make dockerimg

# 2. 导入镜像到 k3d
k3d image import ae-platform-api:latest -c ae-platform
k3d image import aim-mcp:latest -c ae-platform

# 3. 部署
kubectl apply -f k8s/

# 4. 配置数据库（参考 docs/README.md 中的配置说明）

# 5. 验证
kubectl exec -it deployment/ae-platform -- docker ps
```

### 详细步骤

请参考：[AIM-MCP Docker Stdio 集成指南](docs/README.md)

## 💡 工作原理

### 1. stdio 连接流程

```go
// 现有代码无需修改，已支持 docker run
func (c *ConnectionPool) createStdioConnection(
    ctx context.Context, 
    launchInfo *LaunchInfo, 
    sid, aid int32,
) (*ExternalConnection, error) {
    // 创建命令（docker run ...）
    cmd := exec.Command(launchInfo.Command, launchInfo.Args...)
    
    // 使用 MCP SDK 的 CommandTransport
    client := mcp.NewClient(...)
    session, err := client.Connect(ctx, &mcp.CommandTransport{
        Command: cmd,  // docker run -i aim-mcp:latest
    }, nil)
    
    // 返回连接
    return &ExternalConnection{Session: session}, nil
}
```

### 1.1 容器生命周期自动管理

**当 ae-platform pod 停止/重启时，aim-mcp 容器会自动停止：**

```
ae-platform pod 停止
    ↓
Go 进程收到 SIGTERM 信号
    ↓
ConnectionPool.Close() 被调用
    ↓
session.Close() 关闭 MCP 会话
    ↓
docker run -i 的 stdin 被关闭
    ↓
aim-mcp 容器检测到 stdin EOF
    ↓
容器自动退出
    ↓
--rm 标志自动删除容器 ✅
```

**代码验证**：

```go
// src/servers/pools/connect_pool.go:302
func (c *ConnectionPool) Close() {
    for _, service := range c.services {
        for _, instance := range service.InstanceMap {
            for _, conn := range instance.Connections {
                if conn.Session != nil {
                    conn.Session.Close()  // 关闭 stdio 连接
                }
            }
        }
    }
}
```

**必须使用的 Docker 参数**：
- ✅ `--rm`: 容器退出时自动删除
- ✅ `-i`: 交互模式，保持 stdin 打开

### 2. Docker 命令示例

数据库配置中的 `launch_info`：

```json
{
    "command": "docker",
    "args": [
        "run",
        "--rm",           // 退出时自动删除
        "-i",             // 交互模式（保持 stdin）
        "--name", "aim-mcp-instance",
        "aim-mcp:latest"
    ],
    "launch_timeout": 30000
}
```

### 3. stdio 通信

```
ae-platform (Go)          aim-mcp (Docker)
     │                          │
     │  stdin: MCP Request      │
     ├─────────────────────────>│
     │                          │ Process
     │  stdout: MCP Response    │
     │<─────────────────────────┤
     │                          │
```

## ⚠️ 注意事项

### 优点 ✅

1. **简单直接**：利用现有 stdio 实现，无需修改代码
2. **灵活配置**：通过数据库配置 Docker 参数
3. **资源隔离**：每个账号独立容器
4. **自动清理**：使用 `--rm` 自动删除容器

### 缺点 ⚠️

1. **不受 K8s 管理**：容器不在 K8s 调度范围内
2. **安全风险**：Docker Socket 权限很高
3. **监控困难**：需要额外监控 Docker 容器
4. **网络复杂**：容器网络配置需要注意

### 生产环境建议

1. 🔒 **安全加固**：
   - 限制容器权限
   - 使用 SecurityContext
   - 考虑 Docker-in-Docker 或 Kaniko

2. 📊 **监控告警**：
   - 监控容器数量
   - 资源使用情况
   - 日志收集

3. 🔧 **资源管理**：
   - 设置 `--memory`、`--cpus` 限制
   - 定期清理僵尸容器

## 🔍 验证和调试

### 验证 Docker Socket

```bash
# 检查挂载
kubectl describe pod -l app=ae-platform | grep docker-sock

# 测试 docker 命令
kubectl exec -it deployment/ae-platform -- docker ps
kubectl exec -it deployment/ae-platform -- docker images
```

### 查看 aim-mcp 容器

```bash
# 在宿主机或 pod 内
docker ps | grep aim-mcp
docker logs aim-mcp-instance
```

### 查看日志

```bash
# ae-platform 日志
kubectl logs -f deployment/ae-platform

# 查看 stdio 连接日志
kubectl logs deployment/ae-platform | grep "stdio"
```

## 📚 相关文档

- [K3d 部署指南](k8s/README.md)
- [AIM-MCP Docker Stdio 集成指南](docs/README.md)
- [MCP 协议文档](https://github.com/modelcontextprotocol)

## 🤝 替代方案

如果 Docker Socket 方案不适合你的场景，可以考虑：

### 方案 1：Kubernetes Job API
- 使用 K8s client-go 创建 Pod
- 通过 kubectl exec 进行交互
- 完全受 K8s 管理

### 方案 2：Sidecar 模式
- aim-mcp 作为 sidecar 容器
- 在同一个 Pod 内运行
- 通过 localhost 通信

### 方案 3：直接运行二进制
- 将 aim-mcp 编译成二进制
- 复制到容器内
- 不需要 Docker

各方案对比请参考 [AIM-MCP Docker Stdio 集成指南](docs/README.md)。

## ✨ 总结

通过**挂载 Docker Socket** 和配置 **stdio 类型的外部服务**，ae-platform 现在可以：

1. ✅ 在容器内执行 `docker run` 命令
2. ✅ 启动 aim-mcp 容器并建立 stdio 连接
3. ✅ 通过 MCP 协议与 aim-mcp 交互
4. ✅ 支持多账号、多实例管理
5. ✅ 无需修改现有代码逻辑

**现有的 `createStdioConnection` 已经支持这种方式**，只需通过数据库配置即可使用！

## 🎉 下一步

1. 按照快速启动指南部署测试
2. 根据实际需求调整 Docker 参数
3. 配置监控和告警
4. 考虑安全加固措施

---

**问题或建议？** 查看文档或提交 Issue。

