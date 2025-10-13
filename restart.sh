#!/bin/bash

# AgentEarth AgentPlatform 重启服务脚本

echo "========================================"
echo "   AgentEarth AgentPlatform 重启服务"
echo "========================================"

# 获取环境参数
ENV=${1:-dev}

echo "使用环境: $ENV"
echo

# 检查可执行文件是否存在
if [ ! -f "bin/agent-platform" ]; then
    echo "错误: 可执行文件不存在 bin/agent-platform"
    echo "请先运行 ./start.sh 进行编译"
    exit 1
fi

# 停止服务
echo "正在停止现有服务..."
./stop.sh

echo
echo "等待服务完全停止..."
sleep 2

# 直接启动可执行文件
echo "正在启动服务..."
echo "环境: $ENV"
echo "启动时间: $(date)"
echo "========================================"
echo

# 后台启动服务并保存PID
nohup ./bin/agent-platform --env=$ENV >> storage/logs/service-$(date +%F).log 2>&1 &
PID=$!

# 保存PID到文件
echo $PID > bin/agent-platform.pid
echo "服务已启动，PID: $PID"
echo "PID已保存到: bin/agent-platform.pid"
echo "日志文件: storage/logs/service-$(date +%F).log"
echo
echo "使用以下命令管理服务:"
echo "  停止服务: ./stop.sh"
echo "  检查状态: ./status.sh"
