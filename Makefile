.PHONY: build test clean install lint run help deps version-test debug simple-build shell-build

# Project variables
BINARY_NAME=tama
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d %H:%M:%S')
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# 修改 LDFLAGS 格式，避免 macOS 链接错误
VERSION_FLAGS="-X 'github.com/warm3snow/tama/cmd.Version=$(VERSION)' -X 'github.com/warm3snow/tama/cmd.BuildTime=$(BUILD_TIME)' -X 'github.com/warm3snow/tama/cmd.Commit=$(COMMIT)'"
GO_FILES=$(shell find . -name "*.go" -type f -not -path "./vendor/*")

# Default target
all: build

# Build the application - 使用不同的方式传递 ldflags
build:
	@echo "Building $(BINARY_NAME) version $(VERSION) ($(COMMIT))..."
	@go build -ldflags $(VERSION_FLAGS) -o $(BINARY_NAME)

# 使用 shell 脚本构建
shell-build:
	@echo "Building using shell script..."
	@chmod +x build.sh
	@./build.sh

# 简单构建，不使用版本标志
simple-build:
	@echo "Building with simple command (no version info)..."
	@go build -o $(BINARY_NAME)

# Debug build issues
debug:
	@echo "Go version: $(shell go version)"
	@echo "Go env:"
	@go env
	@echo "Module info:"
	@go list -m all
	@echo "Building with verbose output..."
	go build -v -x -ldflags $(VERSION_FLAGS) -o $(BINARY_NAME)

# Test version command
version-test: build
	@echo "Testing version command..."
	@./$(BINARY_NAME) version

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@go clean

# Install the application
install:
	@echo "Installing $(BINARY_NAME)..."
	@go install -ldflags $(VERSION_FLAGS) .

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod tidy
	@go get github.com/chzyer/readline
	@go get github.com/spf13/cobra

# Run linting
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

# Run the application
run:
	@echo "Running $(BINARY_NAME)..."
	@go run -ldflags $(VERSION_FLAGS) .

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -s -w $(GO_FILES)

# Vendor dependencies
vendor:
	@echo "Vendoring dependencies..."
	@go mod vendor

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy

# Help information
help:
	@echo "Tama - Golang Copilot Agent"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the application"
	@echo "  shell-build    Build using shell script"
	@echo "  simple-build   Build without version flags"
	@echo "  debug          Debug build issues with verbose output"
	@echo "  version-test   Build and test the version command"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  clean          Clean build artifacts"
	@echo "  install        Install the application"
	@echo "  deps           Install dependencies"
	@echo "  lint           Run linting"
	@echo "  run            Run the application"
	@echo "  fmt            Format code"
	@echo "  vendor         Vendor dependencies"
	@echo "  deps-update    Update dependencies"
	@echo "  help           Show this help information"
	@echo ""
	@echo "Version: $(VERSION) ($(COMMIT))"

# Default is to show help
.DEFAULT_GOAL := help 