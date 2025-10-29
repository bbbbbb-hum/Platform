#!/bin/bash
# config.sh - 公共配置常量

# 从配置文件读取环境参数
ENV_CONFIG_FILE="/opt/xlconfigs/Env/env.conf"

# 检查配置文件是否存在
if [ ! -f "$ENV_CONFIG_FILE" ]; then
    echo "错误: 环境配置文件不存在: $ENV_CONFIG_FILE"
    exit 1
fi

# 读取配置文件中的 ENV 和 MACHINE_NAME
source "$ENV_CONFIG_FILE"

# 检查 ENV 是否成功读取
if [ -z "$ENV" ]; then
    echo "错误: 未能从配置文件中读取 ENV 变量"
    echo "配置文件: $ENV_CONFIG_FILE"
    exit 1
fi

# 验证 ENV 值是否合法（仅允许 test 或 prod）
if [ "$ENV" != "test" ] && [ "$ENV" != "prod" ]; then
    echo "错误: ENV 值不合法: $ENV"
    echo "仅允许以下值: test | prod"
    exit 1
fi

# 输出读取到的环境信息（用于调试）
echo "✓ 从配置文件读取环境信息:"
echo "  配置文件: $ENV_CONFIG_FILE"
echo "  ENV: $ENV"
if [ -n "$MACHINE_NAME" ]; then
    echo "  MACHINE_NAME: $MACHINE_NAME"
fi
echo

# 服务名称
SERVICE_NAME="agent-platform-api"

# 项目根目录（脚本所在目录的上一级）
PROJECT_ROOT="/tmp/xltmp/AgentEarth-AgentPlatform"

# 源码目录
SOURCE_DIR="$PROJECT_ROOT/src"
MAIN_FILE="$SOURCE_DIR/main.go"

# 可执行文件目录和文件路径
BIN_DIR="/opt/xlapps/AgentEarth-AgentPlatform/bin"
EXEC_FILE="$BIN_DIR/$SERVICE_NAME"

# 日志目录和文件
LOG_DIR="/opt/xllogs/AgentEarth-AgentPlatform"
DATE=$(date +%Y-%m-%d)
LOG_FILE="$LOG_DIR/service-${DATE}.log"

# 服务进程ID记录文件
PID_FILE="$BIN_DIR/${SERVICE_NAME}.pid"

# Go 相关配置
GO_MOD_FILE="$PROJECT_ROOT/go.mod"
MIN_GO_VERSION="1.24.5"