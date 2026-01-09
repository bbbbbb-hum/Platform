# Makefile for MyApp Go Project

# ========================
# Variables
# ========================
BINARY_NAME := agent-platform-api
DIST_DIR := dist
BINARY_DIR := $(DIST_DIR)/bin
CONFIG_DIR := $(DIST_DIR)/config
BINARY_PATH := $(BINARY_DIR)/$(BINARY_NAME)


MAIN_PACKAGE := ./src
TEST_PACKAGE := ./...

COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

GO_BUILD_FLAGS := -buildvcs=false
GOOS := linux
GOARCH := amd64

# ========================
# Docker image configuration
# ========================
DOCKER_IMAGE_NAME := ae-platform-api
DOCKER_IMAGE_TAG := $(shell date '+%Y%m%d%H%M%S')
DOCKER_REP_PATH ?=
XLDOCKER_REP_PATH ?=

# 所有依赖的 MCP 镜像（18个）
MCP_IMAGES := \
	emcp-aim:v1.0 \
	emcp-tavily:v1.0 \
	emcp-skincare:v1.0 \
	emcp-shadcn-ui:v1.0 \
	emcp-serper:v1.0 \
	emcp-qweather:v1.0 \
	emcp-playwright:v1.0 \
	emcp-webresearch:v1.0 \
	emcp-webpage-timestamps:v1.0 \
	emcp-status-observer:v1.0 \
	emcp-learn:v1.0 \
	emcp-airbnb:v1.0 \
	emcp-crypto-price:v1.0 \
	emcp-math:v1.0 \
	emcp-littlesis:v1.0 \
	emcp-asset-price:v1.0 \
	emcp-vibe-check:v1.0 \
	emcp-web-scout:v1.0

# ========================
# Phony targets
# ========================
.PHONY: all build cross-build test test-coverage coverage-html clean help docker-build docker-tag-latest

# ========================
# Default target
# ========================
all: build

# ========================
# Build for local OS/ARCH
# ========================
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	@go build $(GO_BUILD_FLAGS) -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@echo
	@mkdir -p $(CONFIG_DIR)
	@if [ -f ./config/agent_plat_form.yml ]; then cp ./config/agent_plat_form.yml "$(CONFIG_DIR)/"; else echo "[WARN] ./config/agent_plat_form.yml not found, skip copy"; fi
	echo "Branch: ${CI_COMMIT_REF_NAME}, BuildNo: ${BUILD_NUMBER}, BuildTime: ${DATETIME}, CommitID: ${CI_COMMIT_ID}" > "./dist/v_${CI_COMMIT_REF_NAME}_${BUILD_NUMBER}_${DATETIME}_${CI_COMMIT_ID}.txt"
	@echo "Build completed: $(BINARY_PATH)"

# ========================
# Cross-build for Linux/amd64
# ========================
cross-build:
	@echo "Cross-building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BINARY_DIR)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GO_BUILD_FLAGS) -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@if [ -n "$(MACHINE_NAME)" ]; then echo "  MACHINE_NAME: $(MACHINE_NAME)"; fi
	@echo
	@mkdir -p $(CONFIG_DIR)
	@if [ -f ./config/agent_plat_form.yml ]; then cp ./config/agent_plat_form.yml "$(CONFIG_DIR)/"; else echo "[WARN] ./config/agent_plat_form.yml not found, skip copy"; fi
	@echo "Branch: ${CI_COMMIT_REF_NAME}, BuildNo: ${BUILD_NUMBER}, BuildTime: ${DATETIME}, CommitID: ${CI_COMMIT_ID}" > "./dist/v_${CI_COMMIT_REF_NAME}_${BUILD_NUMBER}_${DATETIME}_${CI_COMMIT_ID}.txt"
	@echo "Build completed: $(BINARY_PATH)"

# ========================
# Run tests
# ========================
test:
	@echo "Running tests..."
	@go test -v -short $(TEST_PACKAGE)

# ========================
# Run tests with coverage
# ========================
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -short -coverprofile=$(COVERAGE_FILE) $(TEST_PACKAGE)
	@go tool cover -func=$(COVERAGE_FILE)
	@echo "To view HTML coverage report, run: make coverage-html"

# ========================
# Generate HTML coverage report
# ========================
coverage-html: test-coverage
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "HTML coverage report generated: $(COVERAGE_HTML)"

# ========================
# Clean build artifacts
# ========================
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(DIST_DIR)
	@rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	@echo "Clean completed"

# ========================
# Build docker image
# ========================
dockerimg: build
	@echo "======================================="
	@echo "开始构建docker image..."
	@echo "======================================="
	@echo "检查所有依赖的 MCP 镜像 (18个)..."
	@MISSING_IMAGES=""; \
	for img in $(MCP_IMAGES); do \
		if ! docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^$$img$$"; then \
			echo "❌ 缺失: $$img"; \
			MISSING_IMAGES="$$MISSING_IMAGES$$img "; \
		else \
			echo "✅ 存在: $$img"; \
		fi; \
	done; \
	if [ -n "$$MISSING_IMAGES" ]; then \
		echo ""; \
		echo "=======================================";\
		echo "❌ 错误: 以下镜像不存在:"; \
		echo "$$MISSING_IMAGES" | tr ' ' '\n' | sed 's/^/  - /'; \
		echo ""; \
		echo "请先确保所有镜像存在于本地。"; \
		echo "可以使用以下方法："; \
		echo "  1. 从备份导入: docker load < mcp-images.tar"; \
		echo "  2. 从仓库拉取: docker pull <registry>/IMAGE_NAME"; \
		echo "  3. 逐个构建镜像: 参考各 MCP 项目的构建说明"; \
		echo "=======================================";\
		exit 1; \
	fi
	@echo "✅ 所有 MCP 镜像检查通过 (18/18)"
	@if [ ! -f "$(BINARY_PATH)" ]; then \
		echo "❌ 错误: 找不到二进制文件 $(BINARY_PATH)"; \
		echo "请先运行 make build 构建项目"; \
		exit 1; \
	fi
	@echo "✅ 二进制文件检查通过"
	@echo "开始构建 Docker 镜像..."
	@echo "使用 MCP 镜像仓库前缀: '$(XLDOCKER_REP_PATH)'"
	@docker build \
		--build-arg DOCKER_REP_PATH=$(DOCKER_REP_PATH) \
		--build-arg XLDOCKER_REP_PATH=$(XLDOCKER_REP_PATH) \
		-t $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
		-f Dockerfile $(DIST_DIR)
	@echo "[INFO] docker image构建完成: $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)"
	@echo "======================================="

# ========================
# Show help message
# ========================
help:
	@echo "MyApp Go Project - Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  make              - Build for local OS/ARCH (default)"
	@echo "  make build        - Build for local OS/ARCH"
	@echo "  make cross-build  - Build Linux amd64 executable"
	@echo "  make test         - Run tests"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make coverage-html - Generate HTML coverage report"
	@echo "  make docker-build - Build Docker image with timestamp tag"
	@echo "  make dockerimg    - Build and tag Docker image as latest"
	@echo "  make clean        - Remove build artifacts and coverage files"
	@echo "  make help         - Show this help message"
