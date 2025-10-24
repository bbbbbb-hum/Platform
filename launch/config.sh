#!/bin/bash
# config.sh - 公共配置常量

# 解析环境参数，默认 test，可选值：test | prod
ENV=${1:-test}

# 服务名称
SERVICE_NAME="agent-platform"

# 项目根目录（脚本所在目录的上一级）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 源码目录
SOURCE_DIR="$PROJECT_ROOT/src"
MAIN_FILE="$SOURCE_DIR/main.go"

# 可执行文件目录和文件路径
BIN_DIR="$PROJECT_ROOT/bin"
EXEC_FILE="$BIN_DIR/$SERVICE_NAME"

# 日志目录和文件
LOG_DIR="$PROJECT_ROOT/storage/logs"
DATE=$(date +%Y-%m-%d)
LOG_FILE="$LOG_DIR/service-${DATE}.log"

# 服务进程ID记录文件
PID_FILE="$BIN_DIR/${SERVICE_NAME}.pid"

# Go 相关配置
GO_MOD_FILE="$PROJECT_ROOT/go.mod"
MIN_GO_VERSION="1.24.5"