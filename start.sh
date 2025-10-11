#!/bin/bash

# AgentEarth AgentPlatform 启动脚本

echo "========================================"
echo "   AgentEarth AgentPlatform 启动脚本"
echo "========================================"

# 设置默认环境
ENV=${1:-dev}

echo "使用环境: $ENV"
echo

# 检查 Go 环境
echo "正在检查 Go 环境..."
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go 环境，请先安装 Go 1.24.5 或更高版本"
    exit 1
fi

echo "Go 环境检查通过"
echo

# 检查项目文件
if [ ! -f "go.mod" ]; then
    echo "错误: 未找到 go.mod 文件，请确保在项目根目录运行此脚本"
    exit 1
fi

# 下载依赖
echo "正在下载依赖..."
go mod tidy
if [ $? -ne 0 ]; then
    echo "错误: 依赖下载失败"
    exit 1
fi

echo "依赖检查完成"
echo

# 编译项目
echo "正在编译项目..."
mkdir -p bin
go build -o bin/agent-platform src/main.go
if [ $? -ne 0 ]; then
    echo "错误: 编译失败"
    exit 1
fi

echo "编译完成"
echo

# 创建日志目录
mkdir -p storage/logs

# 启动服务
echo "正在启动服务..."
echo "环境: $ENV"
echo "启动时间: $(date)"
echo "========================================"
echo

# 后台启动服务并保存PID
nohup ./bin/agent-platform --env=$ENV > storage/logs/service.log 2>&1 &
PID=$!

# 保存PID到文件
echo $PID > bin/agent-platform.pid
echo "服务已启动，PID: $PID"
echo "PID已保存到: bin/agent-platform.pid"
echo "日志文件: storage/logs/service.log"
echo
echo "使用以下命令管理服务:"
echo "  停止服务: ./stop.sh"
echo "  检查状态: ./status.sh"
echo "  重启服务: ./restart.sh"
