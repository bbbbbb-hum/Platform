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
echo "正在检查服务状态..."

# 1. 检查是否有同名进程在运行
RUNNING_PIDS=$(pgrep -f "$SERVICE_NAME" 2>/dev/null)
if [ -n "$RUNNING_PIDS" ]; then
    echo "警告: 发现同名服务进程正在运行"
    echo "运行中的进程 PID: $RUNNING_PIDS"
    echo "如需重启服务，请使用: $SCRIPT_DIR/restart.sh"
    exit 1
fi

# 2. 检查端口是否被占用（尝试从配置文件读取，失败则使用默认值）
DEFAULT_PORT="9001"
if [ -n "$ENV" ]; then
    CONFIG_FILE_CHECK="/opt/xlconfigs/AgentEarth-AgentPlatform/.env.$ENV"
else
    CONFIG_FILE_CHECK="config/.env"
fi

if [ -f "$CONFIG_FILE_CHECK" ]; then
    PORT=$(grep "^SERVER_PORT=" "$CONFIG_FILE_CHECK" 2>/dev/null | cut -d'=' -f2 | tr -d ' "'"'"'')
    [ -z "$PORT" ] && PORT="$DEFAULT_PORT"
else
    PORT="$DEFAULT_PORT"
fi

echo "检查端口 $PORT 是否可用..."

# 检查端口占用（优先使用 ss，其次 netstat，最后 lsof）
if command -v ss &> /dev/null; then
    PORT_IN_USE=$(ss -tlnp 2>/dev/null | grep ":$PORT " | grep -v "grep")
elif command -v netstat &> /dev/null; then
    PORT_IN_USE=$(netstat -tlnp 2>/dev/null | grep ":$PORT " | grep -v "grep")
elif command -v lsof &> /dev/null; then
    PORT_IN_USE=$(lsof -i ":$PORT" 2>/dev/null | grep LISTEN)
fi

if [ -n "$PORT_IN_USE" ]; then
    echo "错误: 端口 $PORT 已被占用"
    echo "占用详情:"
    echo "$PORT_IN_USE"
    echo
    echo "请先停止占用端口的进程，或修改配置使用其他端口"
    exit 1
fi

# 3. 清理旧的 PID 文件（如果存在）
if [ -f "$PID_FILE" ]; then
    echo "清理旧的 PID 文件..."
    rm -f "$PID_FILE"
fi

echo "✓ 服务状态检查通过"
echo

# 创建必要的目录
mkdir -p "$BIN_DIR"
mkdir -p "$LOG_DIR"

# 检查配置文件
echo "正在检查配置文件..."
if [ -n "$ENV" ]; then
    CONFIG_FILE="/opt/xlconfigs/AgentEarth-AgentPlatform/.env.$ENV"
else
    CONFIG_FILE="config/.env"
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo "错误: 配置文件不存在: $CONFIG_FILE"
    echo "请确保配置文件存在，或检查环境参数是否正确"
    exit 1
fi
echo "✓ 配置文件检查通过: $CONFIG_FILE"
echo

# 启动服务
echo "正在启动服务..."
echo "环境: $ENV"
echo "配置文件: $CONFIG_FILE"
echo "可执行文件: $EXEC_FILE"
echo "日志目录: $LOG_DIR"
echo "启动时间: $(date)"
echo "========================================"
echo

# 创建临时日志文件用于捕获启动错误
TEMP_LOG="/tmp/agent-platform-startup-$$.log"

# 后台启动服务并保存PID，错误输出到临时文件
nohup "$EXEC_FILE" --env=$ENV 2>"$TEMP_LOG" >/dev/null &
PID=$!

# 等待一下确保进程启动
sleep 2

# 检查进程是否还在运行
if ps -p "$PID" > /dev/null 2>&1; then
    # 保存PID到文件
    echo $PID > "$PID_FILE"
    echo "✓ 服务启动成功"
    echo "  PID: $PID"
    echo "  PID文件: $PID_FILE"
    echo "  配置文件: $CONFIG_FILE"
    echo "  日志目录: $LOG_DIR"
    echo
    # 检查启动日志是否有错误
    if [ -s "$TEMP_LOG" ]; then
        echo "⚠ 注意: 启动时有以下警告或错误信息:"
        cat "$TEMP_LOG"
        echo
    fi
    # 清理临时日志
    rm -f "$TEMP_LOG"
    echo "使用以下命令管理服务:"
    echo "  停止服务: $SCRIPT_DIR/stop.sh"
    echo "  检查状态: $SCRIPT_DIR/status.sh"
    echo "  重启服务: $SCRIPT_DIR/restart.sh"
else
    echo "✗ 服务启动失败"
    echo
    if [ -s "$TEMP_LOG" ]; then
        echo "错误信息:"
        cat "$TEMP_LOG"
    else
        echo "未捕获到错误信息，请检查:"
        echo "  1. 配置文件是否正确: $CONFIG_FILE"
        echo "  2. 可执行文件是否有问题: $EXEC_FILE"
        echo "  3. 端口是否被占用"
        echo "  4. 数据库连接是否正常"
    fi
    # 清理临时日志
    rm -f "$TEMP_LOG"
    exit 1
fi
