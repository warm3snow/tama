.PHONY: build test clean install lint run help

# Project variables
BINARY_NAME=tama
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d %H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X 'main.BuildTime=$(BUILD_TIME)'"
GO_FILES=$(shell find . -name "*.go" -type f -not -path "./vendor/*")

# Default target
all: build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o $(BINARY_NAME) .

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
	@go install $(LDFLAGS) .

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
	@go run $(LDFLAGS) .

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
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  clean          Clean build artifacts"
	@echo "  install        Install the application"
	@echo "  lint           Run linting"
	@echo "  run            Run the application"
	@echo "  fmt            Format code"
	@echo "  vendor         Vendor dependencies"
	@echo "  deps-update    Update dependencies"
	@echo "  help           Show this help information"
	@echo ""
	@echo "Version: $(VERSION)"

# Default is to show help
.DEFAULT_GOAL := help 