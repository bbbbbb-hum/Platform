#!/bin/bash

# Prometheus 状态检查脚本

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 解析环境参数，默认 test，可选值：test | prod
ENV=${1:-test}

CONTAINER_NAME="prometheus-${ENV}"

echo "========================================"
echo "   Prometheus 状态检查"
echo "========================================"
echo "使用环境: $ENV"
echo "容器名称: $CONTAINER_NAME"
echo

# 检查 Prometheus 是否运行
echo "1. 检查 Prometheus 服务..."
if docker ps | grep -q "$CONTAINER_NAME"; then
    echo "✓ Prometheus Docker 容器正在运行"
    docker ps | grep "$CONTAINER_NAME"
    PROM_TYPE="docker"
else
    echo "✗ Prometheus 未运行"
    echo
    echo "容器: $CONTAINER_NAME"
    echo "启动命令: docker start $CONTAINER_NAME"
    echo "或重新安装: $SCRIPT_DIR/prometheus-docker-install.sh"
    exit 1
fi

echo
echo "2. 检查 Prometheus API..."
if curl -s http://localhost:9090/-/healthy > /dev/null 2>&1; then
    echo "✓ Prometheus API 响应正常"
else
    echo "✗ Prometheus API 无响应"
    echo "  URL: http://localhost:9090"
    exit 1
fi

echo
echo "3. 检查配置是否加载..."
CONFIG_STATUS=$(curl -s http://localhost:9090/api/v1/status/config | grep -o '"status":"success"')
if [ -n "$CONFIG_STATUS" ]; then
    echo "✓ 配置文件加载成功"
else
    echo "⚠ 配置文件可能有问题"
fi

echo
echo "4. 检查 Targets 状态..."
TARGETS=$(curl -s http://localhost:9090/api/v1/targets)
echo "$TARGETS" | grep -o '"health":"[^"]*"' | sort | uniq -c

# 显示所有 target
echo
echo "活跃的 Targets:"
echo "----------------------------------------"
curl -s http://localhost:9090/api/v1/targets | \
  grep -o '"labels":{"[^}]*"}' | \
  sed 's/"labels":{"//;s/"}//;s/","/\n  /g' | \
  head -n 20
echo "----------------------------------------"

echo
echo "5. 测试查询接口..."
# 查询 up 指标
UP_QUERY=$(curl -s "http://localhost:9090/api/v1/query?query=up" | grep -o '"status":"success"')
if [ -n "$UP_QUERY" ]; then
    echo "✓ 查询接口正常"
    echo
    echo "当前在线的服务:"
    curl -s "http://localhost:9090/api/v1/query?query=up" | \
      grep -o '"instance":"[^"]*","job":"[^"]*","value":\[[^]]*,"1"' | \
      sed 's/"instance":"//;s/","job":"/  job=/;s/","value":\[[^,]*,"/  status=/;s/"$//' | \
      sed 's/1$/UP/' || echo "  无在线服务"
else
    echo "✗ 查询接口异常"
fi

echo
echo "6. 检查 AgentPlatform 服务指标..."
AGENT_METRICS=$(curl -s "http://localhost:9090/api/v1/query?query=http_requests_total" | grep -c "http_requests_total")
if [ "$AGENT_METRICS" -gt 0 ]; then
    echo "✓ 已收集到 AgentPlatform 指标"
    echo "  http_requests_total 数据点数: $AGENT_METRICS"
else
    echo "⚠ 未收集到 AgentPlatform 指标"
    echo
    echo "可能的原因:"
    echo "  1. AgentPlatform 服务未启动"
    echo "  2. AgentPlatform /metrics 端点无法访问"
    echo "  3. Prometheus 配置中的 target 地址错误"
    echo
    echo "排查步骤:"
    echo "  1. 检查 AgentPlatform 服务: $SCRIPT_DIR/status.sh"
    echo "  2. 测试 metrics 端点: curl http://localhost:9001/metrics"
    echo "  3. 检查 Prometheus 配置: cat /opt/xlconfigs/prometheus/prometheus-${ENV}.yml"
fi

echo
echo "7. 访问信息"
echo "========================================"
echo "环境: $ENV"
echo "容器: $CONTAINER_NAME"
echo
echo "Prometheus UI: http://$(hostname -I | awk '{print $1}'):9090"
echo "或: http://localhost:9090"
echo
echo "常用页面:"
echo "  - Targets: http://localhost:9090/targets"
echo "  - Graph: http://localhost:9090/graph"
echo "  - Status: http://localhost:9090/status"
echo
echo "管理命令:"
echo "  查看日志: docker logs -f $CONTAINER_NAME"
echo "  停止服务: docker stop $CONTAINER_NAME"
echo "  启动服务: docker start $CONTAINER_NAME"
echo "  重启服务: docker restart $CONTAINER_NAME"
echo
echo "========================================"

