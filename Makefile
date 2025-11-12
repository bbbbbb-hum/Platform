# Makefile for MyApp Go Project

# ========================
# Variables
# ========================
ENV_CONFIG_FILE := /opt/xlconfigs/Env/env.conf
BINARY_NAME := agent-platform-api
DIST_DIR := dist
BINARY_DIR := $(DIST_DIR)/bin
CONFIG_DIR := $(DIST_DIR)/config
BINARY_PATH := $(BINARY_DIR)/$(BINARY_NAME)

# ========================
# Load ENV from config (make-time)
# ========================
ifneq (,$(wildcard $(ENV_CONFIG_FILE)))
ENV := $(strip $(shell . "$(ENV_CONFIG_FILE)"; echo $$ENV))
MACHINE_NAME := $(strip $(shell . "$(ENV_CONFIG_FILE)"; echo $$MACHINE_NAME))
export ENV
export MACHINE_NAME
else
$(error 环境配置文件不存在: $(ENV_CONFIG_FILE))
endif

ifeq ($(ENV),)
$(error 未能从配置文件中读取 ENV 变量: $(ENV_CONFIG_FILE))
endif

ifneq ($(filter $(ENV),test prod),$(ENV))
$(error ENV 值不合法: $(ENV) 仅允许: test | prod)
endif

MAIN_PACKAGE := ./src
TEST_PACKAGE := ./...

COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html

GO_BUILD_FLAGS := -buildvcs=false
GOOS := linux
GOARCH := amd64

# ========================
# Phony targets
# ========================
.PHONY: all build cross-build test test-coverage coverage-html clean help

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
	@echo "Get Env"
	@echo "✓ 从配置文件读取环境信息:"
	@echo "  配置文件: $(ENV_CONFIG_FILE)"
	@echo "  ENV: $(ENV)"
	@if [ -n "$(MACHINE_NAME)" ]; then echo "  MACHINE_NAME: $(MACHINE_NAME)"; fi
	@echo
	@mkdir -p $(CONFIG_DIR)
	@cp "./config/.env.$(ENV)" "$(CONFIG_DIR)/.env.$(ENV)"
	@echo "Build completed: $(DIST_DIR)"

# ========================
# Cross-build for Linux/amd64
# ========================
cross-build:
	@echo "Cross-building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BINARY_DIR)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GO_BUILD_FLAGS) -o $(BINARY_PATH) $(MAIN_PACKAGE)
	@echo "Get Env"
	@echo "✓ 从配置文件读取环境信息:"
	@echo "  配置文件: $(ENV_CONFIG_FILE)"
	@echo "  ENV: $(ENV)"
	@if [ -n "$(MACHINE_NAME)" ]; then echo "  MACHINE_NAME: $(MACHINE_NAME)"; fi
	@echo
	@mkdir -p $(CONFIG_DIR)
	@cp "./config/.env.$(ENV)" "$(CONFIG_DIR)/.env.$(ENV)"
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
	@echo "  make clean        - Remove build artifacts and coverage files"
	@echo "  make help         - Show this help message"
