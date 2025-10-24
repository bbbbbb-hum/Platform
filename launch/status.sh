#!/bin/bash

# AgentEarth AgentPlatform 服务状态检查脚本

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 加载配置
source "$SCRIPT_DIR/config.sh"

echo "========================================"
echo "   AgentEarth AgentPlatform 服务状态"
echo "========================================"

# 检查PID文件
if [ ! -f "$PID_FILE" ]; then
    echo "✗ 服务未运行 (PID文件不存在)"
    echo "  使用 $SCRIPT_DIR/start.sh 启动服务"
    echo "========================================"
    exit 0
fi

# 读取PID
PID=$(cat "$PID_FILE")
if [ -z "$PID" ]; then
    echo "✗ 服务未运行 (PID文件为空)"
    echo "  使用 $SCRIPT_DIR/start.sh 启动服务"
    echo "========================================"
    exit 0
fi

echo "从PID文件读取到进程ID: $PID"

# 检查进程是否存在
if ! kill -0 "$PID" 2>/dev/null; then
    echo "✗ 服务未运行 (进程 $PID 不存在)"
    echo "  清理PID文件..."
    rm -f "$PID_FILE"
    echo "  使用 $SCRIPT_DIR/start.sh 启动服务"
    echo "========================================"
    exit 0
fi

echo "✓ 服务正在运行"
echo "  进程 PID: $PID"

# 显示进程详细信息
echo "  进程信息:"
ps -p $PID -o pid,ppid,cmd,etime,pcpu,pmem 2>/dev/null || echo "  无法获取进程详细信息"

echo

# 检查日志文件
if [ -f "$LOG_FILE" ]; then
    echo "✓ 服务日志文件存在"
    echo "  日志文件: $LOG_FILE"
    echo "  文件大小: $(du -h "$LOG_FILE" | cut -f1)"
    echo "  最后修改时间: $(stat -c %y "$LOG_FILE" 2>/dev/null || stat -f %Sm "$LOG_FILE" 2>/dev/null)"
    
    # 显示最近的日志
    echo "  最近日志 (最后10行):"
    echo "  ----------------------------------------"
    tail -10 "$LOG_FILE" | sed 's/^/  /'
    echo "  ----------------------------------------"
else
    echo "✗ 服务日志文件不存在: $LOG_FILE"
fi

# 检查端口占用（如果知道端口号）
# 取消注释下面的代码，并替换为实际的端口号
# PORT=8080
# echo
# echo "检查端口 $PORT 状态..."
# if lsof -ti:$PORT > /dev/null; then
#     echo "✓ 端口 $PORT 正在监听"
# else
#     echo "✗ 端口 $PORT 未在监听"
# fi

echo
echo "========================================"
