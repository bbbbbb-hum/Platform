## Runtime-only image (expects the binary to be provided in build context at ./bin/agent-platform-api)
## Notes:
## - tzdata is required because code loads "Asia/Shanghai" via time.LoadLocation()
## - Node.js is required for aim-mcp (MCP service)
## - aim-mcp is extracted from aim-mcp:latest Docker image (must exist locally)
## - config file is required; by default code reads ./config/.env.local relative to WORKDIR
##   (recommended: mount /opt/xlconfigs/AEPlatformAPI/.env.<env> and run args: --env=<env>)

# ========================================
# 全局构建参数（必须在所有 FROM 之前声明）
# ========================================
ARG DOCKER_REP_PATH=""

# ========================================
# 阶段 1: 从本地 aim-mcp 镜像提取文件
# ========================================
FROM aim-mcp:latest AS aim-mcp-source

# ========================================
# 阶段 2: 构建 ae-platform 运行时镜像
# ========================================
FROM ${DOCKER_REP_PATH}ubuntu:22.04

# 安装 CA 证书 + 时区数据 + Node.js 24.x
RUN sed -i 's|http://archive.ubuntu.com|http://mirrors.aliyun.com|g' /etc/apt/sources.list && \
    sed -i 's|http://security.ubuntu.com|http://mirrors.aliyun.com|g' /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        tzdata \
        curl && \
    # 安装 Node.js 24.x（与 aim-mcp 版本一致）
    curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    # 清理
    update-ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# 目录约定（与你代码中硬编码路径对齐）
RUN mkdir -p \
    /opt/xlapps/AEPlatformAPI/bin \
    /opt/xlapps/AEPlatformAPI/config \
    /opt/xllogs/AEPlatformAPI \
    /opt/xlconfigs/AEPlatformAPI \
    /opt/xltmp/AEPlatformAPI/tmp \
    /opt/xldata/AEPlatformAPI/data \
    /opt/aim-mcp

# 创建非 root 用户运行（安全最佳实践）
RUN useradd -u 10001 -m -s /usr/sbin/nologin appuser && \
    chown -R appuser:appuser /opt/xlapps /opt/xllogs /opt/xlconfigs /opt/xltmp /opt/xldata /opt/aim-mcp

# 设置工作目录（保证 ./config/.env.local 这种相对路径可用）
WORKDIR /opt/xlapps/AEPlatformAPI

# 复制可执行文件（构建上下文是项目根目录）
COPY dist/bin/agent-platform-api /opt/xlapps/AEPlatformAPI/bin/agent-platform-api

# 从 aim-mcp 镜像复制应用文件
COPY --from=aim-mcp-source /app /opt/aim-mcp

# 验证 Node.js 和 aim-mcp 安装
RUN node --version && \
    ls -la /opt/aim-mcp/ && \
    test -f /opt/aim-mcp/dist/index.js

USER appuser

EXPOSE 9001

# 默认启动（k3s 建议在 Deployment args 里传 --configfile=prod.yml）
CMD ["/opt/xlapps/AEPlatformAPI/bin/agent-platform-api"]