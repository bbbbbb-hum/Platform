# Makefile for MyApp Go Project

# ========================
# Variables - 修改这里
# ========================
BINARY_NAME := agent-platform-api                     # 可执行文件名称
DIST_DIR := dist                         # 构建输出目录
BINARY_DIR := $(DIST_DIR)/bin
CONFIG_DIR := $(DIST_DIR)/config
BINARY_PATH := $(BINARY_DIR)/$(BINARY_NAME)

MAIN_PACKAGE := ./cmd/server             # main.go 所在路径
TEST_PACKAGE := ./...                    # 要测试的包路径

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
	@mkdir -p $(CONFIG_DIR)
	@echo "Build completed: $(DIST_DIR)"

# ========================
# Cross-build for Linux/amd64
# ========================
cross-build:
	@echo "Cross-building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BINARY_DIR)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GO_BUILD_FLAGS) -o $(BINARY_PATH) $(MAIN_PACKAGE)
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
