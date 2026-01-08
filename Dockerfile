## Runtime-only image (expects the binary to be provided in build context at ./bin/agent-platform-api)
## Notes:
## - tzdata is required because code loads "Asia/Shanghai" via time.LoadLocation()
## - Node.js 24.x + Python 3.11 are required for MCP services
## - 18 MCP services (17 Node.js + 1 Python) are extracted from local Docker images (must exist locally)
## - config file is required; by default code reads ./config/.env.local relative to WORKDIR
##   (recommended: mount /opt/xlconfigs/AEPlatformAPI/.env.<env> and run args: --env=<env>)

# ========================================
# 全局构建参数（必须在所有 FROM 之前声明）
# ========================================
ARG DOCKER_REP_PATH=""

# ========================================
# 阶段 1-18: 从本地 MCP 镜像提取文件
# ========================================
FROM aim-mcp:v1.0 AS aim-mcp-source
FROM tavily-mcp:v1.0 AS tavily-mcp-source
FROM skincare-mcp:v1.0 AS skincare-mcp-source
FROM shadcn-ui-mcp-server:v1.0 AS shadcn-ui-mcp-server-source
FROM serper-mcp-server:v1.0 AS serper-mcp-server-source
FROM qweather-mcp:v1.0 AS qweather-mcp-source
FROM playwright-mcp:v1.0 AS playwright-mcp-source
FROM mcp-webresearch:v1.0 AS mcp-webresearch-source
FROM mcp-webpage-timestamps:v1.0 AS mcp-webpage-timestamps-source
FROM mcp-status-observer:v1.0 AS mcp-status-observer-source
FROM mcp-server-learn:v1.0 AS mcp-server-learn-source
FROM mcp-server-airbnb:v1.0 AS mcp-server-airbnb-source
FROM mcp-crypto-price:v1.0 AS mcp-crypto-price-source
FROM math-mcp:v1.0 AS math-mcp-source
FROM littlesis-mcp:v1.0 AS littlesis-mcp-source
FROM asset-price-mcp:v1.0 AS asset-price-mcp-source
FROM vibe-check-mcp-server:v1.0 AS vibe-check-mcp-server-source
FROM web-scout-mcp:v1.0 AS web-scout-mcp-source

# ========================================
# 最终阶段: 构建 ae-platform 运行时镜像
# ========================================
FROM ${DOCKER_REP_PATH}ubuntu:22.04

