# Docker 环境变量支持 - 修改记录

**日期**: 2025-01-08  
**版本**: v1.1.0  
**类型**: 功能增强

## 🎯 修改目标

实现 Docker 容器启动时自动传递环境变量，支持多账号环境变量隔离。

## ✅ 已完成的修改

### 1. 代码修改

#### `src/servers/pools/connect_pool_stdio.go`

**修改的函数**:
- `createStdioConnection()` - 添加 Docker 命令检测和环境变量处理逻辑

**新增的函数**:
- `buildDockerArgsWithEnv()` - 将 env 映射转换为 Docker `-e` 参数

**核心改进**:
- ✅ 自动检测 `docker` 命令
- ✅ 将 `env` 字段转换为 `-e KEY=VALUE` 参数
- ✅ 在 `run` 后正确插入环境变量参数
- ✅ 保持非 Docker 命令的原有逻辑不变
- ✅ 向下兼容，无破坏性变更

### 2. 文档新增

| 文件 | 说明 |
|------|------|
| `docs/README.md` | 📚 AIM-MCP Docker Stdio 集成指南（完整配置文档） |
| `CHANGELOG_ENV_SUPPORT.md` | 📋 本文档 |

### 3. 文档更新

| 文件 | 修改内容 |
|------|----------|
| `README.md` | 添加新文档链接 |
| 各文档 | 更新文档链接指向统一的配置指南 |

## 📊 使用示例

### 配置前（手动写死环境变量）

```json
{
    "command": "docker",
    "args": [
        "run", "--rm", "-i",
        "-e", "API_KEY=hardcoded-key",
        "--name", "aim-mcp",
        "aim-mcp:latest"
    ]
}
```

❌ 问题：无法支持多账号

### 配置后（自动注入环境变量）

**服务配置** (`launch_info`):
```json
{
    "command": "docker",
    "args": ["run", "--rm", "-i", "--name", "aim-mcp", "aim-mcp:latest"],
    "env": {
        "$(QWEATHER_API_KEY)": "$(VALUE)",
        "$(QWEATHER_API_BASE)": "$(VALUE)"
    }
}
```

**账号配置** (`auth_info`):
```json
{
    "QWEATHER_API_KEY": "account1-key",
    "QWEATHER_API_BASE": "https://api.qweather.com"
}
```

**自动生成的命令**:
```bash
docker run --rm -i \
  -e QWEATHER_API_KEY=account1-key \
  -e QWEATHER_API_BASE=https://api.qweather.com \
  --name aim-mcp \
  aim-mcp:latest
```

✅ 优势：
- 支持多账号，每个账号独立环境变量
- 服务配置与账号信息分离
- 易于管理和维护

## 🔍 转换流程

```
1. 数据库 launch_info
   ↓
2. 加载到 LaunchInfo 结构体
   ↓
3. 如果有账号：replaceEnvAuthPlaceholders()
   → 将 $(KEY)/$(VALUE) 替换为实际值
   ↓
4. createStdioConnection()
   → 检测是否为 docker 命令
   ↓
5. buildDockerArgsWithEnv()
   → 将 env 转换为 -e 参数
   → 插入到 run 之后
   ↓
6. exec.Command() 执行
```

## 🎨 支持的场景

### ✅ 支持

1. **Docker 容器 + 多账号**
   ```json
   {"command": "docker", "env": {"$(KEY)": "$(VALUE)"}}
   ```

2. **Docker 容器 + 单账号**
   ```json
   {"command": "docker", "args": [..., "-e", "KEY=value", ...]}
   ```

3. **本地进程 + 环境变量**
   ```json
   {"command": "node", "env": {"KEY": "value"}}
   ```

4. **无环境变量**
   ```json
   {"command": "docker", "args": [...]}
   ```

### 🔧 特殊处理

- **Docker 命令**: env → `-e KEY=VALUE` 参数
- **非 Docker 命令**: env → 进程环境变量
- **无 env 字段**: 保持原样

## 📈 兼容性

| 场景 | 修改前 | 修改后 | 兼容性 |
|------|--------|--------|--------|
| Docker 无 env | ✅ 工作 | ✅ 工作 | ✅ 完全兼容 |
| Docker 有 env | ❌ 不生效 | ✅ 自动转换 | ✅ 功能增强 |
| 本地进程 无 env | ✅ 工作 | ✅ 工作 | ✅ 完全兼容 |
| 本地进程 有 env | ✅ 工作 | ✅ 工作 | ✅ 完全兼容 |

## 🧪 验证方法

### 1. 查看转换日志

```bash
kubectl logs -f deployment/ae-platform | grep "Docker命令环境变量转换完成"
```

**期望输出**:
```
Docker命令环境变量转换完成 {"env_count": 2, "args": ["run", "--rm", "-i", "-e", "KEY1=value1", "-e", "KEY2=value2", ...]}
```

### 2. 检查容器环境变量

```bash
# 查看容器
kubectl exec -it deployment/ae-platform -- docker ps

# 检查环境变量
kubectl exec -it deployment/ae-platform -- docker inspect <container> | grep -A 10 Env
```

## 📚 文档导航

- 📖 **配置指南**: [docs/README.md](docs/README.md) - AIM-MCP Docker Stdio 集成指南
- 📋 **完整方案**: [SOLUTION_SUMMARY.md](SOLUTION_SUMMARY.md) - 解决方案总结

## 🎉 总结

本次修改实现了：

1. ✅ **自动环境变量转换** - Docker 容器环境变量自动注入
2. ✅ **多账号支持** - 每个账号独立环境变量
3. ✅ **向下兼容** - 不影响现有功能
4. ✅ **无需修改表结构** - 使用现有 `launch_info` 字段
5. ✅ **灵活配置** - 支持占位符和固定值

## 🔄 下一步

如需使用新功能：

1. **重新构建镜像** (代码已修改):
   ```bash
   make dockerimg
   k3d image import ae-platform-api:latest -c ae-platform
   ```

2. **重启部署**:
   ```bash
   kubectl rollout restart deployment/ae-platform
   ```

3. **配置服务**:
   参考 [AIM-MCP Docker Stdio 集成指南](docs/README.md)

4. **验证功能**:
   查看日志确认环境变量转换成功
