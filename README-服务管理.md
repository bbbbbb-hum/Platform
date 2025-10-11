# AgentEarth AgentPlatform 服务管理

## 脚本说明

### 1. start.sh - 启动服务
```bash
# 使用默认环境（dev）启动
./start.sh

# 指定环境启动
./start.sh dev      # 开发环境
./start.sh test     # 测试环境
./start.sh prod     # 生产环境
```
**功能：**
- 检查 Go 环境和依赖
- 编译项目到 `bin/agent-platform`
- 后台启动服务
- 保存进程 PID 到 `bin/agent-platform.pid`
- 重定向日志到 `storage/logs/service.log`

### 2. stop.sh - 停止服务
```bash
# 停止服务
./stop.sh
```
**功能：**
- 从 PID 文件读取进程 ID
- 优雅停止服务（SIGTERM）
- 如果无法优雅停止，强制停止（SIGKILL）
- 清理 PID 文件

### 3. restart.sh - 重启服务
```bash
# 使用默认环境重启
./restart.sh

# 指定环境重启
./restart.sh dev     # 开发环境
./restart.sh test    # 测试环境
./restart.sh prod    # 生产环境
```
**功能：**
- 检查可执行文件是否存在
- 停止现有服务
- 直接启动 `bin/agent-platform`（不重新编译）
- 更新 PID 文件

### 4. status.sh - 检查服务状态
```bash
# 查看服务运行状态
./status.sh
```
**功能：**
- 检查 PID 文件是否存在
- 验证进程是否运行
- 显示进程详细信息
- 显示最近的服务日志

## 使用示例

### 首次启动
```bash
# 编译并启动服务
./start.sh dev

# 检查状态
./status.sh
```

### 日常管理
```bash
# 重启服务（不重新编译）
./restart.sh dev

# 停止服务
./stop.sh

# 检查状态
./status.sh
```

### 生产环境
```bash
# 启动生产服务
./start.sh prod

# 重启生产服务
./restart.sh prod
```

## PID 文件管理

- **PID 文件位置：** `bin/agent-platform.pid`
- **日志文件位置：** `storage/logs/service.log`
- **可执行文件：** `bin/agent-platform`

### PID 文件的作用
1. 记录服务进程的 PID
2. 用于停止服务时精确控制进程
3. 避免误杀其他同名进程
4. 支持服务状态检查

## 服务管理流程

### 启动流程
1. 检查 Go 环境和依赖
2. 编译项目
3. 后台启动服务
4. 保存 PID 到文件
5. 重定向日志输出

### 停止流程
1. 读取 PID 文件
2. 检查进程是否存在
3. 发送 SIGTERM 信号
4. 等待进程优雅退出
5. 如果超时，发送 SIGKILL 信号
6. 清理 PID 文件

### 重启流程
1. 检查可执行文件
2. 停止现有服务
3. 直接启动可执行文件
4. 更新 PID 文件

## 故障排除

### 服务无法启动
1. 检查 Go 环境：`go version`
2. 检查依赖：`go mod tidy`
3. 检查配置文件是否存在
4. 检查端口是否被占用

### 服务无法停止
1. 检查 PID 文件：`cat bin/agent-platform.pid`
2. 手动停止：`kill -9 $(cat bin/agent-platform.pid)`
3. 清理 PID 文件：`rm bin/agent-platform.pid`

### PID 文件问题
```bash
# 清理 PID 文件
rm bin/agent-platform.pid

# 重新启动服务
./start.sh
```

### 权限问题
```bash
# 给脚本添加执行权限
chmod +x *.sh

# 给可执行文件添加执行权限
chmod +x bin/agent-platform
```

## 日志管理

- **服务日志：** `storage/logs/service.log`
- **应用日志：** `storage/logs/logs.log`
- **按日期分割：** `storage/logs/YYYY-MM-DD.log`

### 查看日志
```bash
# 查看服务日志
tail -f storage/logs/service.log

# 查看应用日志
tail -f storage/logs/logs.log

# 查看最近日志
./status.sh
```

## 环境配置

确保在 `src/config/` 目录下有对应的环境配置文件：
- `.env_dev` - 开发环境
- `.env_test` - 测试环境  
- `.env_prod` - 生产环境