# 安装 CA 证书 + 时区数据 + 基础工具
RUN sed -i 's|http://archive.ubuntu.com|http://mirrors.aliyun.com|g' /etc/apt/sources.list && \
    sed -i 's|http://security.ubuntu.com|http://mirrors.aliyun.com|g' /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        tzdata \
        curl \
        gnupg \
        software-properties-common && \
    update-ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# 安装 Node.js 24.x
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    rm -rf /var/lib/apt/lists/*

# 安装 Python 3.11
RUN add-apt-repository ppa:deadsnakes/ppa -y && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        python3.11 \
        python3.11-venv \
        python3.11-dev \
        python3-pip && \
    rm -rf /var/lib/apt/lists/* && \
    # 创建符号链接以兼容不同的Python路径
    ln -sf /usr/bin/python3.11 /usr/local/bin/python3.11

# 创建目录结构
RUN mkdir -p \
    /opt/xlapps/AEPlatformAPI/bin \
    /opt/xlapps/AEPlatformAPI/config \
    /opt/xllogs/AEPlatformAPI \
    /opt/xlconfigs/AEPlatformAPI \
    /opt/xltmp/AEPlatformAPI/tmp \
    /opt/xldata/AEPlatformAPI/data \
    /opt/mcp-services/aim-mcp \
    /opt/mcp-services/tavily-mcp \
    /opt/mcp-services/skincare-mcp \
    /opt/mcp-services/shadcn-ui-mcp-server \
    /opt/mcp-services/serper-mcp-server \
    /opt/mcp-services/qweather-mcp \
    /opt/mcp-services/playwright-mcp \
    /opt/mcp-services/mcp-webresearch \
    /opt/mcp-services/mcp-webpage-timestamps \
    /opt/mcp-services/mcp-status-observer \
    /opt/mcp-services/mcp-server-learn \
    /opt/mcp-services/mcp-server-airbnb \
    /opt/mcp-services/mcp-crypto-price \
    /opt/mcp-services/math-mcp \
    /opt/mcp-services/littlesis-mcp \
    /opt/mcp-services/asset-price-mcp \
    /opt/mcp-services/vibe-check-mcp-server \
    /opt/mcp-services/web-scout-mcp

# 创建非 root 用户
RUN useradd -u 10001 -m -s /usr/sbin/nologin appuser && \
    chown -R appuser:appuser \
        /opt/xlapps \
        /opt/xllogs \
        /opt/xlconfigs \
        /opt/xltmp \
        /opt/xldata \
        /opt/mcp-services

# 设置工作目录
WORKDIR /opt/xlapps/AEPlatformAPI

# 复制 ae-platform 可执行文件
COPY dist/bin/agent-platform-api /opt/xlapps/AEPlatformAPI/bin/agent-platform-api

# 复制所有 MCP 服务文件（从各自的镜像中提取）
COPY --from=aim-mcp-source /app /opt/mcp-services/aim-mcp
COPY --from=tavily-mcp-source /app /opt/mcp-services/tavily-mcp
COPY --from=skincare-mcp-source /app /opt/mcp-services/skincare-mcp
COPY --from=shadcn-ui-mcp-server-source /app /opt/mcp-services/shadcn-ui-mcp-server
COPY --from=serper-mcp-server-source /app /opt/mcp-services/serper-mcp-server
COPY --from=qweather-mcp-source /usr/src/app /opt/mcp-services/qweather-mcp
COPY --from=playwright-mcp-source /app /opt/mcp-services/playwright-mcp
COPY --from=mcp-webresearch-source /app /opt/mcp-services/mcp-webresearch
COPY --from=mcp-webpage-timestamps-source /app /opt/mcp-services/mcp-webpage-timestamps
COPY --from=mcp-status-observer-source /app /opt/mcp-services/mcp-status-observer
COPY --from=mcp-server-learn-source /app /opt/mcp-services/mcp-server-learn
COPY --from=mcp-server-airbnb-source /app /opt/mcp-services/mcp-server-airbnb
COPY --from=mcp-crypto-price-source /app /opt/mcp-services/mcp-crypto-price
COPY --from=math-mcp-source /app /opt/mcp-services/math-mcp
COPY --from=littlesis-mcp-source /app /opt/mcp-services/littlesis-mcp
COPY --from=asset-price-mcp-source /app /opt/mcp-services/asset-price-mcp
COPY --from=vibe-check-mcp-server-source /app /opt/mcp-services/vibe-check-mcp-server
COPY --from=web-scout-mcp-source /app /opt/mcp-services/web-scout-mcp

# 验证安装（无需安装 Python 包，直接使用源码）
RUN echo "=== 验证运行时环境 ===" && \
    node --version && \
    python3.11 --version && \
    echo "=== 验证 MCP 服务数量 ===" && \
    SERVICE_COUNT=$(ls -1 /opt/mcp-services/ | wc -l) && \
    echo "已安装服务数: $SERVICE_COUNT" && \
    test "$SERVICE_COUNT" = "18" && echo "✓ 服务数量正确 (18个)" && \
    echo "=== 验证关键服务文件 ===" && \
    test -f /opt/mcp-services/aim-mcp/dist/index.js && echo "✓ Node.js services OK" && \
    test -f /opt/mcp-services/serper-mcp-server/pyproject.toml && echo "✓ Python services OK"

USER appuser

EXPOSE 9001

# 默认启动（k3s 建议在 Deployment args 里传 --configfile=prod.yml）
CMD ["/opt/xlapps/AEPlatformAPI/bin/agent-platform-api"]
