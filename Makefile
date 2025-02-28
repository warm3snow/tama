# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

# Binary names
BINARY_NAME=tama
BINARY_UNIX=$(BINARY_NAME)_unix

# Build variables
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
BUILD_DATE=$(shell date -u '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X 'github.com/warm3snow/tama/cmd.Version=$(VERSION)' -X 'github.com/warm3snow/tama/cmd.BuildDate=$(BUILD_DATE)' -X 'github.com/warm3snow/tama/cmd.GitCommit=$(GIT_COMMIT)'"

.PHONY: all build clean install test build-linux help version

all: build

build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

install:
	$(GOINSTALL) $(LDFLAGS) ./...

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_UNIX) .

help:
	@echo "Make targets:"
	@echo "  build       - Build the tama binary"
	@echo "  install     - Install tama to GOPATH"
	@echo "  test        - Run tests"
	@echo "  clean       - Remove build artifacts"
	@echo "  build-linux - Cross-compile for Linux"
	@echo "  version     - Show version info"

version:
	@echo "Version: ${VERSION}"
	@echo "Build Date: ${BUILD_DATE}"
	@echo "Git Commit: ${GIT_COMMIT}"
