# AgentEarth Agent Platform

AgentEarth Agent Platform 是一个基于 MCP (Model Context Protocol) 的智能代理管理平台，支持多种外部服务集成方式。

## 🚀 特性

- **多种连接方式**：支持 SSE、stdio、httpStreamable
- **连接池管理**：自动维护和恢复连接
- **多账号支持**：支持多账号并发和轮询
- **Docker Stdio 集成**：通过 Docker 容器启动外部服务
- **健康检查**：自动检测和恢复异常连接
- **资源限制**：支持实例数量和连接数限制

## 📦 项目结构

```
.
├── src/                    # 源代码
│   ├── servers/           # MCP 服务管理
│   │   └── pools/        # 连接池实现
│   ├── middleware/        # 中间件
│   └── ...
├── k8s/                   # Kubernetes 配置
├── docs/                  # 文档
│   └── README.md                    # AIM-MCP Docker Stdio 集成指南
├── scripts/               # 脚本（可选）
├── Dockerfile            # Docker 镜像构建
├── Makefile             # 构建工具
└── README.md            # 本文档
```

## 🛠️ 快速开始

### 本地开发

```bash
# 1. 安装依赖
go mod download

# 2. 配置环境变量
cp config/.env.example config/.env
# 编辑 config/.env 配置数据库等信息

# 3. 构建
make build

# 4. 运行
./dist/bin/agent-platform-api
```

### Docker 部署

```bash
# 1. 构建镜像
make dockerimg

# 2. 运行
docker run -p 9001:9001 ae-platform-api:latest
```

### Kubernetes 部署

详细部署指南请参考：[K3d 部署指南](k8s/README.md)

```bash
# 1. 创建 k3d 集群
k3d cluster create ae-platform \
  --port "30901:30901@server:0" \
  --port "9001:9001@loadbalancer"

# 2. 构建并导入镜像
make dockerimg
k3d image import ae-platform-api:latest -c ae-platform

# 3. 部署
kubectl apply -f k8s/

# 4. 验证
kubectl get pods -l app=ae-platform
kubectl logs -f deployment/ae-platform
```

## 🎯 aim-mcp Docker Stdio 集成

本项目支持通过 Docker 容器启动外部 MCP 服务（如 aim-mcp），并使用 stdio 方式进行交互。

### 快速集成

```bash
# 1. 导入 aim-mcp 镜像
k3d image import aim-mcp:latest -c ae-platform

# 2. 配置数据库（参考 docs/README.md）

# 3. 验证
kubectl exec -it deployment/ae-platform -- docker ps
```

### 详细文档

- 📚 [AIM-MCP Docker Stdio 集成指南](docs/README.md) - 完整配置文档
- 📋 [解决方案总结](SOLUTION_SUMMARY.md) - 架构和原理

## 📖 MCP 服务配置

### 支持的服务类型

| 类型 | 说明 | 使用场景 |
|------|------|----------|
| `sse` | Server-Sent Events | 远程 MCP 服务（HTTP 流） |
| `stdio` | 标准输入输出 | 本地进程或 Docker 容器 |
| `httpStreamable` | HTTP 流式传输 | 远程 HTTP MCP 服务 |

### 配置示例

#### stdio 类型（Docker 容器）

```sql
INSERT INTO ae_mcp_external_services (
    external_service_id,
    service_name,
    type,
    max_instance,
    launch_info
) VALUES (
    'my-mcp-service',
    'My MCP Service',
    'stdio',
    1,
    '{
        "command": "docker",
        "args": ["run", "--rm", "-i", "my-mcp:latest"],
        "launch_timeout": 30000
    }'
);
```

#### stdio 类型（本地进程）

```sql
INSERT INTO ae_mcp_external_services (
    external_service_id,
    service_name,
    type,
    max_instance,
    launch_info
) VALUES (
    'local-service',
    'Local Service',
    'stdio',
    1,
    '{
        "command": "/path/to/binary",
        "args": ["--arg1", "value1"],
        "launch_timeout": 30000
    }'
);
```

#### SSE 类型

```sql
INSERT INTO ae_mcp_external_services (
    external_service_id,
    service_name,
    type,
    max_instance,
    connect_info
) VALUES (
    'remote-sse-service',
    'Remote SSE Service',
    'sse',
    3,
    '{
        "url": "https://api.example.com/sse",
        "headers": {"Authorization": "Bearer token"},
        "connect_timeout": 10000,
        "max_connect": 5
    }'
);
```

## 🔧 配置说明

### 环境变量

主要配置项（通过 ConfigMap 或 `.env` 文件）：

```yaml
SERVER_PORT: 9001
DB_HOST: localhost
DB_PORT: 5432
DB_DATABASE: agent_earth
DB_USERNAME: user
DB_PASSWORD: password  # 生产环境使用 Secret
LOG_LEVEL: info
```

### 资源限制

在 `k8s/deployment.yaml` 中配置：

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

## 📊 API 端点

| 端点 | 说明 |
|------|------|
| `GET /metrics` | Prometheus 指标 |
| `POST /api/v1/mcp/call` | 调用 MCP 工具 |
| `GET /api/v1/mcp/services` | 获取服务列表 |
| `GET /api/v1/mcp/tools` | 获取工具列表 |

## 🐛 调试和故障排查

### 查看日志

```bash
# Kubernetes
kubectl logs -f deployment/ae-platform

# Docker
docker logs -f <container_id>

# 本地
tail -f /opt/xllogs/AEPlatformAPI/logs.log
```

### 验证 Docker Socket

```bash
# 检查挂载
kubectl describe pod -l app=ae-platform | grep docker-sock

# 测试 docker 命令
kubectl exec -it deployment/ae-platform -- docker ps
```

### 常见问题

详见：
- [K3d 部署常见问题](k8s/README.md#常见问题)
- [aim-mcp 故障排查](docs/README.md#验证和调试)

## 🔒 安全注意事项

### Docker Socket 安全

⚠️ **挂载 Docker Socket 有安全风险**：

1. 容器内可以执行任意 Docker 命令
2. 可能影响宿主机上的其他容器
3. 生产环境建议使用 Kubernetes Job API 或 Sidecar 模式

### 敏感信息管理

- 使用 Kubernetes Secret 管理密码和 API Key
- 不要将敏感信息提交到版本控制
- 配置文件使用占位符和环境变量

## 📈 监控和指标

### Prometheus 指标

```bash
# 查看指标
curl http://localhost:9001/metrics
```

主要指标：
- `mcp_request_total` - 请求总数
- `mcp_request_duration_seconds` - 请求耗时
- `mcp_connection_pool_size` - 连接池大小
- `mcp_active_connections` - 活跃连接数

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

[添加你的许可证信息]

## 📞 联系方式

- 项目维护者：[添加联系方式]
- 问题反馈：[添加 Issue 链接]

---

## 🔗 相关链接

- [MCP 协议官方文档](https://github.com/modelcontextprotocol)
- [K3d 官方文档](https://k3d.io/)
- [Kubernetes 官方文档](https://kubernetes.io/)
- [Go SDK for MCP](https://github.com/modelcontextprotocol/go-sdk)
