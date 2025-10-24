#!/bin/bash

# AgentEarth AgentPlatform 启动脚本

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 加载配置
source "$SCRIPT_DIR/config.sh"

echo "========================================"
echo "   AgentEarth AgentPlatform 启动脚本"
echo "========================================"
echo "使用环境: $ENV"
echo "项目根目录: $PROJECT_ROOT"
echo

# 切换到项目根目录
cd "$PROJECT_ROOT" || exit 1

# 检查可执行文件是否存在
if [ ! -f "$EXEC_FILE" ]; then
    echo "错误: 未找到可执行文件: $EXEC_FILE"
    echo "请先运行构建脚本: $SCRIPT_DIR/build.sh"
    exit 1
fi

# 检查服务是否已经运行
if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE")
    if ps -p "$OLD_PID" > /dev/null 2>&1; then
        echo "警告: 服务已经在运行中 (PID: $OLD_PID)"
        echo "如需重启服务，请使用: $SCRIPT_DIR/restart.sh"
        exit 1
    else
        echo "清理旧的 PID 文件..."
        rm -f "$PID_FILE"
    fi
fi

# 创建必要的目录
mkdir -p "$BIN_DIR"
mkdir -p "$LOG_DIR"

# 启动服务
echo "正在启动服务..."
echo "环境: $ENV"
echo "可执行文件: $EXEC_FILE"
echo "日志文件: $LOG_FILE"
echo "启动时间: $(date)"
echo "========================================"
echo

# 后台启动服务并保存PID
nohup "$EXEC_FILE" --env=$ENV >> "$LOG_FILE" 2>&1 &
PID=$!

# 等待一下确保进程启动
sleep 1

# 检查进程是否还在运行
if ps -p "$PID" > /dev/null 2>&1; then
    # 保存PID到文件
    echo $PID > "$PID_FILE"
    echo "✓ 服务启动成功"
    echo "  PID: $PID"
    echo "  PID文件: $PID_FILE"
    echo "  日志文件: $LOG_FILE"
    echo
    echo "使用以下命令管理服务:"
    echo "  停止服务: $SCRIPT_DIR/stop.sh"
    echo "  检查状态: $SCRIPT_DIR/status.sh"
    echo "  重启服务: $SCRIPT_DIR/restart.sh"
else
    echo "✗ 服务启动失败"
    echo "请查看日志文件: $LOG_FILE"
    exit 1
fi
