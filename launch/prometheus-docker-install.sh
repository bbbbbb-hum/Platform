#!/bin/bash

# Prometheus Docker 安装脚本

# 解析环境参数，默认 test，可选值：test | prod
ENV=${1:-test}

echo "========================================"
echo "   Prometheus Docker 安装脚本"
echo "========================================"
echo "使用环境: $ENV"
echo

# 检查 Docker 是否安装
if ! command -v docker &> /dev/null; then
    echo "错误: Docker 未安装"
    echo "请先安装 Docker: https://docs.docker.com/engine/install/"
    exit 1
fi

echo "✓ Docker 已安装"
docker --version
echo

# 配置路径和容器名称（根据环境）
PROM_CONFIG_DIR="/opt/xlconfigs/Prometheus"
PROM_DATA_DIR="/opt/xldatas/Prometheus"
CONTAINER_NAME="prometheus-${ENV}"

echo "容器名称: $CONTAINER_NAME"
echo "配置目录: $PROM_CONFIG_DIR"
echo "数据目录: $PROM_DATA_DIR"
echo

# 创建目录
echo "创建 Prometheus 目录..."
sudo mkdir -p "$PROM_CONFIG_DIR"
sudo mkdir -p "$PROM_DATA_DIR"

# 检查配置文件
CONFIG_FILE="$PROM_CONFIG_DIR/prometheus-${ENV}.yml"
if [ ! -f "$CONFIG_FILE" ]; then
    echo "⚠ 配置文件不存在: $CONFIG_FILE"
    echo "尝试使用默认配置文件: $PROM_CONFIG_DIR/prometheus.yml"
    CONFIG_FILE="$PROM_CONFIG_DIR/prometheus.yml"
    if [ ! -f "$CONFIG_FILE" ]; then
        echo "错误: 配置文件不存在"
        echo "请先创建配置文件: $PROM_CONFIG_DIR/prometheus-${ENV}.yml"
        exit 1
    fi
fi

echo "✓ 配置文件: $CONFIG_FILE"
echo

# 检查是否已有运行的容器
if docker ps -a | grep -q "$CONTAINER_NAME"; then
    echo "发现已存在的 Prometheus 容器: $CONTAINER_NAME"
    read -p "是否删除并重新创建? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "停止并删除旧容器..."
        docker stop "$CONTAINER_NAME" 2>/dev/null
        docker rm "$CONTAINER_NAME" 2>/dev/null
        echo "✓ 旧容器已删除"
    else
        echo "取消安装"
        exit 0
    fi
fi

# 设置目录权限（Prometheus 容器使用 nobody 用户，UID=65534）
echo "设置目录权限..."
sudo chown -R 65534:65534 "$PROM_DATA_DIR"
sudo chmod -R 755 "$PROM_CONFIG_DIR"

# 启动 Prometheus 容器
echo
echo "启动 Prometheus 容器..."
echo "========================================"

docker run -d \
  --name "$CONTAINER_NAME" \
  --restart unless-stopped \
  -p 9090:9090 \
  -v "$CONFIG_FILE:/etc/prometheus/prometheus.yml:ro" \
  -v "$PROM_DATA_DIR:/prometheus" \
  -u 65534:65534 \
  prom/prometheus:latest \
  --config.file=/etc/prometheus/prometheus.yml \
  --storage.tsdb.path=/prometheus \
  --storage.tsdb.retention.time=30d \
  --web.console.libraries=/usr/share/prometheus/console_libraries \
  --web.console.templates=/usr/share/prometheus/consoles \
  --web.enable-lifecycle

if [ $? -eq 0 ]; then
    echo
    echo "等待 3 秒后检查容器状态..."
    sleep 3
    
    if docker ps | grep -q "$CONTAINER_NAME"; then
        echo
        echo "========================================"
        echo "✓ Prometheus 安装成功！"
        echo "========================================"
        echo
        echo "环境: $ENV"
        echo "容器名称: $CONTAINER_NAME"
        echo
        echo "访问地址:"
        echo "  Prometheus UI: http://$(hostname -I | awk '{print $1}'):9090"
        echo "  或: http://localhost:9090"
        echo
        echo "配置文件: $CONFIG_FILE"
        echo "数据目录: $PROM_DATA_DIR"
        echo
        echo "管理命令:"
        echo "  查看日志: docker logs -f $CONTAINER_NAME"
        echo "  停止服务: docker stop $CONTAINER_NAME"
        echo "  启动服务: docker start $CONTAINER_NAME"
        echo "  重启服务: docker restart $CONTAINER_NAME"
        echo "  删除容器: docker stop $CONTAINER_NAME && docker rm $CONTAINER_NAME"
        echo
        echo "热重载配置:"
        echo "  curl -X POST http://localhost:9090/-/reload"
        echo
        docker ps | grep "$CONTAINER_NAME"
    else
        echo "⚠ 容器启动异常，请查看日志"
        docker logs "$CONTAINER_NAME"
        exit 1
    fi
else
    echo
    echo "✗ Prometheus 启动失败"
    echo "请查看错误信息"
    exit 1
fi

echo
echo "========================================"

