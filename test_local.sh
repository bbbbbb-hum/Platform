#!/bin/bash
# 本地测试 aim-mcp 进程模式集成

set -e

echo "🚀 开始本地测试..."
echo ""

# 1. 检查 aim-mcp 镜像
echo "📦 检查 aim-mcp 镜像..."
if ! docker images | grep -q "aim-mcp.*latest"; then
    echo "❌ 错误：找不到 aim-mcp:latest 镜像"
    echo "请先确保 aim-mcp:latest 镜像存在"
    exit 1
fi
echo "✅ aim-mcp:latest 镜像存在"
echo ""

# 2. 构建 Go 程序
echo "🔨 构建 Go 程序..."
make cross-build
echo "✅ Go 程序构建完成"
echo ""

# 3. 构建 Docker 镜像
echo "🐳 构建 Docker 镜像..."
make dockerimg
echo "✅ Docker 镜像构建完成"
echo ""

# 4. 验证镜像内容
echo "🔍 验证镜像内容..."
echo "检查 aim-mcp 文件..."
docker run --rm ae-platform-api:latest ls -la /opt/aim-mcp/ || {
    echo "❌ 错误：aim-mcp 文件未找到"
    exit 1
}

echo "检查 Node.js..."
docker run --rm ae-platform-api:latest node --version || {
    echo "❌ 错误：Node.js 未安装"
    exit 1
}

echo "检查入口文件..."
docker run --rm ae-platform-api:latest test -f /opt/aim-mcp/dist/index.js && echo "✅ index.js 存在" || {
    echo "❌ 错误：index.js 未找到"
    exit 1
}
echo ""

# 5. 导入到 k3d
echo "📥 导入镜像到 k3d..."
k3d image import ae-platform-api:latest -c dev
echo "✅ 镜像导入完成"
echo ""

# 6. 重新部署
echo "🔄 重新部署..."
kubectl apply -f k8s/deployment.yaml
kubectl rollout restart deployment/ae-platform
echo "✅ 部署完成"
echo ""

# 7. 等待 Pod 就绪
echo "⏳ 等待 Pod 就绪..."
kubectl wait --for=condition=ready pod -l app=ae-platform --timeout=60s
echo "✅ Pod 已就绪"
echo ""

# 8. 显示状态
echo "📊 当前状态："
kubectl get pods -l app=ae-platform
echo ""

echo "📝 查看日志："
echo "kubectl logs -f deployment/ae-platform"
echo ""

echo "🎉 测试准备完成！"
echo ""
echo "📋 验证命令："
echo "  kubectl exec deployment/ae-platform -- node --version"
echo "  kubectl exec deployment/ae-platform -- ls -la /opt/aim-mcp/"
echo "  kubectl logs -f deployment/ae-platform"
