#!/bin/bash

# AgentEarth AgentPlatform 重启服务脚本

source "./config.sh"
SCRIPT_DIR="$BIN_DIR"

echo "========================================"
echo "   AgentEarth AgentPlatform 重启服务"
echo "========================================"
echo "使用环境: $ENV"
echo





# 停止服务
echo "正在停止现有服务..."
"$SCRIPT_DIR/stop.sh"

echo
echo "等待服务完全停止..."
sleep 2

# 检查端口是否已释放
DEFAULT_PORT="9001"
if [ -n "$ENV" ]; then
    CONFIG_FILE_CHECK="${CONFIG_DIR}/.env.${ENV}"
else
    CONFIG_FILE_CHECK="${CONFIG_DIR}/.env"
fi

if [ -f "$CONFIG_FILE_CHECK" ]; then
    PORT=$(grep "^SERVER_PORT=" "$CONFIG_FILE_CHECK" 2>/dev/null | cut -d'=' -f2 | tr -d ' "'"'"'')
    [ -z "$PORT" ] && PORT="$DEFAULT_PORT"
else
    PORT="$DEFAULT_PORT"
fi

echo "检查端口 $PORT 是否已释放..."

# 检查端口占用
if command -v ss &> /dev/null; then
    PORT_IN_USE=$(ss -tlnp 2>/dev/null | grep ":$PORT " | grep -v "grep")
elif command -v netstat &> /dev/null; then
    PORT_IN_USE=$(netstat -tlnp 2>/dev/null | grep ":$PORT " | grep -v "grep")
elif command -v lsof &> /dev/null; then
    PORT_IN_USE=$(lsof -i ":$PORT" 2>/dev/null | grep LISTEN)
fi

if [ -n "$PORT_IN_USE" ]; then
    echo "警告: 端口 $PORT 仍被占用，服务可能未完全停止"
    echo "占用详情:"
    echo "$PORT_IN_USE"
    echo
    echo "等待额外 3 秒..."
    sleep 3
    
    # 再次检查
    if command -v ss &> /dev/null; then
        PORT_IN_USE=$(ss -tlnp 2>/dev/null | grep ":$PORT " | grep -v "grep")
    elif command -v netstat &> /dev/null; then
        PORT_IN_USE=$(netstat -tlnp 2>/dev/null | grep ":$PORT " | grep -v "grep")
    elif command -v lsof &> /dev/null; then
        PORT_IN_USE=$(lsof -i ":$PORT" 2>/dev/null | grep LISTEN)
    fi
    
    if [ -n "$PORT_IN_USE" ]; then
        echo "错误: 端口 $PORT 仍被占用，无法启动服务"
        echo "请手动停止占用端口的进程后重试"
        exit 1
    fi
fi

echo "✓ 端口 $PORT 检查通过"
echo

# 创建必要的目录
mkdir -p "$BIN_DIR"
mkdir -p "$LOG_DIR"

# 检查配置文件
echo "正在检查配置文件..."
if [ -n "$ENV" ]; then
    CONFIG_FILE="${CONFIG_DIR}/.env.${ENV}"
else
    CONFIG_FILE="${CONFIG_DIR}/.env"
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

# 后台启动服务并保存PID
nohup "$EXEC_FILE" --env=$ENV 2>>"${LOG_FILE}" 2>&1 &
PID=$!

# 保存PID到文件
echo $PID > "$PID_FILE"

echo "服务进程已启动 (PID: $PID)"
echo "正在等待服务端口监听..."

# 等待端口监听（无限等待，直到端口监听或进程退出）
WAIT_COUNT=0
PORT_LISTENING=false

while true; do
    # 检查进程是否还在运行
    if ! ps -p "$PID" > /dev/null 2>&1; then
        echo
        echo "✗ 服务启动失败（进程已退出）"
        echo
        if [ -s "$TEMP_LOG" ]; then
            echo "错误信息:"
            cat "$TEMP_LOG"
        else
            echo "未捕获到错误信息，请检查:"
            echo "  1. 配置文件是否正确: $CONFIG_FILE"
            echo "  2. 可执行文件是否有问题: $EXEC_FILE"
            echo "  3. 数据库连接是否正常"
        fi
        rm -f "$TEMP_LOG"
        rm -f "$PID_FILE"
        exit 1
    fi
    
    # 检查端口是否监听
    if command -v ss &> /dev/null; then
        PORT_CHECK=$(ss -tlnp 2>/dev/null | grep ":$PORT " | grep "$PID")
    elif command -v netstat &> /dev/null; then
        PORT_CHECK=$(netstat -tlnp 2>/dev/null | grep ":$PORT " | grep "$PID")
    elif command -v lsof &> /dev/null; then
        PORT_CHECK=$(lsof -i ":$PORT" -sTCP:LISTEN 2>/dev/null | grep "$PID")
    fi
    
    if [ -n "$PORT_CHECK" ]; then
        PORT_LISTENING=true
        break
    fi
    
    # 每3秒检查一次
    WAIT_COUNT=$((WAIT_COUNT + 3))
    echo "  等待中... (${WAIT_COUNT}s)"
    sleep 3
done

echo

if [ "$PORT_LISTENING" = true ]; then
    echo "========================================"
    echo "✓ 服务重启成功！"
    echo "========================================"
    echo "  PID: $PID"
    echo "  PID文件: $PID_FILE"
    echo "  配置文件: $CONFIG_FILE"
    echo "  监听端口: $PORT"
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
    echo "========================================"
fi
