#!/bin/bash

# AgentEarth AgentPlatform 构建脚本

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 加载配置
source "$SCRIPT_DIR/config.sh"

echo "========================================"
echo "   AgentEarth AgentPlatform 构建脚本"
echo "========================================"
echo "使用环境: $ENV"
echo "项目根目录: $PROJECT_ROOT"
echo

# 切换到项目根目录
cd "$PROJECT_ROOT" || exit 1

# 检查 Go 环境
echo "正在检查 Go 环境..."
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go 环境，请先安装 Go $MIN_GO_VERSION 或更高版本"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "Go 版本: $GO_VERSION"
echo "Go 环境检查通过"
echo

# 检查项目文件
if [ ! -f "$GO_MOD_FILE" ]; then
    echo "错误: 未找到 go.mod 文件: $GO_MOD_FILE"
    exit 1
fi

if [ ! -f "$MAIN_FILE" ]; then
    echo "错误: 未找到主程序文件: $MAIN_FILE"
    exit 1
fi

echo "项目文件检查通过"
echo

# 下载依赖
echo "正在下载依赖..."
go mod tidy
if [ $? -ne 0 ]; then
    echo "错误: 依赖下载失败"
    exit 1
fi

echo "依赖下载完成"
echo

# 创建构建目录
mkdir -p "$BIN_DIR"

# 编译项目
echo "正在编译项目..."
echo "源文件: $MAIN_FILE"
echo "输出文件: $EXEC_FILE"
go build -o "$EXEC_FILE" "$MAIN_FILE"
if [ $? -ne 0 ]; then
    echo "错误: 编译失败"
    exit 1
fi

echo "编译完成"
echo "========================================"
echo "可执行文件: $EXEC_FILE"
echo "========================================"

