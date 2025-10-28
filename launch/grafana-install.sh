#!/bin/bash

# Grafana Docker 安装脚本 - 测试环境

echo "========================================"
echo "   Grafana Docker 安装脚本"
echo "========================================"
echo

# 检查 Docker 是否安装
if ! command -v docker &> /dev/null; then
    echo "错误: Docker 未安装"
    exit 1
fi

echo "✓ Docker 已安装"
docker --version
echo

# 配置路径
GRAFANA_DATA_DIR="/opt/xldata/grafana"

# 创建目录
echo "创建 Grafana 数据目录..."
sudo mkdir -p "$GRAFANA_DATA_DIR"

# 设置权限（Grafana 容器使用 UID 472）
sudo chown -R 472:472 "$GRAFANA_DATA_DIR"

# 检查是否已有运行的容器
if docker ps -a | grep -q "grafana-test"; then
    echo "发现已存在的 Grafana 容器"
    read -p "是否删除并重新创建? (y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "停止并删除旧容器..."
        docker stop grafana-test 2>/dev/null
        docker rm grafana-test 2>/dev/null
        echo "✓ 旧容器已删除"
    else
        echo "取消安装"
        exit 0
    fi
fi

# 获取 Prometheus 容器 IP
echo "正在获取 Prometheus 容器 IP..."
PROM_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' prometheus-test 2>/dev/null)

if [ -z "$PROM_IP" ]; then
    echo "⚠ 未找到 Prometheus 容器，将使用 host.docker.internal"
    PROM_URL="http://host.docker.internal:9090"
else
    echo "✓ Prometheus IP: $PROM_IP"
    PROM_URL="http://${PROM_IP}:9090"
fi

# 启动 Grafana 容器
echo
echo "启动 Grafana 容器..."
echo "========================================"

docker run -d \
  --name grafana-test \
  --restart unless-stopped \
  -p 3000:3000 \
  -v "$GRAFANA_DATA_DIR:/var/lib/grafana" \
  -e "GF_SECURITY_ADMIN_PASSWORD=admin" \
  -e "GF_INSTALL_PLUGINS=grafana-piechart-panel" \
  grafana/grafana:latest

if [ $? -eq 0 ]; then
    echo
    echo "等待 Grafana 启动..."
    sleep 5
    
    # 检查 Grafana 是否启动成功
    if curl -s http://localhost:3000/api/health > /dev/null 2>&1; then
        echo
        echo "========================================"
        echo "✓ Grafana 安装成功！"
        echo "========================================"
        echo
        echo "访问信息:"
        echo "  Grafana UI: http://$(hostname -I | awk '{print $1}'):3000"
        echo "  或: http://localhost:3000"
        echo
        echo "登录信息:"
        echo "  默认用户名: admin"
        echo "  默认密码: admin"
        echo "  (首次登录会要求修改密码)"
        echo
        echo "数据目录: $GRAFANA_DATA_DIR"
        echo
        echo "Prometheus 数据源 URL:"
        echo "  $PROM_URL"
        echo
        echo "管理命令:"
        echo "  查看日志: docker logs -f grafana-test"
        echo "  停止服务: docker stop grafana-test"
        echo "  启动服务: docker start grafana-test"
        echo "  重启服务: docker restart grafana-test"
        echo
        echo "下一步:"
        echo "  1. 访问 Grafana UI"
        echo "  2. 使用 admin/admin 登录"
        echo "  3. 添加 Prometheus 数据源"
        echo "  4. 导入或创建 Dashboard"
        echo
        docker ps | grep grafana-test
    else
        echo "⚠ Grafana 启动中，请稍等片刻..."
        echo "  检查状态: docker logs grafana-test"
    fi
else
    echo
    echo "✗ Grafana 启动失败"
    echo "请查看错误信息"
    exit 1
fi

echo
echo "========================================"

