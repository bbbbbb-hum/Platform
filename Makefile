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
dockerimg:
	@echo "======================================="
	@echo "开始构建docker image..."
	@echo "======================================="
	@docker build -t $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) -f Dockerfile .
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
